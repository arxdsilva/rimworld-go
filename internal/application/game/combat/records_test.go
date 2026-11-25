// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF 3.1 records: blasts credit their thrower, and humanlikes keep the
// damage they took.

// lethalHost is fakeHost whose HurtHit kills the ids in lethal the way
// the game does: the fighter stops standing and Gear.Died runs inside
// the hurt (game dieBy), before the caller counts the blow.
type lethalHost struct {
	*fakeHost
	c      *Combat
	lethal map[entity.EntityID]bool
}

func (h *lethalHost) HurtHit(target entity.EntityID, l armor.Landed, by entity.EntityID) {
	h.fakeHost.HurtHit(target, l, by)
	if h.lethal[target] {
		delete(h.pawns, target)
		delete(h.raiders, target)
		h.c.Gear.Died(target, by)
	}
}

func lethalRig(w, ht int) *lethalHost {
	f := newFake(w, ht)
	h := &lethalHost{fakeHost: f, lethal: map[entity.EntityID]bool{}}
	h.c = New(h, Roles{MortarAim: f, Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Draft: f})
	return h
}

// TestBlastCreditsAttacker: a frag grenade's landed damage is its
// thrower's, and the raider it kills is one kill, counted once.
func TestBlastCreditsAttacker(t *testing.T) {
	t.Parallel()
	h := lethalRig(20, 20)
	c := h.c
	h.pawn(1, 2, 2, weapon.FragGrenade)
	h.raider(2, 10, 10, weapon.Fists) // on the centre: killed
	h.raider(3, 11, 11, weapon.Fists) // 1.41 tiles: hurt
	h.lethal[2] = true
	c.Blasts.Explode(1, weapon.FragGrenade, 10, 10, fragBlast)
	d2, _, _ := hurtOf(h.fakeHost, 2)
	d3, _, _ := hurtOf(h.fakeHost, 3)
	if v := c.Gear.View(1); d2 == 0 || d3 == 0 || v.Damage != d2+d3 || v.Kills != 1 {
		t.Fatalf("thrower's records %+v, landed %d + %d", v, d2, d3)
	}
	c.Gear.Died(2, 1) // the game's death hook again: no second kill
	c.Gear.Died(2, 0)
	if v := c.Gear.View(1); v.Kills != 1 {
		t.Fatalf("the killing blast counts once: %+v", v)
	}
}

// TestBlastSetsLastBy: a blast that leaves its target alive makes the
// thrower its last attacker (a death of those wounds is the thrower's
// kill); an unowned blast or the thrower's own leaves it as it was.
func TestBlastSetsLastBy(t *testing.T) {
	t.Parallel()
	h := lethalRig(20, 20)
	c := h.c
	h.pawn(1, 2, 2, weapon.FragGrenade)
	h.raider(3, 11, 11, weapon.Fists)
	c.Blasts.Explode(1, weapon.FragGrenade, 10, 10, fragBlast)
	if c.Gear.LastBy(3) != 1 {
		t.Fatalf("last by %d, want the thrower", c.Gear.LastBy(3))
	}
	c.Gear.Died(3, 0) // bled out
	c.Gear.Died(3, 0)
	if v := c.Gear.View(1); v.Kills != 1 {
		t.Fatalf("a death of blast wounds is one kill: %+v", v)
	}
	h.raider(4, 11, 10, weapon.FragGrenade)
	c.Gear.Count(1, 4, 5, false)
	c.Blasts.Explode(0, weapon.FragGrenade, 10, 10, fragBlast) // an IED
	c.Blasts.Explode(4, weapon.FragGrenade, 10, 10, fragBlast) // its own grenade
	if c.Gear.LastBy(4) != 1 {
		t.Fatalf("unowned and own blasts moved last by to %d", c.Gear.LastBy(4))
	}
}

// TestBlastUnownedAndSelfCredit: an unowned blast (an IED, a cook-off)
// and a fighter's own grenade credit no attacker, yet the humanlikes
// they hurt record the damage taken.
func TestBlastUnownedAndSelfCredit(t *testing.T) {
	t.Parallel()
	h := lethalRig(20, 20)
	c := h.c
	h.raider(3, 11, 11, weapon.FragGrenade)
	c.Blasts.Explode(0, weapon.FragGrenade, 10, 10, fragBlast)
	d, _, _ := hurtOf(h.fakeHost, 3)
	if v := c.Gear.View(0); v.Damage != 0 || v.Kills != 0 || d == 0 || c.Gear.View(3).Taken != d {
		t.Fatalf("unowned: nobody %+v, target took %d of %d", v, c.Gear.View(3).Taken, d)
	}
	c.Blasts.Explode(3, weapon.FragGrenade, 10, 10, fragBlast)
	if v := c.Gear.View(3); v.Damage != 0 || v.Kills != 0 || v.Taken != 2*d || c.Gear.LastBy(3) != 0 {
		t.Fatalf("own grenade: %+v, last by %d", v, c.Gear.LastBy(3))
	}
}

// TestDamageTakenHumanlikeOnly: strikes, shots and blasts record what a
// humanlike took after armor; an animal or a mechanoid (both live in the
// game's animal store and die through killAnimal, never Gear.Died)
// records nothing, so no kit is left pinned for them.
func TestDamageTakenHumanlikeOnly(t *testing.T) {
	t.Parallel()
	h := lethalRig(40, 20)
	c, f := h.c, h.fakeHost
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	c.Strikes.Strike(1, 2, weapon.Rifle) // a shot
	settle(c)
	shot, _, _ := hurtOf(f, 2)
	if shot == 0 || c.Gear.View(2).Taken != shot {
		t.Fatalf("a shot: took %d of %d", c.Gear.View(2).Taken, shot)
	}
	f.raider(3, 3, 5, weapon.Club)
	f.rolls = []int{0, 0, 0}
	c.Strikes.Strike(3, 1, weapon.Club) // a melee blow on the colonist
	blow, _, _ := hurtOf(f, 1)
	if blow == 0 || c.Gear.View(1).Taken != blow || c.Gear.View(3).Damage != blow {
		t.Fatalf("a blow: took %d, dealt %d, landed %d", c.Gear.View(1).Taken, c.Gear.View(3).Damage, blow)
	}
	f.beast(4, 30, 10, animal.State{Species: animal.TimberWolf})
	f.beast(5, 31, 10, animal.State{Species: animal.Harrow})
	f.pawn(6, 30, 11, weapon.Fists)
	c.Blasts.Explode(1, weapon.FragGrenade, 30, 10, fragBlast)
	_, _, wolf := hurtOf(f, 4)
	_, _, mech := hurtOf(f, 5)
	near, _, _ := hurtOf(f, 6)
	if wolf != 1 || mech != 1 || near == 0 {
		t.Fatalf("the blast caught wolf %d, mech %d, colonist %d", wolf, mech, near)
	}
	if c.Gear.View(4).Taken != 0 || c.Gear.View(5).Taken != 0 || c.Gear.View(6).Taken != near {
		t.Fatalf("taken: wolf %d, mech %d, colonist %d of %d", c.Gear.View(4).Taken, c.Gear.View(5).Taken, c.Gear.View(6).Taken, near)
	}
	for _, k := range c.Turrets.Export().Gear {
		if k.K.DamageTaken != 0 && (k.ID == 4 || k.ID == 5) {
			t.Fatalf("an animal's damage taken is saved: %+v", k)
		}
	}
}

// TestDiedDropsDamageTaken: a dead fighter's damage taken goes with its
// last attacker, so its kit is freed; the killing blow, counted after the
// game's death hook ran (dieBy inside HurtHit), adds none back.
func TestDiedDropsDamageTaken(t *testing.T) {
	t.Parallel()
	h := lethalRig(20, 20)
	c, f := h.c, h.fakeHost
	f.pawn(1, 2, 2, weapon.Rifle)
	f.raider(2, 10, 10, weapon.Fists)
	f.raider(3, 12, 12, weapon.Fists)
	c.Gear.Count(1, 2, 10, false)
	c.Gear.Count(0, 3, 7, false) // a trap's blow: no last attacker
	if c.Gear.View(2).Taken != 10 || c.Gear.View(3).Taken != 7 {
		t.Fatalf("taken %d, %d", c.Gear.View(2).Taken, c.Gear.View(3).Taken)
	}
	c.Gear.Died(2, 0)
	c.Gear.Died(3, 0)
	h.lethal[4] = true
	f.raider(4, 16, 16, weapon.Fists)
	c.Blasts.Explode(1, weapon.FragGrenade, 16, 16, fragBlast) // kills 4: Died runs first, then the count
	for _, k := range c.Turrets.Export().Gear {
		if k.ID != 1 {
			t.Fatalf("a dead fighter's kit is kept: %+v", k)
		}
	}
	if v := c.Gear.View(1); v.Kills != 2 {
		t.Fatalf("killer's records %+v", v)
	}
}
