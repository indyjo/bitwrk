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
	"errors"
	"io"
	"log"
	"sync"
	"time"
)

var ErrNoMandate = errors.New("Mandate request rejected")

type ActivityKey int64

type Activity interface {
	//Started() time.Time
	//LastUpdate() time.Time
	//Value() money.Money
	Act() bool
}

type BuyActivity struct {
	condition           *sync.Cond
	manager             *ActivityManager
	key                 ActivityKey
	started, lastUpdate time.Time
	gotRejection        bool
	gotMandate          bool
	price               money.Money
	secret              []byte
	article             string
	workTemporary       cafs.Temporary
	workFile            cafs.File
}

type ActivityManager struct {
	mutex           *sync.Mutex
	activities      map[ActivityKey]Activity
	nextKey         ActivityKey
	workChan        *chan ActivityKey
	storage         cafs.FileStorage
	mandateRequests <-chan MandateRequest
}

type MandateRequest interface {
	GetBid() bitwrk.RawBid
	Accept(identity bitcoin.KeyPair)
	Reject(reason string)
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

func (m *ActivityManager) NewBuy(article string) (*BuyActivity, error) {
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
	if !repeat {
		a.condition.L.Lock()
		a.gotMandate = true
		a.price = money.MustParse("BTC0.001337")
		a.condition.Broadcast()
		a.condition.L.Unlock()
	}
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
	// wait for mandate or rejection

	if a.awaitMandate() {
		log.Printf("Got mandate. Price: %v", a.price)
	} else {
		return nil, ErrNoMandate
	}

	return a.workFile, nil
}

func (a *BuyActivity) awaitMandate() bool {
	// wait for mandate or rejection
	a.condition.L.Lock()
	defer a.condition.L.Unlock()
	for !a.gotMandate && !a.gotRejection {
		log.Println("Waiting for mandate...")
		a.condition.Wait()
	}
	return a.gotMandate
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
