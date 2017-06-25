//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2017  Jonas Eschenburg <jonas@bitwrk.net>
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
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/indyjo/bitwrk-common/bitwrk"
	. "github.com/indyjo/bitwrk-common/protocol"
	"github.com/indyjo/cafs"
	"github.com/indyjo/cafs/remotesync"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
)

var ErrAlreadyDisposed = errors.New("Already disposed")

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
	receiveManager *ReceiveManager,
	storage cafs.FileStorage,
	key bitwrk.Tkey,
	handler WorkHandler) WorkReceiver {
	result := &endpointReceiver{
		endpoint:     receiveManager.NewEndpoint(info),
		storage:      storage,
		log:          log,
		handler:      handler,
		info:         info,
		encResultKey: key,
	}
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		if err := result.handleRequest(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error in request: %v", err)
			result.doDispose(fmt.Errorf("Error handling request from %v: %v", r.RemoteAddr, err))
		}
	}
	result.endpoint.SetHandler(withCompression(handlerFunc))
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
	endpoint         *Endpoint
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
}

func (r *endpointReceiver) URL() string {
	return r.endpoint.URL()
}

func (receiver *endpointReceiver) doDispose(err error) {
	receiver.disposedMutex.Lock()
	defer receiver.disposedMutex.Unlock()

	if !receiver.disposed {
		receiver.disposed = true
		receiver.disposedError = err
		receiver.endpoint.Dispose()
		if receiver.builder != nil {
			receiver.builder.Dispose()
			receiver.builder = nil
		}
		if receiver.workFile != nil {
			receiver.workFile.Dispose()
			receiver.workFile = nil
		}
	}
}

func (receiver *endpointReceiver) Dispose() {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	receiver.doDispose(nil)
}

func (receiver *endpointReceiver) IsDisposed() (bool, error) {
	receiver.disposedMutex.Lock()
	defer receiver.disposedMutex.Unlock()
	return receiver.disposed, receiver.disposedError
}

var ZEROHASH bitwrk.Thash

type todoList struct {
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
		w.Write([]byte(`{"Adler32Chunking": true, "GZIPCompression": true}`))
		return nil
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
			return fmt.Errorf("Failed to parse form data: %v", err)
		}

		t, err := receiver.handleUrlEncodedMessage(r.Form)
		if err != nil {
			return fmt.Errorf("Error handling url-encoded message: %v", err)
		}
		todo = t
	} else if mreader, err := r.MultipartReader(); err == nil {
		todo, err = receiver.handleMultipartMessage(mreader, w)
		if err != nil {
			return fmt.Errorf("Error handling multipart message: %v", err)
		}
	} else {
		return fmt.Errorf("Don't know how to handle message (Content-Type: %v)", r.Header.Get("Content-Type"))
	}

	if (receiver.workFile == nil && receiver.builder == nil) || receiver.buyerSecret == ZEROHASH {
		return fmt.Errorf("Incomplete work message. Got work file: %v. Got chunk hashes: %v. Got buyer secret: %v", receiver.workFile != nil, receiver.builder != nil, receiver.buyerSecret != ZEROHASH)
	}

	if todo.mustHandleReceipt {
		return receiver.handleReceipt()
	} else if todo.mustHandleWork {
		if r, err := receiver.handleWorkAndReturnEncryptedResult(); err != nil {
			return err
		} else if receiver.encResultFile != nil {
			return fmt.Errorf("Encrypted result file exists already")
		} else {
			receiver.log.Printf("Encrypted result file: %v", r)
			receiver.encResultFile = r
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		reader := receiver.encResultFile.Open()
		defer reader.Close()
		if _, err := io.Copy(w, reader); err != nil {
			return fmt.Errorf("Error sending work result back to buyer: %v", err)
		}
	}
	return nil
}

// Decodes MIME multipart/form-data messages and returns a todoList or an error.
func (receiver *endpointReceiver) handleMultipartMessage(mreader *multipart.Reader, w http.ResponseWriter) (*todoList, error) {
	todo := &todoList{}
	responseGiven := false
	// iterate through parts of multipart/form-data content
	for {
		part, err := mreader.NextPart()
		if err == io.EOF {
			receiver.log.Printf("End of stream reached")
			break
		} else if err != nil {
			return nil, fmt.Errorf("Error reading part: %v", err)
		}
		formName := part.FormName()
		receiver.log.Printf("Handling part: %v", formName)

		// The following cases are the messages which require a response.
		// As only one response may be given per request, they need to
		// check and complain.
		switch formName {
		case "work":
			fallthrough
		case "a32chunks":
			fallthrough
		case "chunkdata":
			if responseGiven {
				return nil, fmt.Errorf("Received illegal trailing message part '%v'", formName)
			}
			responseGiven = true
		}

		// Handle individual messages
		switch formName {
		case "buyersecret":
			// Buyer sends a random value to seller to prevent the seller from hijacking other seller's workers.
			if receiver.buyerSecret != ZEROHASH {
				return nil, fmt.Errorf("Buyer's secret already received")
			}
			b := make([]byte, 64)
			if n, err := io.ReadFull(part, b); err != nil {
				return nil, fmt.Errorf("Error reading buyersecret: %v (%v bytes read)", err, n)
			}
			if n, err := hex.Decode(receiver.buyerSecret[:], b); err != nil || n != len(receiver.buyerSecret) {
				return nil, fmt.Errorf("Error decoding buyersecret: %v (%v bytes written)", err, n)
			}
		case "work":
			// Direct work data transmission without chunking. Up to 16MB.
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("Work already received")
			}
			temp := receiver.storage.Create(receiver.info)
			defer temp.Dispose()
			const MAXBYTES = 2 << 24 // 16MB
			// Copy up to MAXBYTES and expect EOF
			if n, err := io.CopyN(temp, part, MAXBYTES); err != io.EOF {
				return nil, fmt.Errorf("Work too long or error: %v (%v bytes read)", err, n)
			}
			if err := temp.Close(); err != nil {
				return nil, fmt.Errorf("Error creating file from temporary data: %v", err)
			}
			receiver.workFile = temp.File()
			todo.mustHandleWork = true
		case "a32chunks":
			// Transmission of hashes of chunked work data.
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("Work already received on 'a32chunks'")
			}
			receiver.builder = remotesync.NewBuilder(receiver.storage, MaxNumberOfChunksInWorkFile, receiver.info)
			w.Header().Set("Content-Type", "application/x-wishlist")
			if err := receiver.builder.WriteWishList(part, w); err != nil {
				return nil, fmt.Errorf("Error handling chunk hashes: %v", err)
			}
		case "chunkdata":
			// Subset of the actual chunk data requested in response to hashes.
			if receiver.builder == nil {
				return nil, fmt.Errorf("Didn't receive chunk hashes")
			}
			if receiver.workFile != nil {
				return nil, fmt.Errorf("Work already received")
			}
			if f, err := receiver.builder.ReconstructFileFromRequestedChunks(part); err != nil {
				return nil, fmt.Errorf("Error reconstructing work from sent chunks: %v", err)
			} else {
				receiver.workFile = f
			}
			todo.mustHandleWork = true
		default:
			return nil, fmt.Errorf("Don't know what to do with part %#v", formName)
		}
	}
	return todo, nil
}

// Decodes URL encoded messages and returns a todoList or an error.
func (receiver *endpointReceiver) handleUrlEncodedMessage(form url.Values) (*todoList, error) {
	todo := &todoList{}
	if form.Get("encresulthash") != "" {
		if receiver.encResultFile == nil {
			return nil, fmt.Errorf("Result not available yet")
		}
		if form.Get("encresulthash") != receiver.encResultFile.Key().String() {
			return nil, fmt.Errorf("Hash sum of encrypted result is wrong")
		}
		// Information is redundant, so no need to do anything here
	}
	if form.Get("encresulthashsig") != "" {
		if receiver.encResultFile == nil {
			return nil, fmt.Errorf("Result not available yet")
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
		defer result.Close()
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
		return nil, fmt.Errorf("Error closing encrypted result temporary: %v", err)
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
		return errors.New("Buyer's secret does not match work hash and work secret hash")
	}
	return nil
}

func (a *SellActivity) dispatchWorkAndSaveEncryptedResult(log bitwrk.Logger, workFile cafs.File) error {
	// Watch transaction state and close connection to worker when transaction expires
	connChan := make(chan io.Closer)
	exitChan := make(chan bool)
	go a.watchdog(log, exitChan, connChan, func() bool { return a.tx.State == bitwrk.StateActive })
	defer func() {
		exitChan <- true
	}()

	st := NewScopedTransport()
	connChan <- st
	defer st.Close()

	reader := workFile.Open()
	defer reader.Close()
	result, err := a.worker.DoWork(reader, NewClient(&st.Transport))
	if err != nil {
		return err
	}
	defer result.Close()

	temp := a.manager.storage.Create(fmt.Sprintf("Sell #%v: encrypted result", a.GetKey()))
	defer temp.Dispose()

	// Use AES-256 to encrypt the result
	block, err := aes.NewCipher(a.encResultKey[:])
	if err != nil {
		return err
	}

	// Create OFB stream with null initialization vector (ok for one-time key)
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	writer := &cipher.StreamWriter{S: stream, W: temp}
	_, err = io.Copy(writer, result)
	if err != nil {
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	if err := result.Close(); err != nil {
		return err
	}

	a.execSync(func() { a.encResultFile = temp.File() })

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

func encrypt(temp cafs.Temporary, reader io.Reader, key bitwrk.Tkey) (err error) {
	// Use AES-256 to encrypt the result
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return
	}

	// Create OFB stream with null initialization vector (ok for one-time key)
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	writer := &cipher.StreamWriter{S: stream, W: temp}
	_, err = io.Copy(writer, reader)
	return
}
