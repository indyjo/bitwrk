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
	"github.com/indyjo/bitwrk-common/bitwrk"
	"google.golang.org/appengine/datastore"
	"time"
)

func QueryAccountKeys(c context.Context, limit int, requestdepositaddress bool, handler func(string)) error {
	query := datastore.NewQuery("Account").KeysOnly().Limit(limit)

	if requestdepositaddress {
		query = query.Filter("DepositAddressRequest >", "")
	}

	iter := query.Run(c)
	for {
		// Iterate accounts in datastore
		if key, err := iter.Next(nil); err == datastore.Done {
			break
		} else if err != nil {
			return err
		} else {
			handler(key.StringID())
		}
	}

	return nil
}

type TxFunc func(key string, tx bitwrk.Transaction)

// Queries transactions matching the given constraints. Invokes handler func for every transaction found.
func QueryTransactions(c context.Context, limit int, article bitwrk.ArticleId, begin, end time.Time, handler TxFunc) error {
	query := datastore.NewQuery("Tx").Limit(limit)
	query = query.Filter("Article =", article)
	query = query.Filter("Matched >=", begin)
	query = query.Filter("Matched <", end)
	query = query.Order("Matched")

	iter := query.Run(c)

	for {
		var tx bitwrk.Transaction

		// Iterate transactions in datastore
		if key, err := iter.Next(txCodec{&tx}); err == datastore.Done {
			break
		} else if err != nil {
			return err
		} else {
			handler(key.Encode(), tx)
		}
	}

	return nil
}

// Queries account movements (ledger entries) in ascending timestamp order, beginning at a specific point in time.
func QueryAccountMovements(c context.Context, begin time.Time, limit int) ([]bitwrk.AccountMovement, error) {
	result := make([]bitwrk.AccountMovement, 0, limit)

	query := datastore.NewQuery("AccountMovement").Limit(limit)
	if !begin.IsZero() {
		query = query.Filter("Timestamp >=", begin)
	}
	query = query.Order("Timestamp")
	iter := query.Run(c)

	for {
		var movement bitwrk.AccountMovement

		// Iterate transactions in datastore
		if key, err := iter.Next(movementCodec{c, &movement}); err == datastore.Done {
			break
		} else if err != nil {
			return nil, err
		} else {
			keyStr := key.Encode()
			movement.Key = &keyStr
			result = append(result, movement)
		}
	}

	return result, nil
}
