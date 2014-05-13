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

package gae

import (
	"appengine"
	"appengine/datastore"
	. "bitwrk"
	"bitwrk/money"
	"fmt"
	"time"
)

func ArticleKey(c appengine.Context, articleId ArticleId) *datastore.Key {
	return datastore.NewKey(c, "ArticleEntity", "a_"+string(articleId), 0, nil)
}

//func AccountingKey(c appengine.Context) *datastore.Key {
//	return datastore.NewKey(c, "Accounting", "singleton", 0, nil)
//}

func AccountKey(c appengine.Context, participant string) *datastore.Key {
	//	return datastore.NewKey(c, "Account", participant, 0, AccountingKey(c))
	return datastore.NewKey(c, "Account", participant, 0, nil)
}

func BidKey(c appengine.Context, bidId string) (key *datastore.Key, err error) {
	key, err = datastore.DecodeKey(bidId)
	return
}

// While in state "Placed", bid's have a corresponding entry in the
// so-called "hot" zone, which allows for better transactional locality.
//
// Each trade article has exactly one hot zone.
//
// Only those informations necessary for matching and expiration are
// held in a HotBid. When matched or expired, the HotBid is deleted from
// the hot zone.
type hotBid struct {
	BidKey  *datastore.Key
	Type    BidType
	Price   money.Money
	Expires time.Time
}

func newHotBid(key *datastore.Key, bid *Bid) *hotBid {
	return &hotBid{
		BidKey:  key,
		Type:    bid.Type,
		Price:   bid.Price,
		Expires: bid.Expires}
}

func GetBid(c appengine.Context, bidId string) (bid *Bid, err error) {
	key, err := datastore.DecodeKey(bidId)
	if err != nil {
		return
	}
	bid = new(Bid)
	err = datastore.Get(c, key, bidCodec{bid})
	return
}

var ErrLimitReached = fmt.Errorf("Limit of objects reached")
var ErrElementsSkipped = fmt.Errorf("Some elements were skipped")
var ErrTransactionTooYoung = fmt.Errorf("Transcation is too young to be retired")
var ErrTransactionAlreadyRetired = fmt.Errorf("Transaction has already been retired")

// Transactional function to enqueue a bid, while keeping accounts in balance
func EnqueueBid(c appengine.Context, bid *Bid) (*datastore.Key, error) {
	var bidKey *datastore.Key
	f := func(c appengine.Context) error {
		dao := NewGaeAccountingDao(c)

		if err := bid.CheckBalance(dao); err != nil {
			return err
		}

		//parentKey := ArticleKey(c, bid.Article)
		//parentKey := AccountKey(c, bid.Participant)
		if key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "Bid", nil), bidCodec{bid}); err != nil {
			return err
		} else {
			bidKey = key
		}

		if err := bid.Book(dao); err != nil {
			return err
		}

		if err := addPlaceBidTask(c, bidKey.Encode(), bid); err != nil {
			return err
		}
		if err := addRetireBidTask(c, bidKey.Encode(), bid); err != nil {
			return err
		}

		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		return nil, err
	}
	
	// Attempt to match the new bid right away. If unsuccessful, that's no problem. A task has already
	// been entered into the task queue that will repeat the operation.
	time.Sleep(250 * time.Millisecond)
	if txKey, err := TryMatchBid(c, bidKey); err != nil {
		c.Warningf("Opportunistic matching failed: %v", err)
	} else if txKey == nil {
		c.Infof("Opportunistic matching yields no match")
	} else {
		c.Infof("Opportunistic matching yields txId %v", txKey.Encode())
	}
	
	return bidKey, nil
}

// This will reimburse the bid's price and fee to the buyer.
func RetireBid(c appengine.Context, key *datastore.Key) error {
	f := func(c appengine.Context) error {
		now := time.Now()
		dao := NewGaeAccountingDao(c)
		var bid Bid
		if err := datastore.Get(c, key, bidCodec{&bid}); err != nil {
			return err
		}

		if bid.State == Matched {
			c.Infof("Not retiring matched bid %v", key)
			return nil
		}

		// Delete any associated "hot" bids
		if bid.State == Placed {
			query := datastore.NewQuery("HotBid").KeysOnly()
			query = query.Ancestor(ArticleKey(c, bid.Article))
			query = query.Filter("BidKey=", key)
			iter := query.Run(c)
			for {
				if hotKey, err := iter.Next(nil); err == datastore.Done {
					break
				} else if err != nil {
					return err
				} else {
					if err := datastore.Delete(c, hotKey); err != nil {
						return err
					}
				}
			}
		}

		if err := bid.Retire(dao, now); err != nil {
			return err
		}

		if _, err := datastore.Put(c, key, bidCodec{&bid}); err != nil {
			return err
		}

		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		return err
	}

	return nil
}

// Transactions in phase FINISHED will cause the price to be credited on the seller's
// account, and the fee to be deducted.
// All other phases will lead to price and fee being reimbursed to the buyer.
// Returns ErrTransactionTooYoung if the transaction has not passed its timout at the
// time of the call.
// Returns ErrTransactionAlreadyRetired if the transaction has already been retired at
// the time of the call.
func RetireTransaction(c appengine.Context, key *datastore.Key) error {
	f := func(c appengine.Context) error {
		now := time.Now()
		dao := NewGaeAccountingDao(c)
		var tx Transaction
		if err := datastore.Get(c, key, txCodec{&tx}); err != nil {
			return err
		}

		if err := tx.Retire(dao, now); err == ErrTooYoung {
			return ErrTransactionTooYoung
		} else if err == ErrAlreadyRetired {
			return ErrTransactionAlreadyRetired
		} else if err != nil {
			return err
		}

		if _, err := datastore.Put(c, key, txCodec{&tx}); err != nil {
			return err
		}

		return dao.Flush()
	}

	return datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true})
}

// Transactional function called by queue handler.
// Queries for matching "hot" bids.
// When a matching bid exists, creates the corresponding
// transaction and marks the bid as MATCHED. Otherwise, the
// bid is marked as PLACED, waiting for other bids to match it.
func TryMatchBid(c appengine.Context, bidKey *datastore.Key) (*datastore.Key, error) {
	var txKey *datastore.Key

	f := func(c appengine.Context) error {
		txKey = nil
		dao := NewGaeAccountingDao(c)

		now := time.Now()
		bid := new(Bid)
		if err := datastore.Get(c, bidKey, bidCodec{bid}); err != nil {
			return err
		}
		if bid.State != InQueue {
			// Nothing to do anymore! Can happen under some circumstances.
			c.Warningf("Bid %v already matched.", bidKey.Encode())
			return nil
		}
		query := datastore.NewQuery("HotBid")
		query = query.Ancestor(ArticleKey(c, bid.Article))
		if bid.Type == Buy {
			query = query.Filter("Type=", Sell).Filter("Price<=", bid.Price.Amount).Order("Price")
		} else {
			query = query.Filter("Type=", Buy).Filter("Price>=", bid.Price.Amount).Order("-Price")
		}

		if otherKey, otherBid, err := findValidBid(c, query.Run(c), now); err != nil {
			// Error searching for matching partner
			return err
		} else if otherKey == nil {
			// No other bid found. Mark bid as placed and put a hot bid into the datastore.
			bid.State = Placed
			hot := newHotBid(bidKey, bid)
			if _, err := datastore.Put(c, datastore.NewIncompleteKey(c, "HotBid", ArticleKey(c, bid.Article)), hotBidCodec{hot}); err != nil {
				return err
			}
		} else {
			tx := NewTransaction(now, bidKey.Encode(), otherKey.Encode(), bid, otherBid)
			if key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "Tx", ArticleKey(c, bid.Article)), txCodec{tx}); err != nil {
				// Error writing transaction
				return err
			} else {
				txKey = key
				txKeyEncoded := txKey.Encode()

				otherBid.Transaction = &txKeyEncoded
				if _, err := datastore.Put(c, otherKey, bidCodec{otherBid}); err != nil {
					// Error writing other bid
					return err
				}

				if err := addRetireTransactionTask(c, txKeyEncoded, tx); err != nil {
					return err
				}

				bid.Transaction = &txKeyEncoded
			}

			var buyerBid *Bid
			if bid.Type == Buy {
				buyerBid = bid
			} else {
				buyerBid = otherBid
			}

			if err := tx.Book(dao, buyerBid); err != nil {
				return err
			}
		}

		if _, err := datastore.Put(c, bidKey, bidCodec{bid}); err != nil {
			// Writing back bid failed
			return err
		}

		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		// Transaction failed
		return nil, err
	}

	return txKey, nil
}

// Finds the first matching hot bid in the list, and deletes it. Then, returns
// the corresponding "real" bid.
func findValidBid(c appengine.Context, iter *datastore.Iterator, now time.Time) (*datastore.Key, *Bid, error) {
	for {
		var hot hotBid
		key, e := iter.Next(hotBidCodec{&hot})
		if e == datastore.Done {
			break
		} else if e != nil {
			// Error case
			return nil, nil, e
		}

		if now.Before(hot.Expires) {
			if err := datastore.Delete(c, key); err != nil {
				return nil, nil, err
			}

			var bid Bid
			if err := datastore.Get(c, hot.BidKey, bidCodec{&bid}); err != nil {
				return nil, nil, err
			}
			return hot.BidKey, &bid, nil
		}
	}
	return nil, nil, nil
}

func GetTransaction(c appengine.Context, key *datastore.Key) (*Transaction, error) {
	var tx Transaction
	if err := datastore.Get(c, key, txCodec{&tx}); err != nil {
		return nil, err
	}

	return &tx, nil
}

func GetTransactionMessages(c appengine.Context, key *datastore.Key) ([]Tmessage, error) {
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
func UpdateTransaction(c appengine.Context, txKey *datastore.Key,
	now time.Time,
	address string,
	values map[string]string,
	document, signature string) error {

	f := func(c appengine.Context) error {
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

		if _, err := datastore.Put(c, txKey, txCodec{tx}); err != nil {
			return err
		}

		return addRetireTransactionTask(c, txKey.Encode(), tx)
	}

	return datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true})
}
