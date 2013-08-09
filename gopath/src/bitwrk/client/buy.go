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
	"bitwrk/bitcoin"
	"bitwrk/cafs"
	"bitwrk/money"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"log"
	"sync"
	"time"
)

type BuyActivity struct {
	condition           *sync.Cond
	manager             *ActivityManager
	key                 ActivityKey
	started, lastUpdate time.Time

	gotRejection bool
	gotMandate   bool
	identity     *bitcoin.KeyPair
	price        money.Money

	article       bitwrk.ArticleId
	workTemporary cafs.Temporary
	workFile      cafs.File

	bidId string
	bid   *bitwrk.Bid

	txId, txETag string
	tx           *bitwrk.Transaction

	secret bitwrk.Thash
}

func (m *ActivityManager) NewBuy(article bitwrk.ArticleId) (*BuyActivity, error) {
	now := time.Now()
	result := &BuyActivity{
		condition:  sync.NewCond(new(sync.Mutex)),
		manager:    m,
		started:    now,
		lastUpdate: now,
		article:    article,
	}
	m.add(m.newKey(), result)
	return result, nil
}

func (a *BuyActivity) Act() bool {
	now := time.Now()
	a.lastUpdate = now
	repeat := a.started.Add(10 * time.Second).After(now)
	return repeat
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
	// Request a mandate for the buy
	a.manager.mandateRequests <- buyMandateRequest{a}

	// wait for mandate or rejection
	log.Println("Waiting for mandate")

	if err := a.awaitMandate(); err != nil {
		return nil, err
	}
	log.Printf("Got mandate. Price: %v", a.price)

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
		log.Printf("Tx: %v", tx)
		log.Printf("Tx-etag: %#v", etag)
	}

	// TODO: Verify the transaction

	// draw random bytes for buyer's secret
	if _, err := rand.Reader.Read(a.secret[:]); err != nil {
		return nil, err
	}

	// Get work hash
	var workHash, workSecretHash bitwrk.Thash
	workHash = bitwrk.Thash(a.workFile.Key())

	// compute workSecretHash = hash(workHash | secret)
	hash := sha256.New()
	hash.Write(workHash[:])
	hash.Write(a.secret[:])
	hash.Sum(workSecretHash[:])

	if err := SendTxMessageEstablishBuyer(a.txId, a.identity, workHash, workSecretHash); err != nil {
		return nil, err
	}

	if err := a.awaitTransactionPhase(bitwrk.PhaseTransmitting, bitwrk.PhaseBuyerEstablished); err != nil {
		return nil, err
	}
	log.Printf("Transaction is now TRANSMITTING")

	return a.workFile, nil
}

func (a *BuyActivity) awaitMandate() error {
	// wait for mandate or rejection
	a.condition.L.Lock()
	defer a.condition.L.Unlock()
	for !a.gotMandate && !a.gotRejection {
		a.condition.Wait()
	}
	if a.gotMandate {
		return nil
	}
	return ErrNoMandate
}

func (a *BuyActivity) awaitBid() error {
	rawBid := bitwrk.RawBid{
		bitwrk.Buy,
		a.article,
		a.price,
	}
	if bidId, err := PlaceBid(&rawBid, a.identity); err != nil {
		return err
	} else {
		a.bidId = bidId
	}
	return nil
}

func (a *BuyActivity) awaitTransaction() error {
	lastETag := ""
	for count := 1; ; count++ {
		if bid, etag, err := FetchBid(a.bidId, lastETag); err != nil {
			return err
		} else if etag != lastETag {
			a.bid = bid
			lastETag = etag
			log.Printf("Bid: %#v", a.bid)
		}

		if a.bid.State == bitwrk.Matched {
			a.txId = *a.bid.Transaction
			break
		} else if a.bid.State == bitwrk.Expired {
			return ErrBidExpired
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}
	return nil
}

func (a *BuyActivity) awaitTransactionPhase(phase bitwrk.TxPhase, viaPhases ...bitwrk.TxPhase) error {
	for count := 1; ; count++ {
		if tx, etag, err := FetchTx(a.txId, ""); err != nil {
			return err
		} else if etag != a.txETag {
			a.tx = tx
			a.txETag = etag
			log.Printf("Tx: %v", tx)
			log.Printf("Tx-etag: %#v", etag)
		}

		if a.tx.State != bitwrk.StateActive {
			return ErrTxExpired
		}

		if phase == a.tx.Phase {
			break
		}

		valid := false
		for _, via := range viaPhases {
			if a.tx.Phase == via {
				valid = true
				break
			}
		}

		if !valid {
			return ErrTxUnexpectedState
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}

	return nil
}

func (a *BuyActivity) End() {
	a.condition.L.Lock()
	defer a.condition.L.Unlock()
	if a.workTemporary != nil {
		a.workTemporary.Dispose()
		a.workTemporary = nil
	}
	if !a.gotMandate && !a.gotRejection {
		a.gotRejection = true
	}
	a.condition.Broadcast()
}
