// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package creature

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

func ringPawn(action string, cycle float64, melee bool, x int) game.PawnView {
	p := game.PawnView{X: x, Y: 3, Action: action, AttackCycle: cycle, Weapon: "rifle"}
	p.WeaponStats.Melee = melee
	return p
}

func TestAimRingSelects(t *testing.T) {
	cull := &Cull{ViewSet: true, X0: 0, Y0: 0, X1: 20, Y1: 20}
	got := AimRings(cull, []game.PawnView{ringPawn(game.ActionAim, 0.4, false, 5)}, nil)
	if len(got) != 1 || got[0].Sweep != 0.4 || got[0].X != 5.5 || got[0].Y != 3.5 {
		t.Fatalf("ranged 0.4: %+v", got)
	}
	unarmed := ringPawn(game.ActionAim, 0.4, false, 5)
	unarmed.Weapon = ""
	for name, p := range map[string]game.PawnView{
		"melee":    ringPawn(game.ActionMelee, 0.4, true, 5),
		"meleeGun": ringPawn(game.ActionAim, 0.4, true, 5),
		"idle":     ringPawn("", 0.4, false, 5),
		"zero":     ringPawn(game.ActionAim, 0, false, 5),
		"one":      ringPawn(game.ActionAim, 1, false, 5),
		"culled":   ringPawn(game.ActionAim, 0.4, false, 90),
		"unarmed":  unarmed,
	} {
		if r := AimRings(cull, []game.PawnView{p}, nil); len(r) != 0 {
			t.Errorf("%s: %+v", name, r)
		}
	}
}

func TestAimRingNoAlloc(t *testing.T) {
	cull := &Cull{ViewSet: true, X1: 20, Y1: 20}
	pawns := make([]game.PawnView, 16)
	for i := range pawns {
		pawns[i] = ringPawn(game.ActionAim, 0.5, false, i)
	}
	var buf [16]AimRing
	if n := testing.AllocsPerRun(100, func() { _ = AimRings(cull, pawns, buf[:0]) }); n != 0 {
		t.Fatalf("allocs = %v", n)
	}
}

func TestAimRingGeometry(t *testing.T) {
	near := func(a, b float32) bool { return a-b < 0.001 && b-a < 0.001 }
	if x, y := ringPoint(10, 10, 4, 0); !near(x, 10) || !near(y, 6) {
		t.Errorf("top = %v,%v", x, y)
	}
	if x, y := ringPoint(10, 10, 4, 0.25); !near(x, 14) || !near(y, 10) {
		t.Errorf("quarter = %v,%v", x, y)
	}
	if ringSegs(0.001) != 1 || ringSegs(0.5) != aimRingSegs/2 {
		t.Errorf("segs %d %d", ringSegs(0.001), ringSegs(0.5))
	}
	// the last segment ends exactly at the sweep, not a full step past it
	n := ringSegs(0.51)
	ex, ey := ringPoint(10, 10, 4, 0.51)
	if x, y := ringSeg(10, 10, 4, n, 0.51); n > 0 && !near(x, ex) || !near(y, ey) {
		t.Errorf("end = %v,%v want %v,%v", x, y, ex, ey)
	}
}

func TestAimRingDraw(t *testing.T) {
	var v render.View
	cull := &Cull{ViewSet: true, X1: 20, Y1: 20}
	img := ebiten.NewImage(64, 64)
	p := ringPawn(game.ActionAim, 0.5, false, 1)
	p.Y = 1
	DrawAimRings(&v, cull, img, []game.PawnView{p}, 8) // below the zoom floor: nothing
	DrawAimRings(&v, cull, img, []game.PawnView{p}, 32)
	DrawAimRings(&v, cull, img, nil, 32)
}
