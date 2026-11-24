// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "testing"

func bulletShares(b *Body, p Part, dmg int) []share {
	sh := planSplit(b, p, dmg, SplitBullet, splitNever)
	return sh.s[:sh.n]
}

// TestBulletInsideBonus: a bullet of 37 into an inside part also deals 37
// to its parent, and into an outer part deals one share; no draw is made.
func TestBulletInsideBonus(t *testing.T) {
	t.Parallel()
	h, q := Body{}, Body{Anatomy: Quadruped}
	cases := []struct {
		name string
		b    *Body
		p    Part
		want []share
	}{
		{"heart", &h, Heart, []share{{Heart, 37}, {Torso, 37}}},
		{"spine", &h, Spine, []share{{Spine, 37}, {Torso, 37}}},
		{"quad brain", &q, QBrain, []share{{QBrain, 37}, {QHead, 37}}},
		{"quad heart", &q, QHeart, []share{{QHeart, 37}, {QTorso, 37}}},
		{"leg", &h, LegL, []share{{LegL, 37}}},
		{"torso", &h, Torso, []share{{Torso, 37}}},
	}
	for _, c := range cases {
		got := bulletShares(c.b, c.p, 37)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %v, want %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: %v, want %v", c.name, got, c.want)
			}
		}
	}
}

// TestBulletMissingParent: a missing parent takes no bonus.
func TestBulletMissingParent(t *testing.T) {
	t.Parallel()
	b := Body{Missing: bit(Torso)}
	if got := bulletShares(&b, Heart, 10); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

// TestBulletStopRule: a humanlike Heart destroyed by its share ends the
// blow before the Torso share; on a quadruped the QTorso share is dealt
// unless the body is dead.
func TestBulletStopRule(t *testing.T) {
	t.Parallel()
	h := Body{}
	StrikeBlow(&h, Heart, 37, Gunshot, SplitBullet, 0, 0, splitNever, 1)
	if !h.isMissing(Heart) || splitDmgs(&h)[Torso] != 0 {
		t.Fatalf("humanlike: heart missing %v, torso %d", h.isMissing(Heart), splitDmgs(&h)[Torso])
	}

	q := Body{Anatomy: Quadruped}
	StrikeBlow(&q, QHeart, 37, Gunshot, SplitBullet, 0, 0, splitNever, 1)
	dead := lossDamage(&q) >= killAt(&q)
	if got := splitDmgs(&q)[QTorso]; dead == (got > 0) {
		t.Fatalf("quad: dead %v but QTorso took %d", dead, got)
	}

	// a lightly scaled blow that leaves the heart alive deals both shares
	q = splitBigQuad()
	StrikeBlow(&q, QHeart, 37, Gunshot, SplitBullet, 0, 0, splitNever, 1)
	d := splitDmgs(&q)
	if d[QHeart] != 37 || d[QTorso] != 37 {
		t.Fatalf("big quad: %v", d)
	}
}

// TestBulletSpreadBlow: on the big quadruped an unplaced bullet totals 2x
// dmg for an inside pick and dmg for an outer one.
func TestBulletSpreadBlow(t *testing.T) {
	t.Parallel()
	for _, p := range []Part{QBrain, QHeart, QTorso, QHead} {
		b := splitBigQuad()
		want := 37
		if inside(&b, p) {
			want = 74
		}
		sh := planBullet(&b, p, 37)
		tot := 0
		for _, x := range sh.s[:sh.n] {
			tot += x.dmg
		}
		if tot != want {
			t.Fatalf("part %v: plan total %d, want %d", p, tot, want)
		}
	}
	for seed := uint64(1); seed <= 200; seed++ {
		probe := splitBigQuad()
		p, ok := probe.PickPart(splitPCG(seed))
		if !ok {
			t.Fatalf("seed %d: nothing to pick", seed)
		}
		want := 37
		if inside(&probe, p) {
			want = 74 // the parent is present on a fresh body
		}
		b := splitBigQuad()
		n := SpreadBlow(&b, 37, Gunshot, SplitBullet, splitPCG(seed), 1)
		tot := 0
		for _, v := range splitDmgs(&b) {
			tot += v
		}
		if n != want || tot != want || b.Damage() != want {
			t.Fatalf("seed %d part %v: returned %d, wounds %d, Damage %d, want %d", seed, p, n, tot, b.Damage(), want)
		}
	}
}

// TestBulletAllocs: planning and dealing a bullet allocates nothing.
func TestBulletAllocs(t *testing.T) { // not parallel: AllocsPerRun
	buf := make([]Wound, 0, 8)
	var b Body
	n := testing.AllocsPerRun(200, func() {
		b = Body{Wounds: buf[:0]}
		StrikeBlow(&b, Spine, 20, Gunshot, SplitBullet, 0, 0, splitNever, 1)
	})
	if n != 0 || len(b.Wounds) != 2 {
		t.Fatalf("%v allocs, %d wounds", n, len(b.Wounds))
	}
}
