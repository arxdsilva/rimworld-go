// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
	"testing"
)

func TestCalendarPolicySaveAndAge(t *testing.T) {
	p := clock.CalendarPolicy{DaysPerQuadrum: 20, Revision: 1}
	cl := temperature.Climate{YearMean: 123, Amplitude: 140, PeakDay: 15, DaySwing: 33}
	g := New(Config{Seed: 5, Width: 20, Height: 12, Calendar: p, Climate: &cl})
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	initial := g.sk.bios[id].AgeDays
	for d := 1; d <= 80; d++ {
		jumpTo(g, d, 8)
		g.dayPasses(id)
	}
	if got := g.sk.bios[id].AgeDays - initial; got != 60 {
		t.Fatalf("biological age +%d want 60", got)
	}
	if g.Date().Year != 1 || g.climate.PeakDay != 20 || g.climate.Calendar != p {
		t.Fatalf("date/climate %v %v", g.Date(), g.climate)
	}
	g.World() // initialise room/roof fields just as the restore owner does
	h := saveAndLoad(t, g)
	if CalendarOf(h) != p || h.climate != g.climate || h.Date() != g.Date() || h.sk.bios[id].AgeDays != g.sk.bios[id].AgeDays {
		t.Fatalf("calendar save mismatch")
	}
	if h.StateHash() != g.StateHash() {
		t.Fatalf("save state changed %x != %x", h.StateHash(), g.StateHash())
	}
	for d := 81; d <= 100; d++ {
		jumpTo(g, d, 8)
		jumpTo(h, d, 8)
		g.dayPasses(id)
		h.dayPasses(id)
	}
	if g.sk.bios[id].AgeDays != initial+75 || g.StateHash() != h.StateHash() {
		t.Fatal("aging resume diverged")
	}
}

func TestCalendarAnimalAgeAndYield(t *testing.T) {
	p := clock.CalendarPolicy{DaysPerQuadrum: 20}
	s := animal.Cow
	now := uint64(100) * clock.TicksPerDay
	a := animal.State{Species: s, Born: animal.BornAt(now, 40*clock.TicksPerDay)}
	if got := animal.AgeDays(a.Born, now, s, p); got != 30 {
		t.Fatalf("age %d", got)
	}
	b := a
	b.Born = animal.BornAt(now, 30*clock.TicksPerDay)
	if animal.CorpseYieldOf(a, now, true, p) != animal.CorpseYieldOf(b, now, true) {
		t.Fatal("corpse yield must agree at same biological age")
	}
}
