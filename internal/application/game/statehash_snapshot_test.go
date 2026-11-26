// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/sim/statehash"
)

// Direct snapshot hashing must match the canonical gob-decoded save.
func TestStateHashMatchesEncodedSavedState(t *testing.T) {
	check := func(t *testing.T, g *Game) {
		t.Helper()
		before := g.mapState
		got := g.StateHash()
		if g.mapState != before {
			t.Fatal("state hash changed the current map")
		}
		var raw bytes.Buffer
		if err := encodeSave(g, &raw, savefmt.Stamp{Name: "statehash"}, nil); err != nil {
			t.Fatal(err)
		}
		dec := gob.NewDecoder(&raw)
		var info SaveInfo
		var saved saveState
		if err := dec.Decode(&info); err != nil {
			t.Fatal(err)
		}
		if err := dec.Decode(&saved); err != nil {
			t.Fatal(err)
		}
		want := statehash.OfFields(normalisedFields(info, &saved))
		if got != want {
			t.Fatalf("direct hash %x, encoded save %x", got, want)
		}
	}
	for _, seed := range []uint64{1, 7, 42} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			g := NewColony(Config{Seed: seed, Width: 48, Height: 40})
			t.Run("initial", func(t *testing.T) { check(t, g) })
			for _, n := range []int{1, 250, 1800, 20000} {
				g.Step(n)
				t.Run(fmt.Sprintf("tick-%d", g.Tick()), func(t *testing.T) { check(t, g) })
			}
			var saved bytes.Buffer
			if err := g.Save(&saved, "colony"); err != nil {
				t.Fatal(err)
			}
			loaded, _, err := Load(&saved)
			if err != nil {
				t.Fatal(err)
			}
			t.Run("reloaded", func(t *testing.T) { check(t, loaded) })
		})
	}
	t.Run("multiple-maps", func(t *testing.T) {
		g, _, _, _, _, _ := twoMaps(t)
		check(t, g)
		g.ViewMap(1)
		check(t, g)
		g.Step(600)
		check(t, g)
	})
}
