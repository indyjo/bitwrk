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

package cafs

import (
	"chunking"
	"encoding/binary"
	"errors"
	"io"
)

// Writes a stream of chunk hash/length pairs into an io.Writer. Length is encoded
// as Varint.
func EncodeChunkHashes(file File, w io.Writer) error {
	chunks := file.Chunks()
	defer chunks.Dispose()
	for chunks.Next() {
		key := chunks.Key()
		if _, err := w.Write(key[:]); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, chunks.Size()); err != nil {
			return err
		}
	}
	return nil
}

func EncodeRequestedChunks(file File, wishList []byte, w io.Writer) error {
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
			if err := binary.Write(w, binary.BigEndian, chunk.Size()); err != nil {
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
	missing       map[SKey]bool
	disposed      bool
	info          string
}

func readKey(r io.Reader, key *SKey) error {
	k := key[:]
	for len(k) > 0 {
		if n, err := r.Read(k); err == io.EOF && n == len(k) {
			return nil
		} else if err != nil {
			return err
		} else {
			k = k[n:]
		}
	}
	return nil
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
		if err := readKey(r, &key); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		var length int64
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
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
		missing:       missing,
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
func (b *Builder) EncodeMissingChunks(w io.Writer) error {
	b.checkValid()
	var requested [1]byte
	for idx, chunk := range b.chunks {
		if _, ok := b.missing[chunk.key]; ok {
			// chunk is missing -> request it
			requested[0] = (requested[0] << 1) | 1
		} else {
			requested[0] = (requested[0] << 1) | 0
		}
		if (idx & 7) == 7 {
			if _, err := w.Write(requested[:]); err != nil {
				return err
			}
			requested[0] = 0
		}
	}
	if (len(b.chunks) & 7) != 0 {
		// Flush final byte
		if _, err := w.Write(requested[:]); err != nil {
			return err
		}
	}
	return nil
}

// Reads a sequence of length-prefixed data chunks and tries to reconstruct a file from that
// information.
func (b *Builder) ReconstructFileFromRequestedChunks(r io.Reader) (File, error) {
	b.checkValid()
	// Counts the number of not immediately helpful chunks received to avoid being spammed.
	bufferedChunks := 0

	temp := b.storage.Create(b.info)
	defer temp.Dispose()
	for len(b.chunks) > 0 {
		if f, ok := b.found[b.chunks[0].key]; ok {
			// chunk is available already, get it from store
			if err := appendChunk(temp, f); err != nil {
				return nil, err
			}
			b.chunks = b.chunks[1:]
			if bufferedChunks > 0 {
				bufferedChunks--
			}
		} else if f, err := b.storage.Get(&b.chunks[0].key); err != nil {
			b.found[b.chunks[0].key] = f
			delete(b.missing, b.chunks[0].key)
		} else {
			if bufferedChunks > 16 {
				return nil, errors.New("Too many buffered chunks")
			}
			bufferedChunks++

			if err := readChunk(b.storage, r); err != nil {
				return nil, err
			}
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

func readChunk(s FileStorage, r io.Reader) error {
	var length int64
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}
	if length < chunking.MIN_CHUNK || length > chunking.MAX_CHUNK {
		return errors.New("Invalid chunk length")
	}
	tempChunk := s.Create("chunk")
	defer tempChunk.Dispose()
	if _, err := io.CopyN(tempChunk, r, length); err != nil {
		return err
	}
	if err := tempChunk.Close(); err != nil {
		return err
	}
	return nil
}
