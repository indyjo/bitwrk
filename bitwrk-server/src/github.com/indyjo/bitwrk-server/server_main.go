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
	"net/http"
)

func init() {
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
	http.HandleFunc("/query/accounts", handleQueryAccounts)
	http.HandleFunc("/_ah/queue/place-bid", handlePlaceBid)
	http.HandleFunc("/_ah/queue/retire-tx", handleRetireTransaction)
	http.HandleFunc("/_ah/queue/retire-bid", handleRetireBid)
}

func handleMyIp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	addr := r.RemoteAddr
	w.Write([]byte(addr))
}
