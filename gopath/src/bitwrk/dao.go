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

package bitwrk

import (
	"errors"
	"fmt"
)

var ErrNoSuchObject = errors.New("No such object")
var ErrKeyNotSet = errors.New("Key not set")

// Interface to accounting
type AccountingDao interface {
	GetAccount(participant string) (ParticipantAccount, error)
	SaveAccount(*ParticipantAccount) error

	GetMovement(key string) (AccountMovement, error)
	SaveMovement(*AccountMovement) error

	NewAccountMovementKey(participant string) (string, error)
}

type CachedAccountingDao interface {
	AccountingDao

	// Flushes all saved elements to the delegate AccountingDao,
	// If an error occurs, the operation is aborted (but can be retried,
	// depending on the error).
	// Subsequent calls of Flush() are idempotent.
	Flush() error
}

// Implementation of a cached accounting dao for use in databases like Google's
// datastore where you don't read your own writes in a transaction.
// This type is not thread-safe. If used in a multi-threaded context,
// proper synchronisation must be applied.
type cachedAccountingDao struct {
	delegate       AccountingDao
	accounts       map[string]ParticipantAccount
	movements      map[string]AccountMovement
	savedAccounts  map[string]bool
	savedMovements map[string]bool
}

func NewCachedAccountingDao(delegate AccountingDao) CachedAccountingDao {
	result := new(cachedAccountingDao)
	result.delegate = delegate
	result.accounts = make(map[string]ParticipantAccount)
	result.movements = make(map[string]AccountMovement)
	result.savedAccounts = make(map[string]bool)
	result.savedMovements = make(map[string]bool)
	return result
}

func (c *cachedAccountingDao) GetAccount(participant string) (account ParticipantAccount, err error) {
	if account, ok := c.accounts[participant]; ok {
		return account, nil
	}

	account, err = c.delegate.GetAccount(participant)
	if err == nil {
		c.accounts[participant] = account
	}

	return
}

func (c *cachedAccountingDao) SaveAccount(account *ParticipantAccount) error {
	if account == nil || account.Participant == "" {
		panic(fmt.Errorf("Can't save account: %v", account))
	}
	c.accounts[account.Participant] = *account
	c.savedAccounts[account.Participant] = true
	return nil
}

func (c *cachedAccountingDao) GetMovement(key string) (AccountMovement, error) {
	if m, ok := c.movements[key]; ok {
		return m, nil
	}

	if m, err := c.delegate.GetMovement(key); err != nil {
		return AccountMovement{}, err
	} else {
		m.Key = &key
		c.movements[key] = m
		return m, nil
	}
	return AccountMovement{}, nil // never reached
}

func (c *cachedAccountingDao) SaveMovement(m *AccountMovement) error {
	if m.Key == nil {
		return ErrKeyNotSet
	}
	c.movements[*m.Key] = *m
	c.savedMovements[*m.Key] = true
	return nil
}

func (c *cachedAccountingDao) NewAccountMovementKey(participant string) (string, error) {
	return c.delegate.NewAccountMovementKey(participant)
}

func (c *cachedAccountingDao) Flush() error {
	for k, _ := range c.savedAccounts {
		account := c.accounts[k]
		if err := c.delegate.SaveAccount(&account); err != nil {
			return err
		}
		delete(c.savedAccounts, k)
	}
	for k, _ := range c.savedMovements {
		movement := c.movements[k]
		if err := c.delegate.SaveMovement(&movement); err != nil {
			return err
		}
		delete(c.savedMovements, k)
	}
	return nil
}
