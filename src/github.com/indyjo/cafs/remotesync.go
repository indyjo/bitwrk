//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014 Jonas Eschenburg <jonas@bitwrk.net>
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

// Implements a differential file synching mechanism based on the content-based chunking
// that is used by CAFS internally.
// Step 1: Sender lists hashes of chunks of file to transmit (32 byte + ~2.5 bytes for length per chunk)
// Step 2: Receiver lists missing chunks (one bit per chunk)
// Step 3: Sender sends content of missing chunks

package cafs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/indyjo/cafs/chunking"
	"io"
)

type byteReader struct {
	r   io.Reader
	buf [1]byte
}

func (r byteReader) ReadByte() (byte, error) {
	_, err := r.r.Read(r.buf[:])
	return r.buf[0], err
}

func readVarint(r io.Reader) (int64, error) {
	return binary.ReadVarint(byteReader{r: r})
}

func writeVarint(w io.Writer, value int64) error {
	var buf [binary.MaxVarintLen64]byte
	_, err := w.Write(buf[:binary.PutVarint(buf[:], value)])
	return err
}

// Writes a stream of chunk hash/length pairs into an io.Writer. Length is encoded
// as Varint.
func WriteChunkHashes(file File, w io.Writer) error {
	chunks := file.Chunks()
	defer chunks.Dispose()
	for chunks.Next() {
		key := chunks.Key()
		if _, err := w.Write(key[:]); err != nil {
			return err
		}
		if err := writeVarint(w, chunks.Size()); err != nil {
			return err
		}
	}
	return nil
}

// Writes a stream of chunk length / data pairs into an io.Writer, based on
// the chunks of a file and a matching list of requested chunks.
func WriteRequestedChunks(file File, wishList []byte, w io.Writer) error {
	if int64(len(wishList)) != (file.NumChunks()+7)/8 {
		return errors.New("Illegal size of wishList")
	}
	var b byte
	bit := 8
	iter := file.Chunks()
	defer iter.Dispose()
	for iter.Next() {
		if bit == 8 {
			b = wishList[0]
			wishList = wishList[1:]
			bit = 0
		}
		if 0 != (b & 0x80) {
			chunk := iter.File()
			defer chunk.Dispose()
			if err := writeVarint(w, chunk.Size()); err != nil {
				return err
			}
			r := chunk.Open()
			defer r.Close()
			if _, err := io.Copy(w, r); err != nil {
				return err
			}
		}
		b <<= 1
		bit++
	}
	return nil
}

type Builder struct {
	storage       FileStorage
	chunks        []chunkRef
	chunksWritten int
	found         map[SKey]File
	disposed      bool
	info          string
}

// Reads a byte sequence encoded with EncodeChunkHashes and returns a new builder.
// The builder can then output a list of chunks that are missing in the local storage
// for complete reconstruction of the file.
func NewBuilder(storage FileStorage, r io.Reader, info string) (*Builder, error) {
	chunks := make([]chunkRef, 0, 1024)
	var key SKey
	var lastPos int64
	found := make(map[SKey]File)
	missing := make(map[SKey]bool)
	for {
		if _, err := io.ReadFull(r, key[:]); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		var length int64
		if l, err := readVarint(r); err != nil {
			return nil, err
		} else {
			length = l
		}
		lastPos += length
		chunks = append(chunks, chunkRef{key, lastPos})
		if file, err := storage.Get(&key); err != nil {
			missing[key] = true
		} else {
			found[key] = file
		}
	}
	result := &Builder{
		storage:       storage,
		chunks:        chunks,
		chunksWritten: 0,
		found:         found,
		disposed:      false,
		info:          info,
	}

	return result, nil
}

func (b *Builder) Dispose() {
	if !b.disposed {
		b.disposed = true
		for _, file := range b.found {
			file.Dispose()
		}
	}
}

func (b *Builder) checkValid() {
	if b.disposed {
		panic("Already disposed")
	}
}

// Outputs a bit stream with '1' for each missing chunk, and
// '0' for each chunk that is already available.
func (b *Builder) WriteWishList(w io.Writer) error {
	b.checkValid()
	var buf [1]byte
	requested := make(map[SKey]bool)
	for idx, chunk := range b.chunks {
		if _, ok := b.found[chunk.key]; !ok && !requested[chunk.key] {
			// chunk is missing and not already requested -> request it
			buf[0] = (buf[0] << 1) | 1
			requested[chunk.key] = true
		} else {
			// chunk either found or already requested
			buf[0] = (buf[0] << 1) | 0
		}
		// Flush byte if 8 bits are set
		if (idx & 7) == 7 {
			if _, err := w.Write(buf[:]); err != nil {
				return err
			}
			buf[0] = 0
		}
	}
	// Flush final byte
	if (len(b.chunks) & 7) != 0 {
		// shift in enough zeros so that first written bit is at msb position
		buf[0] <<= 8 - uint(len(b.chunks)&7)
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	}
	return nil
}

// A datastructure to buffer the chunks sent up to a certain number
type chunkBuffer struct {
	chunks map[SKey]*chunkBufferEntry
	avail  int
}
type chunkBufferEntry struct {
	chunk File
	count int
}

func newChunkBuffer(avail int) *chunkBuffer {
	return &chunkBuffer{make(map[SKey]*chunkBufferEntry), avail}
}
func (b *chunkBuffer) isFull() bool { return b.avail <= 0 }
func (b *chunkBuffer) push(file File) {
	entry := b.chunks[file.Key()]
	if entry == nil {
		entry = &chunkBufferEntry{file, 0}
		b.chunks[file.Key()] = entry
	}
	entry.count++
	b.avail--
}
func (b *chunkBuffer) pop(key SKey) {
	entry := b.chunks[key]
	if entry == nil {
		return
	}
	b.avail++
	entry.count--
	if entry.count == 0 {
		entry.chunk.Dispose()
		delete(b.chunks, key)
	}
}
func (b *chunkBuffer) dispose() {
	for key, entry := range b.chunks {
		entry.chunk.Dispose()
		delete(b.chunks, key)
	}
}

// Reads a sequence of length-prefixed data chunks and tries to reconstruct a file from that
// information. By using a 16-chunk buffer lookahead, we try to be a little bit flexible
// with regard to the exact sequence of chunks.
func (b *Builder) ReconstructFileFromRequestedChunks(r io.Reader) (File, error) {
	b.checkValid()
	// Counts the number of not immediately helpful chunks received to avoid being spammed.
	buffer := newChunkBuffer(16)
	defer buffer.dispose()

	temp := b.storage.Create(b.info)
	defer temp.Dispose()

	// Index of the next chunk to write into temp
	idx := 0
	eof := false

	for {
		// Read as many chunks from the stream as possible
		for !eof && !buffer.isFull() {
			if f, err := readChunk(b.storage, r, fmt.Sprintf("%v #%d", b.info, idx)); err == io.EOF {
				eof = true
			} else if err != nil {
				return nil, err
			} else {
				buffer.push(f)
			}
		}
		// Try to write a chunk of the work file
		if idx < len(b.chunks) {
			key := b.chunks[idx].key
			idx++
			chunk, _ := b.storage.Get(&key)
			buffer.pop(key)
			if chunk != nil {
				// Todo: we could check for chunk size here
				if err := appendChunk(temp, chunk); err != nil {
					chunk.Dispose()
					return nil, err
				}
				chunk.Dispose()
			} else {
				return nil, errors.New("Could not reconstruct file from received chunks")
			}
		} else if !eof {
			return nil, errors.New("Received more chunks than needed.")
		} else {
			break
		}
	}

	if err := temp.Close(); err != nil {
		return nil, err
	}

	return temp.File(), nil
}

func appendChunk(temp Temporary, chunk File) error {
	r := chunk.Open()
	defer r.Close()
	if _, err := io.Copy(temp, r); err != nil {
		return err
	}
	return nil
}

func readChunk(s FileStorage, r io.Reader, info string) (File, error) {
	var length int64
	if n, err := readVarint(r); err != nil {
		return nil, err
	} else {
		length = n
	}
	if length < chunking.MIN_CHUNK || length > chunking.MAX_CHUNK {
		return nil, errors.New("Invalid chunk length")
	}
	tempChunk := s.Create(info)
	defer tempChunk.Dispose()
	if _, err := io.CopyN(tempChunk, r, length); err != nil {
		return nil, err
	}
	if err := tempChunk.Close(); err != nil {
		return nil, err
	}
	return tempChunk.File(), nil
}
