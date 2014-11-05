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

// This package implements a content-addressable file storage that keeps its
// data in RAM.

package cafs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/indyjo/cafs/chunking"
	"hash"
	"io"
	"log"
	"sync"
)

type ramStorage struct {
	mutex               sync.Mutex
	entries             map[SKey]*ramEntry
	bytesUsed, bytesMax int64
	bytesLocked         int64
	youngest, oldest    SKey
}

type ramFile struct {
	storage  *ramStorage
	key      SKey
	entry    *ramEntry
	disposed bool
}

type chunkRef struct {
	key SKey
	// Points to the byte position within the file immediately after this chunk
	nextPos int64
}

type ramEntry struct {
	// Keys to the next older and next younger entry
	younger, older SKey
	info           string
	// Holds data if entry is of simple kind
	data []byte
	// Holds a list of chunk positions if entry is of chunk list type
	chunks []chunkRef
	refs   int
}

type ramDataReader struct {
	data  []byte
	index int
}

type ramChunkReader struct {
	storage    *ramStorage // Storage to read from
	entry      *ramEntry   // Entry containing the chunks
	key        SKey        // SKey of that entry
	chunksTail []chunkRef  // Remaining chunks
	closed     bool        // Whether Close() has been called
	dataReader io.ReadCloser
}

type ramTemporary struct {
	storage   *ramStorage
	info      string           // Info text given by user identifying the current file
	buffer    bytes.Buffer     // Stores bytes since beginning of current chunk
	fileHash  hash.Hash        // hash since the beginning of the file
	chunkHash hash.Hash        // hash since the beginning of the current chunk
	valid     bool             // If false, something has gone wrong
	open      bool             // Set to false on Close()
	chunker   chunking.Chunker // Determines chunk boundaries
	chunks    []chunkRef       // Grows every time a chunk boundary is encountered
}

func NewRamStorage(maxBytes int64) FileStorage {
	return &ramStorage{
		entries:  make(map[SKey]*ramEntry),
		bytesMax: maxBytes,
	}
}

func (s *ramStorage) Get(key *SKey) (File, error) {
	s.mutex.Lock()
	entry, ok := s.entries[*key]
	if ok {
		if entry.refs == 0 {
			s.removeFromChain(key, entry)
			s.bytesLocked += entry.storageSize()
		}
		entry.refs++
	}
	s.mutex.Unlock()
	if ok {
		return &ramFile{s, *key, entry, false}, nil
	} else {
		return nil, ErrNotFound
	}
	return nil, nil // never reached
}

func (s *ramStorage) Create(info string) Temporary {
	return &ramTemporary{
		storage:   s,
		info:      info,
		fileHash:  sha256.New(),
		chunkHash: sha256.New(),
		valid:     true,
		open:      true,
		chunker:   chunking.New(),
		chunks:    make([]chunkRef, 0, 16),
	}
}

func (s *ramStorage) DumpStatistics(log Printer) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Printf("Bytes used: %d, locked: %d, oldest: %x, youngest: %x", s.bytesUsed, s.bytesLocked, s.oldest[:4], s.youngest[:4])
	for key, entry := range s.entries {
		log.Printf("  [%x] refs=%d size=%v [%v] %x (older) %x (younger)", key[:4], entry.refs, entry.storageSize(), entry.info, entry.older[:4], entry.younger[:4])

		prevPos := int64(0)
		for i, chunk := range entry.chunks {
			log.Printf("             chunk %4d: %x (length %6d, ends at %7d)", i, chunk.key[:4], chunk.nextPos-prevPos, chunk.nextPos)
			prevPos = chunk.nextPos
		}
	}
}

func (s *ramStorage) reserveBytes(info string, numBytes int64) error {
	if numBytes > s.bytesMax {
		return ErrNotEnoughSpace
	}
	bytesFree := s.bytesMax - s.bytesUsed
	if bytesFree < numBytes && LoggingEnabled {
		log.Printf("[%v] Need to free %v (currently unlocked %v) more bytes of CAFS space to store object of size %v",
			info, numBytes-bytesFree, s.bytesUsed-s.bytesLocked, numBytes)
	}
	for bytesFree < numBytes {
		oldestKey := s.oldest
		oldestEntry := s.entries[oldestKey]
		if oldestEntry == nil {
			return ErrNotEnoughSpace
		}
		s.removeFromChain(&s.oldest, oldestEntry)
		delete(s.entries, oldestKey)

		oldLocked := s.bytesLocked
		// Dereference all referenced chunks
		for _, chunk := range oldestEntry.chunks {
			s.release(&chunk.key, s.entries[chunk.key])
		}
		oldestSize := oldestEntry.storageSize()
		s.bytesUsed -= oldestSize
		bytesFree += oldestSize
		if LoggingEnabled {
			log.Printf("[%v]   Deleted object of size %v bytes: [%v] %v", info, oldestSize, oldestEntry.info, oldestKey)
			if oldLocked != s.bytesLocked {
				log.Printf("       -> unlocked %d bytes", oldLocked-s.bytesLocked)
			}
		}
	}
	return nil
}

// Puts an entry into the store. If an entry already exists, it must be identical to the old one.
// The newly-created or recycled entry has been lock'ed once and must be release'd properly.
func (s *ramStorage) storeEntry(key *SKey, data []byte, chunks []chunkRef, info string) error {
	if len(data) > 0 && len(chunks) > 0 {
		panic("Illegal entry")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Detect if we're re-writing the same data (or even handle a hash collision)
	var newEntry *ramEntry
	if oldEntry := s.entries[*key]; oldEntry != nil {
		if len(oldEntry.data) != len(data) || len(oldEntry.chunks) != len(chunks) {
			panic(fmt.Sprintf("[%v] Key collision: %v [%v]", info, key, oldEntry.info))
		}
		if LoggingEnabled {
			log.Printf("[%v] Recycling key: %v [%v] (data: %d bytes, chunks: %d)", info, key, oldEntry.info, len(data), len(chunks))
		}

		// Ref the reused entry.
		s.lock(key, oldEntry)

		// re-use old entry
		newEntry = oldEntry
	} else {
		newEntry = &ramEntry{
			info:   info,
			data:   data,
			chunks: chunks,
			refs:   1,
		}
		// Reserve the necessary space for storing the object
		if err := s.reserveBytes(info, newEntry.storageSize()); err != nil {
			return err
		}

		s.entries[*key] = newEntry
		s.bytesUsed += newEntry.storageSize()
		s.bytesLocked += newEntry.storageSize()
		if LoggingEnabled {
			log.Printf("[%v] Stored key: %v (data: %d bytes, chunks: %d)", info, key, len(data), len(chunks))
		}
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

// Mutex lock-protected version of lock()
func (s *ramStorage) lockL(key *SKey, entry *ramEntry) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.lock(key, entry)
}

func (s *ramStorage) lock(key *SKey, entry *ramEntry) {
	if entry.refs == 0 {
		s.removeFromChain(key, entry)
		s.bytesLocked += entry.storageSize()
	}
	entry.refs++
}

// Mutex lock-protected version of release()
func (s *ramStorage) releaseL(key *SKey, entry *ramEntry) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.release(key, entry)
}

func (s *ramStorage) release(key *SKey, entry *ramEntry) {
	if entry.refs == 0 {
		panic(fmt.Sprintf("Can't release entry %v with 0 references", key))
	}
	entry.refs--
	if entry.refs == 0 {
		s.bytesLocked -= entry.storageSize()
		s.insertIntoChain(key, entry)
	}
}

func (e *ramEntry) storageSize() int64 {
	return int64(len(e.data) + 40*len(e.chunks))
}

func (f *ramFile) Key() SKey {
	return f.key
}

func (f *ramFile) Open() io.ReadCloser {
	if len(f.entry.chunks) > 0 {
		f.storage.lockL(&f.key, f.entry)
		return &ramChunkReader{
			storage:    f.storage,
			entry:      f.entry,
			key:        f.key,
			chunksTail: f.entry.chunks,
			closed:     false,
		}
	} else {
		return &ramDataReader{f.entry.data, 0}
	}
}

func (f *ramFile) Size() int64 {
	return int64(len(f.entry.data))
}

func (f *ramFile) Dispose() {
	if !f.disposed {
		f.disposed = true
		f.storage.releaseL(&f.key, f.entry)
	}
}

func (f *ramFile) checkValid() {
	if f.disposed {
		panic("Already disposed")
	}
}

func (f *ramFile) Duplicate() File {
	f.checkValid()
	file, err := f.storage.Get(&f.key)
	if err != nil {
		panic("Couldn't duplicate file")
	}
	return file
}

func (f *ramFile) IsChunked() bool {
	f.checkValid()
	return len(f.entry.chunks) > 0
}

func (f *ramFile) Chunks() FileIterator {
	var chunks []chunkRef
	if len(f.entry.chunks) > 0 {
		chunks = f.entry.chunks
	} else {
		chunks = make([]chunkRef, 1)
		chunks[0] = chunkRef{f.key, f.Size()}
	}
	f.storage.lockL(&f.key, f.entry)
	return &ramChunksIter{
		storage:      f.storage,
		entry:        f.entry,
		key:          f.key,
		chunks:       chunks,
		chunkIdx:     0,
		lastChunkIdx: -1,
		disposed:     false,
	}
}

func (f *ramFile) NumChunks() int64 {
	if len(f.entry.chunks) > 0 {
		return int64(len(f.entry.chunks))
	} else {
		return 1
	}
}

func (ci *ramChunksIter) checkValid() {
	if ci.disposed {
		panic("Already disposed")
	}
}

type ramChunksIter struct {
	storage      *ramStorage
	key          SKey
	entry        *ramEntry
	chunks       []chunkRef
	chunkIdx     int
	lastChunkIdx int
	disposed     bool
}

func (ci *ramChunksIter) Dispose() {
	if !ci.disposed {
		ci.disposed = true
		ci.storage.releaseL(&ci.key, ci.entry)
	}
}

func (ci *ramChunksIter) Duplicate() FileIterator {
	ci.checkValid()
	ci.storage.lockL(&ci.key, ci.entry)
	return &ramChunksIter{
		storage:  ci.storage,
		key:      ci.key,
		entry:    ci.entry,
		chunks:   ci.chunks,
		chunkIdx: ci.chunkIdx,
		disposed: false,
	}
}

func (ci *ramChunksIter) Next() bool {
	ci.checkValid()
	if ci.chunkIdx == len(ci.chunks) {
		ci.Dispose()
		return false
	} else {
		ci.lastChunkIdx = ci.chunkIdx
		ci.chunkIdx++
		return true
	}
}

func (ci *ramChunksIter) Key() SKey {
	ci.checkValid()
	return ci.chunks[ci.lastChunkIdx].key
}

func (ci *ramChunksIter) Size() int64 {
	ci.checkValid()
	startPos := int64(0)
	if ci.lastChunkIdx > 0 {
		startPos = ci.chunks[ci.lastChunkIdx-1].nextPos
	}
	return ci.chunks[ci.lastChunkIdx].nextPos - startPos
}

func (ci *ramChunksIter) File() File {
	ci.checkValid()
	if f, err := ci.storage.Get(&ci.chunks[ci.lastChunkIdx].key); err != nil {
		panic(err)
	} else {
		return f
	}
}

func (r *ramDataReader) Read(b []byte) (n int, err error) {
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

func (r *ramDataReader) Close() error {
	return nil
}

func (r *ramChunkReader) Read(b []byte) (n int, err error) {
	if r.closed {
		err = ErrInvalidState
		return
	}
	for n == 0 && err == nil {
		if r.dataReader == nil {
			if len(r.chunksTail) > 0 {
				if f, e := r.storage.Get(&r.chunksTail[0].key); e != nil {
					panic(e)
				} else {
					defer f.Dispose()
					r.dataReader = f.Open()
					r.chunksTail = r.chunksTail[1:]
				}
			} else {
				return 0, io.EOF
			}
		}

		n, err = r.dataReader.Read(b)
		if err == io.EOF {
			// never pass through delegate EOF
			err = r.dataReader.Close()
			r.dataReader = nil
		}
	}
	return
}

func (r *ramChunkReader) Close() (err error) {
	if r.closed {
		return nil
	}
	r.closed = true

	r.storage.releaseL(&r.key, r.entry)

	if r.dataReader != nil {
		err = r.dataReader.Close()
		r.dataReader = nil
	}

	return
}

// Writes the current buffer into a new chunk and resets the buffer.
// Assumes that chunkHash has already been updated.
func (t *ramTemporary) flushBufferIntoChunk() error {
	if t.buffer.Len() == 0 {
		return nil
	}

	// Copy the chunk's data
	chunkInfo := fmt.Sprintf("%v #%d", t.info, len(t.chunks))
	chunkData := make([]byte, t.buffer.Len())
	copy(chunkData, t.buffer.Bytes())

	// Get the chunk hash
	var key SKey
	t.chunkHash.Sum(key[:0])
	t.chunkHash.Reset()

	if err := t.storage.storeEntry(&key, chunkData, nil, chunkInfo); err != nil {
		return err
	}

	chunk := chunkRef{
		key:     key,
		nextPos: int64(t.buffer.Len()),
	}
	if len(t.chunks) > 0 {
		chunk.nextPos += t.chunks[len(t.chunks)-1].nextPos
	}
	t.chunks = append(t.chunks, chunk)

	t.buffer.Reset()
	return nil
}

func (t *ramTemporary) Write(b []byte) (int, error) {
	if !t.valid || !t.open {
		return 0, ErrInvalidState
	}
	t.valid = false // only temporary -> set to true on successful end of function

	nBytes := len(b)

	for len(b) > 0 {
		nBoundary := t.chunker.Scan(b)
		if _, err := t.buffer.Write(b[:nBoundary]); err != nil {
			return 0, err
		}
		t.chunkHash.Write(b[:nBoundary])
		t.fileHash.Write(b[:nBoundary])
		if nBoundary < len(b) {
			// a chunk boundary was detected
			if err := t.flushBufferIntoChunk(); err != nil {
				return 0, err
			}
			b = b[nBoundary:]
		} else {
			b = nil
		}
	}

	t.valid = true
	return nBytes, nil
}

func (t *ramTemporary) Close() error {
	if !t.valid || !t.open {
		return ErrInvalidState
	}
	t.open = false
	t.valid = false // only temporary -> set to true on successful end of function
	var key SKey
	t.fileHash.Sum(key[:0])

	if len(t.chunks) == 0 {
		// File is single-chunk
		data := make([]byte, t.buffer.Len())
		copy(data, t.buffer.Bytes())
		if err := t.storage.storeEntry(&key, data, nil, t.info); err != nil {
			return err
		}
	} else {
		// Flush buffer contents into one last chunk
		if err := t.flushBufferIntoChunk(); err != nil {
			return err
		}
		finalChunks := make([]chunkRef, len(t.chunks))
		copy(finalChunks, t.chunks)
		if err := t.storage.storeEntry(&key, nil, finalChunks, t.info); err != nil {
			return err
		}
	}
	t.valid = true
	return nil
}

func (t *ramTemporary) File() File {
	if !t.valid {
		panic(ErrInvalidState)
	}
	if t.open {
		panic(ErrStillOpen)
	}

	var key SKey
	t.fileHash.Sum(key[:0])

	file, err := t.storage.Get(&key)
	if err != nil {
		// Shouldn't happen
		panic(err)
	}
	return file
}

func (t *ramTemporary) Dispose() {
	if t.chunks == nil {
		// temporary was already disposed, we allow this
		return
	}

	// dereference single-chunk entry if successfully closed
	if !t.open && t.valid {
		var key SKey
		t.fileHash.Sum(key[:0])
		t.storage.releaseL(&key, t.storage.entries[key])
	} else {
		// dereference all locked chunks otherwise
		// (they have been locked once just by storing them)
		func() {
			t.storage.mutex.Lock()
			defer t.storage.mutex.Unlock()
			for _, chunk := range t.chunks {
				t.storage.release(&chunk.key, t.storage.entries[chunk.key])
			}
		}()
	}

	t.valid = false
	wasOpen := t.open
	t.open = false
	t.buffer = bytes.Buffer{}
	t.chunker = nil
	t.chunks = nil
	if LoggingEnabled {
		if wasOpen {
			log.Printf("[%v] Temporary canceled", t.info)
		} else {
			log.Printf("[%v] Temporary disposed", t.info)
		}
	}
}
