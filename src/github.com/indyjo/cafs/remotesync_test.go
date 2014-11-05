package cafs

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

type logWriter struct {
	w io.Writer
	t *testing.T
}

func (l logWriter) Write(p []byte) (n int, err error) {
	n, err = l.w.Write(p)
	if len(p) > 24 {
		l.t.Logf("Wrote %d bytes (err:%v): %x...", n, err, p[:24])
	} else {
		l.t.Logf("Wrote %d bytes (err:%v): %x", n, err, p)
	}
	return
}

func TestRemoteSync(t *testing.T) {
	storeA := NewRamStorage(8 * 1024 * 1024)
	storeB := NewRamStorage(8 * 1024 * 1024)

	for _, p := range []float64{0, 0.01, 0.05, 0.25, 0.5, 0.75, 0.95, 0.99, 1} {
		for _, nBlocks := range []int{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512} {
			testWithParams(t, storeA, storeB, p, nBlocks)
			storeA.DumpStatistics(logPrinter{})
		}
	}
}

func testWithParams(t *testing.T, storeA, storeB FileStorage, p float64, nBlocks int) {
	t.Logf("Testing with params: p=%f, nBlocks=%d", p, nBlocks)
	tempA := storeA.Create(fmt.Sprintf("Data A(%.2f,%d)", p, nBlocks))
	defer tempA.Dispose()
	tempB := storeB.Create(fmt.Sprintf("Data B(%.2f,%d)", p, nBlocks))
	defer tempB.Dispose()

	createSimilarData(tempA, tempB, p, 8192, nBlocks)
	tempA.Close()
	tempB.Close()
	if true {
		return
	}

	fileA := tempA.File()
	defer fileA.Dispose()

	// task: transfer file A to storage B
	buf := bytes.NewBuffer(nil)
	WriteChunkHashes(fileA, buf)

	builder, _ := NewBuilder(storeB, buf, fmt.Sprintf("Recovered A(%.2f,%d)", p, nBlocks))
	defer builder.Dispose()

	buf.Reset()
	if err := builder.WriteWishList(buf); err != nil {
		t.Fatalf("Error generating wishlist: %v", err)
	}

	t.Logf("Wishlist: %x", buf.Bytes())

	buf2 := bytes.NewBuffer(nil)
	if err := WriteRequestedChunks(fileA, buf.Bytes(), buf2); err != nil {
		t.Fatalf("Error encoding requested chunks: %v", err)
	}

	if buf2.Len() > 24 {
		t.Logf("First 24 bytes of requested chunks: %x...", buf2.Bytes()[:24])
	} else {
		t.Logf("Requested chunks: %x...", buf2.Bytes())
	}

	var fileB File
	if f, err := builder.ReconstructFileFromRequestedChunks(buf2); err != nil {
		t.Fatalf("Error reconstructing: %v", err)
	} else {
		fileB = f
		defer f.Dispose()
	}

	assertEqual(t, fileA.Open(), fileB.Open())
}

func assertEqual(t *testing.T, a, b io.Reader) {
	bufA := make([]byte, 1)
	bufB := make([]byte, 1)
	for {
		nA, errA := a.Read(bufA)
		nB, errB := b.Read(bufB)
		if nA != nB {
			t.Fatal("Files differ in size")
		}
		if errA == io.EOF && errB == io.EOF {
			break
		}
		if errA != errB {
			t.Fatalf("Error a:%v b:%v", errA, errB)
		}
		if bufA[0] != bufB[0] {
			t.Fatal("Files differ in content")
		}
	}
}

func createSimilarData(tempA, tempB io.Writer, p, avgchunk float64, numchunks int) {
	for numchunks > 0 {
		numchunks--
		lengthA := int(avgchunk*rand.NormFloat64()/4 + avgchunk)
		if lengthA < 16 {
			lengthA = 16
		}
		data := randomBytes(lengthA)
		tempA.Write(data)
		same := rand.Float64() <= p
		if same {
			tempB.Write(data)
		} else {
			lengthB := int(avgchunk*rand.NormFloat64()/4 + avgchunk)
			if lengthB < 16 {
				lengthB = 16
			}
			data = randomBytes(lengthB)
			tempB.Write(data)
		}
	}
}

func randomBytes(length int) []byte {
	result := make([]byte, 0, length)
	for len(result) < length {
		result = append(result, byte(rand.Int()))
	}
	return result
}
