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
package bitwrk

import (
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/money"
	"io"
	"net/url"
	"strings"
	"time"
)

type Account interface {
	GetBalance() money.Money
	CanApply(delta money.Money) bool
	Apply(delta money.Money) error
	GetLastMovementKey() *string
}

// Message placed in a participant's account by the payment processor.
// It states that monetary transactions sent to a specific deposit address
// will be credited that particpant.
// When URL-encoding, fields names are converted to lower-case and ordered alphabetically.
type DepositAddressMessage struct {
	Nonce          string // A nonce requested from the BitWrk service
	DepositAddress string // The monetary address
	Participant    string // The participant whose account is credited
	Signer         string // The participant who signed this message
	Reference      string // Aditional data the signer wished to place in the message
	Signature      string // Signature over the URL-encoded message (except the "Signature" field)
}

// Reads fields from an url.Values object. Does not perform any checking
func (m *DepositAddressMessage) FromValues(values url.Values) {
	m.Nonce = values.Get("nonce")
	m.DepositAddress = values.Get("depositaddress")
	m.Participant = values.Get("participant")
	m.Signer = values.Get("signer")
	m.Reference = values.Get("reference")
	m.Signature = values.Get("signature")
}

// Places fields in an url.Values object.
func (m *DepositAddressMessage) ToValues(values url.Values) {
	values.Set("nonce", m.Nonce)
	values.Set("depositaddress", m.DepositAddress)
	values.Set("participant", m.Participant)
	values.Set("signer", m.Signer)
	values.Set("reference", m.Reference)
	values.Set("signature", m.Signature)
}

// Returns the URL-encoded part of the message that is signed.
// The "+" sign is encoded as "%20" to resolve an ambiguity with
// javascript's encodeURIComponent.
func (m *DepositAddressMessage) document() string {
	values := url.Values{}
	m.ToValues(values)
	values.Del("signature")
	return strings.Replace(values.Encode(), "+", "%20", -1)
}

// Signs the message using the specified key pair. Fields "Signer" and "Signature"
// are modified.
func (m *DepositAddressMessage) SignWith(key *bitcoin.KeyPair, rand io.Reader) error {
	m.Signer = key.GetAddress()
	if s, err := key.SignMessage(m.document(), rand); err != nil {
		return err
	} else {
		m.Signature = s
		return nil
	}
}

// Verifies authenticity (or if fed with m.Signer, only integrity) of the message.
func (m *DepositAddressMessage) VerifyWith(signer string) error {
	return bitcoin.VerifySignatureBase64(m.document(), signer, m.Signature)
}

// Message placed in a participant's account by the participant himself.
// It states that the particiant wishes to receive a new deposit address.
// When URL-encoding, fields names are converted to lower-case and ordered alphabetically.
type DepositAddressRequest struct {
	Nonce       string // A nonce requested from the BitWrk service
	Participant string // The account owner
	Signer      string // The participant who signed the message, usually the account owner
	Signature   string // Signature over the URL-encoded message (except the "Signature" field)
}

// Reads fields from an url.Values object. Does not perform any checking
func (r *DepositAddressRequest) FromValues(values url.Values) {
	r.Nonce = values.Get("nonce")
	r.Participant = values.Get("participant")
	r.Signer = values.Get("signer")
	r.Signature = values.Get("signature")
}

// Places fields in an url.Values object.
func (r *DepositAddressRequest) ToValues(values url.Values) {
	values.Set("nonce", r.Nonce)
	values.Set("participant", r.Participant)
	values.Set("signer", r.Signer)
	values.Set("signature", r.Signature)
}

// Returns the URL-encoded part of the request that is signed.
// The "+" sign is encoded as "%20" to resolve an ambiguity with
// javascript's encodeURIComponent.
func (r *DepositAddressRequest) document() string {
	values := url.Values{}
	r.ToValues(values)
	values.Del("signature")
	return strings.Replace(values.Encode(), "+", "%20", -1)
}

// Signs the request using the specified key pair. Fields "Signer" and "Signature"
// are modified.
func (r *DepositAddressRequest) SignWith(key *bitcoin.KeyPair, rand io.Reader) error {
	r.Signer = key.GetAddress()
	if s, err := key.SignMessage(r.document(), rand); err != nil {
		return err
	} else {
		r.Signature = s
		return nil
	}
}

// Verifies authenticity (or if fed with m.Signer, only integrity) of the request.
func (r *DepositAddressRequest) VerifyWith(signer string) error {
	return bitcoin.VerifySignatureBase64(r.document(), signer, r.Signature)
}

// Data stored once for each participant
type ParticipantAccount struct {
	Participant        string
	LastMovementKey    *string
	Available, Blocked money.Money
	// Document containing URL-encoded DepositAddressMessage
	DepositInfo string
	// Timestamp of last DepositAddressMessage
	LastDepositInfo time.Time
	// Document containing URL-encoded DepositAddressRequest
	DepositAddressRequest string
}

func (a *ParticipantAccount) GetAvailable() Account {
	return participantAccountPart{a, &a.Available}
}

func (a *ParticipantAccount) GetBlocked() Account {
	return participantAccountPart{a, &a.Blocked}
}

type participantAccountPart struct {
	account *ParticipantAccount
	balance *money.Money
}

func (a participantAccountPart) GetBalance() money.Money {
	return *a.balance
}

func (a participantAccountPart) CanApply(delta money.Money) bool {
	return a.balance.Currency == delta.Currency && delta.Amount+a.balance.Amount >= 0
}

func (a participantAccountPart) Apply(delta money.Money) error {
	if !a.CanApply(delta) {
		return fmt.Errorf("Can't apply delta of %v to balance of %v", delta, a.balance)
	}
	a.balance.Amount += delta.Amount
	return nil
}

func (a participantAccountPart) GetLastMovementKey() *string {
	return a.account.LastMovementKey
}

type AccountMovement struct {
	Key       *string
	Timestamp time.Time
	Type      AccountMovementType

	AvailableDelta          money.Money
	AvailableAccount        string
	AvailablePredecessorKey *string

	BlockedDelta          money.Money
	BlockedAccount        string
	BlockedPredecessorKey *string

	Fee   money.Money // Money immediately collectable by site owner
	World money.Money // Money delta for the rest of the world

	// References to the entities that caused the movement
	BidKey, TxKey             *string
	DepositKey, WithdrawalKey *string
}

// Places a new account movement between the given accounts, in a
// transaction-safe way. If any error occurs, the transaction must be rolled
// back.
func PlaceAccountMovement(
	dao AccountingDao,
	now time.Time,
	mType AccountMovementType,
	availableParticipant, blockedParticipant string,
	availableDelta, blockedDelta,
	fee, world money.Money,
	bidKey, txKey, depositKey, withdrawalKey *string,
) error {
	m := new(AccountMovement)
	m.Timestamp = now
	m.Type = mType
	m.AvailableDelta = availableDelta
	m.AvailableAccount = availableParticipant
	m.BlockedDelta = blockedDelta
	m.BlockedAccount = blockedParticipant
	m.Fee = fee
	m.World = world
	m.BidKey = bidKey
	m.TxKey = txKey
	m.DepositKey = depositKey
	m.WithdrawalKey = withdrawalKey

	if err := m.Validate(); err != nil {
		return err
	}

	// Fetch new key
	if key, err := dao.NewAccountMovementKey(availableParticipant); err != nil {
		return err
	} else {
		m.Key = &key
	}

	// Apply movement to available account
	if account, err := dao.GetAccount(availableParticipant); err != nil {
		return err
	} else {
		if err := account.GetAvailable().Apply(availableDelta); err != nil {
			return err
		}
		m.AvailablePredecessorKey = account.LastMovementKey
		account.LastMovementKey = m.Key

		if err := dao.SaveAccount(&account); err != nil {
			return err
		}
	}

	// Apply movement to blocked account
	if account, err := dao.GetAccount(blockedParticipant); err != nil {
		return err
	} else {
		if err := account.GetBlocked().Apply(blockedDelta); err != nil {
			return err
		}
		m.BlockedPredecessorKey = account.LastMovementKey
		account.LastMovementKey = m.Key
		if err := dao.SaveAccount(&account); err != nil {
			return err
		}
	}

	// Save movement
	if err := dao.SaveMovement(m); err != nil {
		return err
	}

	return nil
}

type AccountMovementType int8

const (
	AccountMovementInvalid AccountMovementType = iota
	AccountMovementPayIn
	AccountMovementPayOut
	AccountMovementPayOutReimburse
	AccountMovementBid
	AccountMovementBidReimburse
	AccountMovementTransaction
	AccountMovementTransactionFinish
	AccountMovementTransactionReimburse
)

func (t AccountMovementType) String() string {
	switch t {
	case AccountMovementPayIn:
		return "DEPOSIT"
	case AccountMovementPayOut:
		return "WITHDRAWAL"
	case AccountMovementPayOutReimburse:
		return "WITHDRAWAL_REIMBURSE"
	case AccountMovementBid:
		return "BID"
	case AccountMovementBidReimburse:
		return "BID_REIMBURSE"
	case AccountMovementTransaction:
		return "TRANSACTION"
	case AccountMovementTransactionFinish:
		return "TRANSACTION_FINISH"
	case AccountMovementTransactionReimburse:
		return "TRANSACTION_REIMBURSE"
	}
	return fmt.Sprintf("<Invalid Account Movement Type: %v>", int8(t))
}

func (m *AccountMovement) String() string {
	return fmt.Sprintf("%v: %v/UNBLOCKED:%v %v/BLOCKED:%v fee:%v world:%v",
		m.Type,
		m.AvailableAccount, m.AvailableDelta,
		m.BlockedAccount, m.BlockedDelta,
		m.Fee, m.World)
}

func validateCurrency(currency *money.Currency, otherCurrency money.Currency) (*money.Currency, error) {
	if currency == nil {
		return &otherCurrency, nil
	}
	if otherCurrency != *currency {
		return currency, fmt.Errorf("Currencies mixed: %v / %v", currency, otherCurrency)
	}
	return currency, nil
}

func checkFlowDirection(msg string, a int, b int64) error {
	if a < -2 {
		a = -2
	} else if a > +2 {
		a = 2
	}
	rel := []string{"<", "<=", "=", ">=", ">"}[a+2]
	if a < 0 && b < 0 || a >= -1 && a <= 1 && b == 0 || a > 0 && b > 0 {
		return nil
	} else {
		return fmt.Errorf("'%v' must be %v 0, but is %v", msg, rel, b)
	}
	return nil // never reached
}

func (m *AccountMovement) checkCashFlowDirection(available, blocked, fee, world int) error {
	if err := checkFlowDirection("available", available, m.AvailableDelta.Amount); err != nil {
		return err
	}
	if err := checkFlowDirection("blocked", blocked, m.BlockedDelta.Amount); err != nil {
		return err
	}
	if err := checkFlowDirection("fee", fee, m.Fee.Amount); err != nil {
		return err
	}
	if err := checkFlowDirection("world", world, m.World.Amount); err != nil {
		return err
	}
	return nil
}

func (m *AccountMovement) Validate() (err error) {
	var currency *money.Currency

	var sum int64 = 0

	if m.AvailableDelta.Amount != 0 {
		currency, err = validateCurrency(currency, m.AvailableDelta.Currency)
		if err != nil {
			return
		}
		sum += m.AvailableDelta.Amount
	}

	if m.BlockedDelta.Amount != 0 {
		currency, err = validateCurrency(currency, m.BlockedDelta.Currency)
		if err != nil {
			return
		}
		sum += m.BlockedDelta.Amount
	}

	if m.Fee.Amount != 0 {
		currency, err = validateCurrency(currency, m.Fee.Currency)
		if err != nil {
			return
		}
		sum += m.Fee.Amount
	}

	if m.World.Amount != 0 {
		currency, err = validateCurrency(currency, m.World.Currency)
		if err != nil {
			return
		}
		sum += m.World.Amount
	}

	switch m.Type {
	//checkCashFlowDirection(unblocked, blocked, fee, world int)
	case AccountMovementBid:
		err = m.checkCashFlowDirection(-2, 2, 0, 0)
	case AccountMovementBidReimburse:
		err = m.checkCashFlowDirection(2, -2, 0, 0)
	case AccountMovementTransaction:
		err = m.checkCashFlowDirection(1, -1, 0, 0)
	case AccountMovementTransactionFinish:
		err = m.checkCashFlowDirection(2, -2, 1, 0)
	case AccountMovementTransactionReimburse:
		err = m.checkCashFlowDirection(2, -2, 0, 0)
	case AccountMovementPayIn:
		err = m.checkCashFlowDirection(2, 0, 0, -2)
	case AccountMovementPayOut:
		err = m.checkCashFlowDirection(-2, 0, 0, 2)
	default:
		err = fmt.Errorf("Invalid account movement type %v", m.Type)
	}

	if err != nil {
		return
	}

	if sum != 0 {
		err = fmt.Errorf("Sum of amounts is not 0: %v", sum)
	}

	return
}

func (m *AccountMovement) MustValidate() {
	err := m.Validate()
	if err != nil {
		panic(fmt.Sprintf("%v doesn't validate: %v", m, err))
	}
}
