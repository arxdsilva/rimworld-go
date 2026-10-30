package game

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// STO-1: the storage search on the ground index and zone.Stores answers
// exactly as the original search over every zone and every item did.

// storageCellRef is the original storageCell: every zone of each priority,
// two passes, the items found by scanning the item list.
func storageCellRef(g *Game, st item.Stack, nx, ny int, keep func(x, y int) bool, floor zone.Priority) (int, int, bool) {
	loose := func(x, y int) (first entity.EntityID, n int, top bool) {
		for _, id := range g.itemIDs {
			if o := g.stacks.Ref(id); o != nil && o.Carrier == 0 && o.X == x && o.Y == y {
				if n == 0 {
					first = id
				}
				n++
				top = top || o.Kind == st.Kind && haul.Toppable(*o)
			}
		}
		return first, n, top
	}
	free := func(x, y int) bool {
		_, taken := g.reserve.TileOwner(x, y)
		return !taken && (keep == nil || keep(x, y))
	}
	for _, p := range zone.Priorities {
		if p <= floor {
			break
		}
		for pass := 0; pass < 2; pass++ {
			x, y, ok, bestD := 0, 0, false, 0
			g.zones.Each(func(z zone.Zone) bool {
				if !z.Kind.Storage() || z.Priority != p || !z.AllowsStack(st) {
					return true
				}
				shelf := z.Kind == zone.KindShelf
				g.zones.EachCell(z.ID, func(cx, cy int) bool {
					d := pathing.Dist(nx, ny, cx, cy)
					if ok && (d > bestD || d == bestD && (cy > y || cy == y && cx > x)) {
						return true
					}
					first, n, top := loose(cx, cy)
					var good bool
					if pass == 0 {
						o := g.stacks.Ref(first)
						good = first != 0 && free(cx, cy) && (shelf && top || !shelf && o.Kind == st.Kind && haul.Toppable(*o))
					} else {
						good = (shelf && n < zone.ShelfStacks || !shelf && first == 0) && g.grid.Walkable(cx, cy) && free(cx, cy) &&
							g.layerFrameAt(cx, cy, construction.LayerObject) == 0 && (shelf || !g.occupiedAt(cx, cy))
					}
					if good {
						x, y, ok, bestD = cx, cy, true, d
					}
					return true
				})
				return true
			})
			if ok {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// TestStorageCellMatchesFullSearch: on random yards (zones of every
// storage kind and priority, stacks, reservations, an area) the indexed
// search and roomAbove agree with the original search everywhere.
func TestStorageCellMatchesFullSearch(t *testing.T) {
	t.Parallel()
	kinds := []item.Kind{item.Wood, item.Steel, item.Meal, item.Pemmican, item.Rifle, item.CorpseAnimal}
	for seed := uint64(1); seed <= 6; seed++ {
		r := rand.New(rand.NewPCG(seed, 9))
		g := New(Config{Seed: seed, Width: 40, Height: 30})
		for range 7 {
			x, y := r.IntN(34), r.IntN(24)
			kind := []zone.Kind{zone.KindStockpile, zone.KindDump, zone.KindShelf}[r.IntN(3)]
			proto := zone.Zone{Food: true, Wood: true, Stone: true, Corpses: true}
			if kind == zone.KindShelf {
				proto = zone.NewShelf()
			}
			id := g.zones.Paint(kind, proto, zone.RectOf(x, y, x+r.IntN(6), y+r.IntN(5)), nil)
			if p := zone.Priorities[r.IntN(len(zone.Priorities))]; id != 0 {
				g.zones.Update(id, func(z *zone.Zone) { z.Priority = p })
			}
		}
		for range 120 {
			k := kinds[r.IntN(len(kinds))]
			if _, err := g.SpawnStack(k, 1+r.IntN(k.StackLimit()), r.IntN(40), r.IntN(30)); err != nil {
				t.Fatal(err)
			}
		}
		for i := range 15 {
			g.reserve.TryTile(entity.EntityID(900+i), r.IntN(40), r.IntN(30))
		}
		keep, found := func(x, y int) bool { return x < 30 }, 0
		for range 300 {
			st := item.Stack{Kind: kinds[r.IntN(len(kinds))]}
			nx, ny, floor := r.IntN(40), r.IntN(30), zone.Priority(noPriority)
			if r.IntN(3) == 0 {
				floor = zone.Priorities[r.IntN(len(zone.Priorities))]
			}
			k := keep
			if r.IntN(2) == 0 {
				k = nil
			}
			gx, gy, gok := g.storageCell(st, nx, ny, k, floor)
			wx, wy, wok := storageCellRef(g, st, nx, ny, k, floor)
			if gx != wx || gy != wy || gok != wok {
				t.Fatalf("seed %d: storageCell(%v, %d,%d, floor %v) = %d,%d,%v, full search %d,%d,%v", seed, st.Kind, nx, ny, floor, gx, gy, gok, wx, wy, wok)
			}
			if gok {
				found++
			}
			var h haulPrio
			if _, _, any := storageCellRef(g, st, 0, 0, nil, floor); h.roomAbove(g, st.Kind, floor) != any {
				t.Fatalf("seed %d: roomAbove(%v, %v) disagrees with the full search", seed, st.Kind, floor)
			}
		}
		if found < 100 {
			t.Fatalf("seed %d: only %d of 300 searches found a cell", seed, found)
		}
	}
}
