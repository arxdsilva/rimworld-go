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

// TestInsideChildren: insideChildren is the brute-force scan's set (the
// present inside parts other than p whose parent is p) for every part of
// every plan, with and without missing parts, and empty for an invalid p.
func TestInsideChildren(t *testing.T) {
	t.Parallel()
	for a := Anatomy(0); a <= NumAnatomies; a++ {
		for _, missing := range []uint32{0, 0x5555_5555, 0xaaaa_aaaa, ^uint32(0)} {
			b := Body{Anatomy: a, Missing: missing}
			tab := b.table()
			for p := Part(0); p <= b.numParts()+1; p++ {
				var want uint32
				for q := Part(0); q < b.numParts(); q++ {
					if q != p && tab[q].Parent == p && !b.isMissing(q) && inside(&b, q) {
						want |= bit(q)
					}
				}
				if got := insideChildren(&b, p); got != want {
					t.Errorf("%v missing %x %v: %b, want %b", a, missing, p, got, want)
				}
			}
		}
	}
	q := Body{Anatomy: Quadruped}
	if insideChildren(&q, QHead) != bit(QBrain) || insideChildren(&q, QBrain) != 0 {
		t.Error("a quadruped's head holds its brain")
	}
}

// TestOutwardChain: for every part of every plan the chain is p, then
// each parent while the part is inside, ending on the first outer part;
// an invalid part gives no chain.
func TestOutwardChain(t *testing.T) {
	t.Parallel()
	for a := Anatomy(0); a <= NumAnatomies; a++ {
		b := Body{Anatomy: a}
		tab := b.table()
		for p := Part(0); p < b.numParts(); p++ {
			ps, n := outwardChain(&b, p)
			if n < 1 || n > maxShares || ps[0] != p || inside(&b, ps[n-1]) {
				t.Fatalf("%v %v: chain %v", a, p, ps[:n])
			}
			for i := range n - 1 {
				if !inside(&b, ps[i]) || ps[i+1] != tab[ps[i]].Parent {
					t.Fatalf("%v %v: chain %v at %d", a, p, ps[:n], i)
				}
			}
			if (n == 1) == inside(&b, p) {
				t.Fatalf("%v %v: chain %v", a, p, ps[:n])
			}
		}
		for _, p := range []Part{b.numParts(), 200} {
			if _, n := outwardChain(&b, p); n != 0 {
				t.Fatalf("%v invalid %v: n %d", a, p, n)
			}
		}
	}
	var h Body
	if ps, n := outwardChain(&h, Heart); n != 2 || ps[0] != Heart || ps[1] != Torso {
		t.Fatalf("heart: %v", ps[:n])
	}
	q := Body{Anatomy: Quadruped}
	if ps, n := outwardChain(&q, QBrain); n != 2 || ps[1] != QHead {
		t.Fatalf("quadruped brain: %v", ps[:n])
	}
}
