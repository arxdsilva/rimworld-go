// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package quest

import (
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
)

// TestPreviewIsTheSpawnedPawn: a pawn reward's preview is the person the
// seed spawns: the same look, and its best skill is the highest level, the
// first on a tie (QST-087, QST-137).
func TestPreviewIsTheSpawnedPawn(t *testing.T) {
	for seed := uint64(1); seed < 40; seed++ {
		p, v := PawnOf(seed, backstory.Core()), PreviewOf(seed, backstory.Core())
		if n, a := NameAge(seed); n != v.Name || a != v.Age {
			t.Fatalf("seed %d: NameAge %q %d, preview %q %d", seed, n, a, v.Name, v.Age)
		}
		if v.Look != p.Look {
			t.Fatalf("seed %d: preview look %+v, spawned %+v", seed, v.Look, p.Look)
		}
		if v.BestLevel != int(p.Skills[v.Best].Level) {
			t.Fatalf("seed %d: best level %d, skill %v is at %d", seed, v.BestLevel, v.Best, p.Skills[v.Best].Level)
		}
		for s := range skill.NumSkills {
			if l := int(p.Skills[s].Level); l > v.BestLevel || l == v.BestLevel && s < v.Best {
				t.Fatalf("seed %d: skill %v at %d beats the preview's best %v at %d", seed, s, l, v.Best, v.BestLevel)
			}
		}
	}
}

// TestNameAgeIgnoresTheBook: another book may draw other backstories but
// never another name or age, so NameAge needs no book (PLG-024).
func TestNameAgeIgnoresTheBook(t *testing.T) {
	core := backstory.Core().All()
	var list []backstory.Def
	var keys []string
	for i := len(core) - 1; i >= 0; i-- { // the same defs, reversed: other draws
		list, keys = append(list, core[i]), append(keys, "p.b"+core[i].Title)
	}
	other := backstory.NewBook(list, keys)
	for seed := uint64(1); seed < 40; seed++ {
		v, w := PreviewOf(seed, backstory.Core()), PreviewOf(seed, other)
		if v.Name != w.Name || v.Age != w.Age {
			t.Fatalf("seed %d: %q %d with core, %q %d with another book", seed, v.Name, v.Age, w.Name, w.Age)
		}
	}
}
