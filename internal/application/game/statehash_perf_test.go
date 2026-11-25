// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"rimworld-go/internal/domain/sim/statehash"
	"testing"
)

func BenchmarkStateHashStages(b *testing.B) {
	for _, size := range []struct {
		name string
		side int
	}{{"64", 64}, {"160", 160}} {
		b.Run(size.name, func(b *testing.B) {
			g := NewColony(Config{Seed: 42, Width: size.side, Height: size.side})
			g.Step(180)
			var raw bytes.Buffer
			if err := EncodeSave(g, &raw, "statehash"); err != nil {
				b.Fatal(err)
			}
			data := raw.Bytes()
			var header SaveInfo
			var saved saveState
			dec := gob.NewDecoder(bytes.NewReader(data))
			if err := dec.Decode(&header); err != nil {
				b.Fatal(err)
			}
			if err := dec.Decode(&saved); err != nil {
				b.Fatal(err)
			}
			fields := normalisedFields(header, &saved)
			b.Run("full", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = g.StateHash()
				}
			})
			b.Run("gob_round_trip", func(b *testing.B) {
				var buf bytes.Buffer
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					buf.Reset()
					if err := EncodeSave(g, &buf, "statehash"); err != nil {
						b.Fatal(err)
					}
					var info SaveInfo
					var state saveState
					d := gob.NewDecoder(bytes.NewReader(buf.Bytes()))
					if err := d.Decode(&info); err != nil {
						b.Fatal(err)
					}
					if err := d.Decode(&state); err != nil {
						b.Fatal(err)
					}
					_ = statehash.OfFields(normalisedFields(info, &state))
				}
			})
			b.Run("encode", func(b *testing.B) {
				var buf bytes.Buffer
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					buf.Reset()
					if err := EncodeSave(g, &buf, "statehash"); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("decode_normalize", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var x SaveInfo
					var s saveState
					d := gob.NewDecoder(bytes.NewReader(data))
					if err := d.Decode(&x); err != nil {
						b.Fatal(err)
					}
					if err := d.Decode(&s); err != nil {
						b.Fatal(err)
					}
					_ = normalisedFields(x, &s)
				}
			})
			b.Run("canonical_fnv", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = statehash.OfFields(fields)
				}
			})
		})
	}
}
