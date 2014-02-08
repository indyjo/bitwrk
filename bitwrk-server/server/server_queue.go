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
	"appengine/datastore"
	db "bitwrk/gae"
	"net/http"
)

func mustDecodeKey(s string) *datastore.Key {
	if key, err := datastore.DecodeKey(s); err != nil {
		panic(err)
	} else {
		return key
	}
	return nil // never reached
}

func handlePlaceBid(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	bidKeyString := r.FormValue("bid")
	bidKey := mustDecodeKey(bidKeyString)
	c.Infof("Placing bid %v (%v)", bidKeyString, bidKey)
	if txKey, err := db.TryMatchBid(c, bidKey); err != nil {
		c.Errorf("Error placing bid: %v", err)
		http.Error(w, "Error placing Bid", http.StatusInternalServerError)
	} else {
		c.Infof("Successfully placed!")
		if txKey != nil {
			c.Infof(" -> Transaction: %v", txKey.Encode())
		}
	}
}

func handleRetireTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	keyString := r.FormValue("tx")
	key := mustDecodeKey(keyString)
	c.Infof("Retiring transaction %v (%v)", keyString, key)
	if err := db.RetireTransaction(c, key); err == db.ErrTransactionTooYoung {
		c.Infof("Transaction is too young to be retired")
	} else if err == db.ErrTransactionAlreadyRetired {
		c.Infof("Transaction has already been retired")
	} else if err != nil {
		c.Warningf("Error retiring transaction: %v", err)
		http.Error(w, "Error retiring transaction", http.StatusInternalServerError)
	}
}

func handleRetireBid(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	keyString := r.FormValue("bid")
	key := mustDecodeKey(keyString)
	c.Infof("Retiring bid %v (%v)", keyString, key)
	if err := db.RetireBid(c, key); err != nil {
		c.Warningf("Error retiring bid: %v", err)
		http.Error(w, "Error retiring bid", http.StatusInternalServerError)
	}
}
