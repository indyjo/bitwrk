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
	"net/url"
	"time"

	"google.golang.org/appengine/datastore"

	. "github.com/indyjo/bitwrk/common/bitwrk"
	"github.com/indyjo/bitwrk/common/money"
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

// Make sure datastore.PropertyLoadSaver is supported
var _ datastore.PropertyLoadSaver = bidCodec{nil}

func (codec bidCodec) Load(props []datastore.Property) error {
	bid := codec.bid
	bid.Price.Currency = money.BTC
	bid.Fee.Currency = money.BTC
	for _, p := range props {
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

func (codec bidCodec) Save() ([]datastore.Property, error) {
	bid := codec.bid
	props := make([]datastore.Property, 0, 13)
	props = append(props,
		datastore.Property{Name: "Type", Value: int64(bid.Type), NoIndex: true},
		datastore.Property{Name: "State", Value: int64(bid.State), NoIndex: true},
		datastore.Property{Name: "Article", Value: string(bid.Article), NoIndex: true})
	if bid.Price.Currency != money.BTC {
		props = append(props,
			datastore.Property{Name: "Currency", Value: bid.Price.Currency.String(), NoIndex: true})
	}
	props = append(props,
		datastore.Property{Name: "Price", Value: bid.Price.Amount, NoIndex: true},
		datastore.Property{Name: "Fee", Value: bid.Fee.Amount, NoIndex: true},
		datastore.Property{Name: "Participant", Value: string(bid.Participant), NoIndex: true},
		datastore.Property{Name: "Document", Value: string(bid.Document), NoIndex: true},
		datastore.Property{Name: "Signature", Value: bid.Signature, NoIndex: true},
		datastore.Property{Name: "Created", Value: time.Time(bid.Created)},
		datastore.Property{Name: "Expires", Value: time.Time(bid.Expires), NoIndex: true})
	if bid.Matched != nil {
		props = append(props,
			datastore.Property{Name: "Matched", Value: *bid.Matched, NoIndex: true})
	}
	props = append(props,
		datastore.Property{Name: "Transaction", Value: mustDecodeKey(bid.Transaction), NoIndex: true})
	return props, nil
}

type hotBidCodec struct {
	bid *hotBid
}

// Make sure datastore.PropertyLoadSaver is implemented.
var _ datastore.PropertyLoadSaver = hotBidCodec{nil}

func (codec hotBidCodec) Load(props []datastore.Property) error {
	bid := codec.bid
	bid.Price.Currency = money.BTC
	for _, p := range props {
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

func (codec hotBidCodec) Save() ([]datastore.Property, error) {
	bid := codec.bid
	props := make([]datastore.Property, 0, 5)
	props = append(props,
		datastore.Property{Name: "BidKey", Value: bid.BidKey, NoIndex: true},
		datastore.Property{Name: "Type", Value: int64(bid.Type)},
		datastore.Property{Name: "Currency", Value: bid.Price.Currency.String()},
		datastore.Property{Name: "Price", Value: bid.Price.Amount},
		datastore.Property{Name: "Expires", Value: time.Time(bid.Expires)})
	return props, nil
}

type txCodec struct {
	tx *Transaction
}

// Make sure that datastore.PropertyLoadSaver is implemented.
var _ datastore.PropertyLoadSaver = txCodec{nil}

func (codec txCodec) Load(props []datastore.Property) error {
	tx := codec.tx
	tx.Price.Currency = money.BTC
	tx.Fee.Currency = money.BTC
	for _, p := range props {
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

// Save a Transaction object. For the sake of cost reduction, the only indexed properties are:
// - Article
// - Matched
func (codec txCodec) Save() ([]datastore.Property, error) {
	tx := codec.tx
	props := make([]datastore.Property, 0, 20)
	props = append(props,
		datastore.Property{Name: "Revision", Value: int64(tx.Revision), NoIndex: true},
		datastore.Property{Name: "BuyerBid", Value: mustDecodeKey(&tx.BuyerBid), NoIndex: true},
		datastore.Property{Name: "SellerBid", Value: mustDecodeKey(&tx.SellerBid), NoIndex: true},
		datastore.Property{Name: "Buyer", Value: tx.Buyer, NoIndex: true},
		datastore.Property{Name: "Seller", Value: tx.Seller, NoIndex: true},
		datastore.Property{Name: "Article", Value: string(tx.Article)},
		datastore.Property{Name: "Currency", Value: tx.Price.Currency.String()},
		datastore.Property{Name: "Price", Value: tx.Price.Amount, NoIndex: true},
		datastore.Property{Name: "Fee", Value: tx.Fee.Amount, NoIndex: true},
		datastore.Property{Name: "Matched", Value: tx.Matched},
		datastore.Property{Name: "State", Value: int64(tx.State), NoIndex: true},
		datastore.Property{Name: "Phase", Value: int64(tx.Phase), NoIndex: true},
		datastore.Property{Name: "Timeout", Value: tx.Timeout, NoIndex: true})
	if tx.WorkerURL != nil {
		props = append(props,
			datastore.Property{Name: "WorkerURL", Value: *tx.WorkerURL, NoIndex: true})
	}
	if tx.WorkHash != nil {
		props = append(props,
			datastore.Property{Name: "WorkHash", Value: tx.WorkHash[:], NoIndex: true})
	}
	if tx.WorkSecretHash != nil {
		props = append(props,
			datastore.Property{Name: "WorkSecretHash", Value: tx.WorkSecretHash[:], NoIndex: true})
	}
	if tx.BuyerSecret != nil {
		props = append(props,
			datastore.Property{Name: "BuyerSecret", Value: tx.BuyerSecret[:], NoIndex: true})
	}
	if tx.EncryptedResultReceipt != nil {
		props = append(props,
			datastore.Property{Name: "EncryptedResultHash", Value: tx.EncryptedResultReceipt.Hash[:], NoIndex: true},
			datastore.Property{Name: "EncryptedResultHashSignature", Value: tx.EncryptedResultReceipt.HashSignature[:], NoIndex: true},
			datastore.Property{Name: "ResultDecryptionKey", Value: tx.ResultDecryptionKey[:], NoIndex: true})
	}
	return props, nil
}

type accountCodec struct {
	account *ParticipantAccount
}

// Make sure the PropertyLoadSaver interface is implemented.
var _ datastore.PropertyLoadSaver = accountCodec{nil}

func (codec accountCodec) Load(props []datastore.Property) error {
	account := codec.account
	account.Currency = money.BTC // BTC is the default but can be overridden
	for _, p := range props {
		switch p.Name {
		case "Participant":
			account.Participant = p.Value.(string)
		case "LastMovementKey":
			s := p.Value.(*datastore.Key).Encode()
			account.LastMovementKey = &s
		case "Currency":
			account.Currency.MustParse(p.Value.(string))
		case "Available":
			account.AvailableAmount = p.Value.(int64)
		case "Blocked":
			account.BlockedAmount = p.Value.(int64)
		case "DepositInfo":
			account.DepositInfo = p.Value.(string)
		case "LastDepositInfo":
			account.LastDepositInfo = p.Value.(time.Time)
		case "DepositAddressRequest":
			account.DepositAddressRequest = p.Value.(string)
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}

func (codec accountCodec) Save() ([]datastore.Property, error) {
	account := codec.account
	props := make([]datastore.Property, 0, 8)

	props = append(props, datastore.Property{Name: "Participant", Value: string(account.Participant)})
	if account.LastMovementKey != nil {
		props = append(props, datastore.Property{Name: "LastMovementKey", Value: mustDecodeKey(account.LastMovementKey), NoIndex: true})
	}
	props = append(props,
		datastore.Property{Name: "Currency", Value: account.Currency.String()},
		datastore.Property{Name: "Available", Value: account.AvailableAmount, NoIndex: true},
		datastore.Property{Name: "Blocked", Value: account.BlockedAmount, NoIndex: true})
	if account.DepositInfo != "" {
		props = append(props,
			datastore.Property{Name: "DepositInfo", Value: account.DepositInfo},
			datastore.Property{Name: "LastDepositInfo", Value: account.LastDepositInfo})
	}
	if account.DepositAddressRequest != "" {
		props = append(props, datastore.Property{Name: "DepositAddressRequest", Value: account.DepositAddressRequest})
	}
	return props, nil
}

type movementCodec struct {
	context  context.Context
	movement *AccountMovement
}

// Make sure that datastore.PropertyLoadSaver is implemented.
var _ datastore.PropertyLoadSaver = movementCodec{nil, nil}

func (codec movementCodec) Load(props []datastore.Property) error {
	movement := codec.movement
	movement.AvailableDelta.Currency = money.BTC
	movement.BlockedDelta.Currency = money.BTC
	movement.Fee.Currency = money.BTC
	movement.World.Currency = money.BTC
	for _, p := range props {
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
		case "BidKey":
			s := p.Value.(*datastore.Key).Encode()
			movement.BidKey = &s
		case "TxKey":
			s := p.Value.(*datastore.Key).Encode()
			movement.TxKey = &s
		case "DepositKey":
			s := DepositUid(p.Value.(*datastore.Key))
			movement.DepositKey = &s
		case "WithdrawalKey":
			panic("Loading withdrawal keys not implemented yet")
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

func (codec movementCodec) Save() ([]datastore.Property, error) {
	props := make([]datastore.Property, 0, 14)
	movement := codec.movement
	props = append(props,
		datastore.Property{Name: "Timestamp", Value: movement.Timestamp},
		datastore.Property{Name: "Type", Value: int64(movement.Type), NoIndex: true})
	if movement.AvailableDelta.Currency != money.BTC {
		props = append(props,
			datastore.Property{Name: "Currency", Value: movement.AvailableDelta.Currency.String(), NoIndex: true})
	}
	props = append(props,
		datastore.Property{Name: "AvailableDelta", Value: movement.AvailableDelta.Amount, NoIndex: true},
		datastore.Property{Name: "AvailableAccount", Value: string(movement.AvailableAccount), NoIndex: true})
	if movement.AvailablePredecessorKey != nil {
		props = append(props,
			datastore.Property{Name: "AvailablePredecessorKey", Value: mustDecodeKey(movement.AvailablePredecessorKey)})
	}
	props = append(props,
		datastore.Property{Name: "BlockedDelta", Value: movement.BlockedDelta.Amount, NoIndex: true})

	if movement.BlockedAccount != movement.AvailableAccount {
		props = append(props,
			datastore.Property{Name: "BlockedAccount", Value: string(movement.BlockedAccount), NoIndex: true})
		if movement.BlockedPredecessorKey != nil {
			props = append(props,
				datastore.Property{Name: "BlockedPredecessorKey", Value: mustDecodeKey(movement.BlockedPredecessorKey), NoIndex: true})
		}
	}

	props = append(props,
		datastore.Property{Name: "Fee", Value: movement.Fee.Amount, NoIndex: true},
		datastore.Property{Name: "World", Value: movement.World.Amount, NoIndex: true})

	if movement.BidKey != nil {
		props = append(props,
			datastore.Property{Name: "BidKey", Value: mustDecodeKey(movement.BidKey), NoIndex: true})
	}
	if movement.TxKey != nil {
		props = append(props,
			datastore.Property{Name: "TxKey", Value: mustDecodeKey(movement.TxKey), NoIndex: true})
	}
	if movement.DepositKey != nil {
		props = append(props,
			datastore.Property{Name: "DepositKey", Value: DepositKey(codec.context, *movement.DepositKey), NoIndex: true})
	}
	if movement.WithdrawalKey != nil {
		panic("Storing withdrawal keys not yet implemented")
	}

	return props, nil
}

type depositCodec struct {
	deposit *Deposit
}

// Make sure datastore.PropertyLoadSaver is implemented.
var _ datastore.PropertyLoadSaver = depositCodec{nil}

func (codec depositCodec) Save() ([]datastore.Property, error) {
	deposit := codec.deposit
	return []datastore.Property{
		datastore.Property{Name: "Account", Value: deposit.Account},
		datastore.Property{Name: "Amount", Value: deposit.Amount.Amount},
		datastore.Property{Name: "Created", Value: deposit.Created},
		datastore.Property{Name: "Currency", Value: deposit.Amount.Currency.String()},
		datastore.Property{Name: "Document", Value: deposit.Document, NoIndex: true},
		datastore.Property{Name: "Reference", Value: deposit.Reference, NoIndex: true},
		datastore.Property{Name: "Signature", Value: deposit.Signature, NoIndex: true},
		datastore.Property{Name: "Type", Value: int64(deposit.Type)},
	}, nil
}

func (codec depositCodec) Load(props []datastore.Property) error {
	deposit := codec.deposit
	deposit.Amount.Currency = money.BTC

	for _, p := range props {
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

type relationCodec struct {
	relation *Relation
}

// Make sure datastore.PropertyLoadSaver is implemented.
var _ datastore.PropertyLoadSaver = relationCodec{nil}

func (codec relationCodec) Save() ([]datastore.Property, error) {
	relation := codec.relation
	return []datastore.Property{
		datastore.Property{Name: "Source", Value: relation.Source},
		datastore.Property{Name: "Target", Value: relation.Target},
		datastore.Property{Name: "Enabled", Value: relation.Enabled},
		datastore.Property{Name: "LastModified", Value: relation.LastModified},
		datastore.Property{Name: "Document", Value: relation.Document, NoIndex: true},
		datastore.Property{Name: "Signature", Value: relation.Signature, NoIndex: true},
		datastore.Property{Name: "Type", Value: int64(relation.Type)},
	}, nil
}

func (codec relationCodec) Load(props []datastore.Property) error {
	relation := codec.relation

	for _, p := range props {
		switch p.Name {
		case "Source":
			relation.Source = p.Value.(string)
		case "Target":
			relation.Target = p.Value.(string)
		case "Enabled":
			relation.Enabled = p.Value.(bool)
		case "LastModified":
			relation.LastModified = p.Value.(time.Time)
		case "Document":
			relation.Document = p.Value.(string)
		case "Signature":
			relation.Signature = p.Value.(string)
		case "Type":
			relation.Type = RelationType(p.Value.(int64))
		default:
			return fmt.Errorf("Unknown property %s", p.Name)
		}
	}

	return nil
}
