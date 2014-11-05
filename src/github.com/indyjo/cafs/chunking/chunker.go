package chunking

type Chunker interface {
	// Scans the byte sequence for chunk boundaries.
	// Returns the number of bytes from data that can be added to the current chunk.
	// A return value of len(data) means that no chunk boundary has been found in this block.
	Scan(data []byte) int
}
