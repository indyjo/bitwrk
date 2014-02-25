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
	"appengine/taskqueue"
	"bitwrk"
	"fmt"
	"hash/crc32"
	"net/url"
	"time"
)

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
	//task.Name = fmt.Sprintf("%v-%v", tag, key)
	queue := getQueue(string(article))
	newTask, err := taskqueue.Add(c, task, queue)
	if err == nil {
		c.Infof("[Queue %v] Scheduled: '%v' at %v", queue, newTask.Name, newTask.ETA)
	} else {
		c.Errorf("[Queue %v] Error scheduling '%v' at %v: %v", queue, task.Name, task.ETA, err)
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
