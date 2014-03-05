package chunking

import (
	"math/rand"
	"testing"
)

func TestChunker(t *testing.T) {
	data := make([]byte, 1<<24)
	for k, _ := range data {
		data[k] = byte(rand.Int())
	}

	chunker := New()
	chunkCount := 0
	for i := 0; i < len(data); {
		block := data[i:]
		bytes := chunker.Scan(block)
		i += bytes
		if bytes < len(block) {
			t.Logf("Chunk at %d", i)
			chunkCount++
		}
	}

	t.Logf("Generated %d chunks, avg size: %d bytes", chunkCount, len(data)/chunkCount)
}

func TestSingleByteBlocks(t *testing.T) {
	size := (1 << 17) * 10000
	chunker := &adlerChunker{a: 1}
	blocks := 0
	r := rand.New(rand.NewSource(0))
	for i := 0; i < size; i++ {
		val := r.Int()
		for j := uint(1); j < 24; j++ {
			val ^= val >> j
		}
		if chunker.Scan([]byte{byte(val)}) == 0 {
			blocks++
		}
		size--
	}
	blocks += 1
	if blocks <= size / (1<<17) {
		t.Errorf("Test produced only %v blocks -> no chunk boundaries found", blocks)
	} else {
		t.Logf("Test produced %v blocks (avg size: %d)", blocks, size/blocks)
	}
}
