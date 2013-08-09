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
)

type MandateRequest interface {
	Article() bitwrk.ArticleId
	Type() bitwrk.BidType
	Accept(identity *bitcoin.KeyPair, price money.Money)
	Reject()
}

type buyMandateRequest struct {
	a *BuyActivity
}

func (r buyMandateRequest) Type() bitwrk.BidType {
	return bitwrk.Buy
}

func (r buyMandateRequest) Article() bitwrk.ArticleId {
	return r.a.article
}

func (r buyMandateRequest) Accept(identity *bitcoin.KeyPair, price money.Money) {
	a := r.a
	a.condition.L.Lock()
	a.identity = identity
	a.price = price
	a.gotMandate = true
	a.condition.Broadcast()
	a.condition.L.Unlock()
}

func (r buyMandateRequest) Reject() {
	a := r.a
	a.condition.L.Lock()
	a.gotRejection = true
	a.condition.Broadcast()
	a.condition.L.Unlock()
}
