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
	"container/heap"
	"context"
	"encoding/json"
	"time"

	"github.com/indyjo/bitwrk/common/bitwrk"
	"github.com/indyjo/bitwrk/common/money"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
)

// While in state "Placed", bid's have a corresponding entry in the
// so-called "hot" zone, which allows for better transactional locality.
//
// Each article/currency combination has exactly one hot zone.
//
// Only those informations necessary for matching and expiration are
// held in a HotBid. When matched or expired, the HotBid is deleted from
// the hot zone.
type hotBid struct {
	BidKey  *datastore.Key
	Type    bitwrk.BidType
	Price   money.Money
	Expires time.Time
}

// Function hotZoneKey returns a datastore key for a specific hot zone.
// The key is used as ancestor key for all hotBids whose bids have the given matchKey.
func hotZoneKey(c context.Context, matchKey string) *datastore.Key {
	return datastore.NewKey(c, "ArticleEntity", "ac_"+matchKey, 0, nil)
}

func newHotBid(key *datastore.Key, bid *bitwrk.Bid) *hotBid {
	return &hotBid{
		BidKey:  key,
		Type:    bid.Type,
		Price:   bid.Price,
		Expires: bid.Expires}
}

func (this *hotBid) hotterThan(other *hotBid) bool {
	// If prices are equal, sort by expiry date (earliest expiry served first)
	if this.Price.Amount == other.Price.Amount {
		return this.Expires.Before(other.Expires)
	}
	if this.Type == bitwrk.Sell {
		// Order sells by ascending price (cheapest sell bid served first)
		return this.Price.Amount < other.Price.Amount
	} else {
		// Order buys by descending price (highest buy bid served first)
		return this.Price.Amount > other.Price.Amount
	}
}

type storedHotBid struct {
	hotBid hotBid
	key    *datastore.Key
}

// Type hotBidsHeap defines a sorted data structure of hot bids, ordered
// by price. Depending on whether bids are buys or sells, the list is ordered
// so that 'hottest' (i.e. most probably next in row to be matched) are first.
type hotBidsHeap []hotBid

func (h hotBidsHeap) Len() int { return len(h) }
func (h hotBidsHeap) Less(i, j int) bool {
	return h[i].hotterThan(&h[j])
}
func (h hotBidsHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *hotBidsHeap) Push(x interface{}) {
	*h = append(*h, x.(hotBid))
}
func (h *hotBidsHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
func (h hotBidsHeap) Get(i int) hotBid { return h[i] }

// Provides a unified view on a queue consisting both of datastore-persisted hot bids,
// as well as ephemeral bids stored in memory.
// Tries to minimize datastore access by initializing lazily. Buys and sells are
// treated as separate queues, but can be treated almost the same.
type hotBidsQueue struct {
	bidType    bitwrk.BidType
	context    context.Context
	query      *datastore.Query
	iter       *datastore.Iterator
	storedTip  *storedHotBid
	cachedHeap *hotBidsHeap
}

func newHotBidsQueue(c context.Context, query *datastore.Query, bidType bitwrk.BidType) *hotBidsQueue {
	return &hotBidsQueue{
		bidType:    bidType,
		context:    c,
		query:      query,
		iter:       nil,
		storedTip:  nil,
		cachedHeap: &hotBidsHeap{},
	}
}

func (q *hotBidsQueue) init() error {
	if q.iter == nil {
		q.iter = q.query.Run(q.context)
		return q.fetchNext()
	}
	return nil
}

func (q *hotBidsQueue) fetchNext() error {
	var hot hotBid
	if key, err := q.iter.Next(hotBidCodec{&hot}); err == datastore.Done {
		q.storedTip = nil
	} else if err != nil {
		return err
	} else {
		q.storedTip = &storedHotBid{
			key:    key,
			hotBid: hot,
		}
	}
	return nil
}

func (q *hotBidsQueue) withTip(handleStoredTip, handleCachedTip func()) {
	q.init()
	if q.storedTip == nil {
		if q.cachedHeap.Len() != 0 {
			handleCachedTip()
		}
	} else if q.cachedHeap.Len() == 0 {
		if q.storedTip != nil {
			handleStoredTip()
		}
	} else {
		cached := q.cachedHeap.Get(0)
		if q.storedTip.hotBid.hotterThan(&cached) {
			handleStoredTip()
		} else {
			handleCachedTip()
		}
	}
}

// Returns the current tip of the queue, i.e. the 'hottest' bid.
func (q *hotBidsQueue) Tip() (*hotBid, error) {
	if err := q.init(); err != nil {
		return nil, err
	}
	var result hotBid
	found := false
	q.withTip(
		func() { result = q.storedTip.hotBid; found = true },
		func() { result = q.cachedHeap.Get(0); found = true })
	if found {
		return &result, nil
	} else {
		return nil, nil
	}
}

// Pops (removes) the tip of the queue, replacing it by the next-hottest bid.
func (q *hotBidsQueue) Pop() error {
	var err error
	handleStoredTip := func() {
		err = datastore.Delete(q.context, q.storedTip.key)
		if err == nil {
			err = q.fetchNext()
		}
	}
	handleCachedTip := func() {
		heap.Pop(q.cachedHeap)
	}

	q.withTip(handleStoredTip, handleCachedTip)
	return err
}

// Inserts a new hot bid into the heap of ephemeral bids.
func (q *hotBidsQueue) Insert(bid *hotBid) error {
	if err := q.init(); err != nil {
		return err
	}
	heap.Push(q.cachedHeap, *bid)
	return nil
}

// Writes cached entries to datastore
func (q *hotBidsQueue) Persist(parentKey *datastore.Key) error {
	if q.iter == nil {
		return nil // No need to do anything if we're note initialized
	}
	for _, hotBid := range *q.cachedHeap {
		key := datastore.NewIncompleteKey(q.context, "HotBid", parentKey)
		if _, err := datastore.Put(q.context, key, datastore.PropertyLoadSaver(hotBidCodec{&hotBid})); err != nil {
			return err
		}
	}
	return nil
}

func (q *hotBidsQueue) Flush() []string {
	result := make([]string, 0, 16)
	for q.cachedHeap.Len() > 0 {
		result = append(result, q.cachedHeap.Get(0).BidKey.Encode())
		heap.Pop(q.cachedHeap)
	}
	return result
}

func MatchIncomingBids(c context.Context, matchKey string) error {
	incomingBids := make([]hotBid, 0, 16)

	if tasks, err := taskqueue.LeaseByTag(c, 200, "hotbids", 20, matchKey); err != nil {
		return err
	} else {
		defer func() {
			if err := taskqueue.DeleteMulti(c, tasks, "hotbids"); err != nil {
				log.Errorf(c, "Couldn't delete from task queue: %v", err)
			}
		}()
		for index, task := range tasks {
			var hot hotBid
			if err := json.Unmarshal(task.Payload, &hot); err != nil {
				log.Errorf(c, "Couldn't unmarshal task #%v: %v", index, err)
			} else {
				incomingBids = append(incomingBids, hot)
			}
		}
	}

	f := func(c context.Context) error {
		now := time.Now()
		if err := matchIncomingBids(c, now, matchKey, incomingBids); err != nil {
			return err
		} else {
			return deleteExpiredHotBids(c, now, matchKey)
		}
	}

	return datastore.RunInTransaction(c, f, nil)
}

func deleteExpiredHotBids(c context.Context, now time.Time, matchKey string) error {
	parentKey := hotZoneKey(c, matchKey)
	hotBids := datastore.NewQuery("HotBid").Ancestor(parentKey)

	expiredIter := hotBids.Filter("Expires<=", now).KeysOnly().Run(c)
	expiredCount := 0
	for {
		if key, err := expiredIter.Next(nil); err == datastore.Done {
			break
		} else if err != nil {
			return err
		} else if err := datastore.Delete(c, key); err != nil {
			return err
		} else {
			expiredCount++
		}
	}
	log.Infof(c, "Deleted %v hot bids expired before %v", expiredCount, now)
	return nil
}

// Takes a list of hot bids, all belonging to the same article/currency, and tries to match them against
// existing bids, in sequence.
func matchIncomingBids(c context.Context, now time.Time, matchKey string, incomingBids []hotBid) error {
	log.Infof(c, "Matching hot bids [%v]: %v", matchKey, incomingBids)

	parentKey := hotZoneKey(c, matchKey)
	hotBids := datastore.NewQuery("HotBid").Ancestor(parentKey)

	hotBuys := newHotBidsQueue(c, hotBids.Filter("Type=", bitwrk.Buy).Order("-Price"), bitwrk.Buy)
	hotSells := newHotBidsQueue(c, hotBids.Filter("Type=", bitwrk.Sell).Order("Price"), bitwrk.Sell)

	matched := make([]string, 0, 16)

	for len(incomingBids) > 0 {
		bid := incomingBids[0]
		incomingBids = incomingBids[1:]

		var thisQueue, otherQueue *hotBidsQueue
		if bid.Type == bitwrk.Buy {
			thisQueue, otherQueue = hotBuys, hotSells
		} else {
			thisQueue, otherQueue = hotSells, hotBuys
		}

		// Pop bids from queue that have expired
		skipped := 0
		for {
			if other, err := otherQueue.Tip(); err != nil {
				return err
			} else if other == nil || other.Expires.After(now) {
				break
			} else {
				skipped++
				log.Infof(c, "Skipping hot bid %v", other)
				if err := otherQueue.Pop(); err != nil {
					return err
				}
			}
		}
		log.Infof(c, "Skipped %v expired bids", skipped)

		// See if we have a match
		if other, err := otherQueue.Tip(); err != nil {
			return err
		} else if other != nil && other.hotterThan(&bid) {
			// This is a match. Take other bid out of queue and schedule transaction creation
			if err := otherQueue.Pop(); err != nil {
				return err
			}

			matched = append(matched, bid.BidKey.Encode(), other.BidKey.Encode())
		} else {
			// No match. Store bid for later matching.
			if err := thisQueue.Insert(&bid); err != nil {
				return err
			}
		}
	}

	if err := hotBuys.Persist(parentKey); err != nil {
		return err
	}
	if err := hotSells.Persist(parentKey); err != nil {
		return err
	}

	placed := append(hotBuys.Flush(), hotSells.Flush()...)

	if len(matched) == 0 && len(placed) == 0 {
		return nil
	} else {
		return addApplyChangesTask(c, matchKey, now, matched, placed)
	}
}

// Given IDs of two bids, matches both in a transaction.
func MatchBids(c context.Context, matched time.Time, newBidId, oldBidId string) error {
	newKey, err := datastore.DecodeKey(newBidId)
	if err != nil {
		return err
	}
	oldKey, err := datastore.DecodeKey(oldBidId)
	if err != nil {
		return err
	}

	f := func(c context.Context) error {
		var newBid, oldBid bitwrk.Bid
		if err := datastore.Get(c, newKey, bidCodec{&newBid}); err != nil {
			return err
		}
		if err := datastore.Get(c, oldKey, bidCodec{&oldBid}); err != nil {
			return err
		}

		// Older bid may still be in state InQueue, due to asynchronicity
		if oldBid.State == bitwrk.InQueue {
			oldBid.State = bitwrk.Placed
		}

		// Also modifies newBid and oldBid
		var tx *bitwrk.Transaction
		if t, err := bitwrk.NewTransaction(matched, newBidId, oldBidId, &newBid, &oldBid); err != nil {
			return err
		} else {
			tx = t
		}

		if txKey, err := datastore.Put(c, datastore.NewIncompleteKey(c, "Tx", nil),
			datastore.PropertyLoadSaver(txCodec{tx})); err != nil {
			// Error writing transaction
			return err
		} else {
			txKeyEncoded := txKey.Encode()

			// Store both bids and schedule the transaction's retirement

			newBid.Transaction = &txKeyEncoded
			if _, err := datastore.Put(c, newKey, datastore.PropertyLoadSaver(bidCodec{&newBid})); err != nil {
				return err
			}

			oldBid.Transaction = &txKeyEncoded
			if _, err := datastore.Put(c, oldKey, datastore.PropertyLoadSaver(bidCodec{&oldBid})); err != nil {
				return err
			}

			if err := addRetireTransactionTask(c, txKeyEncoded, tx); err != nil {
				return err
			}

			var buyerBid *bitwrk.Bid
			if newBid.Type == bitwrk.Buy {
				buyerBid = &newBid
			} else {
				buyerBid = &oldBid
			}

			dao := NewGaeAccountingDao(c, true)
			if err := tx.Book(dao, txKey.Encode(), buyerBid); err != nil {
				return err
			}

			return dao.Flush()
		}
	}

	return datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true})
}
