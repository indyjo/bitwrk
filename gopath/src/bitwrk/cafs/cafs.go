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
var ErrNotEnoughSpace = errors.New("Not enough space")

type SKey [32]byte

type FileStorage interface {
	// Creates a new temporary that can be written into. The info string will stick
	// with the temporary and also with the file, should it be created, and serves only
	// informational purposes.
	Create(info string) Temporary
	Get(key *SKey) (File, error)
}

type File interface {
	Key() SKey
	Open() io.ReadCloser
	Size() int64
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
	mutex               sync.Mutex
	entries             map[SKey]*ramEntry
	bytesUsed, bytesMax int64
	youngest, oldest    SKey
}

type ramFile struct {
	storage *ramStorage
	key     SKey
	entry   *ramEntry
}

type ramEntry struct {
	// Keys to the next older and next younger entry
	younger, older SKey
	info           string
	data           []byte
}

type ramReader struct {
	data  []byte
	index int
}

type ramTemporary struct {
	storage *ramStorage
	info    string
	buffer  bytes.Buffer
	hash    hash.Hash
	valid   bool
	open    bool
}

func NewRamStorage(maxBytes int64) FileStorage {
	return &ramStorage{
		entries:  make(map[SKey]*ramEntry),
		bytesMax: maxBytes,
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
	s.mutex.Lock()
	entry, ok := s.entries[*key]
	if ok {
		s.removeFromChain(key, entry)
		s.insertIntoChain(key, entry)
	}
	s.mutex.Unlock()
	if ok {
		return &ramFile{s, *key, entry}, nil
	} else {
		return nil, ErrNotFound
	}
	return nil, nil // never reached
}

func (s *ramStorage) Create(info string) Temporary {
	return &ramTemporary{
		storage: s,
		info:    info,
		hash:    sha256.New(),
		valid:   true,
		open:    true,
	}
}

func (s *ramStorage) reserveBytes(info string, numBytes int64) error {
	if numBytes > s.bytesMax {
		return ErrNotEnoughSpace
	}
	bytesFree := s.bytesMax - s.bytesUsed
	if bytesFree < numBytes {
		log.Printf("[%v] Need to free %v more bytes of CAFS space to store object of size %v", info, numBytes-bytesFree, numBytes)
	}
	for bytesFree < numBytes {
		oldestKey := s.oldest
		oldestEntry := s.entries[s.oldest]
		if oldestEntry == nil {
			log.Panic("No more elements left while freeing CAFS space")
		}
		delete(s.entries, s.oldest)
		s.removeFromChain(&s.oldest, oldestEntry)
		oldestSize := int64(len(oldestEntry.data))
		s.bytesUsed -= oldestSize
		bytesFree += oldestSize
		log.Printf("[%v]   Deleted object of size %v bytes: [%v] %v", info, oldestSize, oldestEntry.info, oldestKey)
	}
	return nil
}

func (s *ramStorage) removeFromChain(key *SKey, entry *ramEntry) {
	if youngerEntry := s.entries[entry.younger]; youngerEntry != nil {
		youngerEntry.older = entry.older
	} else if s.youngest == *key {
		s.youngest = entry.older
	}
	if olderEntry := s.entries[entry.older]; olderEntry != nil {
		olderEntry.younger = entry.younger
	} else if s.oldest == *key {
		s.oldest = entry.younger
	}
	// clear outgoing links
	entry.younger, entry.older = SKey{}, SKey{}
}

func (s *ramStorage) insertIntoChain(key *SKey, entry *ramEntry) {
	entry.older = s.youngest
	if youngestEntry := s.entries[s.youngest]; youngestEntry != nil {
		// chain former youngest entry to new one
		youngestEntry.younger = *key
	} else {
		// empty map, new entry will also be oldest
		s.oldest = *key
	}
	s.youngest = *key
}

func (f *ramFile) Key() SKey {
	return f.key
}

func (f *ramFile) Open() io.ReadCloser {
	return &ramReader{f.entry.data, 0}
}

func (f *ramFile) Size() int64 {
	return int64(len(f.entry.data))
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
	{
		defer t.storage.mutex.Unlock()

		// Reserve the necessary space for storing the object
		if err := t.storage.reserveBytes(t.info, int64(t.buffer.Len())); err != nil {
			return err
		}

		// Detect if we're re-writing the same data (or even handle a hash collision)
		var newEntry *ramEntry
		if oldEntry := t.storage.entries[key]; oldEntry != nil {
			log.Printf("[%v] Overwriting key: %v [%v]", t.info, &key, oldEntry.info)
			t.storage.removeFromChain(&key, oldEntry)
			t.storage.bytesUsed -= int64(len(oldEntry.data))
			// re-use old entry
			newEntry = oldEntry
		} else {
			newEntry = new(ramEntry)
			t.storage.entries[key] = newEntry
		}
		newEntry.data = t.buffer.Bytes()
		newEntry.info = t.info
		t.storage.bytesUsed += int64(len(newEntry.data))
		t.storage.insertIntoChain(&key, newEntry)
	}
	log.Printf("[%v] Stored key: %v", t.info, &key)
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
		log.Printf("[%v] Temporary canceled", t.info)
	} else {
		log.Printf("[%v] Temporary disposed", t.info)
	}
}
