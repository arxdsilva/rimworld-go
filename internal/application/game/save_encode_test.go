// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game_test

import (
	"bytes"
	"io"
	"reflect"
	"runtime"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
)

// The autosave queue (filesave.Queue) freezes owned save values on the
// simulation goroutine and serializes, canonicalizes and compresses on its worker:
// the resulting file must be the one Save writes.

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

// BenchmarkSaveEncode500 measures complete synchronous canonical encoding,
// retaining the original workload for historical comparisons. The autosave
// frame now runs BenchmarkSaveFreeze500 instead (target ≤ 8 ms).
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

// BenchmarkSaveCapture500 retains the previous on-frame export and Gob
// serialization workload for comparison with the detached capture.
func BenchmarkSaveCapture500(b *testing.B) {
	g := loadFixture(b, 500, false)
	var reuse []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := game.CaptureSaveAs(g, savefmt.Stamp{Name: "bench"}, reuse)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(c.Size()))
		reuse = c.Recycle()
	}
}

// BenchmarkSaveCanonical500 measures the worker's canonicalization alone,
// with the same captured input and reusable output buffer.
func BenchmarkSaveCanonical500(b *testing.B) {
	g := loadFixture(b, 500, false)
	c, err := game.CaptureSaveAs(g, savefmt.Stamp{Name: "bench"}, nil)
	if err != nil {
		b.Fatal(err)
	}
	var reuse []byte
	b.ReportAllocs()
	b.SetBytes(int64(c.Size()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reuse, err = c.CanonicalBytes(reuse)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSaveCompress500 is the worker's final step: gzip of canonical
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

// BenchmarkSaveFreeze500 measures the current queue's caller-side capture.
// It includes every mutable copy, but excludes worker serialization.
func BenchmarkSaveFreeze500(b *testing.B) {
	g := loadFixture(b, 500, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := game.FreezeSaveAs(g, savefmt.Stamp{Name: "bench"})
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(f)
	}
}

// BenchmarkSaveWorkerEncode500 measures standard Gob encoding of the
// frozen values, with the same reusable buffer as the queue.
func BenchmarkSaveWorkerEncode500(b *testing.B) {
	g := loadFixture(b, 500, false)
	f, err := game.FreezeSaveAs(g, savefmt.Stamp{Name: "bench"})
	if err != nil {
		b.Fatal(err)
	}
	var reuse []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := f.Encode(reuse)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(c.Size()))
		reuse = c.Recycle()
	}
}
