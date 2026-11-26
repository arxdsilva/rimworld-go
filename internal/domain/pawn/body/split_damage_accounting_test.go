// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "testing"

// Pick a part by the same coverage intervals as the native planner.
func review269Pick(b *Body, p Part) int {
	r := 0
	for i, d := range b.table() {
		if Part(i) == p {
			return r
		}
		if !b.isMissing(Part(i)) {
			r += d.Coverage
		}
	}
	panic("missing part")
}
func review269MaskPick(b *Body, mask uint32, p Part) int {
	r := 0
	for i, d := range b.table() {
		if Part(i) == p {
			return r
		}
		if mask&bit(Part(i)) != 0 {
			r += d.Coverage
		}
	}
	panic("missing mask part")
}

// Both fixtures use the same quadruped Scale as an elephant (170 HP).
// All forced draws are valid native coverage draws.
func TestSplitDamageAncestorAccounting(t *testing.T) {
	b := Body{Anatomy: Quadruped, Scale: 1700}
	leg, paw := b.maxHP(QLegHR), b.maxHP(QPawHR)
	b.hurtPart(QLegHR, leg-1, Cut, 1)
	b.hurtPart(QPawHR, paw-1, Cut, 1)
	before := lossDamage(&b)
	room := killAt(&b) - before
	pick := review269Pick(&b, QLegHR)
	extra := review269MaskPick(&b, cutCandidates(&b, QLegHR), QPawHR)
	draws := []int{pick, 30, extra, review269Pick(&b, QTorso)}
	i := 0
	roll := func(n int) int {
		if i >= len(draws) || draws[i] >= n {
			t.Fatalf("draw %d: %v bound %d", i, draws, n)
		}
		r := draws[i]
		i++
		return r
	}
	got := SpreadBlowWithin(&b, 12, Cut, SplitCut, room, roll, 2)
	if b.Missing&bit(QLegHR) == 0 || b.Missing&bit(QPawHR) == 0 {
		t.Fatal("fixture did not destroy leg and paw")
	}
	total := cutTotal(12, 2)
	// Exactly one new point suffices to destroy the leg, leaving the rest
	// of the planned new blow for spill. The old paw wound is not a new hit.
	t.Logf("room=%d legHP=%d pawHP=%d planned=%d returned=%d newTorso=%d actualNew=%d anatomicalDelta=%d draws=%d", room, leg, paw, total, got, b.partDmg(QTorso), 1+b.partDmg(QTorso), lossDamage(&b)-before, i)
	if got != total {
		t.Fatalf("returned %d != planned %d", got, total)
	}
	if b.partDmg(QTorso) != total-1 {
		t.Errorf("new hit budget %d, but leg+torso received %d", total, 1+b.partDmg(QTorso))
	}
}

func TestSplitDamageUnappliedLethalShare(t *testing.T) {
	b := Body{Anatomy: Quadruped, Scale: 1700}
	b.hurtPart(QHeart, b.maxHP(QHeart)-1, Cut, 1)
	// Injury may be higher than the body's score after starvation.
	// Base hit 10 is survivable; split total 20 is lethal at room 11.
	// Heart cannot take another point, so only the torso can take the
	// current planned share. The pending spill must not simply vanish.
	room := 11
	pick := review269Pick(&b, QHeart)
	torso := review269Pick(&b, QTorso)
	draws := []int{pick, torso}
	i := 0
	roll := func(n int) int {
		if i >= len(draws) || draws[i] >= n {
			t.Fatalf("draw %d: %v bound %d", i, draws, n)
		}
		r := draws[i]
		i++
		return r
	}
	got := SpreadBlowWithin(&b, 10, Gunshot, SplitBullet, room, roll, 2)
	t.Logf("room=%d returned=%d newTorso=%d planned=20 draws=%d", room, got, b.partDmg(QTorso), i)
	if got != 20 || b.partDmg(QTorso) != 20 || i != 2 {
		t.Errorf("lethal return %d while actual newly placed damage %d is below HP %d", got, b.partDmg(QTorso), room)
	}
}

// A deferred spill can itself exhaust the owner before all pending
// damage is placed. The dying animal must not receive another pick.
func TestSplitDamageSpillStopsAtOwnerHP(t *testing.T) {
	b := Body{Anatomy: Quadruped, Scale: 1700}
	b.hurtPart(QHeart, b.maxHP(QHeart)-1, Cut, 1)
	b.hurtPart(QLegHR, b.maxHP(QLegHR)-1, Cut, 1)
	draws := []int{review269Pick(&b, QHeart), review269Pick(&b, QLegHR)}
	i := 0
	roll := func(n int) int {
		if i >= len(draws) || draws[i] >= n {
			t.Fatalf("unexpected draw %d/%d", i, n)
		}
		r := draws[i]
		i++
		return r
	}
	got := SpreadBlowWithin(&b, 10, Gunshot, SplitBullet, 11, roll, 2)
	if got != 11 || b.partDmg(QTorso) != 10 || !b.isMissing(QLegHR) || i != 2 {
		t.Fatalf("lethal spill: returned %d torso %d leg missing %v draws %d", got, b.partDmg(QTorso), b.isMissing(QLegHR), i)
	}
}
