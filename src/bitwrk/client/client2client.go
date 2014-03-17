//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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
	"bitwrk"
	"bitwrk/cafs"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
)

var ErrAlreadyDisposed = errors.New("Already disposed")

type WorkHandler interface {
	// Called when the work file arrives.
	// Returns a ReadCloser (which must be closed by the caller) and/or an error.
	HandleWork(log bitwrk.Logger, work cafs.File, buyerSecret bitwrk.Thash) (io.ReadCloser, error)

	HandleReceipt(log bitwrk.Logger, encresultHash, encResultHashSig string) error
}

type WorkReceiver interface {
	Dispose()
	URL() string
}

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
	result.endpoint.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		if err := result.handleRequest(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error in request: %v", err)
			result.Dispose()
		}
	})
	return result
}

type endpointReceiver struct {
	mutex            sync.Mutex
	disposed         bool
	endpoint         *Endpoint
	storage          cafs.FileStorage
	log              bitwrk.Logger
	handler          WorkHandler
	encResultKey     bitwrk.Tkey
	builder          *cafs.Builder
	buyerSecret      bitwrk.Thash
	workFile         cafs.File
	encResultFile    cafs.File
	info             string
	encResultHashSig string
}

func (r *endpointReceiver) URL() string {
	return r.endpoint.URL()
}

func (receiver *endpointReceiver) doDispose() {
	if !receiver.disposed {
		receiver.disposed = true
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
	receiver.doDispose()
}

var ZEROHASH bitwrk.Thash

type todoList struct {
	mustHandleChunkHashes bool
	mustHandleWork        bool
	mustHandleReceipt     bool
}

func (receiver *endpointReceiver) handleRequest(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "OPTIONS" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Adler32Chunking": true}`))
		return nil
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil
	}

	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	// Verify we haven't been disposed yet
	if receiver.disposed {
		return ErrAlreadyDisposed
	}

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
		todo, err = receiver.handleMultipartMessage(mreader)
		if err != nil {
			return fmt.Errorf("Error multipart mesage: %v", err)
		}
	} else {
		return fmt.Errorf("Don't know how to handle message (Content-Type: %v)", r.Header.Get("Content-Type"))
	}

	if (receiver.workFile == nil && receiver.builder == nil) || receiver.buyerSecret == ZEROHASH {
		return fmt.Errorf("Incomplete work message. Got work file: %v. Got chunk hashes: %v. Got buyer secret: %v", receiver.workFile != nil, receiver.builder != nil, receiver.buyerSecret != ZEROHASH)
	}

	if todo.mustHandleReceipt {
		return receiver.handleReceipt()
	} else if todo.mustHandleChunkHashes {
		w.Header().Set("Content-Type", "application/x-missing-chunks")
		return receiver.builder.EncodeMissingChunks(w)
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
			fmt.Printf("Error sending work result back to buyer: %v", err)
		}
		return nil // No use in returning an error, http connection is probably closed anyway
	} else {
		panic("Shouldn't get here")
	}
}

func (receiver *endpointReceiver) handleMultipartMessage(mreader *multipart.Reader) (*todoList, error) {
	todo := &todoList{}
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
		switch formName {
		case "buyersecret":
			if receiver.buyerSecret != ZEROHASH {
				return nil, fmt.Errorf("Buyer's secret already received")
			}
			b := make([]byte, 64)
			n, err := part.Read(b)
			if err != nil || n != len(b) {
				return nil, fmt.Errorf("Error reading buyersecret: %v (%v bytes read)", err, n)
			}
			n, err = hex.Decode(receiver.buyerSecret[:], b)
			if err != nil || n != len(receiver.buyerSecret) {
				return nil, fmt.Errorf("Error decoding buyersecret: %v (%v bytes written)", err, n)
			}
		case "work":
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
			if receiver.builder != nil || receiver.workFile != nil {
				return nil, fmt.Errorf("Work already received on 'a32chunks'")
			}
			if b, err := cafs.NewBuilder(receiver.storage, part, receiver.info); err != nil {
				return nil, fmt.Errorf("Error receiving chunk hashes: %v", err)
			} else {
				receiver.builder = b
				todo.mustHandleChunkHashes = true
			}
		case "chunkdata":
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

func (receiver *endpointReceiver) handleUrlEncodedMessage(form url.Values) (*todoList, error) {
	todo := &todoList{}
	if form.Get("encresulthash") != "" {
		if receiver.encResultFile == nil {
			return nil, fmt.Errorf("Result not available yet")
		}
		if form.Get("encresulthash") != receiver.encResultFile.Key().String() {
			return nil, fmt.Errorf("Hash sum of encrypted result is wrong")
		}
		// Informtion is redundant, so no need to do anything here
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

	reader := workFile.Open()
	defer reader.Close()
	result, err := a.worker.DoWork(reader, connChan)
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

	a.encResultFile = temp.File()

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
