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

// This package implements a content-addressable file storage

package cafs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"log"
	"sync"
)

var ErrNotFound = errors.New("Not found")
var ErrStillOpen = errors.New("Temporary still open")
var ErrInvalidState = errors.New("Invalid temporary state")

type SKey [32]byte

type FileStorage interface {
	Create() Temporary
	Get(key *SKey) (File, error)
}

type File interface {
	Key() SKey
	Open() io.ReadCloser
}

type Temporary interface {
	// Stores the temporary file into the FileStorage, where it
	// can be retrieved by key - after Close has been called.
	io.WriteCloser

	File() (File, error)

	// Must be called when the temporary is no longer needed.
	Dispose()
}

type ramStorage struct {
	mutex sync.RWMutex
	files map[SKey][]byte
}

type ramFile struct {
	storage *ramStorage
	key     SKey
	data    []byte
}

type ramReader struct {
	data  []byte
	index int
}

type ramTemporary struct {
	storage *ramStorage
	buffer  bytes.Buffer
	hash    hash.Hash
	valid   bool
	open    bool
}

func NewRamStorage() FileStorage {
	return &ramStorage{
		files: make(map[SKey][]byte),
	}
}

func (k SKey) String() string {
	return hex.EncodeToString(k[:])
}

func ParseKey(s string) (*SKey, error) {
	if len(s) != 64 {
		return nil, errors.New("Invalid key length")
	}

	var result SKey
	if b, err := hex.DecodeString(s); err != nil {
		return nil, err
	} else {
		copy(result[:], b)
	}

	return &result, nil
}

func (s *ramStorage) Get(key *SKey) (File, error) {
	s.mutex.RLock()
	data, ok := s.files[*key]
	s.mutex.RUnlock()
	if ok {
		return &ramFile{s, *key, data}, nil
	} else {
		return nil, ErrNotFound
	}
	return nil, nil // never reached
}

func (s *ramStorage) Create() Temporary {
	return &ramTemporary{
		storage: s,
		hash:    sha256.New(),
		valid:   true,
		open:    true,
	}
}

func (f *ramFile) Key() SKey {
	return f.key
}

func (f *ramFile) Open() io.ReadCloser {
	return &ramReader{f.data, 0}
}

func (r *ramReader) Read(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	if r.index >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(b, r.data[r.index:])
	r.index += n
	return
}

func (r *ramReader) Close() error {
	return nil
}

func (t *ramTemporary) Write(b []byte) (n int, err error) {
	n, err = t.buffer.Write(b)
	if err != nil {
		t.valid = false
	}
	t.hash.Write(b)
	return
}

func (t *ramTemporary) Close() error {
	if !t.valid || !t.open {
		return ErrInvalidState
	}
	t.open = false
	var key SKey
	t.hash.Sum(key[:0])

	t.storage.mutex.Lock()
	t.storage.files[key] = t.buffer.Bytes()
	t.storage.mutex.Unlock()
	log.Printf("Stored key: %v", &key)
	return nil
}

func (t *ramTemporary) File() (File, error) {
	if !t.valid {
		return nil, ErrInvalidState
	}
	if t.open {
		return nil, ErrStillOpen
	}

	var key SKey
	t.hash.Sum(key[:0])

	return t.storage.Get(&key)
}

func (t *ramTemporary) Dispose() {
	var key SKey
	t.hash.Sum(key[:0])

	t.valid = false
	wasOpen := t.open
	t.open = false
	t.buffer = bytes.Buffer{}
	if wasOpen {
		log.Printf("Canceled temporary: %v", key)
	} else {
		log.Printf("Temporary disposed: %v", key)
	}
}
