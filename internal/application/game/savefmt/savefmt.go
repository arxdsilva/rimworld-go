// Package savefmt is the save file's container: a gzip stream around the
// game's gob encoding. The game encodes raw gob (it is what the state hash
// reads); a file on disk is that stream compressed.
package savefmt

import (
	"compress/gzip"
	"io"
)

// Level is the save's compression level. BestSpeed takes a fifth of the
// default level's time for a file about a sixth larger (NG-1 §6.8: a 500²
// save in ≤ 300 ms, ≤ 4 MB).
const Level = gzip.BestSpeed

// NewWriter compresses a save into w. Close it to flush the stream.
func NewWriter(w io.Writer) *gzip.Writer {
	zw, err := gzip.NewWriterLevel(w, Level)
	if err != nil { // only a bad level fails, and Level is a valid constant
		panic(err)
	}
	return zw
}

// NewReader decompresses a save file.
func NewReader(r io.Reader) (*gzip.Reader, error) { return gzip.NewReader(r) }
