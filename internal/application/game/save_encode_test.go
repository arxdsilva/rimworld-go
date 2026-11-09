package game_test

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
)

// The autosave queue (filesave.Queue) encodes raw gob on the simulation's
// goroutine and compresses it on its worker: that file must be the one
// Save writes.

// TestCompressedEncodeLoadsAsSave: savefmt.Compress over EncodeSave's bytes
// loads to the state Save's file loads to.
func TestCompressedEncodeLoadsAsSave(t *testing.T) {
	t.Parallel()
	g := game.NewColony(game.Config{Seed: 7, Width: 48, Height: 48})
	g.Step(300)
	var direct bytes.Buffer
	if err := g.Save(&direct, "col"); err != nil {
		t.Fatal(err)
	}
	var raw, split bytes.Buffer
	if err := game.EncodeSave(g, &raw, "col"); err != nil {
		t.Fatal(err)
	}
	if err := savefmt.Compress(&split, raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	a, ia, err := game.Load(&direct)
	if err != nil {
		t.Fatal(err)
	}
	b, ib, err := game.Load(&split)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ia, ib) {
		t.Fatalf("save info %+v, split %+v", ia, ib)
	}
	if a.StateHash() != b.StateHash() || b.StateHash() != g.StateHash() {
		t.Fatalf("state hash: live %x, Save %x, EncodeSave+Compress %x", g.StateHash(), a.StateHash(), b.StateHash())
	}
}

// BenchmarkSaveEncode500 is the part of an autosave left on the frame: the
// raw gob of the grown 500² colony, into a buffer kept between saves as
// the queue keeps it (target ≤ 8 ms).
func BenchmarkSaveEncode500(b *testing.B) {
	g := loadFixture(b, 500, false)
	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := game.EncodeSave(g, &buf, "bench"); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(buf.Len()))
}

// BenchmarkSaveCompress500 is the queue worker's share: gzip of that raw
// gob (the file I/O is not counted).
func BenchmarkSaveCompress500(b *testing.B) {
	g := loadFixture(b, 500, false)
	var raw bytes.Buffer
	if err := game.EncodeSave(g, &raw, "bench"); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(raw.Len()))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := savefmt.Compress(io.Discard, raw.Bytes()); err != nil {
			b.Fatal(err)
		}
	}
}
