// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package roof holds the per-tile roof model (SIM-018): which tiles are
// roofed, by colonists or by overhead rock, which are planned for building
// or removal, and which a wall, door or rock holds up. It is headless and
// deterministic; the game owns the grid and the jobs that change it.
package roof

// Cell is the roof state of one tile, a set of flags.
type Cell uint8

const (
	// Built: a colonist-built roof.
	Built Cell = 1 << iota
	// Natural: thin overhead rock, removable and collapsible.
	Natural
	// PlanBuild: a roof is planned here (by the player or auto-roofing).
	PlanBuild
	// PlanRemove: the built roof here is planned for removal.
	PlanRemove
	// Supported: a support (wall, door or rock) lies within SupportRange.
	// Derived on every room rebuild; never saved as authoritative.
	Supported
	// NoAuto: the player removed this roof; auto-roofing leaves it off.
	NoAuto
	// NaturalThick (with Natural): deep overhead rock from map generation
	// (MAPGEN-2). Nothing drops through it and no tool removes it.
	NaturalThick
	// AreaBuild (PLAY-015): the build-roof area. Colonists keep a roof
	// built here (rebuilding after a collapse) until the area is cleared.
	AreaBuild
)

const (
	// SupportRange is how far (circular tile radius) a roof reaches from the
	// nearest support.
	SupportRange = 6
	// AutoMaxCells is the largest room colonists roof on their own.
	AutoMaxCells = 150
	// BuildWork and RemoveWork are the work ticks per tile.
	BuildWork  = 30
	RemoveWork = 15
)

// Roofed reports any roof, built or natural.
func (c Cell) Roofed() bool { return c&(Built|Natural) != 0 }

// Thick reports deep natural rock overhead.
func (c Cell) Thick() bool { return c.Has(Natural | NaturalThick) }

// Has reports every flag of f set.
func (c Cell) Has(f Cell) bool { return c&f == f }

// InRange is the same support footprint used by the field and previews.
func InRange(dx, dy int) bool { return dx*dx+dy*dy <= SupportRange*SupportRange }

// EachNear visits the circular radius in stable row-major order.
func EachNear(w, h, x, y int, visit func(int, int)) {
	for ny := max(0, y-SupportRange); ny <= min(h-1, y+SupportRange); ny++ {
		for nx := max(0, x-SupportRange); nx <= min(w-1, x+SupportRange); nx++ {
			if InRange(nx-x, ny-y) {
				visit(nx, ny)
			}
		}
	}
}

// SupportField computes exact circular distance. Two horizontal passes
// supply nearest-row distances; a bounded vertical pass needs at most
// 13 row checks per cell, independent of the number of supports.
func SupportField(w, h int, support func(int, int) bool) []bool {
	if w <= 0 || h <= 0 {
		return []bool{}
	}
	out := make([]bool, w*h)
	dist := make([]uint8, w*h)
	anySupport := false
	for y := 0; y < h; y++ {
		d := SupportRange + 1
		for x := 0; x < w; x++ {
			if support(x, y) {
				anySupport = true
				d = 0
			} else {
				d = min(d+1, SupportRange+1)
			}
			dist[y*w+x] = uint8(d)
		}
		d = SupportRange + 1
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			d = min(d+1, int(dist[i]))
			dist[i] = uint8(d)
		}
	}
	if !anySupport {
		return out
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if int(dist[i]) <= SupportRange {
				out[i] = true
				continue
			}
			for dy := 1; dy <= SupportRange && !out[i]; dy++ {
				for _, ny := range [2]int{y - dy, y + dy} {
					if ny >= 0 && ny < h && InRange(int(dist[ny*w+x]), dy) {
						out[i] = true
						break
					}
				}
			}
		}
	}
	return out
}
