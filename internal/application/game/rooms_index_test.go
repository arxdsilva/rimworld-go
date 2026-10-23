package game

import (
	"maps"
	"slices"
	"testing"

	"rimworld-go/pkg/randx"
)

// TestRoomsIncrementalMatchesFull: a colony rebuilding its rooms
// incrementally and its twin rebuilding them in full (regions off) stay
// identical — room grid and numbering, contents and scores, temperatures
// and residual air, roof cells, plans and the collapse flag — through
// random walls, doors, furniture, floors, mining, rock, smoothing,
// removals, roof designations and the passing hours. (The self-check also
// proves every incremental rebuild against room.Detect.)
func TestRoomsIncrementalMatchesFull(t *testing.T) {
	kinds := []string{"wall", "wall", "wall", "wall", "door", "bed", "table", "chair", "lamp", "floor", "fence"}
	stuffs := []string{"wood", "granite blocks", "steel"}
	for seed := uint64(1); seed <= 3; seed++ {
		inc := NewColony(Config{Seed: seed, Width: 70, Height: 56})
		ful := NewColony(Config{Seed: seed, Width: 70, Height: 56})
		ful.regs.off = true
		r := randx.New(seed)
		both := func(f func(g *Game)) { f(inc); f(ful) }
		for batch := 0; batch < 25; batch++ {
			for k := 0; k < 12; k++ {
				x, y := r.IntN(70), r.IntN(56)
				kind, stuff := kinds[r.IntN(len(kinds))], stuffs[r.IntN(len(stuffs))]
				switch v := r.IntN(12); {
				case v < 6:
					both(func(g *Game) { g.BuildNow(kind, stuff, "south", x, y) })
				case v == 6: // a walled box: a room
					x1, y1 := x+3+r.IntN(6), y+3+r.IntN(6)
					both(func(g *Game) {
						for yy := y; yy <= y1; yy++ {
							for xx := x; xx <= x1; xx++ {
								if xx == x || xx == x1 || yy == y || yy == y1 {
									g.BuildNow("wall", stuff, "south", xx, yy)
								}
							}
						}
					})
				case v == 7:
					both(func(g *Game) { g.SetGround(x, y, x, y, "soil", "", "") })
				case v == 8:
					both(func(g *Game) { g.SetGround(x, y, x+1, y+1, "rock", "granite", "") })
				case v == 9:
					both(func(g *Game) { g.SetGround(x, y, x+2, y, "stone_smooth", "granite", "") })
				case v == 10:
					if n := len(inc.bldgIDs); n > 0 {
						id := inc.bldgIDs[r.IntN(n)]
						both(func(g *Game) { g.removeBuilding(id) })
					}
				default:
					build := r.IntN(3) > 0
					both(func(g *Game) { g.designateRoof(x, y, x+4, y+4, build) })
				}
			}
			both(func(g *Game) { g.ensureRooms() })
			compareRooms(t, inc, ful)
			if batch%5 == 4 {
				both(func(g *Game) { g.Step(300) })
				compareRooms(t, inc, ful)
			}
		}
		t.Logf("seed %d: %d rooms, %d residual cells, %d plans", seed, len(inc.rooms), len(inc.residual), len(inc.roofx.plans))
		if inc.StateHash() != ful.StateHash() {
			t.Fatalf("seed %d: state hashes differ", seed)
		}
	}
}

func compareRooms(t *testing.T, a, b *Game) {
	t.Helper()
	a.ensureRooms()
	b.ensureRooms()
	switch {
	case !slices.Equal(a.roomGrid, b.roomGrid):
		t.Fatal("room grids differ")
	case !slices.Equal(a.rooms, b.rooms):
		t.Fatalf("room stats differ:\n%+v\n%+v", a.rooms, b.rooms)
	case !slices.Equal(a.roomTemps, b.roomTemps):
		t.Fatalf("room temperatures differ:\n%v\n%v", a.roomTemps, b.roomTemps)
	case !maps.Equal(a.residual, b.residual):
		t.Fatalf("residual air differs: %d vs %d cells", len(a.residual), len(b.residual))
	case !slices.Equal(a.roofs, b.roofs):
		t.Fatal("roof cells differ")
	case !slices.Equal(a.roofx.plans, b.roofx.plans):
		t.Fatal("roof plans differ")
	case a.roofx.unsupported != b.roofx.unsupported:
		t.Fatal("collapse flags differ")
	}
	for r := range a.rooms {
		if a.roomRoofed(r) != b.roomRoofed(r) {
			t.Fatalf("room %d roofed share differs", r)
		}
	}
}
