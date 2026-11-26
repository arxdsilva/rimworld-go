// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package globe

import (
	"image/color"

	"rimworld-go/internal/domain/faction"
)

// territoryMix is how much of a faction's accent a land tile takes.
const territoryMix = 0.4

// territoryTint is the owner tint state, kept in planetGeom: the per-tile
// owner slice (shared read-only with the source), its version, and each
// faction's accent.
type territoryTint struct {
	owner  []int16
	ver    uint64
	accent []rgb
}

// tintOf is the colour c of land tile i with its owner's accent mixed in; an
// unowned tile or an owner out of range keeps c.
func tintOf(t *territoryTint, i int, c rgb) rgb {
	if i >= len(t.owner) {
		return c
	}
	f := int(t.owner[i])
	if f < 0 || f >= len(t.accent) {
		return c
	}
	return MixRGB(c, t.accent[f], territoryMix)
}

// SetTerritory tints the land by owning faction. owner is per tile (a negative
// value is unowned) and shared read-only; ver is its version and types the
// faction types by index. A nil owner turns the tint off. The same ver in the
// same on/off state does nothing; otherwise the tiles recolour once.
func (g *View) SetTerritory(owner []int16, ver uint64, types []faction.Type) {
	t := &g.geom.terr
	on := owner != nil
	if was := t.owner != nil; was == on && (!on || t.ver == ver) {
		return
	}
	t.owner, t.ver = owner, ver
	t.accent = t.accent[:0]
	if on {
		for _, ft := range types {
			a := Accent(ft)
			t.accent = append(t.accent, toRGB(color.RGBA(a)))
		}
	}
	g.recolour()
}
