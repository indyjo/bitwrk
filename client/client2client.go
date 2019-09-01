//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2019  Jonas Eschenburg <jonas@bitwrk.net>
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/indyjo/bitwrk/client/assist"
	"github.com/indyjo/bitwrk/client/gziputil"
	"github.com/indyjo/bitwrk/client/receiveman"
	"github.com/indyjo/cafs/remotesync/httpsync"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/cafs"
	"github.com/indyjo/cafs/remotesync"
)

// Interface WorkHandler is where a WorkReceiver delegates its main lifecycle events to.
// It is implemented by SellActivity.
type WorkHandler interface {
	// Function HandleWork is called when the buyer has finished transmitting the work file.
	// Returns a ReadCloser for the result (which must be closed by the caller) and/or an error.
	HandleWork(log bitwrk.Logger, work cafs.File, buyerSecret bitwrk.Thash) (io.ReadCloser, error)

	// Function HandleReceipt is called when the buyer confirms receival of the result file.
	// The encoded result hash is the actual hash of the encoded result data in hexadecimal.
	// The signature is supposed to be a valid Bitcoin signature thereof.
	// The implementation is supposed to actually verify the signature, for which it needs
	// information about the buyer.
	HandleReceipt(log bitwrk.Logger, encResultHash, encResultHashSig string) error
}

// Interface WorkReceiver is how the SellActivity controls a work receiver. It has the same life cycle as
// the sell operation itself.
type WorkReceiver interface {
	// Must be called by user.
	Dispose()
	// Function IsDisposed() returns whether the work receiver was disposed, and if so, if there was an error.
	// A work receiver will auto-dispose on error.
	IsDisposed() (bool, error)
	// Function URL returns the URL the receiver listens on for requests by the buyer.
	URL() string
}

// A work receiver accepts incoming connections on a private URL and implements the seller side of a BitWrk
// transaction.
//
// `info` is forwarded to CAFS for marking files appropriately.
// `key` contains an AES-256 key for encrypting result data.
// `handler` delegate for performing actual work.
func NewWorkReceiver(log bitwrk.Logger,
	info string,
	receiveManager *receiveman.ReceiveManager,
	storage cafs.FileStorage,
	key bitwrk.Tkey,
	handler WorkHandler) WorkReceiver {
	ctx, cancel := context.WithCancel(context.Background())
	result := &endpointReceiver{
		ctx:            ctx,
		cancel:         cancel,
		endpoint:       receiveManager.NewEndpoint(info),
		storage:        storage,
		log:            log,
		handler:        handler,
		encResultKey:   key,
		info:           info,
		unspentTickets: make(map[string]bool),
	}
	result.endpoint.SetHandler(gziputil.WithCompression(result.serveHTTP))
	return result
}

// An object of this type is created for each sell and registered as an HTTP handler on a
// randomly-generated path. See receive.go for how that works.
// Function handleRequest(...) is called multiple times as client-client communication
// is performed in several requests.
// The endpointReceiver keeps the necessary state to correctly handle each step.
type endpointReceiver struct {
	// Must be acquired for every read or write within endpointReceiver.
	// Exception: reading from disposed* fields.
	mutex sync.Mutex
	// Must be acquired for reading and writing the disposed* fields
	disposedMutex    sync.Mutex
	disposed         bool
	disposedError    error
	ctx              context.Context // A Context representing the lifetime of the receiver
	cancel           context.CancelFunc
	endpoint         *receiveman.Endpoint
	storage          cafs.FileStorage
	log              bitwrk.Logger
	handler          WorkHandler
	encResultKey     bitwrk.Tkey
	builder          *remotesync.Builder
	buyerSecret      bitwrk.Thash
	workFile         cafs.File
	encResultFile    cafs.File
	info             string
	encResultHashSig string

	// Stores which assistive download tickets haven't been consumed.
	unspentTickets map[string]bool

	// Handler that enables other clients to perform assistive downloads.
	assistiveHandler *httpsync.FileHandler
}

func (receiver *endpointReceiver) URL() string {
	return receiver.endpoint.URL()
}

// Pattern recognizing the URL path format for assistive downloads.
// The group captures the ticket id, which is a hexadecimal number with an even count of digits.
var assistPattern = regexp.MustCompile(`.*/assist/((?:[a-f0-9][a-f0-9])+)`)

// Function serveHTTP handles all incoming HTTP requests. It's job is to decide whether to
// handle the incoming request using the assistive download handler, or using the
// handleRequest function.
func (receiver *endpointReceiver) serveHTTP(w http.ResponseWriter, r *http.Request) {
	assistMatches := assistPattern.FindStringSubmatch(r.URL.Path)
	if assistMatches != nil {
		// URL matches the pattern for assistive downloads.
		ticket := assistMatches[1]

		// Verify the ticket and consume it in case of a POST request.
		receiver.mutex.Lock()
		b := receiver.unspentTickets[ticket]
		if b && r.Method == http.MethodPost {
			receiver.unspentTickets[ticket] = false
		}
		receiver.mutex.Unlock()
		if b {
			receiver.assistiveHandler.ServeHTTP(w, r)
		} else {
			log.Printf("assistive download ticket was not found: ", ticket)
			http.NotFound(w, r)
		}
	} else if err := receiver.handleRequest(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error in request: %v", err)
		receiver.doDispose(fmt.Errorf("error handling request from %v: %v", r.RemoteAddr, err))
	}
}

// Function doDispose is an internal function that disposes all held resources
// and sets the error returned by IsDisposed to `err`. This function does
// nothing if the receiver has already been disposed.
func (receiver *endpointReceiver) doDispose(err error) {
	// First critical section: mark receiver as disposed.
	receiver.disposedMutex.Lock()
	// If already disposed, do nothing
	if receiver.disposed {
		receiver.disposedMutex.Unlock()
		return
	}
	receiver.disposed = true
	receiver.disposedError = err
	receiver.disposedMutex.Unlock()

	// Cancel the receiver's Context and all active assistive work transmissions
	receiver.cancel()

	// Second critical section: memorize objects we want to clean up, reset in receiver.
	receiver.mutex.Lock()
	endpoint := receiver.endpoint
	builder := receiver.builder
	workFile := receiver.workFile
	encResultFile := receiver.encResultFile
	assistHandler := receiver.assistiveHandler
	receiver.endpoint = nil
	receiver.builder = nil
	receiver.workFile = nil
	receiver.encResultFile = nil
	receiver.assistiveHandler = nil
	receiver.mutex.Unlock()

	// Actual cleanup can be performed asynchronously
	endpoint.Dispose()
	if builder != nil {
		builder.Dispose()
	}
	if workFile != nil {
		workFile.Dispose()
	}
	if encResultFile != nil {
		encResultFile.Dispose()
	}
	if assistHandler != nil {
		assistHandler.Dispose()
	}
}

func (receiver *endpointReceiver) Dispose() {
	receiver.doDispose(nil)
}

func (receiver *endpointReceiver) IsDisposed() (bool, error) {
	receiver.disposedMutex.Lock()
	defer receiver.disposedMutex.Unlock()
	return receiver.disposed, receiver.disposedError
}

var ZEROHASH bitwrk.Thash

type todoList struct {
	mustWriteWishList bool
	mustHandleWork    bool
	mustHandleReceipt bool
}

// This function handles all (http) requests from buyer to seller.
// - an OPTIONS request for querying protocol options
// - A POST where the buyer sends a list of block hashes and the seller returns which are missing
// - A POST where the buyer sends work data and a "buyer secret" and the seller publishes it, does the work and
//   returns the finished result, encrypted with one-time key.
//   This comes in two variants: A simple one that contains complete work data and one that contains the chunks
//   requested using the previous request.
// - A POST where the buyer acknowledges the reception of the encrypted result using a signature and the seller
//   publishes this information and returns the decryption key
func (receiver *endpointReceiver) handleRequest(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "OPTIONS" {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"Adler32Chunking": true, "GZIPCompression": true, "SyncInfo": true}`))
		return err
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil
	}

	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	receiver.log.Printf("Handling request from %v on %v", r.RemoteAddr, r.URL)
	defer receiver.log.Printf("Done handling request from %v on %v", r.RemoteAddr, r.URL)

	var todo *todoList
	if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err != nil {
			return fmt.Errorf("failed to parse form data: %v", err)
		}

		t, err := receiver.handleUrlEncodedMessage(r.Form)
		if err != nil {
			return fmt.Errorf("error handling url-encoded message: %v", err)
		}
		todo = t
	} else if mreader, err := r.MultipartReader(); err == nil {
		todo, err = receiver.handleMultipartMessage(mreader)
		if err != nil {
			return fmt.Errorf("error handling multipart message: %v", err)
		}
	} else {
		return fmt.Errorf("don't know how to handle message (Content-Type: %v)", r.Header.Get("Content-Type"))
	}

	if (receiver.workFile == nil && receiver.builder == nil) || receiver.buyerSecret == ZEROHASH {
		return fmt.Errorf("incomplete work message (work file: %v, chunk hashes: %v. buyer secret: %v)", receiver.workFile != nil, receiver.builder != nil, receiver.buyerSecret != ZEROHASH)
	}

	if todo.mustHandleReceipt {
		return receiver.handleReceipt()
	} else if todo.mustWriteWishList {
		w.Header().Set("Content-Type", "application/x-wishlist")
		receiver.sendCreatedAssistiveDownloadURLs(w)
		// Leave lock temporarily to enable streaming
		receiver.mutex.Unlock()
		defer receiver.mutex.Lock()
		return receiver.builder.WriteWishList(w.(remotesync.FlushWriter))
	} else if todo.mustHandleWork {
		if r, err := receiver.handleWorkAndReturnEncryptedResult(); err != nil {
			return err
		} else if receiver.encResultFile != nil {
			return fmt.Errorf("encrypted result file exists already")
		} else {
			receiver.log.Printf("Encrypted result file: %v", r)
			receiver.encResultFile = r
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		reader := receiver.encResultFile.Open()
		if _, err := io.Copy(w, reader); err != nil {
			receiver.close(reader, "encResultFile")
			return fmt.Errorf("error sending work result back to buyer: %v", err)
		}
		return reader.Close()
	} else {
		panic("Shouldn't get here")
	}
}

// Function sendCreatedAssistiveDownloadURLs communicates tickets back to the buyer by
// putting them into a special HTTP response header.
func (receiver *endpointReceiver) sendCreatedAssistiveDownloadURLs(w http.ResponseWriter) {
	urls := make([]string, 0, 2)
	for ticket, unspent := range receiver.unspentTickets {
		if !unspent {
			continue
		}
		urls = append(urls, receiver.URL()+"/assist/"+ticket)
	}
	if len(urls) == 0 {
		return
	}
	if js, err := json.Marshal(urls); err != nil {
		panic(err)
	} else {
		w.Header().Set(assist.HeaderName, string(js))
	}
}

// Decodes MIME multipart/form-data messages and returns a todoList or an error.
func (receiver *endpointReceiver) handleMultipartMessage(mreader *multipart.Reader) (*todoList, error) {
	todo := &todoList{}
	// iterate through parts of multipart/form-data content
	for {
		part, err := mreader.NextPart()
		if err == io.EOF {
			receiver.log.Printf("End of stream reached")
			break
		} else if err != nil {
			return nil, fmt.Errorf("error reading part: %v", err)
		}
		formName := part.FormName()
		receiver.log.Printf("Handling part: %v", formName)
		switch formName {
		case "buyersecret":
			// Buyer sends a random value to seller to prevent the seller from hijacking other seller's workers.
			if receiver.buyerSecret != ZEROHASH {
				return nil, fmt.Errorf("buyer's secret already received")
			}
			b := make([]byte, 64)
			if n, err := io.ReadFull(part, b); err != nil {
				return nil, fmt.Errorf("error reading buyersecret: %v (%v bytes read)", err, n)
			}
			if n, err := hex.Decode(receiver.buyerSecret[:], b); err != nil || n != len(receiver.buyerSecret) {
				return nil, fmt.Errorf("error decoding buyersecret: %v (%v bytes written)", err, n)
			}
		case "work":
			// Direct work data transmission without chunking. Up to 16MB.
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("work already received")
			}
			temp := receiver.storage.Create(receiver.info)
			// Copy up to 16MB and expect EOF
			if n, err := io.CopyN(temp, part, 1<<24); err != io.EOF {
				temp.Dispose()
				return nil, fmt.Errorf("work too long or error: %v (%v bytes read)", err, n)
			}
			if err := temp.Close(); err != nil {
				temp.Dispose()
				return nil, fmt.Errorf("error creating file from temporary data: %v", err)
			}
			receiver.workFile = temp.File()
			temp.Dispose()
			todo.mustHandleWork = true
		case "assisturl":
			// We received a ticket for an assistive download endpoint
			buf := new(bytes.Buffer)
			if _, err := io.CopyN(buf, part, 1024); err != io.EOF {
				return nil, fmt.Errorf("error reading assistive download ticket: %v", err)
			} else if url, err := verifyAssistTicket(buf.String()); err != nil {
				return nil, fmt.Errorf("error verifying assistive download ticket: %v", err)
			} else {
				receiver.log.Printf("Starting assistive download from: %v", url)
				go func() {
					f, err := httpsync.SyncFrom(receiver.ctx, receiver.storage, http.DefaultClient, url.String(), url.String())
					receiver.log.Printf("Assistive download returned: %v", err)
					if f != nil {
						f.Dispose()
					}
				}()
			}
		case "syncinfojson":
			// Transmission of information about work data.
			// Information such as chunk hashes, chunk lengths, transmission order
			// permutation.
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("work already received on 'syncinfojson'")
			}
			todo.mustWriteWishList = true
			var syncinfo remotesync.SyncInfo
			// guard against DOS, sync info may not be longer than a rough estimate
			// based on max chunks allowed
			r := io.LimitReader(part, 100*MaxNumberOfChunksInWorkFile+1<<16)
			if err := json.NewDecoder(r).Decode(&syncinfo); err != nil {
				return nil, fmt.Errorf("error decoding sync info: %v", err)
			}
			receiver.builder = remotesync.NewBuilder(receiver.storage, &syncinfo, 32, receiver.info)
			// TODO: Generate unpredictable assistive download tickets
			receiver.unspentTickets["00000000"] = true
			receiver.unspentTickets["00000001"] = true
			receiver.assistiveHandler = httpsync.NewFileHandlerFromSyncInfo(
				syncinfo.Shuffle(), receiver.storage).WithPrinter(receiver.log)
		case "a32chunks":
			// Backwards-compatible transmission of hashes of chunked work data.
			// TODO: remove backwards-compatibility
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("work already received on 'a32chunks'")
			}
			todo.mustWriteWishList = true

			// guard against DOS, max length of hashes stream depends on max number of
			// chunks in work file
			r := io.LimitReader(part, 1<<18)
			var syncinfo remotesync.SyncInfo
			syncinfo.SetTrivialPermutation()
			if err := syncinfo.ReadFromLegacyStream(r); err != nil {
				return nil, fmt.Errorf("error reading chunk hashes from legacy stream: %v", err)
			}

			receiver.builder = remotesync.NewBuilder(receiver.storage, &syncinfo, len(syncinfo.Chunks), receiver.info)
		case "chunkdata":
			// Subset of the actual chunk data requested in response to hashes.
			if receiver.builder == nil {
				return nil, fmt.Errorf("didn't receive chunk hashes")
			}
			if receiver.workFile != nil {
				return nil, fmt.Errorf("work already received")
			}

			reconstruct := func() (cafs.File, error) {
				// Reconstruct file but drop mutex to enable concurrent download of wishlist
				b := receiver.builder
				receiver.mutex.Unlock()
				defer receiver.mutex.Lock()
				return b.ReconstructFileFromRequestedChunks(part)
			}

			if f, err := reconstruct(); err != nil {
				return nil, fmt.Errorf("error reconstructing work from sent chunks: %v", err)
			} else {
				receiver.workFile = f
			}
			todo.mustHandleWork = true
		default:
			return nil, fmt.Errorf("don't know what to do with part %#v", formName)
		}
	}
	return todo, nil
}

func verifyAssistTicket(s string) (*url.URL, error) {
	if url, err := url.Parse(s); err != nil {
		return nil, err
	} else if !assistPattern.MatchString(url.Path) {
		return nil, fmt.Errorf("assist ticket didn't match name convention: %v", url)
	} else {
		return url, nil
	}
}

// Decodes URL encoded messages and returns a todoList or an error.
func (receiver *endpointReceiver) handleUrlEncodedMessage(form url.Values) (*todoList, error) {
	todo := &todoList{}
	if form.Get("encresulthash") != "" {
		if receiver.encResultFile == nil {
			return nil, fmt.Errorf("result not available yet")
		}
		if form.Get("encresulthash") != receiver.encResultFile.Key().String() {
			return nil, fmt.Errorf("hash sum of encrypted result is wrong")
		}
		// Information is redundant, so no need to do anything here
	}
	if form.Get("encresulthashsig") != "" {
		if receiver.encResultFile == nil {
			return nil, fmt.Errorf("result not available yet")
		}
		if receiver.encResultHashSig != "" {
			return nil, fmt.Errorf("encresulthashsig already received")
		}
		receiver.encResultHashSig = form.Get("encresulthashsig")
		todo.mustHandleReceipt = true
	}
	return todo, nil
}

func (receiver *endpointReceiver) handleWorkAndReturnEncryptedResult() (cafs.File, error) {
	// Temporarily step out of mutex lock
	receiver.mutex.Unlock()
	defer receiver.mutex.Lock()

	result, err := receiver.handler.HandleWork(
		receiver.log.New("handle work"),
		receiver.workFile,
		receiver.buyerSecret)
	if result != nil {
		defer receiver.close(result, "result data")
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		panic("Handler returned nil result")
	}

	temp := receiver.storage.Create(fmt.Sprintf("%v: encrypted result", receiver.info))
	defer temp.Dispose()

	if err := encrypt(temp, result, receiver.encResultKey); err != nil {
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("error closing encrypted result temporary: %v", err)
	}
	return temp.File(), nil
}

func verifyBuyerSecret(workHash, workSecretHash, buyerSecret *bitwrk.Thash) error {
	sha := sha256.New()
	sha.Write(workHash[:])
	sha.Write(buyerSecret[:])
	var result bitwrk.Thash
	sha.Sum(result[:0])
	if result != *workSecretHash {
		return errors.New("buyer's secret does not match work hash and work secret hash")
	}
	return nil
}

func (receiver *endpointReceiver) handleReceipt() error {
	sig := receiver.encResultHashSig
	hash := receiver.encResultFile.Key().String()
	// Step out of lock temporarily for calling out to work handler
	receiver.mutex.Unlock()
	defer receiver.mutex.Lock()
	return receiver.handler.HandleReceipt(receiver.log, hash, sig)
}

// Function encrypt reads from `reader` and writes encryped data into `writer`.
func encrypt(writer io.Writer, reader io.Reader, key bitwrk.Tkey) (err error) {
	// Use AES-256 to encrypt the result
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return
	}

	// Create OFB stream with null initialization vector (ok for one-time key)
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	encWriter := &cipher.StreamWriter{S: stream, W: writer}
	_, err = io.Copy(encWriter, reader)
	return
}

// Utility function to safely close any closable. Logs and ignores any errors.
func (receiver *endpointReceiver) close(c io.Closer, info string) {
	if err := c.Close(); err != nil {
		receiver.log.Printf("Ignoring error closing %v: %v", info, err)
	}
}
