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

package gae

import (
	"appengine/datastore"
	"fmt"
	. "github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/money"
	"net/url"
	"time"
)

func mustDecodeKey(s *string) *datastore.Key {
	if s == nil {
		return nil
	}
	if key, err := datastore.DecodeKey(*s); err != nil {
		panic(err)
	} else {
		return key
	}
	return nil // never reached
}

type bidCodec struct {
	bid *Bid
}

func (codec bidCodec) Load(c <-chan datastore.Property) error {
	bid := codec.bid
	bid.Price.Currency = money.BTC
	bid.Fee.Currency = money.BTC
	for p := range c {
		switch p.Name {
		case "Type":
			bid.Type = BidType(p.Value.(int64))
		case "State":
			bid.State = BidState(p.Value.(int64))
		case "Article":
			bid.Article = ArticleId(p.Value.(string))
		case "Currency":
			bid.Price.Currency.MustParse(p.Value.(string))
			bid.Fee.Currency = bid.Price.Currency
		case "Price":
			bid.Price.Amount = p.Value.(int64)
		case "Fee":
			bid.Fee.Amount = p.Value.(int64)
		case "Participant":
			bid.Participant = p.Value.(string)
		case "Document":
			bid.Document = p.Value.(string)
		case "Signature":
			bid.Signature = p.Value.(string)
		case "ValidFrom":
			fallthrough
		case "Created":
			bid.Created = p.Value.(time.Time)
		case "ValidUntil":
			fallthrough
		case "Expires":
			bid.Expires = p.Value.(time.Time)
		case "Matched":
			t := p.Value.(time.Time)
			bid.Matched = &t
		case "Transaction":
			if p.Value == nil {
				bid.Transaction = nil
			} else {
				s := p.Value.(*datastore.Key).Encode()
				bid.Transaction = &s
			}
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}

func (codec bidCodec) Save(c chan<- datastore.Property) error {
	bid := codec.bid
	c <- datastore.Property{Name: "Type", Value: int64(bid.Type)}
	c <- datastore.Property{Name: "State", Value: int64(bid.State)}
	c <- datastore.Property{Name: "Article", Value: string(bid.Article)}
	if bid.Price.Currency != money.BTC {
		c <- datastore.Property{Name: "Currency", Value: bid.Price.Currency.String()}
	}
	c <- datastore.Property{Name: "Price", Value: bid.Price.Amount}
	c <- datastore.Property{Name: "Fee", Value: bid.Fee.Amount}
	c <- datastore.Property{Name: "Participant", Value: string(bid.Participant)}
	c <- datastore.Property{Name: "Document", Value: string(bid.Document), NoIndex: true}
	c <- datastore.Property{Name: "Signature", Value: string(bid.Signature), NoIndex: true}
	c <- datastore.Property{Name: "Created", Value: time.Time(bid.Created)}
	c <- datastore.Property{Name: "Expires", Value: time.Time(bid.Expires)}
	if bid.Matched != nil {
		c <- datastore.Property{Name: "Matched", Value: *bid.Matched}
	}
	c <- datastore.Property{Name: "Transaction", Value: mustDecodeKey(bid.Transaction), NoIndex: true}
	close(c)
	return nil
}

type hotBidCodec struct {
	bid *hotBid
}

func (codec hotBidCodec) Load(c <-chan datastore.Property) error {
	bid := codec.bid
	bid.Price.Currency = money.BTC
	for p := range c {
		switch p.Name {
		case "BidKey":
			bid.BidKey = p.Value.(*datastore.Key)
		case "Type":
			bid.Type = BidType(p.Value.(int64))
		case "Currency":
			bid.Price.Currency.MustParse(p.Value.(string))
		case "Price":
			bid.Price.Amount = p.Value.(int64)
		case "Expires":
			bid.Expires = p.Value.(time.Time)
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}

func (codec hotBidCodec) Save(c chan<- datastore.Property) error {
	bid := codec.bid
	c <- datastore.Property{Name: "BidKey", Value: bid.BidKey}
	c <- datastore.Property{Name: "Type", Value: int64(bid.Type)}
	if bid.Price.Currency != money.BTC {
		c <- datastore.Property{Name: "Currency", Value: bid.Price.Currency.String()}
	}
	c <- datastore.Property{Name: "Price", Value: bid.Price.Amount}
	c <- datastore.Property{Name: "Expires", Value: time.Time(bid.Expires), NoIndex: true}
	close(c)
	return nil
}

type txCodec struct {
	tx *Transaction
}

func (codec txCodec) Load(c <-chan datastore.Property) error {
	tx := codec.tx
	tx.Price.Currency = money.BTC
	tx.Fee.Currency = money.BTC
	for p := range c {
		switch p.Name {
		case "Revision":
			tx.Revision = int(p.Value.(int64))
		case "BuyerBid":
			tx.BuyerBid = p.Value.(*datastore.Key).Encode()
		case "SellerBid":
			tx.SellerBid = p.Value.(*datastore.Key).Encode()
		case "Buyer":
			tx.Buyer = p.Value.(string)
		case "Seller":
			tx.Seller = p.Value.(string)
		case "Article":
			tx.Article = ArticleId(p.Value.(string))
		case "Currency":
			tx.Price.Currency.MustParse(p.Value.(string))
			tx.Fee.Currency = tx.Price.Currency
		case "Price":
			tx.Price.Amount = p.Value.(int64)
		case "Fee":
			tx.Fee.Amount = p.Value.(int64)
		case "Matched":
			tx.Matched = p.Value.(time.Time)
		case "State":
			tx.State = TxState(p.Value.(int64))
		case "Phase":
			tx.Phase = TxPhase(p.Value.(int64))
		case "Timeout":
			tx.Timeout = p.Value.(time.Time)
		case "WorkerURL":
			u := p.Value.(string)
			if _, err := url.ParseRequestURI(u); err != nil {
				return err
			} else {
				tx.WorkerURL = &u
			}
		case "WorkHash":
			tx.WorkHash = new(Thash)
			copy(tx.WorkHash[:], p.Value.([]byte))
		case "WorkSecretHash":
			tx.WorkSecretHash = new(Thash)
			copy(tx.WorkSecretHash[:], p.Value.([]byte))
		case "BuyerSecret":
			tx.BuyerSecret = new(Thash)
			copy(tx.BuyerSecret[:], p.Value.([]byte))
		case "EncryptedResultHash":
			if tx.EncryptedResultReceipt == nil {
				tx.EncryptedResultReceipt = new(Treceipt)
			}
			copy(tx.EncryptedResultReceipt.Hash[:], p.Value.([]byte))
		case "EncryptedResultHashSignature":
			if tx.EncryptedResultReceipt == nil {
				tx.EncryptedResultReceipt = new(Treceipt)
			}
			copy(tx.EncryptedResultReceipt.HashSignature[:], p.Value.([]byte))
		case "ResultDecryptionKey":
			tx.ResultDecryptionKey = new(Tkey)
			copy(tx.ResultDecryptionKey[:], p.Value.([]byte))
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}

func (codec txCodec) Save(c chan<- datastore.Property) error {
	tx := codec.tx
	c <- datastore.Property{Name: "Revision", Value: int64(tx.Revision)}
	c <- datastore.Property{Name: "BuyerBid", Value: mustDecodeKey(&tx.BuyerBid), NoIndex: true}
	c <- datastore.Property{Name: "SellerBid", Value: mustDecodeKey(&tx.SellerBid), NoIndex: true}
	c <- datastore.Property{Name: "Buyer", Value: string(tx.Buyer)}
	c <- datastore.Property{Name: "Seller", Value: string(tx.Seller)}
	c <- datastore.Property{Name: "Article", Value: string(tx.Article)}
	if tx.Price.Currency != money.BTC {
		c <- datastore.Property{Name: "Currency", Value: tx.Price.Currency.String()}
	}
	c <- datastore.Property{Name: "Price", Value: tx.Price.Amount}
	c <- datastore.Property{Name: "Fee", Value: tx.Fee.Amount}
	c <- datastore.Property{Name: "Matched", Value: time.Time(tx.Matched)}
	c <- datastore.Property{Name: "State", Value: int64(tx.State)}
	c <- datastore.Property{Name: "Phase", Value: int64(tx.Phase)}
	c <- datastore.Property{Name: "Timeout", Value: time.Time(tx.Timeout)}
	if tx.WorkerURL != nil {
		c <- datastore.Property{Name: "WorkerURL", Value: *tx.WorkerURL, NoIndex: true}
	}
	if tx.WorkHash != nil {
		c <- datastore.Property{Name: "WorkHash", Value: tx.WorkHash[:], NoIndex: true}
	}
	if tx.WorkSecretHash != nil {
		c <- datastore.Property{Name: "WorkSecretHash", Value: tx.WorkSecretHash[:], NoIndex: true}
	}
	if tx.BuyerSecret != nil {
		c <- datastore.Property{Name: "BuyerSecret", Value: tx.BuyerSecret[:], NoIndex: true}
	}
	if tx.EncryptedResultReceipt != nil {
		c <- datastore.Property{Name: "EncryptedResultHash", Value: tx.EncryptedResultReceipt.Hash[:], NoIndex: true}
		c <- datastore.Property{Name: "EncryptedResultHashSignature", Value: tx.EncryptedResultReceipt.HashSignature[:], NoIndex: true}
		c <- datastore.Property{Name: "ResultDecryptionKey", Value: tx.ResultDecryptionKey[:], NoIndex: true}
	}
	close(c)
	return nil
}

type accountCodec struct {
	account *ParticipantAccount
}

func (codec accountCodec) Load(c <-chan datastore.Property) error {
	account := codec.account
	account.Available.Currency = money.BTC
	account.Blocked.Currency = money.BTC
	for p := range c {
		switch p.Name {
		case "Participant":
			account.Participant = p.Value.(string)
		case "LastMovementKey":
			s := p.Value.(*datastore.Key).Encode()
			account.LastMovementKey = &s
		case "Currency":
			account.Available.Currency.MustParse(p.Value.(string))
			account.Blocked.Currency = account.Available.Currency
		case "Available":
			account.Available.Amount = p.Value.(int64)
		case "Blocked":
			account.Blocked.Amount = p.Value.(int64)
		case "DepositInfo":
			account.DepositInfo = p.Value.(string)
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}

func (codec accountCodec) Save(c chan<- datastore.Property) error {
	account := codec.account
	c <- datastore.Property{Name: "Participant", Value: string(account.Participant)}
	if account.LastMovementKey != nil {
		c <- datastore.Property{Name: "LastMovementKey", Value: mustDecodeKey(account.LastMovementKey)}
	}
	if account.Available.Currency != money.BTC {
		c <- datastore.Property{Name: "Currency", Value: account.Available.Currency.String()}
	}
	c <- datastore.Property{Name: "Available", Value: account.Available.Amount}
	c <- datastore.Property{Name: "Blocked", Value: account.Blocked.Amount}
	c <- datastore.Property{Name: "DepositInfo", Value: account.DepositInfo, NoIndex: true}
	close(c)
	return nil
}

type movementCodec struct {
	movement *AccountMovement
}

func (codec movementCodec) Load(c <-chan datastore.Property) error {
	movement := codec.movement
	movement.AvailableDelta.Currency = money.BTC
	movement.BlockedDelta.Currency = money.BTC
	movement.Fee.Currency = money.BTC
	movement.World.Currency = money.BTC
	for p := range c {
		switch p.Name {
		case "Timestamp":
			movement.Timestamp = p.Value.(time.Time)
		case "Type":
			movement.Type = AccountMovementType(p.Value.(int64))
		case "Currency":
			movement.AvailableDelta.Currency.MustParse(p.Value.(string))
			movement.BlockedDelta.Currency = movement.AvailableDelta.Currency
			movement.Fee.Currency = movement.AvailableDelta.Currency
			movement.World.Currency = movement.AvailableDelta.Currency
		case "AvailableDelta":
			movement.AvailableDelta.Amount = p.Value.(int64)
		case "AvailableAccount":
			movement.AvailableAccount = p.Value.(string)
		case "AvailablePredecessorKey":
			s := p.Value.(*datastore.Key).Encode()
			movement.AvailablePredecessorKey = &s
		case "BlockedDelta":
			movement.BlockedDelta.Amount = p.Value.(int64)
		case "BlockedAccount":
			movement.BlockedAccount = p.Value.(string)
		case "BlockedPredecessorKey":
			s := p.Value.(*datastore.Key).Encode()
			movement.BlockedPredecessorKey = &s
		case "Fee":
			movement.Fee.Amount = p.Value.(int64)
		case "World":
			movement.World.Amount = p.Value.(int64)
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	if movement.BlockedAccount == "" {
		movement.BlockedAccount = movement.AvailableAccount
		movement.BlockedPredecessorKey = movement.AvailablePredecessorKey
	}

	return nil
}

func (codec movementCodec) Save(c chan<- datastore.Property) error {
	movement := codec.movement
	c <- datastore.Property{Name: "Timestamp", Value: movement.Timestamp}
	c <- datastore.Property{Name: "Type", Value: int64(movement.Type)}
	if movement.AvailableDelta.Currency != money.BTC {
		c <- datastore.Property{Name: "Currency", Value: movement.AvailableDelta.Currency.String()}
	}
	c <- datastore.Property{Name: "AvailableDelta", Value: movement.AvailableDelta.Amount}
	c <- datastore.Property{Name: "AvailableAccount", Value: string(movement.AvailableAccount)}
	if movement.AvailablePredecessorKey != nil {
		c <- datastore.Property{Name: "AvailablePredecessorKey", Value: mustDecodeKey(movement.AvailablePredecessorKey)}
	}
	c <- datastore.Property{Name: "BlockedDelta", Value: movement.BlockedDelta.Amount}

	if movement.BlockedAccount != movement.AvailableAccount {
		c <- datastore.Property{Name: "BlockedAccount", Value: string(movement.BlockedAccount)}
		if movement.BlockedPredecessorKey != nil {
			c <- datastore.Property{Name: "BlockedPredecessorKey", Value: mustDecodeKey(movement.BlockedPredecessorKey)}
		}
	}

	c <- datastore.Property{Name: "Fee", Value: movement.Fee.Amount}
	c <- datastore.Property{Name: "World", Value: movement.World.Amount}
	close(c)
	return nil
}

type depositCodec struct {
	deposit *Deposit
}

func (codec depositCodec) Save(c chan<- datastore.Property) error {
	deposit := codec.deposit
	c <- datastore.Property{Name: "Account", Value: string(deposit.Account)}
	c <- datastore.Property{Name: "Amount", Value: deposit.Amount.Amount}
	c <- datastore.Property{Name: "Created", Value: deposit.Created}
	c <- datastore.Property{Name: "Currency", Value: deposit.Amount.Currency.String()}
	c <- datastore.Property{Name: "Document", Value: deposit.Document, NoIndex: true}
	c <- datastore.Property{Name: "Reference", Value: deposit.Reference, NoIndex: true}
	c <- datastore.Property{Name: "Signature", Value: deposit.Signature, NoIndex: true}
	c <- datastore.Property{Name: "Type", Value: int64(deposit.Type)}
	close(c)
	return nil
}

func (codec depositCodec) Load(c <-chan datastore.Property) error {
	deposit := codec.deposit
	deposit.Amount.Currency = money.BTC

	for p := range c {
		switch p.Name {
		case "Account":
			deposit.Account = p.Value.(string)
		case "Amount":
			deposit.Amount.Amount = p.Value.(int64)
		case "Created":
			deposit.Created = p.Value.(time.Time)
		case "Currency":
			deposit.Amount.Currency.MustParse(p.Value.(string))
		case "Document":
			deposit.Document = p.Value.(string)
		case "Reference":
			deposit.Reference = p.Value.(string)
		case "Signature":
			deposit.Signature = p.Value.(string)
		case "Type":
			deposit.Type = DepositType(p.Value.(int64))
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}
