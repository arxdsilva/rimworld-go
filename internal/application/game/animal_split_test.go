// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/hit"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// CBT 3.4: an animal's cut or blunt blow is split on its body
// (body.SpreadBlowWithin at its hit points, woundAnimal); a fixture blow is not.

// twin is a copy of stream r: the same draws from here on.
func twin(t *testing.T, r *randx.Rand) *randx.Rand {
	t.Helper()
	st, err := r.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	c := randx.New(0)
	if err := c.UnmarshalBinary(st); err != nil {
		t.Fatal(err)
	}
	return c
}

// sameStream reports whether a and b are at the same draw.
func sameStream(a, b *randx.Rand) bool {
	x, _ := a.MarshalBinary()
	y, _ := b.MarshalBinary()
	return slices.Equal(x, y)
}

// wildOne is a game of seed with one animal of species key.
func wildOne(t *testing.T, seed uint64, key string) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 24, Height: 14})
	id, err := g.SpawnAnimal(key, 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	return g, id
}

// cutBlow is a cut-form blow of dmg (an unplaced animal hit).
func cutBlow(dmg int) hit.Blow { return hit.Blow{Dmg: dmg, Kind: body.Cut, Split: body.SplitCut} }

// woundsOf is b's wounds without their infection times (set after the
// split), for comparing a body against its twin.
func woundsOf(b *body.Body) []body.Wound {
	out := slices.Clone(b.Wounds)
	for i := range out {
		out[i].InfectAt = 0
	}
	return out
}

// TestAnimalCutBlowSplits: a cut-form blow on an animal raises Injury by
// SpreadBlow's total, which is more than dmg on a split, and no wound is
// trimmed, on a small animal and on one above health.MaxHP.
func TestAnimalCutBlowSplits(t *testing.T) {
	t.Parallel()
	const dmg = 10
	for _, key := range []string{"deer", "elephant"} {
		for _, c := range []struct {
			seed         uint64
			total, parts int
		}{{1, 16, 3}, {2, dmg, 1}, {10, 18, 4}} {
			g, id := wildOne(t, c.seed, key)
			a, _ := g.animals.Get(id)
			ref := animal.NewBody(a.Species)
			want := body.SpreadBlowWithin(ref, dmg, body.Cut, body.SplitCut, a.HP(), twin(t, g.stream(streamMedical)).IntN, uint64(g.clock.Now())) // woundAnimal's room
			g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
			a, _ = g.animals.Get(id)
			if want != c.total || a.Injury != want || woundDmg(a.Body) != want || woundedParts(a.Body) != c.parts {
				t.Fatalf("%s seed %d: Injury %d, wounds %d on %d parts, want %d on %d", key, c.seed, a.Injury, woundDmg(a.Body), woundedParts(a.Body), c.total, c.parts)
			}
			if !slices.Equal(woundsOf(a.Body), woundsOf(ref)) {
				t.Fatalf("%s seed %d: no wound is trimmed: %+v, want %+v", key, c.seed, a.Body.Wounds, ref.Wounds)
			}
		}
	}
}

// TestAnimalFixtureBlowUnchanged: Injure on an animal (no split) raises
// Injury by exactly dmg with Spread's draws, as before CBT 3.4.
func TestAnimalFixtureBlowUnchanged(t *testing.T) {
	t.Parallel()
	const dmg = 12
	for seed := uint64(1); seed <= 10; seed++ {
		g, id := wildOne(t, seed, "deer")
		a, _ := g.animals.Get(id)
		now := uint64(g.clock.Now())
		med := twin(t, g.stream(streamMedical))
		ref := animal.NewBody(a.Species)
		body.Spread(ref, dmg, body.Bruise, med.IntN, now)
		if infection.WoundChance(body.Bruise, -1, infection.RoomOther) > 0 {
			med.IntN(infection.DelaySpan + 1)
		}
		g.Injure(id, dmg)
		a, _ = g.animals.Get(id)
		if a.Injury != dmg || a.Body == nil || !slices.Equal(woundsOf(a.Body), woundsOf(ref)) {
			t.Fatalf("seed %d: Injury %d (want %d), wounds %+v want %+v", seed, a.Injury, dmg, a.Body, ref.Wounds)
		}
		if !sameStream(g.stream(streamMedical), med) {
			t.Fatalf("seed %d: the fixture blow draws as Spread did", seed)
		}
	}
}

// TestAnimalSplitExtraKills: an animal whose base dmg leaves it 1 hit
// point dies of the split's extra through harm (killAnimal): it is gone,
// not among the animal patients, and no infection delay was drawn for it.
func TestAnimalSplitExtraKills(t *testing.T) {
	t.Parallel()
	const dmg = 10
	kills := 0
	for seed := uint64(1); seed <= 10; seed++ {
		g, id := wildOne(t, seed, "deer")
		a, _ := g.animals.Get(id)
		a.Injury = animal.DefOf(a.Species).MaxHP - dmg - 1
		g.animals.Set(id, a)
		med := twin(t, g.stream(streamMedical))
		total := body.SpreadBlowWithin(animal.NewBody(a.Species), dmg, body.Cut, body.SplitCut, a.HP(), med.IntN, uint64(g.clock.Now())) // woundAnimal's room
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		if total <= dmg { // one part: the base dmg leaves it 1 hit point
			if got, ok := g.animals.Get(id); !ok || got.HP() != 1 {
				t.Fatalf("seed %d: an unsplit cut leaves 1 hit point: %+v", seed, got)
			}
			continue
		}
		kills++
		if _, ok := g.animals.Get(id); ok {
			t.Fatalf("seed %d: the split's extra %d kills the animal", seed, total-dmg)
		}
		if slices.Contains(g.animalPatientIDs(), id) {
			t.Fatalf("seed %d: a dead animal is not a patient", seed)
		}
		if !sameStream(g.stream(streamMedical), med) {
			t.Fatalf("seed %d: no infection delay is drawn for a dead animal", seed)
		}
	}
	if kills != 8 { // seeds 2 and 7 cut one part (TestAnimalCutBlowSplits)
		t.Fatalf("%d of 10 split cuts kill, want 8", kills)
	}
}

// TestAnimalSplitInjuryNeverShort: an elephant (MaxHP 170) alive past
// body.KillDamage takes the whole split (its body dies at its own scaled
// KillDamage), and Injury rises by exactly what the body took (R5).
func TestAnimalSplitInjuryNeverShort(t *testing.T) {
	t.Parallel()
	// a full split (at most 12*1.78 plus four bonuses of a share) stays under its 60 left
	const dmg, pre = 12, 110
	// by seed: R1 totals with parent bonuses, none under dmg
	rises := [...]int{0, 20, 12, 17, 17, 17, 20, 12, 20, 17, 21}
	for seed := uint64(1); seed <= 10; seed++ {
		g, id := wildOne(t, seed, "elephant")
		a, _ := g.animals.Get(id)
		a.Body = animal.NewBody(a.Species)
		body.Spread(a.Body, pre, body.Cut, randx.New(seed).IntN, 0)
		a.Injury = a.Body.Damage()
		g.animals.Set(id, a)
		ref := *a.Body
		ref.Wounds = slices.Clone(a.Body.Wounds)
		before := ref.Damage()
		took := body.SpreadBlow(&ref, dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, uint64(g.clock.Now()))
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		got, ok := g.animals.Get(id)
		if !ok || got.Body == nil {
			t.Fatalf("seed %d: the elephant survives a cut of %d at %d of %d", seed, dmg, pre, animal.DefOf(a.Species).MaxHP)
		}
		t.Logf("seed %d: body damage %d, SpreadBlow %d, rise %d, Injury %d -> %d", seed, before, took, got.Body.Damage()-before, a.Injury, got.Injury)
		if took != rises[seed] || got.Body.Damage()-before != took || got.Injury-a.Injury != took || woundDmg(got.Body) > got.Injury {
			t.Fatalf("seed %d: body rise %d, Injury rise %d, want both %d", seed, got.Body.Damage()-before, got.Injury-a.Injury, rises[seed])
		}
	}
}

// bodyCopy is a copy of b sharing no slice with it.
func bodyCopy(b *body.Body) *body.Body {
	c := *b
	c.Wounds, c.Conds, c.Fitted = slices.Clone(b.Wounds), slices.Clone(b.Conds), slices.Clone(b.Fitted)
	return &c
}

// TestAnimalSplitStopNeverLowersInjury: when the split stops early (the
// blow's rise reached the hit points the animal had, SpreadBlowWithin's
// room) it returns less than the split's total; Injury still rises by
// exactly what the body took and never falls. The body here is an
// elephant missing three legs (lost parts count toward the stop's rise
// only as the blow changes them). woundAnimal runs on a local state after
// harm's pre-add, because a stopped split kills and harm removes the dead.
func TestAnimalSplitStopNeverLowersInjury(t *testing.T) {
	t.Parallel()
	const dmg = 12
	short := 0
	for seed := uint64(1); seed <= 6; seed++ {
		g, id := wildOne(t, seed, "elephant")
		a, _ := g.animals.Get(id)
		a.Body = animal.NewBody(a.Species)
		a.Body.Missing |= 1<<body.QLegFL | 1<<body.QLegFR | 1<<body.QLegHL
		now := uint64(g.clock.Now())
		full := body.SpreadBlowWithin(bodyCopy(a.Body), dmg, body.Cut, body.SplitCut, 1000, twin(t, g.stream(streamMedical)).IntN, now)
		if full <= dmg {
			continue // one part: nothing to stop
		}
		room := dmg + 1 + int(seed)%(full-dmg) // dmg < room <= the split's total
		a.Injury = animal.DefOf(a.Species).MaxHP - room
		before := a.Injury
		took := body.SpreadBlowWithin(bodyCopy(a.Body), dmg, body.Cut, body.SplitCut, room, twin(t, g.stream(streamMedical)).IntN, now)
		dmgBefore := a.Body.Damage()
		a.Injury += dmg // as harm does
		g.woundAnimal(id, &a, dmg, body.Cut, body.SplitCut, 0, now)
		if a.Injury < before || a.Injury != before+max(took, 0) {
			t.Fatalf("seed %d: Injury %d -> %d (room %d), want a rise of exactly %d", seed, before, a.Injury, room, took)
		}
		if took < full {
			short++
			if !a.Dead() || a.Body == nil || a.Body.Damage()-dmgBefore != took {
				t.Fatalf("seed %d: a stop at %d of %d kills (dead %v) and the body rose by it", seed, took, full, a.Dead())
			}
		}
	}
	if short == 0 {
		t.Fatal("no seed took the early-stop path")
	}
}

// TestAnimalSplitNegativeRiseClamped: Injury never falls on a hit (lead
// ruling, QA W2-integrate r1 C9; woundAnimal's max(…, 0) is now a guard
// SpreadBlowWithin with room > 0 does not reach: plan R-2). In
// the drift state (three legs lost, the last leg and its paw 1 HP short
// of loss), a split cut can destroy that leg: its wounds are swapped for
// its MaxHP and Damage falls. The body's own rule (SpreadBlow) then stops
// with a negative rise (seed 7: -24); the room rule (SpreadBlowWithin, the
// game's) never stops on a fall and returns the blow's total.
func TestAnimalSplitNegativeRiseClamped(t *testing.T) {
	t.Parallel()
	const dmg = 12
	neg, destroyed := 0, 0
	for seed := uint64(1); seed <= 200; seed++ {
		g, id := wildOne(t, seed, "elephant")
		a, _ := g.animals.Get(id)
		a.Body = animal.NewBody(a.Species)
		b := a.Body
		b.Missing |= 1<<body.QLegFL | 1<<body.QLegFR | 1<<body.QLegHL
		leg, paw := b.PartDef(body.QLegHR).MaxHP, b.PartDef(body.QPawHR).MaxHP
		b.Wounds = append(b.Wounds,
			body.Wound{Part: body.QLegHR, Dmg: leg - 1, Kind: body.Cut, Size: int32(leg - 1)},
			body.Wound{Part: body.QPawHR, Dmg: paw - 1, Kind: body.Cut, Size: int32(paw - 1)})
		a.Injury = woundDmg(b)
		g.animals.Set(id, a)
		now := uint64(g.clock.Now())
		took := body.SpreadBlow(bodyCopy(b), dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, now)
		if took < 0 {
			neg++ // the domain still returns a negative rise here
		}
		if seed == 7 && took != -24 {
			t.Fatalf("seed 7: SpreadBlow returned %d, want -24", took)
		}
		ref := bodyCopy(b)
		took = body.SpreadBlowWithin(ref, dmg, body.Cut, body.SplitCut, a.HP(), twin(t, g.stream(streamMedical)).IntN, now)
		if ref.Missing&(1<<body.QLegHR) != 0 {
			destroyed++
		}
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		if got, ok := g.animals.Get(id); !ok || got.Injury < a.Injury || got.Injury != a.Injury+max(took, 0) {
			t.Fatalf("seed %d: a rise of %d leaves Injury at %d, got %d (alive %v)", seed, took, a.Injury, got.Injury, ok)
		}
	}
	if neg == 0 || destroyed == 0 {
		t.Fatalf("%d seeds gave a negative SpreadBlow return, %d destroyed the leg", neg, destroyed)
	}
}

// healedLeg gives animal id a lost front left leg and heals Injury to 0 by
// path: "natural" (naturalHeal alone), or "tended" (an open cut too, a
// tend, heal hours until it closes, then naturalHeal). Injury equals the
// open wounds plus the lost leg's HP until naturalHeal, which leaves the
// body unchanged (#269's drift). It returns the lost leg's HP.
func healedLeg(t *testing.T, g *Game, id entity.EntityID, path string) int {
	t.Helper()
	a, _ := g.animals.Get(id)
	a.Food = max(a.Food, animal.FeedStart)
	a.Body = animal.NewBody(a.Species)
	a.Body.Missing |= 1 << body.QLegFL
	lost := a.Body.Damage()
	now := uint64(g.clock.Now())
	if path == "tended" {
		a.Body.Wounds = append(a.Body.Wounds, body.Wound{Part: body.QLegHL, Dmg: 6, Kind: body.Cut, Size: 6, At: now})
		a.Injury = woundDmg(a.Body) + lost
		tendAnimalBody(&a, 500, now)
		g.animals.Set(id, a)
		g.markAnimalPatient(id)
		h := 0
		for ; woundDmg(a.Body) > 0; h++ {
			if a.Injury != woundDmg(a.Body)+lost || h > 24*30 {
				t.Fatalf("hour %d: Injury %d, wounds %d + lost %d", h, a.Injury, woundDmg(a.Body), lost)
			}
			toNextHour(g)
			var ok bool
			if a, ok = g.animals.Get(id); !ok || a.Body == nil {
				t.Fatalf("hour %d: the animal or its body is gone", h)
			}
		}
		if h < 2 || a.Injury != lost {
			t.Fatalf("the cut closed in %d hours, leaving Injury %d, want %d", h, a.Injury, lost)
		}
		now = uint64(g.clock.Now())
	} else {
		a.Injury = lost // the lost leg alone
	}
	for ; a.Injury > 0; now++ {
		g.naturalHeal(id, &a, now)
	}
	if a.Body == nil || a.Body.Damage() != lost || a.Body.Missing != 1<<body.QLegFL {
		t.Fatalf("%s: the healed body is %+v, want only the lost leg (%d)", path, a.Body, lost)
	}
	g.animals.Set(id, a)
	return lost
}

// TestAnimalHealedLostPartSplitNotCutShort (#269): after a heal that left
// a labrador's lost leg on its body (Injury 0, the body still counting the
// leg's HP), a split cut is dealt in full while the animal has hit points
// left: Injury is SpreadBlowWithin's take at the animal's MaxHP, it lives,
// and the medical stream drew as that blow (and the infection delay) did.
func TestAnimalHealedLostPartSplitNotCutShort(t *testing.T) {
	t.Parallel()
	const dmg = 28 // the labrador's 47 HP less its 14 lost: the old rule stops at a rise of 33
	for _, path := range []string{"natural", "tended"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			counted, short := 0, 0
			for seed := uint64(1); seed <= 16; seed++ {
				g, id := wildOne(t, seed, "labrador")
				lost := healedLeg(t, g, id, path)
				a, _ := g.animals.Get(id)
				maxHP, now := animal.DefOf(a.Species).MaxHP, uint64(g.clock.Now())
				old := body.SpreadBlow(bodyCopy(a.Body), dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, now)
				med := twin(t, g.stream(streamMedical))
				full := body.SpreadBlowWithin(bodyCopy(a.Body), dmg, body.Cut, body.SplitCut, maxHP, med.IntN, now)
				if infection.WoundChance(body.Cut, -1, infection.RoomOther) > 0 {
					med.IntN(infection.DelaySpan + 1) // a survivor's infection delay
				}
				g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
				if full >= maxHP {
					continue // the full split kills: not this bug
				}
				counted++
				got, ok := g.animals.Get(id)
				t.Logf("seed %d: lost %d, old take %d, full take %d, Injury %d of %d", seed, lost, old, full, got.Injury, maxHP)
				if !ok || got.Injury != full || got.Injury < dmg || got.Injury >= maxHP {
					t.Fatalf("seed %d: Injury %d (alive %v), want the full take %d", seed, got.Injury, ok, full)
				}
				if !sameStream(g.stream(streamMedical), med) {
					t.Fatalf("seed %d: the medical stream is not at the full blow's draw", seed)
				}
				// independent of SpreadBlowWithin: Injury (0 before) rose by what the
				// body rose by (Damage: wounds plus lost-part HP, the healed leg's
				// excluded), which is the new wounds alone when no part was lost
				newLoss := got.Body.Missing != 1<<body.QLegFL
				if got.Injury != got.Body.Damage()-lost || !newLoss && got.Injury != woundDmg(got.Body) {
					t.Fatalf("seed %d: Injury %d, body Damage %d (wounds %d) less the lost %d", seed, got.Injury, got.Body.Damage(), woundDmg(got.Body), lost)
				}
				if old < full {
					short++
				}
			}
			if counted == 0 || short == 0 {
				t.Fatalf("%d seeds counted, %d cut short by the old rule", counted, short)
			}
		})
	}
}

// TestAnimalStarvedSplitStopsAtHP (AC-4): a starved animal (no body,
// Injury near MaxHP) gets a fresh body from the blow, and the split stops
// at the share that brings Injury to MaxHP: it dies, is no patient, and
// the medical stream drew as SpreadBlowWithin at its hit points did.
func TestAnimalStarvedSplitStopsAtHP(t *testing.T) {
	t.Parallel()
	const dmg = 10
	kills, stopped := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		g, id := wildOne(t, seed, "deer")
		a, _ := g.animals.Get(id)
		a.Body, a.Injury = nil, animal.DefOf(a.Species).MaxHP-dmg-1
		g.animals.Set(id, a)
		now := uint64(g.clock.Now())
		med := twin(t, g.stream(streamMedical))
		took := body.SpreadBlowWithin(animal.NewBody(a.Species), dmg, body.Cut, body.SplitCut, a.HP(), med.IntN, now)
		whole := body.SpreadBlowWithin(animal.NewBody(a.Species), dmg, body.Cut, body.SplitCut, 1000, twin(t, g.stream(streamMedical)).IntN, now)
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		if took < a.HP() {
			continue // one part: it lives on 1 hit point (TestAnimalSplitExtraKills)
		}
		kills++
		if g.animals.Has(id) || slices.Contains(g.animalPatientIDs(), id) || !sameStream(g.stream(streamMedical), med) {
			t.Fatalf("seed %d: a split of %d at %d hit points kills, with SpreadBlowWithin's draws", seed, took, a.HP())
		}
		if took < whole {
			stopped++
		}
	}
	if kills == 0 || stopped == 0 {
		t.Fatalf("%d kills, %d stopped before the whole split", kills, stopped)
	}
}
