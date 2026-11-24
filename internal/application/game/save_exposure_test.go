// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"math"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// A save whose exposure entry is outside int32 (a hand-edited or corrupt
// file) loads clamped to the int32 range, not wrapped to a small or
// opposite-sign severity.
func TestSaveExposureClampsOutOfRangeSeverity(t *testing.T) {
	g := busyColony(t)
	var buf bytes.Buffer
	if err := g.Save(&buf, "exposure"); err != nil {
		t.Fatal(err)
	}
	const id = entity.EntityID(1)
	edited := downgradeSave(t, buf.Bytes(), func(s *saveState) {
		s.Exposure[id] = [2]int{1 << 40, -1 << 40}
	})
	got, _, err := Load(bytes.NewReader(edited))
	if err != nil {
		t.Fatal(err)
	}
	e := got.exposure[id]
	if e.Hypo != math.MaxInt32 || e.Heat != math.MinInt32 {
		t.Fatalf("exposure = {Hypo %d, Heat %d}, want {%d, %d}", e.Hypo, e.Heat, math.MaxInt32, math.MinInt32)
	}
}
