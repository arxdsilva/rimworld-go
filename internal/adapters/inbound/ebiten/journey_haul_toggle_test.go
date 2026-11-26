// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/command"
)

func TestJourneyHaulTrainingToggle(t *testing.T) {
	j := newJourney(t, jPawns([2]int{10, 6}))
	id, err := j.g.SpawnAnimal("husky", 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	j.g.TameAnimal(id, "Rex")
	check := func(want bool) {
		t.Helper()
		for _, an := range j.world().Animals {
			if an.ID == id {
				for _, tr := range an.Training {
					if tr.Skill == int(training.Haul) {
						if !tr.Allowed || tr.On != want {
							t.Fatalf("Haul: %+v want On=%v", tr, want)
						}
						return
					}
				}
			}
		}
		t.Fatal("Haul training row missing")
	}
	j.lookAt(12, 6)
	j.update(1)
	j.clickTile(12, 6, inputframe.Left, 0)
	check(false)
	for _, want := range []bool{true, false} {
		j.reset()
		if !j.clickID("animal.train.3") {
			t.Fatal("Haul toggle absent")
		}
		if c, ok := j.lastCmd(command.SetTraining); !ok || c.Target != uint64(id) || c.Index != int(training.Haul) || c.Enabled != want {
			t.Fatalf("native command %+v (%v), want %v", c, ok, want)
		}
		check(want)
	}
}
