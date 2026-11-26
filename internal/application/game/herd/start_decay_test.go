// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
)

// TestStartDecayFromNowNotBackdated: a missing timer starts one interval
// after now, whatever the last session, and the first decide after that
// costs nothing; an existing timer and an animal that cannot decay are left.
func TestStartDecayFromNowNotBackdated(t *testing.T) {
	const now = 61 * 60000
	wolf := animal.State{Species: animal.TimberWolf, Tame: true, TrainDecay: 60000}
	wolf.Trained[training.Obedience] = 1
	interval := training.DecayInterval(animal.Wildness(wolf.Species))
	m := StartDecay(wolf, training.Maintenance{}, now)
	if m.DecayAt != now+interval {
		t.Fatalf("DecayAt %d, want %d", m.DecayAt, now+interval)
	}
	if AdvanceTraining(&wolf, &m, now+60) || wolf.Trained[training.Obedience] != 1 || m.TameLoss != 0 {
		t.Fatalf("decayed right after the start: %+v %+v", wolf.Trained, m)
	}
	if got := StartDecay(wolf, training.Maintenance{DecayAt: 5}, now); got.DecayAt != 5 {
		t.Fatalf("existing timer moved to %d", got.DecayAt)
	}
	husky := animal.State{Species: animal.Husky, Tame: true} // low wildness, nothing trained
	deer := animal.State{Species: animal.Deer, Tame: true}   // penned
	deer.Trained[training.Obedience] = 1
	for _, a := range []animal.State{husky, deer} {
		if got := StartDecay(a, training.Maintenance{}, now); got.DecayAt != 0 {
			t.Fatalf("%s: timer for an animal that cannot decay", a.Species.Label())
		}
	}
}
