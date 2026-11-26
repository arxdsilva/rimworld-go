// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wealth

import (
	"math"
	"testing"
)

type mutableSource struct{ value int }

func (s *mutableSource) WealthOf(from, to int) (int, int) {
	if from == 0 && to > 0 {
		return s.value, 1
	}
	return 0, 1
}

func TestMoodValueUsesSavedInitialWealth(t *testing.T) {
	t.Parallel()
	src := &mutableSource{value: 17000}
	tracker := New(src)
	if tracker.MoodValue() != 17000 {
		t.Fatal("initial mood wealth must use actual colony wealth")
	}
	tracker.PrimeForMood()
	saved := tracker.Export()
	if saved == nil || !saved.InitialCounted || saved.InitialValue != 17000 {
		t.Fatalf("initial sample not retained: %+v", saved)
	}
	src.value = 32000
	if tracker.MoodValue() != 17000 || tracker.Value() != 32000 {
		t.Fatal("mood must use a stable sample while wealth UI remains live")
	}
	loaded := New(src)
	loaded.Import(saved)
	if loaded.MoodValue() != 17000 {
		t.Fatal("save/load lost the pending wealth sample")
	}
	for now := uint64(Pass); now < 2*Pass; now += Stride {
		tracker.Step(now)
		loaded.Step(now)
	}
	if tracker.MoodValue() != 32000 || loaded.MoodValue() != 32000 {
		t.Fatalf("first completed wealth sweep changed samples: %+v and %+v", tracker.Export(), loaded.Export())
	}
	if sum, _ := src.WealthOf(0, math.MaxInt); sum != 32000 {
		t.Fatal("source changed")
	}
}
