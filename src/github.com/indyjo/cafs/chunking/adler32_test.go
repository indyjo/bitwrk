package chunking

import (
	"hash/adler32"
	"math/rand"
	"testing"
)

const LEN = 4096

func TestAdler32(t *testing.T) {
	data := make([]byte, LEN)
	for i := 0; i < LEN; i++ {
		data[i] = byte(rand.Int())
	}
	//t.Logf("Data is: %x", data)

	for chunk := 1; chunk <= 2048; chunk++ {
		for run := 0; run <= LEN; run += chunk {
			var d uint32
			d = 1
			for i := 0; i <= LEN-chunk; i += chunk {
				if i >= run {
					d2 := Checksum(data[i-run : i])
					if uint32(d) != d2 {
						t.Fatalf("Failed for [%d..%d), d=%08x, d2=%08x chunk=%d", i-run, i, d, d2, chunk)
					}
					//t.Logf("Adler32 of [%6d..%6d) is %08x - %08x", i-run, i, d, d2)
				}
				d = pushBack(d, data[i:i+chunk])
				if i+chunk > run {
					d = popFront(d, data[i-run:i+chunk-run], run+chunk)
				}
			}
		}
	}
}

func TestOverflow(t *testing.T) {
	data := make([]byte, 65536)
	for k, _ := range data {
		data[k] = 255
	}
	for i := 17343; i <= 17343; i++ {
		d := uint32(1)
		d = pushBack(d, data[:i])
		if uint32(d) != adler32.Checksum(data[:i]) {
			t.Fatalf("pushBack seems to be wrong")
		}
		d = popFront(d, data[:i], i)
		if d != uint32(1) {
			t.Errorf("Overflow at length %d detected: d=%08x", i, d)
		}
	}
}

var data = generateSample()

func generateSample() []byte {
	result := make([]byte, 16384)
	for k, _ := range result {
		result[k] = byte(rand.Int())
	}
	return result
}

func BenchmarkPushBack(b *testing.B) {
	d := uint32(1)
	for i := 0; i < b.N; i++ {
		j := i & 16383
		pushBack(d, data[j:j+1])
	}
}

func BenchmarkPushBack16(b *testing.B) {
	d := uint32(1)
	for i := 0; i < b.N; i += 16 {
		j := i & 16383
		pushBack(d, data[j:j+16])
	}
}

func BenchmarkPopFront(b *testing.B) {
	d := uint32(1)
	for i := 0; i < b.N; i++ {
		j := i & 16383
		popFront(d, data[j:j+1], 33)
	}
}

func BenchmarkPopFront16(b *testing.B) {
	d := uint32(1)
	for i := 0; i < b.N; i += 16 {
		j := i & 16383
		popFront(d, data[j:j+16], 33)
	}
}
