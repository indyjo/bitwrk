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
	"bitwrk/cafs"
	"errors"
	"log"
	"sync"
	"time"
)

var ErrNoMandate = errors.New("Mandate request rejected")
var ErrBidExpired = errors.New("Bid expired without match")
var ErrTxExpired = errors.New("Transaction no longer active")
var ErrTxUnexpectedState = errors.New("Transaction in unexpected state")

type ActivityKey int64

type Activity interface {
	//Started() time.Time
	//LastUpdate() time.Time
	//Value() money.Money
	Act() bool
}

type ActivityManager struct {
	mutex           *sync.Mutex
	activities      map[ActivityKey]Activity
	nextKey         ActivityKey
	workChan        *chan ActivityKey
	storage         cafs.FileStorage
	mandateRequests chan MandateRequest
}

var activityManager = ActivityManager{
	new(sync.Mutex),
	make(map[ActivityKey]Activity),
	1,
	nil,
	cafs.NewRamStorage(),
	make(chan MandateRequest),
}

func GetActivityManager() *ActivityManager {
	return &activityManager
}

func (m *ActivityManager) GetStorage() cafs.FileStorage {
	return m.storage
}

func (m *ActivityManager) GetMandateRequests() <-chan MandateRequest {
	return m.mandateRequests
}

func (m *ActivityManager) newKey() ActivityKey {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := m.nextKey
	m.nextKey++
	return result
}

func (m *ActivityManager) add(key ActivityKey, activity Activity) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.activities[key] = activity
	if m.workChan == nil {
		ch := make(chan ActivityKey, 1024)
		m.workChan = &ch
		go func() {
			m.work(ch)
		}()
	}
	*m.workChan <- key
}

// Marks an activity as done and removes it from the list.
// Additionally, terminates the worker goroutine if the
// list of activities is now empty.
func (m *ActivityManager) done(key ActivityKey) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	wasEmpty := len(m.activities) == 0
	delete(m.activities, key)
	isEmpty := len(m.activities) == 0
	if isEmpty && !wasEmpty {
		close(*m.workChan)
		m.workChan = nil
	}
}

func (m *ActivityManager) work(channel chan ActivityKey) {
	log.Printf("Started working")
	defer func() {
		log.Printf("Stopped working")
	}()

	for {
		if key, ok := <-channel; !ok {
			// channel closed
			m.mutex.Lock()
			m.workChan = nil
			m.mutex.Unlock()
			break
		} else {
			m.mutex.Lock()
			activity := m.activities[key]
			m.mutex.Unlock()
			if repeat := activity.Act(); repeat {
				go func() {
					time.Sleep(2 * time.Second)
					channel <- key
				}()
			} else {
				m.done(key)
			}
		}
	}
}
