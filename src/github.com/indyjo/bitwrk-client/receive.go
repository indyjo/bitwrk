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

package client

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

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
	var key string
	if idx := strings.LastIndex(r.URL.Path, "/"); idx == -1 {
		http.NotFound(w, r)
		return
	} else {
		key = r.URL.Path[idx+1:]
	}

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
	r := make([]byte, 4)
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

type gzipBody struct {
	compressed, uncompressed io.ReadCloser
}

func newGZIPBody(compressed io.ReadCloser) (*gzipBody, error) {
	if gz, err := gzip.NewReader(compressed); err != nil {
		return nil, err
	} else {
		return &gzipBody{compressed, gz}, nil
	}
}

func (gz *gzipBody) Read(data []byte) (int, error) {
	return gz.uncompressed.Read(data)
}

func (gz *gzipBody) Close() error {
	if err := gz.uncompressed.Close(); err != nil {
		gz.compressed.Close()
		return err
	} else if err := gz.compressed.Close(); err != nil {
		return err
	} else {
		return nil
	}
}

// Given a handler, returns a handler with transparent support for receiving gzip-compressed POST data
func withCompression(handle func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.Header.Get("Content-Encoding") == "gzip" {
			log.Printf("Handling GZIP-compressed POST.\n")
			if gz, err := newGZIPBody(r.Body); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				// copy request data, substitute body
				r2 := *r
				r2.Body = gz
				// Call original handler
				handle(w, &r2)
			}
		} else {
			handle(w, r)
		}
	}
}

func (e *Endpoint) URL() string {
	prefix := e.m.GetUrlPrefix()
	if strings.HasSuffix(prefix, "/") {
		return prefix + e.key
	}
	return prefix + "/" + e.key
}
