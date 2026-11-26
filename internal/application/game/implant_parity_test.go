// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

func TestAugmentationTraitsStateAndOpinion(t *testing.T) {
	g, a, b := socialPair(t)
	withTraits(g, a, false, trait.AugmentFan)
	withTraits(g, b, false, trait.NaturalPurist)
	before := socPoints(g, b, a)
	find := func(id entity.EntityID, k mood.Kind) (int, bool) {
		for _, th := range g.bodyThoughts(id) {
			if th.Kind == k {
				return th.Value, true
			}
		}
		return 0, false
	}
	if v, ok := find(a, mood.AugmentFan); !ok || v != -4 {
		t.Fatalf("bare augmentation enthusiast mood %d/%v", v, ok)
	}
	if v, ok := find(b, mood.NaturalPurist); !ok || v != 0 {
		t.Fatalf("bare augmentation skeptic mood %d/%v", v, ok)
	}
	v := g.VitalOf(a)
	ab := g.ensureBody(a, &v)
	body.AddImplant(ab, body.Brain, item.NerveDampener)
	g.bodyChanged(a, ab, &v)
	if m, ok := find(a, mood.AugmentFan); !ok || m != 4 {
		t.Fatalf("one enhancement mood %d/%v", m, ok)
	}
	after := socPoints(g, b, a)
	if after != before-8 {
		t.Fatalf("augmentation skeptic opinion stale: %d -> %d", before, after)
	}
	v2 := g.VitalOf(b)
	bb := g.ensureBody(b, &v2)
	body.AddImplant(bb, body.Brain, item.BlissCircuit)
	g.bodyChanged(b, bb, &v2)
	if m, ok := find(b, mood.NaturalPurist); !ok || m != -10 {
		t.Fatalf("purist personal mood %d/%v", m, ok)
	}
}

func TestArtificialInjuryReceivesHourlyCare(t *testing.T) {
	g := New(Config{Seed: 91, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Missing |= 1 << body.LegL
	b.SetReplacement(body.LegL, item.ServoLeg)
	b.Replacements[body.LegL].Damage = 8
	g.bodyChanged(id, b, &v)
	if !g.indexedPatient(id) {
		t.Fatal("damageable part excluded from patient index")
	}
	for hour := 0; hour < 48 && body.HasDamagedReplacement(b); hour++ {
		g.stepCare(uint64(hour+1) * 2500)
	}
	if body.HasDamagedReplacement(b) || b.Damage() != 0 {
		t.Fatalf("servo leg did not heal: %+v", b)
	}
	if g.indexedPatient(id) {
		t.Fatal("healed bionic remained indexed")
	}
}
