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
	"crypto/md5"
	"fmt"
	"net/http"
	"time"
)

type Nonce struct {
	Created, Expires      time.Time
	UserAgent, RemoteAddr string
}

// Nonces are placed in 256 shards for better concurrency, using the first
// two hexadecimal characters as shard ID.
func nonceShardKey(c appengine.Context, nonce string) *datastore.Key {
	return datastore.NewKey(c, "Nonces", nonce[:2], 0, nil)
}

func NonceKey(c appengine.Context, nonce string) *datastore.Key {
	return datastore.NewKey(c, "Nonce", nonce, 0, nonceShardKey(c, nonce))
}

// Handler function for /nonce
func handleGetNonce(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	hash := md5.New()
	now := time.Now()

	fmt.Fprintf(hash, "%v%v", appengine.RequestID(c), now.UnixNano())
	nonce := fmt.Sprintf("%x", hash.Sum(make([]byte, 0, 16)))

	obj := &Nonce{
		now,
		now.Add(180 * time.Second),
		r.UserAgent(),
		r.RemoteAddr}

	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		_, err := datastore.Put(c, NonceKey(c, nonce), obj)
		return err
	}, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	// Make _really_ sure this is not cached
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.Write([]byte(nonce))

	go func() {
		// Delete expired nonces of a different shard
		if err := deleteExpired(c, now, nonceShardKey(c, nonce[2:])); err != nil {
			c.Warningf("deleteExpired failed: %v", err)
		}
	}()
}

var errInvalidNonce = fmt.Errorf("Nonce invalid")

func checkNonce(c appengine.Context, nonce string) error {
	now := time.Now()

	if len(nonce) < 24 || len(nonce) > 32 {
		return errInvalidNonce
	}

	key := NonceKey(c, nonce)
	return datastore.RunInTransaction(c, func(c appengine.Context) error {
		var dbNonce Nonce
		if err := datastore.Get(c, key, &dbNonce); err != nil {
			return errInvalidNonce
		}

		if !dbNonce.Expires.After(now) {
			return errInvalidNonce
		}

		if err := datastore.Delete(c, key); err != nil {
			return err
		}

		return nil
	}, nil)
}

func deleteExpired(c appengine.Context, now time.Time, parentKey *datastore.Key) error {
	query := datastore.NewQuery("Nonce").KeysOnly().Limit(1000)
	query = query.Ancestor(parentKey)
	query = query.Filter("Expires <=", now)
	keys, err := query.GetAll(c, nil)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	c.Infof("Delete %v expired nonces", len(keys))
	datastore.DeleteMulti(c, keys)
	if err != nil {
		return err
	}

	return nil
}
