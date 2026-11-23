// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "testing"

// TestInsideMasks: the humanlike inside parts are the Inner organs; every
// inside part hangs from an outer part; the solid parts are the spines.
func TestInsideMasks(t *testing.T) {
	t.Parallel()
	var h Body
	for p := Part(0); p < NumParts; p++ {
		if inside(&h, p) != Inner(p) {
			t.Errorf("humanlike %v: inside %v, Inner %v", p, inside(&h, p), Inner(p))
		}
	}
	spines := [NumAnatomies]Part{Humanlike: Spine, Quadruped: QSpine, Bird: BSpine, Snake: SSpine, Lizard: LSpine}
	for a := Anatomy(0); a < NumAnatomies; a++ {
		b := Body{Anatomy: a}
		tab := b.table()
		if insideMask[a]>>len(tab) != 0 || solidMask[a]>>len(tab) != 0 {
			t.Errorf("%v: a mask bit past the plan", a)
		}
		for p := Part(0); p < b.numParts(); p++ {
			if inside(&b, p) && (inside(&b, tab[p].Parent) || tab[p].Parent == p) {
				t.Errorf("%v %v: inside under an inside part or the root", a, p)
			}
			if solid(&b, p) && !inside(&b, p) {
				t.Errorf("%v %v: solid but not inside", a, p)
			}
		}
		want := uint32(0)
		if a != Insect {
			want = bit(spines[a])
		}
		if solidMask[a] != want {
			t.Errorf("%v: solid %b, want %b", a, solidMask[a], want)
		}
	}
	for _, c := range []struct {
		b    Body
		p    Part
		want bool
	}{
		{Body{Anatomy: Quadruped}, QBrain, true}, {Body{Anatomy: Quadruped}, QHead, false},
		{Body{Anatomy: Insect}, INerveCord, true}, {Body{Anatomy: Insect}, ICarapace, false},
		{Body{}, Brain, false}, {Body{}, Part(40), false}, {Body{Anatomy: 99}, Heart, true},
	} {
		if got := inside(&c.b, c.p); got != c.want {
			t.Errorf("%v %v: inside %v", c.b.Anatomy, c.p, got)
		}
	}
	if solid(&h, Part(40)) || !solid(&h, Spine) || solid(&h, Heart) {
		t.Error("solid")
	}
	q := Body{Anatomy: Quadruped}
	for _, c := range []struct {
		b       *Body
		p, want Part
	}{{&h, Heart, Torso}, {&h, ArmL, ArmL}, {&h, Torso, Torso}, {&q, QBrain, QHead}, {&q, QLiver, QTorso}} {
		if got := outerOf(c.b, c.p); got != c.want {
			t.Errorf("outerOf(%v) = %v, want %v", c.p, got, c.want)
		}
	}
}
