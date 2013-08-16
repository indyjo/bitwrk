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
	"log"
	"sync"
	"time"
)

type Trade struct {
	condition           *sync.Cond
	manager             *ActivityManager
	key                 ActivityKey
	started, lastUpdate time.Time
	
	bidType             bitwrk.BidType
	article             bitwrk.ArticleId

	rejected bool
	accepted bool
	identity *bitcoin.KeyPair
	price    money.Money

	bidId string
	bid   *bitwrk.Bid

	txId, txETag string
	tx           *bitwrk.Transaction

	buyerSecret   *bitwrk.Thash
	workFile      cafs.File
	
	encResultFile    cafs.File
	encResultKey     *bitwrk.Tkey
	encResultHashSig string
	
	resultFile cafs.File
}

func (t *Trade) awaitPermission() error {
	// wait for permission or rejection
	t.condition.L.Lock()
	defer t.condition.L.Unlock()
	for !t.accepted && !t.rejected {
		t.condition.Wait()
	}
	if t.accepted {
		return nil
	}
	return ErrNoPermission
}

func (t *Trade) awaitBid() error {
	rawBid := bitwrk.RawBid{
		t.bidType,
		t.article,
		t.price,
	}
	if bidId, err := PlaceBid(&rawBid, t.identity); err != nil {
		return err
	} else {
		t.bidId = bidId
	}
	return nil
}

func (t *Trade) awaitTransaction() error {
	lastETag := ""
	for count := 1; ; count++ {
		if bid, etag, err := FetchBid(t.bidId, lastETag); err != nil {
			return err
		} else if etag != lastETag {
			t.bid = bid
			lastETag = etag
			log.Printf("Bid: %#v", t.bid)
		}

		if t.bid.State == bitwrk.Matched {
			t.txId = *t.bid.Transaction
			break
		} else if t.bid.State == bitwrk.Expired {
			return ErrBidExpired
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}
	return nil
}

func (t *Trade) awaitTransactionPhase(phase bitwrk.TxPhase, viaPhases ...bitwrk.TxPhase) error {
    log.Printf("Awaiting transaction phase %v...", phase)
	for count := 1; ; count++ {
		if tx, etag, err := FetchTx(t.txId, ""); err != nil {
			return err
		} else if etag != t.txETag {
			t.tx = tx
			t.txETag = etag
			log.Printf("Tx-etag: %#v", etag)
		}

		if t.tx.State != bitwrk.StateActive {
			return ErrTxExpired
		}

		if phase == t.tx.Phase {
			break
		}

		valid := false
		for _, via := range viaPhases {
			if t.tx.Phase == via {
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

	log.Printf("Phase %v reached.", phase)
	return nil
}

func (t *Trade) waitForTransactionPhase(phase bitwrk.TxPhase, viaPhases ...bitwrk.TxPhase) error {
    log.Printf("Waiting for transaction phase %v...", phase)
	for {
	    t.condition.L.Lock()
	    t.condition.Wait()
	    currentPhase := t.tx.Phase
	    currentState := t.tx.State
	    t.condition.L.Unlock()
	    
		if currentState != bitwrk.StateActive {
			return ErrTxExpired
		}

		if currentPhase == phase {
			break
		}

		valid := false
		for _, via := range viaPhases {
			if currentPhase == via {
				valid = true
				break
			}
		}

		if !valid {
			return ErrTxUnexpectedState
		}

	}

	log.Printf("Phase %v reached.", phase)
	return nil
}

func (t *Trade) waitWhile(f func() bool) {
    t.condition.L.Lock()
    defer t.condition.L.Unlock()
    for {
        stay := f()
        if !stay {
            return
        }
        t.condition.Wait()
    }
}

// Polls the transaction state in a separate go-routine
func (t *Trade) pollTransaction() {
    
	for count := 1; ; count++ {
	    
		if tx, etag, err := FetchTx(t.txId, ""); err != nil {
			log.Printf("Error polling transaction: %v", err)
			return
		} else if etag != t.txETag {
		    t.condition.L.Lock()
			t.tx = tx
			t.txETag = etag
			expired := t.tx.State != bitwrk.StateActive
			t.condition.Broadcast()
			t.condition.L.Unlock()
			log.Printf("Tx-etag: %#v", etag)
			if expired {
			    log.Printf(" --> expired")
			    return
			}
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}

}
