// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
)

func blockBirthCells(g *Game, x, y int) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			g.grid.SetWalkable(x+dx, y+dy, false)
		}
	}
}

func TestPregnancyRetainsBlockedAndPartialLitterAcrossSave(t *testing.T) {
	for _, size := range []int{24, 64} {
		t.Run(map[int]string{24: "compact", 64: "legacy-sized"}[size], func(t *testing.T) {
			g := New(Config{Seed: 921, Width: size, Height: size})
			mother := spawnSexed(t, g, "goat", 8, 8, true, true, "Mother")
			setAnimal(g, mother, func(a *animal.State) { a.NextThink = 1 << 40 })
			due := uint64(g.clock.Now()) + 1
			setAnimal(g, mother, func(a *animal.State) {
				a.Pregnant = due
				a.Mated = 3
			})
			blockBirthCells(g, 8, 8)
			advance(g, due)
			g.Step(1)
			if got := animalOf(g, mother); got.Pregnant == 0 || got.Mated != 3 || animalCount(g, animal.Goat) != 1 {
				t.Fatalf("blocked birth lost litter: %+v", got)
			}
			g.grid.SetWalkable(9, 8, true)
			g.Step(1)
			if got := animalOf(g, mother); got.Pregnant == 0 || got.Mated != 2 || animalCount(g, animal.Goat) != 2 {
				t.Fatalf("partial birth lost remaining litter: %+v animals=%d", got, animalCount(g, animal.Goat))
			}
			h := saveLoad(t, g)
			// The base fork rebuilds RoomGrid and Roofs on Load, changing its
			// entire StateHash even for a new, unmodified game. Compare the
			// authoritative animal record and birth counts here.
			if before, after := animalOf(g, mother), animalOf(h, mother); before.Mated != after.Mated ||
				before.Pregnant != after.Pregnant || animalCount(g, animal.Goat) != animalCount(h, animal.Goat) {
				t.Fatal("animal pregnancy record or child count changed after load")
			}
			if got := animalOf(h, mother); got.Mated != 2 || got.Pregnant != due {
				t.Fatalf("pending litter was not durable: %+v", got)
			}
			h.grid.SetWalkable(8, 8, true)
			h.Step(1)
			if got := animalOf(h, mother); got.Pregnant != 0 || got.Mated != 0 || animalCount(h, animal.Goat) != 4 {
				t.Fatalf("pending litter did not complete: %+v animals=%d", got, animalCount(h, animal.Goat))
			}
			kids := 0
			for _, id := range h.animalIDs {
				if id == mother {
					continue
				}
				if g := animalOf(h, id); g.Mother != uint64(mother) {
					t.Fatalf("kid %d lost mother id %+v", id, g)
				}
				kids++
			}
			if kids != 3 {
				t.Fatalf("expected three kids, got %d", kids)
			}
		})
	}
}

func TestHatchPreservesBlockedAndPartialEggStacks(t *testing.T) {
	for _, size := range []int{24, 64} {
		t.Run(map[int]string{24: "compact", 64: "legacy-sized"}[size], func(t *testing.T) {
			g := New(Config{Seed: 922, Width: size, Height: size})
			sid, err := g.SpawnStack(item.FertileEgg, 3, 8, 8)
			if err != nil {
				t.Fatal(err)
			}
			st, _ := g.stacks.Get(sid)
			st.Animal, st.Tame, st.Hatch = uint8(animal.Chicken), true, animal.HatchTicks(animal.Chicken)
			g.putStack(sid, st)
			blockBirthCells(g, 8, 8)
			g.hatch(sid, st)
			if got, ok := g.stacks.Get(sid); !ok || got.Qty != 3 || got.Hatch != st.Hatch {
				t.Fatalf("blocked eggs were consumed: %+v found=%v", got, ok)
			}
			g.grid.SetWalkable(9, 8, true)
			g.hatch(sid, st)
			rem, ok := g.stacks.Get(sid)
			if !ok || rem.Qty != 2 || animalCount(g, animal.Chicken) != 1 {
				t.Fatalf("partial hatch did not preserve eggs: %+v found=%v animals=%d", rem, ok, animalCount(g, animal.Chicken))
			}
			h := saveLoad(t, g)
			if restored, present := h.stacks.Get(sid); !present || restored != rem ||
				animalCount(g, animal.Chicken) != animalCount(h, animal.Chicken) {
				t.Fatalf("animal egg quantity, incubation or live chick state changed after load: %+v found=%t", restored, present)
			}
			rem, _ = h.stacks.Get(sid)
			h.grid.SetWalkable(8, 8, true)
			h.hatch(sid, rem)
			if _, exists := h.stacks.Get(sid); exists || animalCount(h, animal.Chicken) != 3 {
				t.Fatalf("remaining eggs were lost or duplicated: remaining=%v chicks=%d", exists, animalCount(h, animal.Chicken))
			}
			total := 0
			for _, id := range h.animalIDs {
				a := animalOf(h, id)
				if a.Species == animal.Chicken && a.Tame {
					total++
				}
			}
			if total != 3 {
				t.Fatalf("hatching lost tame ownership: %d", total)
			}
		})
	}
}
