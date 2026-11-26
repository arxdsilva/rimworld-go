// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package roof

import "slices"

// Risk is one roof tile that loses its last support if a selected
// support is removed. Thick roofs leave collapsed rock behind.
type Risk struct {
	X, Y  int
	Thick bool
}

func RemovalRisk(w, h, x, y int, cells []Cell, support func(int, int) bool) []Risk {
	if x < 0 || y < 0 || x >= w || y >= h || len(cells) != w*h || !support(x, y) {
		return nil
	}
	var out []Risk
	EachNear(w, h, x, y, func(nx, ny int) {
		c := cells[ny*w+nx]
		if !c.Roofed() {
			return
		}
		held := false
		EachNear(w, h, nx, ny, func(sx, sy int) {
			if !held && (sx != x || sy != y) && support(sx, sy) {
				held = true
			}
		})
		if !held {
			out = append(out, Risk{nx, ny, c.Thick()})
		}
	})
	return out
}

// NoteLoss keeps the natural-roof candidates after an actual support
// removal. Old maps may contain unsupported natural roofs; only a real
// support-loss event arms them, not a read-model rebuild or old save load.
func NoteLoss(old []int32, w, h, x, y int, cells []Cell) []int32 {
	if len(cells) != w*h {
		return old
	}
	EachNear(w, h, x, y, func(nx, ny int) {
		i := ny*w + nx
		if cells[i].Has(Natural) {
			at, ok := slices.BinarySearch(old, int32(i))
			if !ok {
				old = slices.Insert(old, at, int32(i))
			}
		}
	})
	return old
}

// Falling returns the stable set of unsupported roofs for one climate
// pass. Natural roofs need a real loss event; built ones always need support.
func Falling(cells []Cell, loss []int32) []int {
	var out []int
	for i, c := range cells {
		if c.Has(Supported) || !c.Roofed() {
			continue
		}
		if c.Has(Natural) {
			if _, ok := slices.BinarySearch(loss, int32(i)); !ok {
				continue
			}
		}
		out = append(out, i)
	}
	return out
}

func Removable(c Cell) bool { return c.Roofed() && !c.Thick() }
func Designate(c Cell, wall, build bool) Cell {
	if build {
		c &^= PlanRemove | NoAuto
		if !wall && !c.Has(Natural) {
			c |= AreaBuild
		}
		if !wall && c.Has(Supported) && !c.Roofed() {
			c |= PlanBuild
		}
	} else {
		c = c&^(PlanBuild|AreaBuild) | NoAuto
		if Removable(c) {
			c |= PlanRemove
		}
	}
	return c
}

func AreaPlan(c Cell, wall bool) Cell {
	if !wall && c.Has(AreaBuild|Supported) && !c.Roofed() && !c.Has(PlanRemove) {
		c |= PlanBuild
	}
	return c
}
func WithSupport(c Cell, supported, wall bool) Cell {
	c &^= Supported
	if supported {
		c |= Supported
	}
	if wall {
		c &^= PlanBuild
	}
	return AreaPlan(c, wall)
}

func NormalizeLoss(v []int32, n int) []int32 {
	v = slices.DeleteFunc(v, func(i int32) bool { return i < 0 || int(i) >= n })
	slices.Sort(v)
	v = slices.Compact(v)
	if len(v) == 0 {
		return nil
	}
	return v
}

func AutoPlan(c Cell) Cell {
	if !c.Roofed() && c.Has(Supported) && c&(PlanBuild|PlanRemove|NoAuto) == 0 {
		return c | PlanBuild
	}
	return c
}

func IsSupported(w, h, x, y int, support func(int, int) bool) bool {
	for sy := max(0, y-SupportRange); sy < min(h, y+SupportRange+1); sy++ {
		for sx := max(0, x-SupportRange); sx < min(w, x+SupportRange+1); sx++ {
			if InRange(sx-x, sy-y) && support(sx, sy) {
				return true
			}
		}
	}
	return false
}
