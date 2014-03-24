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

package main

import (
	"bitwrk/client"
	"io"
	"net/http"
	"strings"
)

type HttpRelay struct {
	localPathPrefix string
	remoteUrlPrefix string
	client          *http.Client
}

func (r *HttpRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if !strings.HasPrefix(path, r.localPathPrefix) {
		http.Error(w, "Unknown path "+path, http.StatusBadRequest)
	}

	target := r.remoteUrlPrefix + path[len(r.localPathPrefix):]
	if remoteReq, err := client.NewRequest(req.Method, target, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		remoteReq.Header.Set("Accept", req.Header.Get("Accept"))
		remoteResp, err := r.client.Do(remoteReq)
		if remoteResp != nil && remoteResp.Body != nil {
			defer remoteResp.Body.Close()
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(remoteResp.StatusCode)
			io.Copy(w, remoteResp.Body)
		}
	}
}
