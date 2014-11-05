package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/indyjo/cafs/chunking"
	"io"
	"os"
)

const APP_VERSION = "0.1"

// The flag package provides a default help printer via -h switch
var versionFlag *bool = flag.Bool("v", false, "Print the version number.")

func main() {
	flag.Parse() // Scan the arguments list

	if *versionFlag {
		fmt.Println("Version:", APP_VERSION)
	}

	for _, arg := range flag.Args() {
		if err := chunkFile(arg); err != nil {
			fmt.Println("Failed: ", err)
		}
	}
}

type Handprint struct {
	Fingerprints [][]byte
}

func NewHandprint() *Handprint {
	return &Handprint{make([][]byte, 0, 5)}
}

func (h *Handprint) String() string {
	result := make([]byte, 0, 20)
	for _, v := range h.Fingerprints {
		result = append(result, []byte(fmt.Sprintf("%4x", v[:2]))...)
	}
	return string(result)
}

func (h *Handprint) Insert(fingerprint []byte) {
	for i, other := range h.Fingerprints {
		if bytes.Compare(other, fingerprint) <= 0 {
			continue
		}
		fingerprint, h.Fingerprints[i] = other, fingerprint
	}
	if len(h.Fingerprints) < cap(h.Fingerprints) {
		h.Fingerprints = append(h.Fingerprints, fingerprint)
	}
}

func chunkFile(filename string) error {
	handprint := NewHandprint()
	//fmt.Printf("Chunking %s\n", filename)
	fi, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fi.Close()

	buffer := make([]byte, 16384)
	chunker := chunking.New()

	numChunks := 1
	numBytes := 0
	sha := sha256.New()
	chunkLen := 0
	for {
		n, err := fi.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			handprint.Insert(sha.Sum(make([]byte, 0, 32)))
			break
		}
		numBytes += n
		slice := buffer[:n]
		for len(slice) > 0 {
			bytesInChunk := chunker.Scan(slice)
			chunkLen += bytesInChunk
			sha.Write(slice[:bytesInChunk])
			if bytesInChunk < len(slice) {
				handprint.Insert(sha.Sum(make([]byte, 0, 32)))
				// fmt.Printf("%032x %d\n", shavalue, chunkLen)
				sha.Reset()
				chunkLen = 0
				numChunks++
			}
			slice = slice[bytesInChunk:]
		}
	}

	//fmt.Printf("Generated %d chunks on avg %d bytes long.\n", numChunks, numBytes/numChunks)
	fmt.Printf("%-20s %s\n", handprint, filename)
	return nil
}
