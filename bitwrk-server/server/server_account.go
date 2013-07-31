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

package server

import (
	"appengine"
	"bitbucket.org/ww/goautoneg"
	"bitwrk"
	db "bitwrk/gae"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

const accountViewHtml = `
<!doctype html>
<html>
<head><title>View Account</title></head>
<body>
<table>
<tr><th>Participant</th><td>{{.Participant}}</td></tr>
<tr><th>Available</th><td>{{.Available}}</td></tr>
<tr><th>Blocked</th><td>{{.Blocked}}</td></tr>
</table>
<script src="/js/getjson.js" ></script>
</body>
</html>
`

var accountViewTemplate = template.Must(template.New("accountView").Parse(accountViewHtml))

// Handler function for /account/<accountId>
func handleAccount(w http.ResponseWriter, r *http.Request) {
	accountId := r.URL.Path[9:]

	if r.Method == "GET" {
		acceptable := []string{"text/html", "application/json"}
		contentType := goautoneg.Negotiate(r.Header.Get("Accept"), acceptable)
		if contentType == "" {
			http.Error(w,
				fmt.Sprintf("No accepted content type found. Supported: %v", acceptable),
				http.StatusNotAcceptable)
			return
		}

		if err := checkBitcoinAddress(accountId); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		c := appengine.NewContext(r)
		dao := db.NewGaeAccountingDao(c)
		var err error
		account, err := dao.GetAccount(accountId)

		if err != nil {
			http.Error(w, "Error retrieving account", http.StatusInternalServerError)
			c.Errorf("Error getting account %v: %v", accountId, err)
			return
		}

		w.Header().Set("Content-Type", contentType)
		if contentType == "application/json" {
			err = renderAccountJson(w, &account)
		} else {
			err = renderAccountHtml(w, &account)
		}

		if err != nil {
			http.Error(w, "Error rendering account", http.StatusInternalServerError)
			c.Errorf("Error rendering %v as %v: %v", r.URL, contentType, err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func renderAccountHtml(w http.ResponseWriter, account *bitwrk.ParticipantAccount) (err error) {
	return accountViewTemplate.Execute(w, account)
}

func renderAccountJson(w http.ResponseWriter, account *bitwrk.ParticipantAccount) (err error) {
	return json.NewEncoder(w).Encode(*account)
}
