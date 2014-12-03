//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014  Jonas Eschenburg <jonas@bitwrk.net>
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

package server

import (
	"appengine"
	"appengine/datastore"
	"fmt"
	"net/http"
	"strconv"
)

func handleQueryAccounts(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	w.Header().Set("Content-Type", "text/plain")
	limit := r.FormValue("limit")
	if limit == "" {
		limit = "100"
	}

	query := datastore.NewQuery("Account").KeysOnly()

	if r.FormValue("requestdepositaddress") != "" {
		query = query.Filter("DepositAddressRequest >", "")
	}

	if n, err := strconv.ParseUint(limit, 10, 10); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		query = query.Limit(int(n))
	}

	if keys, err := query.GetAll(c, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		for _, key := range keys {
			fmt.Fprintf(w, "%v\n", key.StringID())
		}
	}
}
