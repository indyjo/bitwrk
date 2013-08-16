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
    "crypto/rand"
    "encoding/hex"
    "log"
    "net/http"
    "strings"
    "sync"
)

type ReceiveManager struct {
    mutex *sync.Mutex
    urlPrefix string
    endpoints map[string]*Endpoint
}

type Endpoint struct {
    m *ReceiveManager
    key string
    handle func(http.ResponseWriter, *http.Request)
}

func NewReceiveManager(urlPrefix string) (*ReceiveManager) {
    var m ReceiveManager
    m.mutex = new(sync.Mutex)
    m.urlPrefix = urlPrefix
    m.endpoints = make(map[string]*Endpoint)
    return &m
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

func (m *ReceiveManager) NewEndpoint() *Endpoint {
    r := make([]byte, 4)
    if _, err := rand.Reader.Read(r); err != nil {
        panic(err)
    }
    key := hex.EncodeToString(r)
    e := &Endpoint{m, key, nil}
    m.mutex.Lock()
    m.endpoints[key] = e
    m.mutex.Unlock()
    log.Printf("New endpoint: %v", key)
    return e
}

func (e *Endpoint) Dispose() {
    log.Printf("Disposing endpoint: %v", e.key)
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
    if strings.HasSuffix(e.m.urlPrefix, "/") {
        return e.m.urlPrefix + e.key
    }
    return e.m.urlPrefix + "/" + e.key
}
