//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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
	"crypto/rand"
	"errors"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/money"
	"github.com/indyjo/cafs"
	"github.com/indyjo/cafs/ram"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"
)

var ErrInterrupted = errors.New("The request was interrupted")
var ErrNoPermission = errors.New("Permission request rejected")
var ErrBidExpired = errors.New("Bid expired without match")
var ErrTxExpired = errors.New("Transaction no longer active")
var ErrTxUnexpectedState = errors.New("Transaction in unexpected state")

// Activity keys identify activities (trades), as well as mandates within
// the activity manager. Name spaces may overlap.
type ActivityKey int64

type Activity interface {
	GetKey() ActivityKey
	GetState() *ActivityState

	// Publish the activity.
	// Returns true if the call caused the activity to be published.
	Publish(identity *bitcoin.KeyPair, price money.Money) bool

	// Forbid the activity.
	// Returns true if the call caused the activity to be rejected.
	Forbid() bool

	// If the activity is a trade, returns the trade information
	GetTrade() *Trade
}

type ActivityState struct {
	Type        string
	Article     bitwrk.ArticleId
	Alive       bool // Whether the activity is still alive
	Accepted    bool // true iff the activity can no longer be published
	Rejected    bool
	Amount      money.Money
	BidId, TxId string
	Info        string
	Phase       string // The phase the activity's active object is in

	// Information about a transmission in progress
	BytesTotal       int64
	BytesToTransfer  int64
	BytesTransferred int64
}

type ActivityManager struct {
	logger     bitwrk.Logger
	mutex      *sync.Mutex
	activities map[ActivityKey]Activity
	mandates   map[ActivityKey]*Mandate
	history    []Activity
	nextKey    ActivityKey
	storage    cafs.FileStorage
	bidTokens  map[string]chan bool
}

var activityManager = ActivityManager{
	bitwrk.Root().New("ActivityManager"),
	new(sync.Mutex),
	make(map[ActivityKey]Activity),
	make(map[ActivityKey]*Mandate),
	make([]Activity, 0, 5), //history
	1,
	ram.NewRamStorage(512 * 1024 * 1024), // 512 MByte
	make(map[string]chan bool),
}

func GetActivityManager() *ActivityManager {
	return &activityManager
}

func (m *ActivityManager) NewBuy(article bitwrk.ArticleId) (*BuyActivity, error) {
	now := time.Now()
	result := &BuyActivity{
		Trade: Trade{
			condition:         sync.NewCond(new(sync.Mutex)),
			manager:           m,
			key:               m.NewKey(),
			started:           now,
			lastUpdate:        now,
			bidType:           bitwrk.Buy,
			article:           article,
			alive:             true,
			awaitingClearance: true,
		},
	}
	m.register(result.key, result)
	return result, nil
}

func (m *ActivityManager) NewSell(worker Worker, localOnly bool) (*SellActivity, error) {
	now := time.Now()

	result := &SellActivity{
		Trade: Trade{
			condition:         sync.NewCond(new(sync.Mutex)),
			manager:           m,
			key:               m.NewKey(),
			started:           now,
			lastUpdate:        now,
			bidType:           bitwrk.Sell,
			article:           worker.GetWorkerState().Info.Article,
			encResultKey:      new(bitwrk.Tkey),
			alive:             true,
			awaitingClearance: true,
			localOnly:         localOnly,
		},
		worker: worker,
	}

	// Get a random key for encrypting the result
	if _, err := io.ReadFull(rand.Reader, result.encResultKey[:]); err != nil {
		return nil, err
	}

	m.register(result.key, result)
	return result, nil
}

func (m *ActivityManager) GetStorage() cafs.FileStorage {
	return m.storage
}

func (m *ActivityManager) GetActivities() []Activity {
	result := make([]Activity, 0, 8)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, a := range m.activities {
		result = append(result, a)
	}
	result = append(result, m.history...)

	return result
}

func (m *ActivityManager) GetMandates() map[ActivityKey]*Mandate {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make(map[ActivityKey]*Mandate)
	for k, v := range m.mandates {
		result[k] = v
	}
	return result
}

func (k *ActivityKey) Parse(s string) error {
	if v, err := strconv.ParseInt(s, 10, 64); err != nil {
		return err
	} else {
		*k = ActivityKey(v)
		return nil
	}
}

// Returns the activity associated with the given key, or nil.
func (m *ActivityManager) GetActivityByKey(k ActivityKey) Activity {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if a, ok := m.activities[k]; ok {
		return a
	} else {
		return nil
	}
}

type sorted []Activity

func (s sorted) Len() int {
	return len(s)
}

func (s sorted) Less(i, j int) bool {
	return s[i].GetKey() < s[j].GetKey()
}

func (s sorted) Swap(i, j int) {
	s[j], s[i] = s[i], s[j]
}

func (m *ActivityManager) GetActivitiesSorted() []Activity {
	result := m.GetActivities()
	sort.Sort(sorted(result))
	return result
}

func (m *ActivityManager) NewKey() ActivityKey {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := m.nextKey
	m.nextKey++
	return result
}

func (m *ActivityManager) register(key ActivityKey, activity Activity) {
	now := time.Now()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.activities[key] = activity

	trade := activity.GetTrade()
	if trade == nil {
		return // Only trades are activities we care about at the moment
	}

	// First try to find a local match
	for key2, activity2 := range m.activities {
		if key2 == key {
			continue // can't match activity with itself
		}
		// can only match with activities of same article and opposite type (buy/sell)
		trade2 := activity2.GetTrade()
		if trade2 == nil || trade2.article != trade.article || trade2.bidType == trade.bidType {
			continue
		}
		// other activity could be a valid local match
		matched := false
		trade2.execSync(func() {
			if trade2.alive && trade2.awaitingClearance {
				trade.localMatch = trade2
				trade2.localMatch = trade
				trade.awaitingClearance = false
				trade2.awaitingClearance = false
				matched = true
				m.logger.Printf("Local match: #%v (new) - #%v (old)", key, key2)
			}
		})

		if matched {
			return // Early exit when a match was found
		}
	}

	// Try to apply all known mandates to the new activity, until a matching mandate
	// was applied successfully.
	for mandateKey, mandate := range m.mandates {
		applied := mandate.Apply(activity, now)
		if mandate.Expired() {
			delete(m.mandates, mandateKey)
		}
		if applied {
			break
		}
	}
}

func (m *ActivityManager) unregister(key ActivityKey) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	activity, ok := m.activities[key]
	if !ok {
		return
	}

	// Append to history
	if len(m.history) == 100 {
		copy(m.history[:len(m.history)-1], m.history[1:])
		m.history = m.history[:len(m.history)-1]
	}
	m.history = append(m.history, activity)
	delete(m.activities, key)
}

// Registers the mandate (using an activity key for identification)
func (m *ActivityManager) RegisterMandate(key ActivityKey, mandate *Mandate) {
	m.mutex.Lock()
	m.mandates[key] = mandate
	m.mutex.Unlock()
	m.applyMandate(m.GetActivities(), mandate, key)
}

func (m *ActivityManager) UnregisterMandate(key ActivityKey) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.mandates, key)
}

func (m *ActivityManager) applyMandate(activities []Activity, mandate *Mandate, mandateKey ActivityKey) {
	now := time.Now()
	// Iterate over activities and try to apply the mandate to each.
	// If the mandate expires, remove it.
	for _, a := range activities {
		state := a.GetState()
		if !state.Accepted {
			mandate.Apply(a, now)
			if mandate.Expired() {
				m.UnregisterMandate(mandateKey)
			}
		}
	}
}

// Consume a limited resource. The resource is named by the key parameter and limited to up
// to 'limit' checked out tokens.
func (m *ActivityManager) checkoutToken(key string, limit int, interrupt <-chan bool) error {
	m.mutex.Lock()
	tokenChan := m.bidTokens[key]
	if tokenChan == nil {
		// Initialize tokens if not done so already
		tokenChan = make(chan bool, limit)
		m.bidTokens[key] = tokenChan
		for i := 0; i < limit; i++ {
			tokenChan <- true
		}
	}
	m.mutex.Unlock()
	select {
	case <-interrupt:
		return ErrInterrupted
	case <-tokenChan:
		return nil
	}
}

func (m *ActivityManager) returnToken(key string) {
	m.mutex.Lock()
	tokenChan := m.bidTokens[key]
	m.mutex.Unlock()
	tokenChan <- true
}
