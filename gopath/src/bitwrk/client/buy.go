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
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type BuyActivity struct {
	Trade

	workTemporary cafs.Temporary
}

func (m *ActivityManager) NewBuy(article bitwrk.ArticleId) (*BuyActivity, error) {
	now := time.Now()
	result := &BuyActivity{
		Trade: Trade{
			condition:  sync.NewCond(new(sync.Mutex)),
			manager:    m,
			key:        m.newKey(),
			started:    now,
			lastUpdate: now,
			bidType:    bitwrk.Buy,
			article:    article,
		},
	}
	m.register(result.key, result)
	return result, nil
}

func (a *BuyActivity) WorkWriter() io.WriteCloser {
	if a.workTemporary != nil {
		panic("Temporary requested twice")
	}
	a.workTemporary = a.manager.storage.Create()
	return buyWorkWriter{a}
}

type buyWorkWriter struct {
	a *BuyActivity
}

func (w buyWorkWriter) Write(b []byte) (n int, err error) {
	n, err = w.a.workTemporary.Write(b)
	return
}

func (w buyWorkWriter) Close() error {
	a := w.a
	if err := a.workTemporary.Close(); err != nil {
		return err
	}

	if file, err := a.workTemporary.File(); err != nil {
		return err
	} else {
		a.workFile = file
	}

	return nil
}

func (a *BuyActivity) GetResult() (cafs.File, error) {
	defer a.manager.unregister(a.key)
	// wait for grant or reject
	log.Println("Waiting for permission")

	// Get a permission for the buy
	if err := a.awaitPermission(); err != nil {
		return nil, err
	}
	log.Printf("Got permission. Price: %v", a.price)

	if err := a.awaitBid(); err != nil {
		return nil, err
	}
	log.Printf("Got bid id: %v", a.bidId)

	if err := a.awaitTransaction(); err != nil {
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

	// Get work hash
	var workHash, workSecretHash bitwrk.Thash
	workHash = bitwrk.Thash(a.workFile.Key())

	// compute workSecretHash = hash(workHash | secret)
	hash := sha256.New()
	hash.Write(workHash[:])
	hash.Write(secret[:])
	hash.Sum(workSecretHash[:0])

	if err := SendTxMessageEstablishBuyer(a.txId, a.identity, workHash, workSecretHash); err != nil {
		return nil, err
	}

	if err := a.awaitTransactionPhase(bitwrk.PhaseTransmitting, bitwrk.PhaseBuyerEstablished); err != nil {
		return nil, err
	}

	if err := a.transmitWorkAndReceiveEncryptedResult(); err != nil {
		return nil, err
	}

	if err := a.signReceipt(); err != nil {
		return nil, err
	}

	if err := a.awaitTransactionPhase(bitwrk.PhaseUnverified, bitwrk.PhaseTransmitting, bitwrk.PhaseWorking); err != nil {
		return nil, err
	}

	a.encResultKey = a.tx.ResultDecryptionKey

	if err := a.decryptResult(); err != nil {
		return nil, err
	}

	if err := SendTxMessageAcceptResult(a.txId, a.identity); err != nil {
		return nil, fmt.Errorf("Failed to send 'accept result' message: %v", err)
	}

	return a.resultFile, nil
}

func (a *BuyActivity) End() {
	a.condition.L.Lock()
	defer a.condition.L.Unlock()
	if a.workTemporary != nil {
		a.workTemporary.Dispose()
		a.workTemporary = nil
	}
	if !a.accepted && !a.rejected {
		a.rejected = true
	}
	a.condition.Broadcast()
}

func (a *BuyActivity) transmitWorkAndReceiveEncryptedResult() error {
	// Send work to client
	pipeIn, pipeOut := io.Pipe()
	mwriter := multipart.NewWriter(pipeOut)
	go func() {
		part, err := mwriter.CreateFormFile("work", "workfile.bin")
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
		work := a.workFile.Open()
		_, err = io.Copy(part, work)
		work.Close()
		if err != nil {
			pipeOut.CloseWithError(err)
			return
		}
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
		pipeOut.Close()
	}()

	var response io.ReadCloser
	if req, err := newRequest("POST", *a.tx.WorkerURL, pipeIn); err != nil {
		return err
	} else {
		req.Header.Set("Content-Type", mwriter.FormDataContentType())
		if resp, err := client.Do(req); err != nil {
			return err
		} else {
			response = resp.Body
		}
	}

	temp := a.manager.storage.Create()
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

	if f, err := temp.File(); err != nil {
		return err
	} else {
		a.encResultFile = f
	}

	log.Printf("Got encrypted result: %v", a.encResultFile.Key())
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

	temp := a.manager.GetStorage().Create()
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

	if file, err := temp.File(); err != nil {
		return err
	} else {
		a.resultFile = file
	}

	return nil
}
