// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plant

import "encoding/binary"

// GrassFingerprint reproduces the content-based version of a grass layer.
func GrassFingerprint(cells, growth []uint8) uint64 {
	h := uint64(0xcbf29ce484222325)
	mix := func(b []uint8) {
		for len(b) >= 8 {
			h = (h ^ binary.LittleEndian.Uint64(b)) * 0x100000001b3
			h ^= h >> 29
			b = b[8:]
		}
		for _, v := range b {
			h = (h ^ uint64(v)) * 0x100000001b3
		}
	}
	mix(cells)
	mix(growth)
	return h
}
