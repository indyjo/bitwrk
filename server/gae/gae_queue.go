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
	"fmt"
	"hash/crc32"
	"net/url"
	"strings"
	"time"

	"github.com/indyjo/bitwrk-common/bitwrk"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
)

// Adds a task to the queue associated with an article/currency combination.
// matchKey: The article/currency combination for which to enqueue the task.
// tag:    The type of task, such as "retire-tx". Will cause the URL to be
//         "/_ah/queue/retire-tx".
// key:    The identifying key for the task (must be unique). The tasks's name
//         is set to "tag-key".
// eta:    The desired time of execution for the task, or zero if the task should execute
//         instantly.
// values: The data to send to the task handler (via POST).
func addTaskForArticle(c context.Context,
	matchKey string,
	tag string, key string,
	eta time.Time,
	delay time.Duration,
	values url.Values) (err error) {

	task := taskqueue.NewPOSTTask("/_ah/queue/"+tag, values)
	task.ETA = eta
	task.Delay = delay
	//task.Name = fmt.Sprintf("%v-%v", tag, key)
	queue := getQueue(matchKey)
	newTask, err := taskqueue.Add(c, task, queue)
	if err == nil {
		log.Infof(c, "[Queue %v] Scheduled: '%v' at %v", queue, newTask.Name, newTask.ETA)
	} else {
		log.Errorf(c, "[Queue %v] Error scheduling '%v' at %v: %v", queue, task.Name, task.ETA, err)
	}
	return
}

func addApplyChangesTask(c context.Context, matchKey string, matched time.Time, matchedBids []string, placedBids []string) error {
	matchedBidKeysString := strings.Join(matchedBids, " ")
	placedBidKeysString := strings.Join(placedBids, " ")
	log.Infof(c, "Scheduling for PLACED: %v", placedBidKeysString)
	log.Infof(c, "Scheduling for MATCHED: %v", matchedBidKeysString)
	return addTaskForArticle(c, matchKey, "apply-changes", "", time.Time{}, time.Duration(0),
		url.Values{"matched": {matchedBidKeysString}, "placed": {placedBidKeysString}, "timestamp": {matched.Format(time.RFC3339Nano)}})
}

func addRetireTransactionTask(c context.Context, txKey string, tx *bitwrk.Transaction) error {
	taskKey := fmt.Sprintf("%v-%v", txKey, tx.Phase)
	return addTaskForArticle(c, tx.MatchKey(), "retire-tx", taskKey, tx.Timeout, time.Duration(0),
		url.Values{"tx": {txKey}})
}

func addRetireBidTask(c context.Context, bidKey string, bid *bitwrk.Bid) error {
	return addTaskForArticle(c, bid.MatchKey(), "retire-bid", bidKey, bid.Expires, time.Duration(0),
		url.Values{"bid": {bidKey}})
}

// Function getQueue returns the name of a work queue for the given matchKey.
// This helps balancing the load onto up to 8 queues.
func getQueue(matchKey string) string {
	h := crc32.NewIEEE()
	h.Write([]byte(matchKey))
	return fmt.Sprintf("worker-%v", h.Sum32()%8)
}
