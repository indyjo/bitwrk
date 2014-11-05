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

// This package specifies a content-addressable file storage.

package cafs

import (
	"encoding/hex"
	"errors"
	"io"
)

var ErrNotFound = errors.New("Not found")
var ErrStillOpen = errors.New("Temporary still open")
var ErrInvalidState = errors.New("Invalid temporary state")
var ErrNotEnoughSpace = errors.New("Not enough space")

var LoggingEnabled = false

type SKey [32]byte

type FileStorage interface {
	// Creates a new temporary that can be written into. The info string will stick
	// with the temporary and also with the file, should it be created, and serves only
	// informational purposes.
	Create(info string) Temporary

	// Queries a file from the storage that can be read from. If the file exists, a File
	// interface is returned that has been locked once and that must be released correctly.
	// If the file does not exist, then (nil, ErrNotFound) is returned.
	Get(key *SKey) (File, error)

	DumpStatistics(log Printer)
}

type File interface {
	// Signals that this file handle is no longer in use.
	// If no handles exist on a file anymore, the storage space
	// bound by it can ce reclaimed by the garbage collector.
	// It is an error to call Open() or Duplicate() after Dispose().
	// It is ok to call Dispose() more than once.
	Dispose()
	Key() SKey
	Open() io.ReadCloser
	Size() int64
	// Creates a new handle to the same file that must be Dispose()'d
	// independently.
	Duplicate() File

	// Returns true if the file is stored in chunks internally.
	// It is an error to call this function after Dispose().
	IsChunked() bool
	// Returns an iterator to the chunks of the file. The iterator must be disposed after use.
	Chunks() FileIterator
	// Returns the number of chunks in this file, or 1 if file is not chunked
	NumChunks() int64
}

// Iterate over a set of files or chunks.
type FileIterator interface {
	// Must be called after using this iterator.
	Dispose()
	// Returns a copy of this iterator that must be Dispose()'d independently.
	Duplicate() FileIterator

	// Advances the iterator and returns true if successful, or false if no further chunks
	// could be read.
	// Must be called before calling File().
	Next() bool

	// Returns the key of the last file or chunk successfully read by Next().
	// Before calling this function, Next() must have been called and returned true.
	Key() SKey

	// Returns the size of the last file or chunk successfully read by Next().
	// Before calling this function, Next() must have been called and returned true.
	Size() int64

	// Returns the last file or chunk successfully read by Next() as a file.
	// The received File must be Dispose()'d.
	// Before calling this function, Next() must have been called and returned true.
	File() File
}

type Temporary interface {
	// Stores the temporary file into the FileStorage, where it
	// can be retrieved by key - after Close() has been called.
	io.WriteCloser

	// Returns a handle to the stored file, once Close() has been
	// called and no error occurred. Otherwise, panics.
	File() File

	// Must be called when the temporary is no longer needed.
	// It's ok to call Dispose() more than once.
	Dispose()
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
