// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import "testing"

var benchPreset Named
var benchSettings Settings
var benchPresetType Preset

func BenchmarkPresetLookup(b *testing.B) {
	for _, p := range []Preset{Wanderer, Settled, Seasoned, Merciless} {
		b.Run(p.Key(), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSettings = Of(p)
			}
		})
	}
}

func BenchmarkPresetMatch(b *testing.B) {
	s := Of(Seasoned)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchPresetType = Match(s)
	}
}

func BenchmarkPresetList(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v := Presets()
		benchPreset = v[0]
	}
}
