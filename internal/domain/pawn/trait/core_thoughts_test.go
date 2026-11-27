// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trait

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
)

// The code-valued thoughts a trait builds read core's labels and values.
func TestCodeValuedThoughtsReadCoreDefs(t *testing.T) {
	t.Parallel()
	core := mood.Core()
	ts := AppendImplantThoughts(nil, []ID{AugmentFan, NaturalPurist, Sunny}, 2)
	if len(ts) != 2 || ts[0].Kind != mood.AugmentFan || ts[1].Kind != mood.NaturalPurist {
		t.Fatalf("implant thoughts %+v", ts)
	}
	if ts[0].Label != core.DefOf(mood.AugmentFan).Label || ts[0].Value != ImplantMood(AugmentFan, 2) {
		t.Fatalf("augment fan %+v", ts[0])
	}

	minor := ApplyThoughts(Core(), nil, nil, skill.Minor, 0)
	if len(minor) != 1 || minor[0].Label != core.DefOf(mood.LovesWork).Label || minor[0].Value != core.DefOf(mood.LovesWork).Value {
		t.Fatalf("minor passion %+v", minor)
	}
	major := ApplyThoughts(Core(), nil, nil, skill.Major, -3)
	if len(major) != 2 || major[0].Value != 4 || major[1].Kind != mood.Difficulty || major[1].Value != -3 ||
		major[1].Label != core.DefOf(mood.Difficulty).Label {
		t.Fatalf("major passion with difficulty %+v", major)
	}
}
