//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014-2019  Jonas Eschenburg <jonas@bitwrk.net>
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
	"io"
	"strconv"
	"time"

	"github.com/indyjo/bitwrk/common/bitcoin"
	"github.com/indyjo/bitwrk/common/money"
)

type DepositType int

const (
	DEPOSIT_TYPE_INJECTION DepositType = 0
	DEPOSIT_TYPE_BITCOIN   DepositType = 1
	// More types to follow, insert between
	DEPOSIT_TYPE_MAX DepositType = 1
)

type Deposit struct {
	Type                DepositType
	Amount              money.Money
	Account             string
	Reference           string // An external reference
	Document, Signature string
	Created             time.Time
}

func ParseDeposit(depositType, depositAccount, depositAmount, depositNonce, depositUid, depositRef, depositSig string) (*Deposit, error) {
	// Parse deposit amount
	var amount money.Money
	if err := amount.Parse(depositAmount); err != nil {
		return nil, err
	}

	// Parse deposit type and check for value range
	var dtype DepositType
	if n, err := strconv.ParseInt(depositType, 10, 8); err != nil {
		return nil, err
	} else if n < 0 || n > int64(DEPOSIT_TYPE_MAX) {
		return nil, fmt.Errorf("Bad deposit type: %d", n)
	} else {
		dtype = DepositType(n)
	}

	// Perform deposit amount value range check unless it is an injection
	if dtype != DEPOSIT_TYPE_INJECTION && amount.Amount <= 0 {
		return nil, fmt.Errorf("Non-positive deposit amount not allowed: %s", depositAmount)
	}

	if len(depositUid) < 8 || len(depositUid) > 64 {
		return nil, fmt.Errorf("Deposit UID length must be >= 8 and <= 64")
	}

	if len(depositRef) > 64 {
		return nil, fmt.Errorf("Deposit Ref length must be <= 64")
	}

	// check deposit uid for illegal characters
	for _, c := range depositUid {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
			return nil, fmt.Errorf("Deposit UID contains illegal character (only A-Z, a-z, 0-9 and '-')")
		}
	}

	// check deposit ref for illegal characters
	for _, c := range depositRef {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
			return nil, fmt.Errorf("Deposit ref contains illegal character (only A-Z, a-z, 0-9 and '-')")
		}
	}

	// Construct the document which is checked against the signature.
	// It is built like a URL query, with parameters ordered strictly
	// alphabetically.
	document := fmt.Sprintf(
		"account=%s&amount=%s&nonce=%s&ref=%s&type=%s&uid=%s",
		normalize(depositAccount),
		normalize(depositAmount),
		normalize(depositNonce),
		normalize(depositRef),
		normalize(depositType),
		normalize(depositUid))

	deposit := Deposit{
		Type:      dtype,
		Amount:    amount,
		Account:   depositAccount,
		Reference: depositRef,
		Document:  document,
		Signature: depositSig,
		Created:   time.Now(),
	}

	return &deposit, nil
}

// Signs the deposit using the given key and random number sources.
func (deposit *Deposit) SignWith(key *bitcoin.KeyPair, rand io.Reader, uid, nonce string) error {
	doc := fmt.Sprintf(
		"account=%s&amount=%s&nonce=%s&ref=%s&type=%v&uid=%s",
		normalize(deposit.Account),
		normalize(deposit.Amount.String()),
		normalize(nonce),
		normalize(deposit.Reference),
		deposit.Type,
		normalize(uid))
	if sig, err := key.SignMessage(doc, rand); err != nil {
		return err
	} else {
		deposit.Document = doc
		deposit.Signature = sig
		return nil
	}
}

func (deposit *Deposit) Verify(trustedAccount string) error {
	if err := bitcoin.VerifySignatureBase64(deposit.Document, trustedAccount, deposit.Signature); err != nil {
		return fmt.Errorf("Could not validate signature: %v", err)
	}
	return nil
}

func (this *Deposit) Equals(other *Deposit) bool {
	if this == other {
		return true
	}
	if this.Account != other.Account {
		return false
	}
	return this.Account == other.Account &&
		this.Amount == other.Amount &&
		this.Reference == other.Reference &&
		this.Type == other.Type
}

func (deposit *Deposit) Place(uid string, dao AccountingDao) (err error) {
	if previous, err := dao.GetDeposit(uid); err == ErrNoSuchObject {
		// This is the expected case, handled below.
	} else if err != nil {
		// An error occurred
		return err
	} else {
		// Previous deposit exists. We have to compare for equality as placing
		// deposits should be an idempotent operation. We disallow changing
		// a deposit once it has been created, though.
		if previous.Equals(deposit) {
			return nil
		} else {
			return fmt.Errorf("A different deposit exists already with uid %v.", uid)
		}
	}

	// No deposit exist with the given uid
	amType := AccountMovementPayIn
	if deposit.Amount.Amount < 0 && deposit.Type == DEPOSIT_TYPE_INJECTION {
		// Special case: Injections can be negative, in which case the deposit is
		// to be treated like a withdrawal
		amType = AccountMovementPayOut
	}
	zero := money.Money{Currency: deposit.Amount.Currency, Amount: 0}
	err = PlaceAccountMovement(dao, deposit.Created, amType,
		deposit.Account, deposit.Account,
		deposit.Amount, zero,
		zero, deposit.Amount.Neg(),
		nil, nil, &uid, nil)
	if err != nil {
		return
	}

	err = dao.SaveDeposit(uid, deposit)
	return
}
