// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalview

import (
	"rimworld-go/internal/domain/sim/entity"
	"testing"

	"rimworld-go/internal/domain/animal"
)

func TestGaitAndActionProjection(t *testing.T) {
	for _, mode := range []animal.Mode{animal.Feed, animal.Leave, animal.Rest} {
		a := animal.State{Mode: mode}
		if ActionState(a).Mode != animal.Wander || a.Mode != mode {
			t.Fatalf("action projection changed live state %v", mode)
		}
	}
	if ActionState(animal.State{Mode: animal.Attack}).Mode != animal.Attack {
		t.Fatal("fight action was normalized to wander")
	}
	a := animal.State{Mode: animal.Wander, StepGait: animal.GaitWalk}
	if got := Gait(a, 40, 7, 45, true); got != "walk" {
		t.Fatalf("walking gait = %q", got)
	}
	if got := Gait(a, 40, 7, 47, false); got != "" {
		t.Fatalf("completed gait = %q", got)
	}
	a.StepGait = animal.GaitRun
	if got := Gait(a, 40, 7, 45, true); got != "run" {
		t.Fatalf("running gait = %q", got)
	}
	a.Downed = true
	if got := Gait(a, 40, 7, 45, true); got != "" {
		t.Fatalf("downed animal has gait %q", got)
	}
	a = animal.State{Mode: animal.Rest, StepGait: animal.GaitAmble}
	if got := Gait(a, 40, 7, 45, true); got != "" {
		t.Fatalf("sleeping animal has gait %q", got)
	}
}

func TestLegacyAnimalSexForView(t *testing.T) {
	for id := uint64(1); id < 100; id++ {
		a := animal.State{HasSex: false}
		if Female(entity.EntityID(id), a) != animal.ArtFemale(id) {
			t.Fatalf("legacy animal %d changed sex", id)
		}
		a.HasSex, a.Female = true, true
		if !Female(entity.EntityID(id), a) {
			t.Fatalf("saved sex was ignored for animal %d", id)
		}
	}
}
