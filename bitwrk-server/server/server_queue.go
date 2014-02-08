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

package server

import (
	"appengine"
	"appengine/datastore"
	"appengine/taskqueue"
	"bitwrk"
	db "bitwrk/gae"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/url"
	"time"
)

func mustDecodeKey(s string) *datastore.Key {
	if key, err := datastore.DecodeKey(s); err != nil {
		panic(err)
	} else {
		return key
	}
	return nil // never reached
}

func handlePlaceBid(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	bidKeyString := r.FormValue("bid")
	bidKey := mustDecodeKey(bidKeyString)
	c.Infof("Placing bid %v (%v)", bidKeyString, bidKey)
	if txKey, tx, err := db.TryMatchBid(c, bidKey); err != nil {
		c.Errorf("Error placing bid: %v", err)
		http.Error(w, "Error placing Bid", http.StatusInternalServerError)
	} else {
		c.Infof("Successfully placed!")
		if txKey != nil {
			c.Infof(" -> Transaction: %v", txKey.Encode())
			addRetireTransactionTask(c, txKey.Encode(), tx)
		}
	}
}

func handleRetireTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	keyString := r.FormValue("tx")
	key := mustDecodeKey(keyString)
	c.Infof("Retiring transaction %v (%v)", keyString, key)
	if err := db.RetireTransaction(c, key); err == db.ErrTransactionTooYoung {
		c.Infof("Transaction is too young to be retired")
	} else if err == db.ErrTransactionAlreadyRetired {
		c.Infof("Transaction has already been retired")
	} else if err != nil {
		c.Warningf("Error retiring transaction: %v", err)
		http.Error(w, "Error retiring transaction", http.StatusInternalServerError)
	}
}

func handleRetireBid(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	keyString := r.FormValue("bid")
	key := mustDecodeKey(keyString)
	c.Infof("Retiring bid %v (%v)", keyString, key)
	if err := db.RetireBid(c, key); err != nil {
		c.Warningf("Error retiring bid: %v", err)
		http.Error(w, "Error retiring bid", http.StatusInternalServerError)
	}
}

// Adds a task to the queue associated with an article.
// article: The article id for which to enqueue the task.
// tag:    The type of task, such as "retire-tx". Will cause the URL to be
//         "/_ah/queue/retire-tx".
// key:    The identifying key for the task (must be unique). The tasks's name
//         is set to "tag-key".
// eta:    The desired time of execution for the task, or zero if the task should execute
//         instantly.
// values: The data to send to the task handler (via POST).
func addTaskForArticle(c appengine.Context,
	article bitwrk.ArticleId,
	tag string, key string,
	eta time.Time,
	values url.Values) (err error) {

	task := taskqueue.NewPOSTTask("/_ah/queue/"+tag, values)
	task.ETA = eta
	task.Name = fmt.Sprintf("%v-%v", tag, key)
	queue := getQueue(string(article))
	newTask, err := taskqueue.Add(c, task, queue)
	if err == nil {
		c.Infof("[Queue %v] Scheduled: %v at %v", queue, newTask.Name, newTask.ETA)
	} else {
		c.Errorf("[Queue %v] Error scheduling %v at %v: %v", queue, task.Name, task.ETA, err)
	}
	return
}

func addRetireTransactionTask(c appengine.Context, txKey string, tx *bitwrk.Transaction) error {
	taskKey := fmt.Sprintf("%v-%v", txKey, tx.Phase)
	return addTaskForArticle(c, tx.Article, "retire-tx", taskKey, tx.Timeout,
		url.Values{"tx": {txKey}})
}

func addRetireBidTask(c appengine.Context, bidKey string, bid *bitwrk.Bid) error {
	return addTaskForArticle(c, bid.Article, "retire-bid", bidKey, bid.Expires,
		url.Values{"bid": {bidKey}})
}

func addPlaceBidTask(c appengine.Context, bidKey string, bid *bitwrk.Bid) error {
	return addTaskForArticle(c, bid.Article, "place-bid", bidKey, time.Time{},
		url.Values{"bid": {bidKey}})
}

func getQueue(article string) string {
	h := crc32.NewIEEE()
	h.Write([]byte(article))
	return fmt.Sprintf("worker-%v", h.Sum32()%8)
}
