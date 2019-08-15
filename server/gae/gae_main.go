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

package gae

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/indyjo/bitwrk-common/bitwrk"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/taskqueue"
)

//func AccountingKey(c context.Context) *datastore.Key {
//	return datastore.NewKey(c, "Accounting", "singleton", 0, nil)
//}

func AccountKey(c context.Context, participant string) *datastore.Key {
	//	return datastore.NewKey(c, "Account", participant, 0, AccountingKey(c))
	return datastore.NewKey(c, "Account", participant, 0, nil)
}

func DepositKey(c context.Context, uid string) *datastore.Key {
	return datastore.NewKey(c, "Deposit", uid, 0, nil)
}

func DepositUid(key *datastore.Key) string {
	return key.StringID()
}

func GetBid(c context.Context, bidId string) (bid *Bid, err error) {
	key, err := datastore.DecodeKey(bidId)
	if err != nil {
		return
	}
	bid = new(Bid)
	err = datastore.Get(c, key, bidCodec{bid})
	return
}

var ErrTransactionTooYoung = fmt.Errorf("Transaction is too young to be retired")
var ErrTransactionAlreadyRetired = fmt.Errorf("Transaction has already been retired")

// Transactional function to enqueue a bid, while keeping accounts in balance
func EnqueueBid(c context.Context, bid *Bid) (*datastore.Key, error) {
	var bidKey *datastore.Key
	f := func(c context.Context) error {
		dao := NewGaeAccountingDao(c, true)

		if err := bid.CheckBalance(dao); err != nil {
			return err
		}

		//parentKey := ArticleKey(c, bid.Article)
		//parentKey := AccountKey(c, bid.Participant)
		if key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "Bid", nil),
			datastore.PropertyLoadSaver(bidCodec{bid})); err != nil {
			return err
		} else {
			bidKey = key
		}

		if err := bid.Book(dao, bidKey.Encode()); err != nil {
			return err
		}

		if err := addRetireBidTask(c, bidKey.Encode(), bid); err != nil {
			return err
		}

		// Encode the new bid as a hotBid and put it into a pull queue
		hot := newHotBid(bidKey, bid)
		if bytes, err := json.Marshal(*hot); err != nil {
			return err
		} else {
			var task taskqueue.Task
			task.Method = "PULL"
			task.Payload = bytes
			task.Tag = bid.MatchKey()
			if _, err := taskqueue.Add(c, &task, "hotbids"); err != nil {
				return err
			}
		}

		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		return nil, err
	}

	return bidKey, nil
}

// Function TriggerBatchProcessing performs the actual matching process specific to a matchKey.
// A semaphore placed in memcache ensures that only one matching process is active at any time,
// per matchKey.
func TriggerBatchProcessing(c context.Context, matchKey string) error {
	// Instead of submitting a task to match incoming bids, resulting in one task per bid,
	// we collect bids for up to two seconds and batch-process them afterwards.
	semaphoreKey := "semaphore-" + matchKey
	if semaphore, err := memcache.Increment(c, semaphoreKey, 1, 0); err != nil {
		return err
	} else if semaphore >= 2 {
		log.Infof(c, "%v batch processing tasks currently active for %v. Nothing to do.", semaphore, matchKey)
		memcache.IncrementExisting(c, semaphoreKey, -1)
		return nil
	} else {
		log.Infof(c, "Waiting one second for batch processing")
		time.Sleep(1 * time.Second)
		log.Infof(c, "Starting batch processing...")
		memcache.IncrementExisting(c, semaphoreKey, -1)
		time_before := time.Now()
		matchingErr := MatchIncomingBids(c, matchKey)
		time_after := time.Now()
		duration := time_after.Sub(time_before)
		if duration > 1000*time.Millisecond {
			log.Errorf(c, "Batch processing finished after %v. Limit exceeded!", duration)
		} else if duration > 500*time.Millisecond {
			log.Warningf(c, "Batch processing finished after %v. Limit in danger.", duration)
		} else {
			log.Infof(c, "Batch processing finished after %v.", duration)
		}
		return matchingErr
	}
}

// This will reimburse the bid's price and fee to the buyer.
func RetireBid(c context.Context, key *datastore.Key) error {
	f := func(c context.Context) error {
		now := time.Now()
		dao := NewGaeAccountingDao(c, true)
		var bid Bid
		if err := datastore.Get(c, key, bidCodec{&bid}); err != nil {
			return err
		}

		if bid.State == Matched {
			log.Infof(c, "Not retiring matched bid %v", key)
			return nil
		}

		if err := bid.Retire(dao, key.Encode(), now); err != nil {
			return err
		}

		if _, err := datastore.Put(c, key, datastore.PropertyLoadSaver(bidCodec{&bid})); err != nil {
			return err
		}

		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		return err
	}

	return nil
}

// Marks a bid as placed. This is purely informational for the user.
func PlaceBid(c context.Context, bidId string) error {
	var key *datastore.Key
	if k, err := datastore.DecodeKey(bidId); err != nil {
		return err
	} else {
		key = k
	}

	f := func(c context.Context) error {
		var bid Bid
		if err := datastore.Get(c, key, datastore.PropertyLoadSaver(bidCodec{&bid})); err != nil {
			return err
		}

		if bid.State != InQueue {
			log.Infof(c, "Not placing bid %v : State=%v", key, bid.State)
			return nil
		}

		bid.State = Placed

		if _, err := datastore.Put(c, key, datastore.PropertyLoadSaver(bidCodec{&bid})); err != nil {
			return err
		}
		return nil
	}

	return datastore.RunInTransaction(c, f, nil)
}

// Transactions in phase FINISHED will cause the price to be credited on the seller's
// account, and the fee to be deducted.
// All other phases will lead to price and fee being reimbursed to the buyer.
// Returns ErrTransactionTooYoung if the transaction has not passed its timout at the
// time of the call.
// Returns ErrTransactionAlreadyRetired if the transaction has already been retired at
// the time of the call.
func RetireTransaction(c context.Context, key *datastore.Key) error {
	f := func(c context.Context) error {
		now := time.Now()
		dao := NewGaeAccountingDao(c, true)
		var tx Transaction
		if err := datastore.Get(c, key, txCodec{&tx}); err != nil {
			return err
		}

		if err := tx.Retire(dao, key.Encode(), now); err == ErrTooYoung {
			return ErrTransactionTooYoung
		} else if err == ErrAlreadyRetired {
			return ErrTransactionAlreadyRetired
		} else if err != nil {
			return err
		}

		if _, err := datastore.Put(c, key, datastore.PropertyLoadSaver(txCodec{&tx})); err != nil {
			return err
		}

		return dao.Flush()
	}

	return datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true})
}

func GetTransaction(c context.Context, key *datastore.Key) (*Transaction, error) {
	var tx Transaction
	if err := datastore.Get(c, key, txCodec{&tx}); err != nil {
		return nil, err
	}

	return &tx, nil
}

func GetTransactionMessages(c context.Context, key *datastore.Key) ([]Tmessage, error) {
	query := datastore.NewQuery("Tmessage").Ancestor(key).Limit(101).Order("Received")
	messages := make([]Tmessage, 0, 101)
	if _, err := query.GetAll(c, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

// Sends a message (defined by its argument values) to the transaction and performs
// the corresponding changes atomically.
// Returns the updated transaction on success.
func UpdateTransaction(c context.Context, txKey *datastore.Key,
	now time.Time,
	address string,
	values map[string]string,
	document, signature string) error {

	f := func(c context.Context) error {
		tx, err := GetTransaction(c, txKey)
		if err != nil {
			return err
		}

		message := tx.SendMessage(now, address, values)

		if !message.Accepted {
			return fmt.Errorf("Message not accepted: %v", message.RejectMessage)
		}

		message.Received = now
		message.Document = document
		message.Signature = signature

		_, err = datastore.Put(c, datastore.NewIncompleteKey(c, "Tmessage", txKey), message)
		if err != nil {
			return err
		}

		if _, err := datastore.Put(c, txKey, datastore.PropertyLoadSaver(txCodec{tx})); err != nil {
			return err
		}

		return addRetireTransactionTask(c, txKey.Encode(), tx)
	}

	return datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true})
}
