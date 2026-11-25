// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package pen owns the deterministic livestock pen rules.
package pen

import (
	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/world/plant"
)

// DefaultSettings: every penned species, both sexes; auto-cut trees,
// cacti and brambles.
func DefaultSettings() mapview.PenSettings {
	var s mapview.PenSettings
	for _, sp := range animal.All() {
		if animal.RosterOf(sp).Penned {
			s.Species = append(s.Species, mapview.PenSpecies{Key: sp.String(), Allow: true, Male: true, Female: true})
		}
	}
	s.AutoCut = DefaultAutoCut()
	return s
}

// DefaultAutoCut is the auto-cut list a new pen starts with.
func DefaultAutoCut() []string {
	var out []string
	for s := plant.Species(0); ; s++ {
		d := plant.DefOf(s)
		if d.Name == "" {
			break
		}
		if d.Trunk || s == plant.Bramble || s == plant.Cactus || s == plant.BallCactus {
			out = append(out, d.Name)
		}
	}
	return out
}

// Accepts reports that settings s take an animal of species sp and sex
// female. A species the list doesn't name is taken.
func Accepts(s mapview.PenSettings, sp animal.Species, female bool) bool {
	key := sp.String()
	for _, e := range s.Species {
		if e.Key == key {
			return e.Allow && (female && e.Female || !female && e.Male)
		}
	}
	return true
}

// Region describes a bounded enclosure. Open regions have no cell bits.
type Region struct {
	Enclosed               bool
	Bits                   []uint64
	Cells                  int
	MinX, MinY, MaxX, MaxY int
}

// Fill floods a marker's four-connected standable cells. Reaching a map
// edge or exceeding the cell limit means open country, not an enclosure.
func Fill(x, y, w, h, limit int, stand func(int, int) bool) Region {
	var r Region
	if x < 0 || y < 0 || x >= w || y >= h || !stand(x, y) {
		return r
	}
	bits := make([]uint64, (w*h+63)/64)
	set := func(i int) { bits[i>>6] |= 1 << (uint(i) & 63) }
	has := func(i int) bool { return bits[i>>6]&(1<<(uint(i)&63)) != 0 }
	queue := []int32{int32(y*w + x)}
	set(y*w + x)
	r.MinX, r.MinY, r.MaxX, r.MaxY = x, y, x, y
	for qi := 0; qi < len(queue); qi++ {
		i := int(queue[qi])
		cx, cy := i%w, i/w
		if cx == 0 || cy == 0 || cx == w-1 || cy == h-1 || len(queue) > limit {
			return Region{}
		}
		r.MinX, r.MaxX = min(r.MinX, cx), max(r.MaxX, cx)
		r.MinY, r.MaxY = min(r.MinY, cy), max(r.MaxY, cy)
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			j := ny*w + nx
			if has(j) || !stand(nx, ny) {
				continue
			}
			set(j)
			queue = append(queue, int32(j))
		}
	}
	r.Enclosed, r.Bits, r.Cells = true, bits, len(queue)
	return r
}
