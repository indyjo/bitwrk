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

	. "github.com/indyjo/bitwrk-common/bitwrk"
	"google.golang.org/appengine/datastore"
)

type gaeAccountingDao struct {
	c         context.Context
	low, high int64
}

func (dao *gaeAccountingDao) GetAccount(participant string) (account ParticipantAccount, err error) {
	key := AccountKey(dao.c, participant)
	err = datastore.Get(dao.c, key, accountCodec{&account})
	if err == datastore.ErrNoSuchEntity {
		err = ErrNoSuchObject
	}

	return
}

func (dao *gaeAccountingDao) SaveAccount(account *ParticipantAccount) (err error) {
	if account == nil || account.Participant == "" {
		panic(fmt.Errorf("Can't save account: %v", account))
	}
	key := AccountKey(dao.c, account.Participant)
	_, err = datastore.Put(dao.c, key, datastore.PropertyLoadSaver(accountCodec{account}))
	return
}

func (dao *gaeAccountingDao) GetMovement(key string) (movement AccountMovement, err error) {
	k, err := datastore.DecodeKey(key)
	if err != nil {
		return
	}
	err = datastore.Get(dao.c, k, datastore.PropertyLoadSaver(movementCodec{dao.c, &movement}))
	if err == datastore.ErrNoSuchEntity {
		err = ErrNoSuchObject
	}
	return
}

func (dao *gaeAccountingDao) SaveMovement(movement *AccountMovement) (err error) {
	// don't check for nil here -> programmer's error
	key, err := datastore.DecodeKey(*movement.Key)
	if err != nil {
		return
	}
	_, err = datastore.Put(dao.c, key, datastore.PropertyLoadSaver(movementCodec{dao.c, movement}))
	return
}

func (dao *gaeAccountingDao) NewAccountMovementKey(participant string) (string, error) {
	parent := AccountKey(dao.c, participant)
	if dao.low == dao.high {
		if l, h, err := datastore.AllocateIDs(dao.c, "AccountMovement", parent, 2); err != nil {
			return "", err
		} else {
			dao.low, dao.high = l, h
		}
	}
	r := dao.low
	dao.low++
	return datastore.NewKey(dao.c, "AccountMovement", "", r, parent).Encode(), nil
}

func (dao *gaeAccountingDao) GetDeposit(uid string) (Deposit, error) {
	key := DepositKey(dao.c, uid)
	deposit := Deposit{}
	if err := datastore.Get(dao.c, key, depositCodec{&deposit}); err == datastore.ErrNoSuchEntity {
		return Deposit{}, ErrNoSuchObject
	} else {
		return deposit, err
	}
}

func (dao *gaeAccountingDao) SaveDeposit(uid string, deposit *Deposit) error {
	key := DepositKey(dao.c, uid)
	_, err := datastore.Put(dao.c, key, datastore.PropertyLoadSaver(depositCodec{deposit}))
	return err
}

func NewGaeAccountingDao(c context.Context, transactional bool) CachedAccountingDao {
	return NewCachedAccountingDao(&gaeAccountingDao{c: c}, transactional)
}
