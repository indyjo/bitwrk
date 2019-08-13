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

package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"bitbucket.org/ww/goautoneg"
	"github.com/indyjo/bitwrk-common/bitwrk"
	db "github.com/indyjo/bitwrk/server/gae"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

const movementViewHtml = `
<!doctype html>
<html>
<head><title>Ledger entry</title></head>
<body>
<table>
<tr><th>Time</th><td>{{.Timestamp}}</td></tr>
<tr><th>Type</th><td>{{.Type}}
{{if .BidKey}}
&raquo; <a href="/bid/{{.BidKey}}">Bid</a>
{{end}}
{{if .TxKey}}
&raquo; <a href="/tx/{{.TxKey}}">Tx</a>
{{end}}
{{if .DepositKey}}
&raquo; <a href="/deposit/{{.DepositKey}}">Deposit</a>
{{end}}
</td></tr>
<tr><th>Fee</th><td>{{.Fee}}</td></tr>
<tr><th>World</th><td>{{.World}}</td></tr>
<tr><th>Account</th><td><a href="/account/{{.AvailableAccount}}">{{.AvailableAccount}}</a></td></tr>
<tr><th>Available delta</th><td>{{.AvailableDelta}}</td></tr>
{{if .AvailablePredecessorKey}}
<tr><td></td><td><a href="/ledger/{{.AvailablePredecessorKey}}">Previous entry</a></td></tr>
{{end}}
<tr><th>Account</th><td><a href="/account/{{.BlockedAccount}}">{{.BlockedAccount}}</a></td></tr>
<tr><th>Blocked delta</th><td>{{.BlockedDelta}}</td></tr>
{{if .BlockedPredecessorKey}}
<tr><td></td><td><a href="/ledger/{{.BlockedPredecessorKey}}">Previous entry</a></td></tr>
{{end}}
</table>
<script src="/js/getjson.js" ></script>
</body>
</html>
`

var movementViewTemplate = template.Must(template.New("movementView").Parse(movementViewHtml))

// Handler function for /ledger/<accountMovementKey>
func handleAccountMovement(w http.ResponseWriter, r *http.Request) {
	movementKey := r.URL.Path[8:]

	if r.Method == "GET" {
		acceptable := []string{"text/html", "application/json"}
		contentType := goautoneg.Negotiate(r.Header.Get("Accept"), acceptable)
		if contentType == "" {
			http.Error(w,
				fmt.Sprintf("No accepted content type found. Supported: %v", acceptable),
				http.StatusNotAcceptable)
			return
		}

		c := appengine.NewContext(r)
		dao := db.NewGaeAccountingDao(c, false)
		var err error
		movement, err := dao.GetMovement(movementKey)

		if err == bitwrk.ErrNoSuchObject {
			http.Error(w, "No such ledger entry", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "Error retrieving ledger entry", http.StatusInternalServerError)
			log.Errorf(c, "Error getting account movement %v: %v", movementKey, err)
			return
		}

		w.Header().Set("Content-Type", contentType)
		if contentType == "application/json" {
			err = renderAccountMovementJson(w, &movement)
		} else {
			err = renderAccountMovementHtml(w, &movement)
		}

		if err != nil {
			http.Error(w, "Error rendering ledger entry", http.StatusInternalServerError)
			log.Errorf(c, "Error rendering account movement %v as %v: %v", r.URL, contentType, err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func renderAccountMovementHtml(w http.ResponseWriter, movement *bitwrk.AccountMovement) (err error) {
	return movementViewTemplate.Execute(w, movement)
}

func renderAccountMovementJson(w http.ResponseWriter, movement *bitwrk.AccountMovement) (err error) {
	return json.NewEncoder(w).Encode(*movement)
}
