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

// Package receiveman hosts the ReceiveManager.
package receiveman

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

const keyBytes = 4

type ReceiveManager struct {
	mutex     *sync.Mutex
	urlPrefix string
	endpoints map[string]*Endpoint
}

type Endpoint struct {
	m      *ReceiveManager
	key    string
	info   string
	handle func(http.ResponseWriter, *http.Request)
}

func NewReceiveManager(urlPrefix string) *ReceiveManager {
	var m ReceiveManager
	m.mutex = new(sync.Mutex)
	m.urlPrefix = urlPrefix
	m.endpoints = make(map[string]*Endpoint)
	return &m
}

func (m *ReceiveManager) GetUrlPrefix() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.urlPrefix
}

func (m *ReceiveManager) SetUrlPrefix(newPrefix string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.urlPrefix = newPrefix
}

func (m *ReceiveManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got HTTP %v on %v.", r.Method, r.URL)
	if !strings.HasPrefix(r.URL.Path, "/") ||
		len(r.URL.Path) < 2*keyBytes+1 ||
		len(r.URL.Path) > 2*keyBytes+1 && r.URL.Path[2*keyBytes+1] != '/' {
		http.NotFound(w, r)
		return
	}
	key := r.URL.Path[1 : 2*keyBytes+1]

	m.mutex.Lock()
	endpoint, ok := m.endpoints[key]
	m.mutex.Unlock()

	if !ok || endpoint.handle == nil {
		log.Printf("Don't know how to handle request. Endpoint: %v", endpoint)
		http.NotFound(w, r)
		return
	}

	endpoint.handle(w, r)
}

func (m *ReceiveManager) NewEndpoint(info string) *Endpoint {
	r := make([]byte, keyBytes)
	if _, err := io.ReadFull(rand.Reader, r); err != nil {
		panic(err)
	}
	key := hex.EncodeToString(r)
	e := &Endpoint{m, key, info, nil}
	m.mutex.Lock()
	m.endpoints[key] = e
	m.mutex.Unlock()
	log.Printf("[%#v] New endpoint: %v", info, key)
	return e
}

func (e *Endpoint) Dispose() {
	log.Printf("[%#v] Disposing endpoint: %v", e.info, e.key)
	e.m.mutex.Lock()
	delete(e.m.endpoints, e.key)
	e.m.mutex.Unlock()
}

func (e *Endpoint) SetHandler(handle func(http.ResponseWriter, *http.Request)) {
	e.m.mutex.Lock()
	defer e.m.mutex.Unlock()
	e.handle = handle
}

func (e *Endpoint) URL() string {
	prefix := e.m.GetUrlPrefix()
	if strings.HasSuffix(prefix, "/") {
		return prefix + e.key
	}
	return prefix + "/" + e.key
}
