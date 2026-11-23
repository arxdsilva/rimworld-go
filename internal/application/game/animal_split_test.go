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
// (body.SpreadBlow, woundAnimal); a fixture blow is not.

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
			want := body.SpreadBlow(ref, dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, uint64(g.clock.Now()))
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
		total := body.SpreadBlow(animal.NewBody(a.Species), dmg, body.Cut, body.SplitCut, med.IntN, uint64(g.clock.Now()))
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

// TestAnimalSplitStopNeverLowersInjury: when SpreadBlow stops early (the
// body dead by its own rule, body/split.go bodyDead) it returns under dmg, so
// woundAnimal's `- dmg` term is negative; Injury still rises by exactly
// what the body took and never falls. The body here is an elephant missing
// three legs (lost parts count toward the body's rule, not toward Injury).
func TestAnimalSplitStopNeverLowersInjury(t *testing.T) {
	t.Parallel()
	const dmg = 12
	short := 0
	for seed := uint64(1); seed <= 6; seed++ {
		g, id := wildOne(t, seed, "elephant")
		a, _ := g.animals.Get(id)
		a.Body = animal.NewBody(a.Species)
		a.Body.Missing |= 1<<body.QLegFL | 1<<body.QLegFR | 1<<body.QLegHL
		body.Spread(a.Body, animal.DefOf(a.Species).MaxHP+5-a.Body.Damage(), body.Cut, randx.New(seed).IntN, 0)
		a.Injury = woundDmg(a.Body)
		g.animals.Set(id, a)
		ref := *a.Body
		ref.Wounds = slices.Clone(a.Body.Wounds)
		took := body.SpreadBlow(&ref, dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, uint64(g.clock.Now()))
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		got, ok := g.animals.Get(id)
		if !ok || got.Injury < a.Injury || got.Injury != a.Injury+max(took, 0) {
			t.Fatalf("seed %d: Injury %d -> %d (alive %v), want a rise of exactly %d", seed, a.Injury, got.Injury, ok, took)
		}
		if took < dmg {
			short++
		}
	}
	if short == 0 {
		t.Fatal("no seed took the early-stop path")
	}
}

// TestAnimalSplitNegativeRiseClamped pins woundAnimal's max(…, 0) (lead
// ruling, QA W2-integrate r1 C9). In the drift state (three legs lost, the
// last leg and its paw 1 HP short of loss), a split cut can destroy that
// leg: its wounds are swapped for its MaxHP, Damage falls, the body is dead
// by its own rule, and SpreadBlow returns a negative rise (seed 7: -24).
// Injury must not fall with it.
func TestAnimalSplitNegativeRiseClamped(t *testing.T) {
	t.Parallel()
	const dmg = 12
	neg := 0
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
		ref := *b
		ref.Wounds = slices.Clone(b.Wounds)
		took := body.SpreadBlow(&ref, dmg, body.Cut, body.SplitCut, twin(t, g.stream(streamMedical)).IntN, uint64(g.clock.Now()))
		if took >= 0 {
			continue
		}
		neg++
		g.harm(id, cutBlow(dmg), 0, health.CauseInjury)
		if got, ok := g.animals.Get(id); !ok || got.Injury != a.Injury {
			t.Fatalf("seed %d: a rise of %d leaves Injury at %d, got %d (alive %v)", seed, took, a.Injury, got.Injury, ok)
		}
	}
	if neg == 0 {
		t.Fatal("no seed gave a negative SpreadBlow return")
	}
}
