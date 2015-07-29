//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2015  Jonas Eschenburg <jonas@bitwrk.net>
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
	"appengine"
	"appengine/datastore"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"time"
)

func QueryAccountKeys(c appengine.Context, limit int, requestdepositaddress bool, handler func(string)) error {
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
func QueryTransactions(c appengine.Context, limit int, article bitwrk.ArticleId, begin, end time.Time, handler TxFunc) error {
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
