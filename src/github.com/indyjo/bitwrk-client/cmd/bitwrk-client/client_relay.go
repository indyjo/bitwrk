//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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

package main

import (
	"github.com/indyjo/bitwrk-common/protocol"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type cacheEntry struct {
	key  string
	data []byte
	time time.Time
}

type HttpRelay struct {
	localPathPrefix string
	remoteUrlPrefix string
	client          *http.Client
	// Cache the last recently updated data
	cached   *cacheEntry
	duration time.Duration
}

func NewHttpRelay(localPathPrefix, remoteUrlPrefix string, client *http.Client) *HttpRelay {
	return &HttpRelay{localPathPrefix, remoteUrlPrefix, client, nil, 30 * time.Second}
}

func (r *HttpRelay) CacheFor(d time.Duration) *HttpRelay {
	r.duration = d
	return r
}

func (r *HttpRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if !strings.HasPrefix(path, r.localPathPrefix) {
		http.Error(w, "Unknown path "+path, http.StatusBadRequest)
	}

	target := r.remoteUrlPrefix + path[len(r.localPathPrefix):]

	key := target + "-" + req.Header.Get("Accept")
	cached := r.cached
	if cached != nil && cached.key == key && cached.time.Add(r.duration).Before(time.Now()) {
		w.Write(cached.data)
		return
	}

	if remoteReq, err := protocol.NewRequest(req.Method, target, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		remoteReq.Header.Set("Accept", req.Header.Get("Accept"))
		reqTime := time.Now()
		if remoteResp, err := r.client.Do(remoteReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else if data, err := ioutil.ReadAll(remoteResp.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else {
			if remoteResp.StatusCode == http.StatusOK {
				r.cached = &cacheEntry{key, data, reqTime}
			}
			w.WriteHeader(remoteResp.StatusCode)
			w.Write(data)
		}
	}
}
