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

package query

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	db "github.com/indyjo/bitwrk/server/gae"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

// Handles requests for sets of account IDs.
func HandleQueryAccounts(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	limitStr := r.FormValue("limit")
	var limit int
	if limitStr == "" {
		limit = 100
	} else if n, err := strconv.ParseUint(limitStr, 10, 10); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		limit = int(n)
	}

	requestdepositaddress := r.FormValue("requestdepositaddress") != ""

	buffer := new(bytes.Buffer)
	handler := func(key string) {
		fmt.Fprintf(buffer, "%v\n", key)
	}

	if err := db.QueryAccountKeys(c, limit, requestdepositaddress, handler); err != nil {
		log.Errorf(c, "QueryAccountKeys failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	buffer.WriteTo(w)
}
