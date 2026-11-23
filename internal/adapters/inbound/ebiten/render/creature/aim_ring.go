// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package creature

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// SHT-204: a ring on a ranged attacker sweeps PawnView.AttackCycle (the
// aim, then the cooldown, as one sweep). Procedural, no assets, drawn in
// the world pass; a free function because Set is at its method ceiling.

// AimRingMinTile is the smallest tile size (physical px) that draws rings.
const AimRingMinTile = WorkBarMinTile

const (
	aimRingR    = 0.42 // radius, tiles
	aimRingSegs = 24   // segments in a full sweep
)

// aimRingShade is the ring's dark under-stroke.
var aimRingShade = color.RGBA{R: 0x0E, G: 0x0F, B: 0x12, A: 0xD0}

// AimRing is one ring: its centre in tiles and how far it has swept, 0..1.
type AimRing struct {
	X, Y  float64
	Sweep float64
}

// AimRings appends a ring for each in-view pawn holding a ranged weapon
// that is aiming with 0 < AttackCycle < 1.
func AimRings(cull *Cull, pawns []game.PawnView, dst []AimRing) []AimRing {
	for i := range pawns {
		p := &pawns[i]
		if p.Action != game.ActionAim || p.AttackCycle <= 0 || p.AttackCycle >= 1 {
			continue
		}
		if p.Weapon == "" || p.WeaponStats.Melee || !cull.InView(p.X, p.Y, 1, 1) {
			continue
		}
		dst = append(dst, AimRing{X: float64(p.X) + 0.5, Y: float64(p.Y) + 0.5, Sweep: p.AttackCycle})
	}
	return dst
}

// ringPoint is the arc point at fraction f of a turn, from the top, clockwise.
func ringPoint(cx, cy, r float32, f float64) (float32, float32) {
	ang := f * 2 * math.Pi
	return cx + r*float32(math.Sin(ang)), cy - r*float32(math.Cos(ang))
}

// ringSegs is the number of segments an arc of this sweep is drawn in.
func ringSegs(sweep float64) int { return max(int(math.Ceil(sweep*aimRingSegs)), 1) }

// ringSeg is segment s (1-based) of an arc of this sweep: its end point.
func ringSeg(cx, cy, r float32, s int, sweep float64) (float32, float32) {
	return ringPoint(cx, cy, r, min(float64(s)/aimRingSegs, sweep))
}

// DrawAimRings strokes each ring as an arc from the top, clockwise.
func DrawAimRings(v *render.View, cull *Cull, screen *ebiten.Image, pawns []game.PawnView, tile float64) {
	if tile < AimRingMinTile {
		return
	}
	var buf [16]AimRing
	r := float32(aimRingR * tile)
	w := max(float32(tile*0.06), 1.5)
	for _, a := range AimRings(cull, pawns, buf[:0]) {
		cx, cy := v.WorldToScreenF(a.X, a.Y)
		px, py := ringPoint(cx, cy, r, 0)
		for s := 1; s <= ringSegs(a.Sweep); s++ {
			nx, ny := ringSeg(cx, cy, r, s, a.Sweep)
			vector.StrokeLine(screen, px, py, nx, ny, w+2, aimRingShade, false)
			vector.StrokeLine(screen, px, py, nx, ny, w, uikit.Accent, false)
			px, py = nx, ny
		}
	}
}
