// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
)

func asthmaOn(b *body.Body, part body.Part) bool {
	for _, c := range b.Conds {
		if c.Kind == body.CondAsthma && c.Part == part {
			return true
		}
	}
	return false
}

func TestAnimalAsthmaOnsetMatchesAnatomy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		species animal.Species
		lungs   int
	}{
		{"quadruped", animal.Cow, 2},
		{"bird", animal.Chicken, 2},
		{"snake", animal.Cobra, 1},
		{"lizard", animal.Iguana, 2},
		{"insect", animal.HiveBeetle, 0},
		{"mechanoid", animal.Harrow, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := animal.State{Species: tc.species}
			if got := animal.DevelopAsthma(&a, 123); got != (tc.lungs > 0) {
				t.Fatalf("onset=%v, want lung count %d", got, tc.lungs)
			}
			if tc.lungs == 0 {
				if a.Body != nil {
					t.Fatalf("unsupported animal affected: %+v", a.Body)
				}
				return
			}
			lungs, n := body.AsthmaLungs(a.Body.Anatomy)
			if n != tc.lungs || len(a.Body.Conds) != tc.lungs {
				t.Fatalf("anatomy mismatch lungs=%d conds=%+v", n, a.Body.Conds)
			}
			for _, p := range lungs[:n] {
				if !asthmaOn(a.Body, p) {
					t.Errorf("missing asthma on %s", a.Body.PartLabel(p))
				}
			}
			if a.Body.Capacity(body.Breathing) != 900 || a.Moving() >= 1000 {
				t.Fatalf("disease has no respiratory or movement effect: breathing=%d moving=%d",
					a.Body.Capacity(body.Breathing), a.Moving())
			}
			// The existing condition identity also blocks duplicate onset.
			if animal.DevelopAsthma(&a, 124) {
				t.Fatal("repeated asthma on a lung already affected")
			}
		})
	}
}

func TestAnimalAsthmaCareAndSaveRoundTrip(t *testing.T) {
	g, ada, dog := bondYard(t, 84)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.SpawnStack(item.HerbalMedicine, 3, 8, 3)
	a, _ := g.animals.Get(dog)
	if !animal.DevelopAsthma(&a, uint64(g.clock.Now())) {
		t.Fatal("failed to seed respiratory condition")
	}
	g.animals.Set(dog, a)
	g.markAnimalPatient(dog)
	b := animalBody(g, dog)
	if b == nil || b.Anatomy != body.Quadruped || !slices.Contains(g.animalPatientIDs(), dog) {
		t.Fatalf("animal disease not indexed: %+v", b)
	}
	v := animalViewOf(t, g, dog)
	if len(v.Asthma) != 2 || v.BreathingPct != 90 || v.MovingPct >= 100 || !v.NeedsTend || v.Health != 100 {
		t.Fatalf("asthmatic uninjured animal not visible: %+v", v)
	}
	if !runCare(g, 400, func() bool {
		a, _ := g.animals.Get(dog)
		return a.Body != nil && !a.Body.TendDue(uint64(g.clock.Now()))
	}, ada) {
		t.Fatal("doctor did not treat chronic asthma using the existing veterinary job")
	}
	a, _ = g.animals.Get(dog)
	if a.Body == nil || a.Body.TendDue(uint64(g.clock.Now())) || medicineLeft(g, item.HerbalMedicine) != 2 {
		t.Fatalf("vet failed to apply timed treatment: %+v, medicine=%d", a.Body, medicineLeft(g, item.HerbalMedicine))
	}
	loaded := saveLoad(t, g)
	lb := animalBody(loaded, dog)
	if lb == nil || !reflect.DeepEqual(a.Body, lb) || g.StateHash() != loaded.StateHash() {
		t.Fatal("chronic animal condition changed through save/load")
	}
	av := animalViewOf(t, loaded, dog)
	if len(av.Asthma) != 2 || !strings.Contains(av.Asthma[0].Lung, "lung") {
		t.Fatalf("saved asthma absent from animal read model: %+v", av.Asthma)
	}
	toNextHour(g)
	toNextHour(loaded)
	if g.StateHash() != loaded.StateHash() {
		t.Fatal("chronic condition became nondeterministic after resume")
	}
}

func TestAnimalAsthmaHourlyProgressAndVetCooldown(t *testing.T) {
	g := New(Config{Seed: 85, Width: 20, Height: 12})
	id, _ := g.SpawnAnimal("cow", 9, 5)
	a, _ := g.animals.Get(id)
	animal.DevelopAsthma(&a, 0)
	g.animals.Set(id, a)
	g.markAnimalPatient(id)
	if !g.animalTendDue(id, &a, 0) {
		t.Fatal("untended lung not discovered")
	}
	start := a.Body.Conds[0].Sev
	for hour := 1; hour <= 48; hour++ {
		g.stepAnimalBody(id, &a, uint64(hour*clock.TicksPerHour))
	}
	if a.Body.Conds[0].Sev <= start || a.Body.Capacity(body.Breathing) >= 900 {
		t.Fatalf("asthma has no untreated progression: %+v", a.Body.Conds)
	}
	now := uint64(48 * clock.TicksPerHour)
	beforeCare := a.Body.Conds[0].Sev
	tendAnimalBody(&a, 1000, now)
	for h := 49; h <= 72; h++ {
		g.stepAnimalBody(id, &a, uint64(h*clock.TicksPerHour))
	}
	if a.Body.Conds[0].Sev >= beforeCare {
		t.Fatalf("asthma severity did not respond to veterinary care: %d -> %d",
			beforeCare, a.Body.Conds[0].Sev)
	}
	if a.Body.TendDue(now) || g.animalTendDue(id, &a, now) {
		t.Fatal("veterinarian will repeatedly tend a freshly treated animal")
	}
	if a.Body.TendDue(now + 6*clock.TicksPerDay) {
		t.Fatal("veterinary care expired before seven days")
	}
	if !a.Body.TendDue(now + 7*clock.TicksPerDay) {
		t.Fatal("veterinary care did not expire after seven days")
	}
}

func TestAnimalAsthmaActualBirthdayPhase(t *testing.T) {
	for _, tame := range []bool{false, true} {
		g := New(Config{Seed: 91, Width: 24, Height: 20})
		id := spawnSexed(t, g, "cow", 10, 10, true, tame, "Cow")
		now := cadence.Next(uint64(3000*clock.TicksPerDay), clock.TicksPerDay, cadence.OffMedical, id)
		g.clock = clock.NewAt(clock.Tick(now))
		a := animalOf(g, id)
		age := (animal.LifeOf(a.Species).LifeDays + 2*clock.DaysPerYear - 1) / (2 * clock.DaysPerYear) * clock.DaysPerYear
		a.Born = animal.BornAt(now, uint64(age)*clock.TicksPerDay)
		g.animals.Set(id, a)
		g.stream(streamMedical)
		g.streams.m[streamMedical] = asthmaSuccessRoll(t)
		g.phaseAnimals()
		got := animalOf(g, id)
		if got.Body == nil || len(got.Body.Conds) != 2 || got.Tame != tame || !slices.Contains(g.animalPatientIDs(), id) {
			t.Fatalf("birthday did not cause indexed natural asthma, tame=%t: %+v", tame, got)
		}
	}
}
