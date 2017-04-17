//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2017  Jonas Eschenburg <jonas@bitwrk.net>
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

package bitwrk

import (
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/money"
	"net/url"
	"strings"
	"time"
)

var ErrInsufficientFunds = fmt.Errorf("Insufficient funds")

type BidType int8

const (
	Buy BidType = iota
	Sell
)

func (t BidType) String() string {
	switch t {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	}
	return fmt.Sprintf("BidType(%d)", t)
}

func (t BidType) FormString() string {
	switch t {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	}
	return fmt.Sprintf("BidType(%d)", t)
}

type BidState int8

const (
	InQueue BidState = iota
	Placed
	Matched
	Expired
)

func (s BidState) String() string {
	switch s {
	case InQueue:
		return "INQUEUE"
	case Placed:
		return "PLACED"
	case Matched:
		return "MATCHED"
	case Expired:
		return "EXPIRED"
	}
	return fmt.Sprintf("BidState(%d)", s)
}

type ArticleId string
type UserId string
type BidId string

func (a ArticleId) FormString() string {
	return url.QueryEscape(string(a))
}

type Bid struct {
	Type                BidType
	State               BidState
	Article             ArticleId
	Price, Fee          money.Money
	Participant         string
	Document, Signature string
	Created, Expires    time.Time
	Matched             *time.Time
	Transaction         *string
}

type RawBid struct {
	Type    BidType
	Article ArticleId
	Price   money.Money
}

func (bid *Bid) Verify() error {
	if err := bitcoin.VerifySignatureBase64(bid.Document, bid.Participant, bid.Signature); err != nil {
		return fmt.Errorf("Could not validate signature: %v", err)
	}
	return nil
}

var MinimumPrice = money.MustParse("uBTC 0")

func NewBid(bidType BidType, article ArticleId, price money.Money, participant, document, signature string) (*Bid, error) {
	if bidType != Buy && bidType != Sell {
		return nil, fmt.Errorf("Illegal bid type")
	}
	if price.Currency != MinimumPrice.Currency || price.Amount < MinimumPrice.Amount {
		return nil, fmt.Errorf("Invalid price %v, must be >= %v", price, MinimumPrice)
	}

	now := time.Now()
	result := &Bid{Type: bidType,
		State:       InQueue,
		Article:     article,
		Price:       price,
		Fee:         money.Money{Currency: price.Currency, Amount: (3*price.Amount + 99) / 100},
		Participant: participant,
		Document:    document,
		Signature:   signature,
		Created:     now,
		Expires:     now.Add(120 * time.Second)}
	return result, nil
}

func ParseBid(bidType, article, price, participant, nonce, signature string) (*Bid, error) {
	var outType BidType
	if bidType == "BUY" {
		outType = Buy
	} else if bidType == "SELL" {
		outType = Sell
	} else {
		return nil, fmt.Errorf("Unknown bid type %s", bidType)
	}

	var outPrice money.Money
	if err := outPrice.Parse(price); err != nil {
		return nil, err
	}

	document := fmt.Sprintf(
		"article=%s&type=%s&price=%s&address=%s&nonce=%s",
		normalize(article),
		bidType,
		normalize(price),
		normalize(participant),
		normalize(nonce))

	return NewBid(outType, ArticleId(article), outPrice, participant, document, signature)
}

func (bid *Bid) CheckBalance(dao AccountingDao) error {
	if bid.Type == Sell {
		return nil
	}

	price := bid.Price.Add(bid.Fee)
	if price.Amount < 0 {
		return fmt.Errorf("Invalid bid price (including fees) of %v", price)
	}

	if account, err := dao.GetAccount(bid.Participant); err != nil {
		return err
	} else if account.Available.Sub(price).Amount < 0 {
		return ErrInsufficientFunds
	}

	return nil
}

func (bid *Bid) Book(dao CachedAccountingDao, key string) error {
	if bid.Type == Sell {
		return nil
	}

	priceIncludingFee := bid.Price.Add(bid.Fee)
	zero := money.Money{Currency: bid.Price.Currency, Amount: 0}
	return PlaceAccountMovement(dao, bid.Created, AccountMovementBid,
		bid.Participant, bid.Participant,
		priceIncludingFee.Neg(), priceIncludingFee,
		zero, zero,
		&key, nil, nil, nil,
	)
}

func (bid *Bid) Retire(dao AccountingDao, key string, now time.Time) error {
	if bid.State != Placed && bid.State != InQueue {
		// Only reimburse unmatched bids
		return nil
	}
	if bid.Type == Sell {
		bid.State = Expired
		return nil
	}

	price := bid.Price.Add(bid.Fee)

	zero := money.Money{Currency: bid.Price.Currency, Amount: 0}
	if err := PlaceAccountMovement(dao, now, AccountMovementBidReimburse,
		bid.Participant, bid.Participant,
		price, price.Neg(),
		zero, zero,
		&key, nil, nil, nil); err != nil {
		return err
	}

	bid.State = Expired
	return nil
}

func normalize(s string) string {
	return url.QueryEscape(strings.Replace(s, " ", "", -1))
}
