// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"maps"
	"testing"

	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// Bedded reuses its pass over the buildings while the buildings key holds. Every
// change a bed can undergo (claims, forbidding, prison and medical marks, a
// bed built or destroyed, a pawn going to sleep) must reach the answer the next
// time it is asked, as a never-cached Beds gives it.
func TestBeddedCacheMatchesAnUncachedPassAfterEveryBedChange(t *testing.T) {
	g, a, b, s1, s2, double := bedroom(t)
	ids := g.pawnIDs
	check := func(label string) {
		t.Helper()
		got := maps.Clone(g.rest.Beds.Bedded(ids))
		want := rest.New(newRestHost(g)).Beds.Bedded(ids)
		if !maps.Equal(got, want) {
			t.Fatalf("%s: cached Bedded %v, uncached %v", label, got, want)
		}
	}
	check("start")
	steps := []struct {
		name string
		do   func()
	}{
		{"owner assigned", func() { g.rest.Beds.AssignOwner(s1, a, 0) }},
		{"owner assigned by command", func() {
			g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(double), Pawn: uint64(b), Index: 1})
			g.Step(1)
		}},
		{"bed forbidden", func() {
			g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(s2), Enabled: true})
			g.Step(1)
		}},
		{"bed allowed", func() { g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(s2)}); g.Step(1) }},
		{"bed for prisoners", func() {
			g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(s2), Enabled: true})
			g.Step(1)
		}},
		{"bed medical", func() {
			g.Enqueue(command.Command{Kind: command.SetBedMedical, Target: uint64(s1), Enabled: true})
			g.Step(1)
		}},
		{"owner cleared", func() { g.rest.Beds.AssignOwner(double, 0, 1) }},
		{"another bed built", func() { buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2) }},
		{"bed destroyed", func() { g.removeBuilding(s2) }},
		{"a pawn goes to bed", func() { tireOut(g, a); g.Step(2) }},
	}
	for _, s := range steps {
		s.do()
		check(s.name)
		for range 5 { // and across ticks nothing stale survives
			g.Step(1)
			check(s.name + " +tick")
		}
	}
}
