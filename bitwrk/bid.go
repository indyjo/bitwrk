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

package bitwrk

import (
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/money"
	"math"
	"net/url"
	"strings"
	"time"
)

var ErrInsufficientFunds = fmt.Errorf("Insufficient funds")
var ErrWrongCurrency = fmt.Errorf("Wrong currency")

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

// Type NewBidDefaults defines a policy for initializing fields of new bids that are not
// customer-controlled.
type NewBidDefaults struct {
	InitialState        BidState
	FeeRatioNumerator   int64
	FeeRatioDenominator int64
	Timeout             time.Duration
}

// Function NewBid constructs a new Bid object out of the given arguments.
// It only verifies whether the BidTyoe is valid and whether the p price is non-negative.
// Non customer-controllable fields are initialized using the given NewBidDefaults.
func NewBid(
	bidType BidType,
	article ArticleId,
	price money.Money,
	participant, document, signature string,
	defaults *NewBidDefaults,
) (*Bid, error) {
	if bidType != Buy && bidType != Sell {
		return nil, fmt.Errorf("Illegal bid type")
	}
	if price.Amount < 0 {
		return nil, fmt.Errorf("Invalid price %v, must be >= 0", price)
	}
	if price.Amount >= math.MaxInt64/(defaults.FeeRatioNumerator+1) {
		return nil, fmt.Errorf("Bid price %v to high to calculate fee", price)
	}

	// Calculate fee using ratio, rounding up
	feeAmount := (defaults.FeeRatioNumerator*price.Amount + defaults.FeeRatioDenominator - 1) / defaults.FeeRatioDenominator

	now := time.Now()
	result := &Bid{Type: bidType,
		State:       defaults.InitialState,
		Article:     article,
		Price:       price,
		Fee:         money.Money{Currency: price.Currency, Amount: feeAmount},
		Participant: participant,
		Document:    document,
		Signature:   signature,
		Created:     now,
		Expires:     now.Add(defaults.Timeout)}
	return result, nil
}

// Function ParseBid creates a new bid out of string arguments, applying the given defaults.
func ParseBid(bidType, article, price, participant, nonce, signature string,
	defaults *NewBidDefaults) (*Bid, error) {
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

	return NewBid(outType, ArticleId(article), outPrice, participant, document, signature, defaults)
}

func (bid *Bid) CheckBalance(dao AccountingDao) error {
	price := bid.Price

	if bid.Type == Buy {
		// Buyers must pay for fee
		price = price.Add(bid.Fee)
		if price.Amount < 0 {
			// Sanity check
			return fmt.Errorf("Invalid bid price (including fees) of %v", price)
		}
		price = price.Neg()
	}

	if account, err := dao.GetAccount(bid.Participant); err != nil {
		return err
	} else if !account.GetAvailable().CanApply(price) {
		if bid.Type == Buy {
			return ErrInsufficientFunds
		}
		// A sell can only be rejected if the account is currently using another currency.
		return ErrWrongCurrency
	}

	return nil // Happy case
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

// Function MatchKey returns a string that is equal for Bids that might match.
// Currently, encodes article id and currency.
func (b *Bid) MatchKey() string {
	// Keep in sync with Transaction.MatchKey()!
	return fmt.Sprintf("%v:%v", b.Article, b.Price.Currency)
}
