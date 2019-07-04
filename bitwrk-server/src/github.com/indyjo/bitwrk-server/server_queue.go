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
	db "github.com/indyjo/bitwrk-server/gae"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"net/http"
	"strings"
	"time"
)

func mustDecodeKey(s string) *datastore.Key {
	if key, err := datastore.DecodeKey(s); err != nil {
		panic(err)
	} else {
		return key
	}
	return nil // never reached
}

func handleRetireTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	keyString := r.FormValue("tx")
	key := mustDecodeKey(keyString)
	log.Infof(c, "Retiring transaction %v (%v)", keyString, key)
	if err := db.RetireTransaction(c, key); err == db.ErrTransactionTooYoung {
		log.Infof(c, "Transaction is too young to be retired")
	} else if err == db.ErrTransactionAlreadyRetired {
		log.Infof(c, "Transaction has already been retired")
	} else if err != nil {
		log.Warningf(c, "Error retiring transaction: %v", err)
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
	log.Infof(c, "Retiring bid %v (%v)", keyString, key)
	if err := db.RetireBid(c, key); err != nil {
		log.Warningf(c, "Error retiring bid: %v", err)
		http.Error(w, "Error retiring bid", http.StatusInternalServerError)
	}
}

func handleApplyChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	c := appengine.NewContext(r)
	log.Infof(c, "Placing bids: %v", r.FormValue("placed"))
	placedKeys := strings.Split(r.FormValue("placed"), " ")
	if len(placedKeys) == 1 && placedKeys[0] == "" {
		placedKeys = []string{}
	}

	for _, key := range placedKeys {
		if err := db.PlaceBid(c, key); err != nil {
			log.Errorf(c, "Couldn't place bid %v: %v", key, err)
		}
	}

	log.Infof(c, "Creating transactions for: %v", r.FormValue("matched"))
	bidKeys := strings.Split(r.FormValue("matched"), " ")
	if len(bidKeys) == 1 && bidKeys[0] == "" {
		bidKeys = []string{}
	}
	var timestamp time.Time
	if t, err := time.Parse(time.RFC3339Nano, r.FormValue("timestamp")); err != nil {
		log.Errorf(c, "Couldn't parse time '%v': %v", r.FormValue("timestamp"), err)
		return
	} else {
		timestamp = t
	}

	for len(bidKeys) != 0 {
		newKey, oldKey := bidKeys[0], bidKeys[1]
		bidKeys = bidKeys[2:]
		if err := db.MatchBids(c, timestamp, newKey, oldKey); err != nil {
			log.Errorf(c, "Couldn't match bids %v and %v: %v", newKey, oldKey, err)
		}
	}
}
