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

package server

import (
	"fmt"
	"github.com/indyjo/bitwrk-server/query"
	"github.com/indyjo/bitwrk-server/util"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
	"net/http"
)

func init() {
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/bid", handleCreateBid)
	http.HandleFunc("/bid/", handleRenderBid)
	http.HandleFunc("/nonce", handleGetNonce)
	http.HandleFunc("/tx/", handleTx)
	http.HandleFunc("/account/", handleAccount)
	http.HandleFunc("/ledger/", handleAccountMovement)
	http.HandleFunc("/myip", handleMyIp)
	http.HandleFunc("/motd", handleMessageOfTheDay)
	http.HandleFunc("/deposit", handleCreateDeposit)
	http.HandleFunc("/deposit/", handleRenderDeposit)
	http.HandleFunc("/query/accounts", query.HandleQueryAccounts)
	http.HandleFunc("/query/ledger", query.HandleQueryAccountMovements)
	http.HandleFunc("/query/prices", query.HandleQueryPrices)
	http.HandleFunc("/query/trades", query.HandleQueryTrades)
	http.HandleFunc("/_ah/queue/apply-changes", handleApplyChanges)
	http.HandleFunc("/_ah/queue/retire-tx", handleRetireTransaction)
	http.HandleFunc("/_ah/queue/retire-bid", handleRetireBid)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		url, err := user.LoginURL(c, r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User: %v\n", u)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	if u := user.Current(c); u != nil {
		url, err := user.LogoutURL(c, r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
	}
}

func handleMyIp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	addr := r.RemoteAddr
	stripped := util.StripPort(addr)
	log.Infof(r.Context(), "Got MYIP request from '%v', returning '%v'", addr, stripped)
	w.Write([]byte(stripped))
}
