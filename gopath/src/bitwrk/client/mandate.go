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
	"bitwrk/money"
	"sync"
	"time"
)

type Mandate struct {
	mutex         sync.Mutex
	expired       bool
	Identity      *bitcoin.KeyPair
	BidType       bitwrk.BidType // Buy or Sell
	Article       bitwrk.ArticleId
	Price         money.Money
	UseTradesLeft bool
	TradesLeft    int
	UseUntil      bool
	Until         time.Time
}

func (m *Mandate) Expired() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.expired
}

// Applies the mandate to the given activity and returns whether the
// mandate could be applied, i.e. mandate searching can be stopped
// for the trade at hand.
func (m *Mandate) Apply(activity Activity, now time.Time) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.expired {
		return false
	}

	// Currently, the generalization of trades into activities makes
	// no sense. Only look at the trade.
	t := activity.GetTrade()
	if t == nil {
		return false
	}

	if t.article != m.Article {
		return false
	}

	if t.bidType != m.BidType {
		return false
	}

	// If counter reaches zero -> expired
	if m.UseTradesLeft && m.TradesLeft <= 0 {
		m.expired = true
		return false
	}

	// If after expiration date -> expired
	if m.UseUntil && !m.Until.After(now) {
		m.expired = true
		return false
	}

	result := t.Permit(m.Identity, m.Price)
	if result {
		m.TradesLeft--
	}

	return result
}
