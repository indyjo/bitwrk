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
	"sort"
	"strconv"
	"sync"
	"time"
)

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

	// Permit the activity.
	// Returns true if the call caused the activity to be accepted.
	Permit(identity *bitcoin.KeyPair, price money.Money) bool

	// Forbid the activity.
	// Returns true if the call caused the activity to be rejected.
	Forbid() bool

	// If the activity is a trade, returns the trade information
	GetTrade() *Trade
}

type ActivityState struct {
	Type     string
	Article  bitwrk.ArticleId
	Accepted bool
	Amount   money.Money
	Info     string
}

type ActivityManager struct {
	mutex      *sync.Mutex
	activities map[ActivityKey]Activity
	mandates   map[ActivityKey]*Mandate
	nextKey    ActivityKey
	storage    cafs.FileStorage
}

var activityManager = ActivityManager{
	new(sync.Mutex),
	make(map[ActivityKey]Activity),
	make(map[ActivityKey]*Mandate),
	1,
	cafs.NewRamStorage(),
}

func GetActivityManager() *ActivityManager {
	return &activityManager
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
