// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// The zero Book and Core draw exactly what the package's Pick does, and a
// plugin incident is drawn by its weight and keeps its like for behaviour.
func TestBookPicksLikePackage(t *testing.T) {
	t.Parallel()
	now := uint64(40 * clock.TicksPerDay)
	o := Odds{Disease: FullFreq, Climate: FullFreq}
	var st State
	total := Total(Nature, st, now, biome.TemperateForest, nil)
	for _, b := range []Book{{}, Core()} {
		if got := b.Total(Nature, st, now, biome.TemperateForest, o, nil); got != total {
			t.Fatalf("Total %d, want %d", got, total)
		}
		for roll := range total {
			want, _ := Pick(Nature, st, now, biome.TemperateForest, roll, nil)
			if got, _ := b.Pick(Nature, st, now, biome.TemperateForest, o, roll, nil); got != want {
				t.Fatalf("roll %d: %d, want %d", roll, got, want)
			}
		}
	}
	core := Core()
	list, keys := core.All(), core.Keys()
	list, keys = append(list, Def{Key: "p.x", Name: "X", Category: Nature, Weight: 7, Cooldown: 5, Like: Flu}), append(keys, "p.x")
	b := NewBook(list, keys)
	id, ok := b.IDOf("p.x")
	if !ok || b.Like(id) != Flu || b.Like(Flu) != Flu || b.DefOf(id).ID != id {
		t.Fatalf("plugin %d %v", id, ok)
	}
	if got := b.Total(Nature, st, now, biome.TemperateForest, o, nil); got != total+7 {
		t.Fatalf("Total with the plugin %d, want %d", got, total+7)
	}
	if got, _ := b.Pick(Nature, st, now, biome.TemperateForest, o, total+6, nil); got != id {
		t.Fatalf("last roll picked %d, want the plugin's %d", got, id)
	}
	st.Fired(id, now)
	if b.DefOf(id).Allowed(st, now, biome.TemperateForest) {
		t.Fatal("its own cooldown holds")
	}
}
