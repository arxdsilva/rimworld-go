// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import (
	"testing"

	"rimworld-go/pkg/randx"
)

// TestStartRelationsUseNewBand: over many seeds a generated faction is
// hostile exactly when its goodwill is at or below HostileAt.
func TestStartRelationsUseNewBand(t *testing.T) {
	t.Parallel()
	slots := []Slot{{Settlers, 4}, {Clans, 4}, {Drifters, 3}, {Freebooters, 3}}
	seenHostileDrifter, seenCalmDrifter := false, false
	for seed := uint64(1); seed <= 300; seed++ {
		for _, f := range Generate(slots, randx.New(seed)) {
			if got, want := f.Relation == Hostile, f.Goodwill <= HostileAt; got != want {
				t.Fatalf("seed %d %s gw %d hostile=%v want %v", seed, f.Name, f.Goodwill, got, want)
			}
			switch f.Type {
			case Settlers, Clans:
				if f.Relation == Hostile {
					t.Fatalf("seed %d %s starts hostile", seed, f.Name)
				}
			case Freebooters:
				if f.Relation != Hostile {
					t.Fatalf("seed %d freebooter not hostile", seed)
				}
			case Drifters:
				inBand := f.Goodwill >= -90 && f.Goodwill <= -75
				if inBand != (f.Relation == Hostile) {
					t.Fatalf("seed %d drifter gw %d relation %d", seed, f.Goodwill, f.Relation)
				}
				if inBand {
					seenHostileDrifter = true
				} else {
					seenCalmDrifter = true
				}
			}
		}
	}
	if !seenHostileDrifter || !seenCalmDrifter {
		t.Errorf("drifters hostile seen %v, calm seen %v", seenHostileDrifter, seenCalmDrifter)
	}
	if f := Generate([]Slot{{Swarm, 1}}, randx.New(1)); len(f) != 1 || f[0].Relation != Hostile {
		t.Errorf("swarm not hostile: %+v", f)
	}
}

// TestDrifterHostileStartShare pins the Drifters' start range at -90 and its
// hostile share at 16 of 101 (the old band gave 11 of 71).
func TestDrifterHostileStartShare(t *testing.T) {
	t.Parallel()
	d := DefOf(Drifters)
	if d.GoodwillMin != -90 {
		t.Fatalf("Drifters GoodwillMin = %d, want -90", d.GoodwillMin)
	}
	hostile, total := 0, 0
	for gw := d.GoodwillMin; gw <= d.GoodwillMax; gw++ {
		total++
		if RelationFor(Neutral, gw) == Hostile {
			hostile++
		}
	}
	if hostile != 16 || total != 101 {
		t.Errorf("hostile share %d/%d, want 16/101", hostile, total)
	}
}
