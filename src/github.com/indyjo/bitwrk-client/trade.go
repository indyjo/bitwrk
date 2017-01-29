//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2015  Jonas Eschenburg <jonas@bitwrk.net>
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
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/money"
	"github.com/indyjo/bitwrk-common/protocol"
	"github.com/indyjo/cafs"
	"io"
	"sync"
	"time"
)

type Trade struct {
	condition           *sync.Cond
	manager             *ActivityManager
	key                 ActivityKey
	lastError           error
	started, lastUpdate time.Time

	bidType bitwrk.BidType
	article bitwrk.ArticleId

	alive             bool   // Set to false on end of life
	clearanceDenied   bool   // Set to true on Forbid
	clearedForTrade   bool   // Set to true on Permit
	localMatch        *Trade // Set to a matching trade on local match
	awaitingClearance bool   // Set to false on local match, Forbid and Permit

	// Information stored on Permit(...)
	identity *bitcoin.KeyPair
	price    money.Money

	// Remote bid information
	bidId string
	bid   *bitwrk.Bid

	// Remote transaction information
	txId, txETag string
	tx           *bitwrk.Transaction

	buyerSecret *bitwrk.Thash
	workFile    cafs.File

	bytesToTransfer  uint64
	bytesTransferred uint64

	encResultFile    cafs.File
	encResultKey     *bitwrk.Tkey
	encResultHashSig string

	resultFile cafs.File
}

// Configuration value for the maximum number of unmatched bids to allow at a time
var NumUnmatchedBids = 1

func (a *Trade) beginRemoteTrade(log bitwrk.Logger, interrupt <-chan bool) error {
	// Prevent too many unmatched bids on server
	key := fmt.Sprintf("%v-%v", a.bidType, a.article)
	if err := a.manager.checkoutToken(key, NumUnmatchedBids, interrupt); err != nil {
		return err
	}
	defer a.manager.returnToken(key)

	if err := a.awaitBid(); err != nil {
		return fmt.Errorf("Error awaiting bid: %v", err)
	}
	log.Printf("Got bid id: %v", a.bidId)

	if err := a.awaitTransaction(log); err != nil {
		return fmt.Errorf("Error awaiting transaction: %v", err)
	}
	log.Printf("Got transaction id: %v", a.txId)

	if tx, etag, err := protocol.FetchTx(a.txId, ""); err != nil {
		return err
	} else {
		a.execSync(func() {
			a.tx = tx
			a.txETag = etag
		})
	}

	return nil
}

// Executes a short function that modifies the trade's internal state.
// Then broadcasts a signal to condition listeners.
func (t *Trade) execSync(f func()) {
	t.condition.L.Lock()
	defer t.condition.L.Unlock()
	f()
	t.condition.Broadcast()
}

// Waits until either user has granted permission for this trade or the trade has been matched locally.
// On success, the caller may query the appropriate state fields to find out which action to take.
// Returns nil on clearance (either local or trade) and
// ErrNoPermission if permission was rejected.
// Returns ErrInterrupted if a boolean can be read from 'interrupt' while waiting.
func (t *Trade) awaitClearance(log bitwrk.Logger, interrupt <-chan bool) error {
	// wait for grant or reject
	log.Println("Awaiting clearance")

	err := t.interruptibleWaitWhile(interrupt, func() bool { return t.awaitingClearance })

	if err != nil {
		fmt.Errorf("Error awaiting clearance: %v", err)
		return err
	} else if t.clearanceDenied {
		fmt.Errorf("Clearance denied")
		return ErrNoPermission
	} else {
		if t.clearedForTrade {
			log.Printf("Got trade clearance. Price: %v", t.price)
		} else if t.localMatch != nil {
			log.Printf("Got local clearance. Matched with #%v.", t.localMatch.key)
		} else {
			panic("unexpected state")
		}
		return nil
	}
}

var bidMutex sync.Mutex

func (t *Trade) awaitBid() error {
	bidMutex.Lock()
	defer func() {
		bidMutex.Unlock()
	}()

	rawBid := bitwrk.RawBid{
		t.bidType,
		t.article,
		t.price,
	}
	if bidId, err := protocol.PlaceBid(&rawBid, t.identity); err != nil {
		return err
	} else {
		t.bidId = bidId
	}
	return nil
}

func (t *Trade) awaitTransaction(log bitwrk.Logger) error {
	lastETag := ""
	for count := 1; ; count++ {
		if bid, etag, err := protocol.FetchBid(t.bidId, lastETag); err != nil {
			return fmt.Errorf("Error in FetchBid awaiting transaction: %v", err)
		} else if bid != nil {
			log.Printf("Bid: %#v ETag: %v lastETag: %v", *bid, etag, lastETag)
			t.bid = bid
			lastETag = etag
			if t.bid.State == bitwrk.Matched {
				t.txId = *t.bid.Transaction
				break
			} else if t.bid.State == bitwrk.Expired {
				return ErrBidExpired
			}
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}
	return nil
}

func (t *Trade) waitForTransactionPhase(log bitwrk.Logger, phase bitwrk.TxPhase, viaPhases ...bitwrk.TxPhase) error {
	log.Printf("Waiting for transaction phase %v...", phase)

	if err := t.updateTransaction(log); err != nil {
		return err
	}

	var currentPhase bitwrk.TxPhase
	var currentState bitwrk.TxState
	t.waitWhile(func() bool {
		currentPhase = t.tx.Phase
		currentState = t.tx.State
		log.Printf("Phase: %v - State: %v", currentPhase, currentState)
		if currentState != bitwrk.StateActive {
			return false
		}
		if currentPhase == phase {
			return false
		}
		valid := false
		for _, via := range viaPhases {
			if currentPhase == via {
				valid = true
				break
			}
		}
		return valid
	})
	if currentState != bitwrk.StateActive {
		return ErrTxExpired
	}

	if currentPhase == phase {
		log.Printf("Phase %v reached.", phase)
		return nil
	}

	return ErrTxUnexpectedState
}

// Waits on state changes and repeatedly evaluates f() until it returns false once. Then returns.
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

// Waits on state changes until either f() returns false or a boolean was read from interrupt,
// in which case ErrInterrupted is returned. Otherwise, returns nil.
func (t *Trade) interruptibleWaitWhile(interrupt <-chan bool, f func() bool) error {
	// On end of function, send a boolean through 'exit' to stop
	exit := make(chan bool)
	defer func() {
		exit <- true
	}()

	// Listen for interrupt and exit signal in parallel goroutine
	interrupted := false
	go func() {
		for {
			select {
			case <-interrupt:
				interrupted = true
				t.condition.Broadcast()
			case <-exit:
				return
			}
		}
	}()

	t.waitWhile(func() bool { return !interrupted && f() })
	if interrupted {
		return ErrInterrupted
	}
	return nil
}

// Closes resources upon exit of a function or when some condition no longer holds
// Arguments:
//  - exitChan: Signals the watchdog to exit
//  - closerChan: Signals the watchdog to add an io.Closer to the list of closers
//  - f: Defines the OK condition. When false, all current closers are closed
func (t *Trade) watchdog(log bitwrk.Logger, exitChan <-chan bool, closerChan <-chan io.Closer, f func() bool) {
	closers := make([]io.Closer, 0, 1)
	for {
		select {
		case closer := <-closerChan:
			closers = append(closers, closer)
		case <-exitChan:
			// Exit from watchdog if surrounding function has terminated
			log.Print("Watchdog exiting by request")
			return
		default:
		}

		// Check if condition still holds
		t.condition.L.Lock()
		ok := f()
		t.condition.L.Unlock()
		if !ok {
			log.Printf("Watchdog: closing %v channels", len(closers))
			for _, c := range closers {
				err := c.Close()
				if err != nil {
					log.Printf("Error closing channel: %v", err)
				}
			}
			closers = closers[:0] // clear list of current closers
		}

		time.Sleep(250 * time.Millisecond)
	}
}

func (t *Trade) updateTransaction(log bitwrk.Logger) error {
	attemptsLeft := 3
	for attemptsLeft > 0 {
		attemptsLeft--
		if tx, etag, err := protocol.FetchTx(t.txId, ""); err != nil {
			log.Printf("Error updating transaction: %v (attempts left: %d)", err, attemptsLeft)
			if attemptsLeft > 0 {
				time.Sleep(5 * time.Second)
			} else {
				return err
			}
		} else {
			expired := false
			func() {
				t.condition.L.Lock()
				defer t.condition.L.Unlock()
				if etag != t.txETag {
					t.tx = tx
					t.txETag = etag
					expired = t.tx.State != bitwrk.StateActive
					t.condition.Broadcast()
					log.Printf("Tx change detected: phase=%v, expired=%v", t.tx.Phase, expired)
				}
			}()
			if expired {
				break
			}
		}
	}
	return nil
}

// Polls the transaction state in a separate go-routine. Returns on abort signal, or
// when the polled transaction expires.
func (t *Trade) pollTransaction(log bitwrk.Logger, abort <-chan bool) {
	defer func() {
		log.Printf("Transaction polling has stopped")
	}()

	for count := 1; ; count++ {
		select {
		case <-abort:
			log.Printf("Aborting transaction polling while transaction active")
			return
		default:
		}

		if tx, etag, err := protocol.FetchTx(t.txId, ""); err != nil {
			log.Printf("Error polling transaction: %v", err)
		} else if etag != t.txETag {
			t.condition.L.Lock()
			t.tx = tx
			t.txETag = etag
			expired := t.tx.State != bitwrk.StateActive
			t.condition.Broadcast()
			t.condition.L.Unlock()
			log.Printf("Tx change detected: phase=%v, expired=%v", t.tx.Phase, expired)
			if expired {
				break
			}
		}

		// Sleep for gradually longer durations
		time.Sleep(time.Duration(count) * 500 * time.Millisecond)
	}

	log.Printf("Transaction has expired.")
	// This is necessary so that the surrounding function call doesn't deadlock
	<-abort
}

// Implement Activity
func (t *Trade) GetKey() ActivityKey {
	return t.key
}

func (t *Trade) GetState() *ActivityState {
	t.condition.L.Lock()
	defer t.condition.L.Unlock()

	info := ""
	if t.lastError != nil {
		info = t.lastError.Error()
	}

	phase := ""
	if t.tx != nil {
		phase = t.tx.Phase.String()
	} else if t.bid != nil {
		phase = t.bid.State.String()
	} else if t.localMatch != nil {
		phase = fmt.Sprintf("LOCAL (#%v)", t.localMatch.GetKey())
	}

	price := t.price
	if t.tx != nil {
		price = t.tx.Price
	}

	result := &ActivityState{
		Type:     t.bidType.String(),
		Article:  t.article,
		Alive:    t.alive,
		Accepted: !t.awaitingClearance && !t.clearanceDenied,
		Rejected: t.clearanceDenied,
		Amount:   price,
		BidId:    t.bidId,
		TxId:     t.txId,
		Info:     info,
		Phase:    phase,

		BytesToTransfer:  t.bytesToTransfer,
		BytesTransferred: t.bytesTransferred,
	}

	if t.workFile != nil {
		result.BytesTotal = uint64(t.workFile.Size())
	}

	return result
}

func (t *Trade) Permit(identity *bitcoin.KeyPair, price money.Money) bool {
	t.condition.L.Lock()
	defer t.condition.L.Unlock()
	if !t.awaitingClearance {
		return false
	}
	t.identity = identity
	t.price = price
	t.clearedForTrade = true
	t.awaitingClearance = false
	t.condition.Broadcast()
	return true
}

func (t *Trade) Forbid() bool {
	t.condition.L.Lock()
	defer t.condition.L.Unlock()
	if !t.awaitingClearance {
		return false
	}
	t.clearanceDenied = true
	t.awaitingClearance = false
	t.condition.Broadcast()
	return true
}

func (t *Trade) GetTrade() *Trade {
	return t
}

func (t *Trade) Dispose() {
	t.manager.unregister(t.GetKey())
	files := []cafs.File{
		t.workFile,
		t.resultFile,
		t.encResultFile,
	}
	for _, f := range files {
		if f != nil {
			f.Dispose()
			f = nil
		}
	}
}
