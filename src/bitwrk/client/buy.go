//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013  Jonas Eschenburg <jonas@bitwrk.net>
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
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type BuyActivity struct {
	Trade
}

func (m *ActivityManager) NewBuy(article bitwrk.ArticleId) (*BuyActivity, error) {
	now := time.Now()
	result := &BuyActivity{
		Trade: Trade{
			condition:  sync.NewCond(new(sync.Mutex)),
			manager:    m,
			key:        m.NewKey(),
			started:    now,
			lastUpdate: now,
			bidType:    bitwrk.Buy,
			article:    article,
			alive:      true,
		},
	}
	m.register(result.key, result)
	return result, nil
}

// Manages the complete lifecycle of a buy.
// When a bool can be read from interrupt, the buy is aborted.
func (a *BuyActivity) PerformBuy(log bitwrk.Logger, interrupt <-chan bool, workFile cafs.File) (cafs.File, error) {
	a.workFile = workFile.Duplicate()
	
	file, err := a.doPerformBuy(log, interrupt)
	if err != nil {
		a.lastError = err
	}
	a.alive = false
	return file, err
}

func (a *BuyActivity) doPerformBuy(log bitwrk.Logger, interrupt <-chan bool) (cafs.File, error) {
	// wait for grant or reject
	log.Println("Waiting for permission")

	// Get a permission for the buy
	if err := a.awaitPermission(interrupt); err != nil {
		return nil, err
	}
	log.Printf("Got permission. Price: %v", a.price)

	if err := a.awaitBid(); err != nil {
		return nil, err
	}
	log.Printf("Got bid id: %v", a.bidId)

	if err := a.awaitTransaction(log); err != nil {
		return nil, err
	}
	log.Printf("Got transaction id: %v", a.txId)

	if tx, etag, err := FetchTx(a.txId, ""); err != nil {
		return nil, err
	} else {
		a.tx = tx
		a.txETag = etag
		log.Printf("Tx-etag: %#v", etag)
	}

	// TODO: Verify the transaction

	// draw random bytes for buyer's secret
	var secret bitwrk.Thash
	if _, err := rand.Reader.Read(secret[:]); err != nil {
		return nil, err
	}
	a.buyerSecret = &secret
	log.Printf("Computed buyer's secret.")

	// Get work hash
	var workHash, workSecretHash bitwrk.Thash
	workHash = bitwrk.Thash(a.workFile.Key())

	// compute workSecretHash = hash(workHash | secret)
	hash := sha256.New()
	hash.Write(workHash[:])
	hash.Write(secret[:])
	hash.Sum(workSecretHash[:0])

	if err := SendTxMessageEstablishBuyer(a.txId, a.identity, workHash, workSecretHash); err != nil {
		return nil, fmt.Errorf("Error establishing buyer: %v", err)
	}

	if err := a.awaitTransactionPhase(log.New("establishing"), bitwrk.PhaseTransmitting, bitwrk.PhaseBuyerEstablished); err != nil {
		return nil, fmt.Errorf("Error awaiting TRANSMITTING phase: %v", err)
	}

	if err := a.transmitWorkAndReceiveEncryptedResult(log.New("transmitting")); err != nil {
		return nil, fmt.Errorf("Error transmitting work and receiving encrypted result: %v", err)
	}

	if err := a.signReceipt(); err != nil {
		return nil, fmt.Errorf("Error signing receipt for encrypted result: %v", err)
	}

	if err := a.awaitTransactionPhase(log, bitwrk.PhaseUnverified, bitwrk.PhaseTransmitting, bitwrk.PhaseWorking); err != nil {
		return nil, fmt.Errorf("Error awaiting UNVERIFIED phase: %v", err)
	}

	a.encResultKey = a.tx.ResultDecryptionKey

	if err := a.decryptResult(); err != nil {
		return nil, fmt.Errorf("Error decrypting result: %v", err)
	}

	if err := SendTxMessageAcceptResult(a.txId, a.identity); err != nil {
		return nil, fmt.Errorf("Failed to send 'accept result' message: %v", err)
	}

	return a.resultFile, nil
}

func (a *BuyActivity) transmitWorkAndReceiveEncryptedResult(log bitwrk.Logger) error {
	// Send work to client
	pipeIn, pipeOut := io.Pipe()
	mwriter := multipart.NewWriter(pipeOut)

	// Write work file into pipe for HTTP request
	go func() {
		part, err := mwriter.CreateFormFile("work", "workfile.bin")
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		work := a.workFile.Open()
		log.Printf("Sending work data to seller [%v].", *a.tx.WorkerURL)
		_, err = io.Copy(part, work)
		work.Close()
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		log.Printf("Sending buyer's secret to seller.")
		err = mwriter.WriteField("buyersecret", a.buyerSecret.String())
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		err = mwriter.Close()
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		err = pipeOut.Close()
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		log.Printf("Work transmitted successfully.")
	}()

	var response io.ReadCloser
	if req, err := newRequest("POST", *a.tx.WorkerURL, pipeIn); err != nil {
		return fmt.Errorf("Error creating transmit request: %v", err)
	} else {
		req.Header.Set("Content-Type", mwriter.FormDataContentType())

		var controlledClient http.Client = client
		controlledClient.Transport = &http.Transport{
			Dial: func(network, addr string) (conn net.Conn, err error) {
				conn, err = net.DialTimeout(network, addr, 10*time.Second)
				if err == nil {
					// Keep watching tx and close connection when retired before end of transmission
					go func() {
						if err := a.awaitTransactionPhase(log, bitwrk.PhaseUnverified, bitwrk.PhaseWorking, bitwrk.PhaseTransmitting); err != nil {
							log.Printf("Closing connection to seller: %v", err)
							conn.Close()
						}
					}()
				}
				return
			},
		}

		if resp, err := controlledClient.Do(req); err != nil {
			return err
		} else {
			response = resp.Body
		}
	}

	temp := a.manager.storage.Create(fmt.Sprintf("Buy #%v: encrypted result", a.GetKey()))
	defer temp.Dispose()

	if _, err := io.Copy(temp, response); err != nil {
		return err
	}
	if err := response.Close(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}

	a.encResultFile = temp.File()

	return nil
}

// Signs a receipt for the encrypted result that the seller can use to
// prove that the result was transmitted correctly. In exchange, we get the
// key to unlock the encrypted result.
func (a *BuyActivity) signReceipt() error {
	encresulthash := a.encResultFile.Key().String()
	if sig, err := a.identity.SignMessage(encresulthash, rand.Reader); err != nil {
		return err
	} else {
		a.encResultHashSig = sig
	}

	formValues := url.Values{}
	formValues.Set("encresulthash", encresulthash)
	formValues.Set("encresulthashsig", a.encResultHashSig)

	if resp, err := client.PostForm(*a.tx.WorkerURL, formValues); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error sending receipt for encrypted result: %v", resp.Status)
	}

	return nil
}

func (a *BuyActivity) decryptResult() error {
	block, err := aes.NewCipher(a.encResultKey[:])
	if err != nil {
		return err
	}

	temp := a.manager.GetStorage().Create(fmt.Sprintf("Buy #%v: result", a.GetKey()))
	defer temp.Dispose()

	encrypted := a.encResultFile.Open()
	defer encrypted.Close()

	// Create OFB stream with null initialization vector (ok for one-time key)
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	reader := &cipher.StreamReader{S: stream, R: encrypted}
	_, err = io.Copy(temp, reader)
	if err != nil {
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	a.resultFile = temp.File()

	return nil
}
