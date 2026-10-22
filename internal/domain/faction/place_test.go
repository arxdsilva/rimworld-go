package faction

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/randx"
)

// fakeGlobe is a W x H grid of tiles with four neighbours each (no wrap).
// Columns [0, seaCols) are ocean; the rest is land split into bands: the
// western third is forest, then ice, extreme desert, and temperate/arid
// bands, with a hilly stripe.
type fakeGlobe struct {
	w, h    int
	seaCols int
	hops    int // Hops calls, to keep an eye on cost
}

func (g *fakeGlobe) TileCount() int { return g.w * g.h }
func (g *fakeGlobe) Land(i int) bool {
	return i%g.w >= g.seaCols
}
func (g *fakeGlobe) Biome(i int) biome.ID {
	x, y := i%g.w, i/g.w
	switch {
	case x < g.seaCols:
		return biome.TemperateForest // ocean tiles carry a dummy biome
	case y < 6:
		return biome.IceSheet
	case y < 10:
		return biome.ExtremeDesert
	case x < g.seaCols+(g.w-g.seaCols)/3:
		return biome.BorealForest
	case x < g.seaCols+2*(g.w-g.seaCols)/3:
		return biome.TemperateForest
	}
	return biome.AridShrubland
}
func (g *fakeGlobe) Hills(i int) uint8 {
	if i%g.w%10 < 3 { // vertical hill stripes
		return 3
	}
	return 1
}
func (g *fakeGlobe) Hops(a, b, limit int) int {
	g.hops++
	ax, ay, bx, by := a%g.w, a/g.w, b%g.w, b/g.w
	d := abs(ax-bx) + abs(ay-by)
	return min(d, limit)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func place(t *testing.T, pop int, seed uint64) (*fakeGlobe, []Faction) {
	t.Helper()
	g := &fakeGlobe{w: 240, h: 130, seaCols: 20}
	fs := Generate(DefaultSlots(), randx.New(seed))
	PlaceSettlements(g, fs, pop, randx.New(seed+100))
	return g, fs
}

func TestPlaceSettlementsRules(t *testing.T) {
	t.Parallel()
	land := 0
	{
		g := &fakeGlobe{w: 240, h: 130, seaCols: 20}
		for i := 0; i < g.TileCount(); i++ {
			if g.Land(i) {
				land++
			}
		}
	}
	for _, pop := range []int{50, 100, 160} {
		g, fs := place(t, pop, 7)
		spacing := Spacing(pop)
		var all []int
		names := map[string]bool{}
		for _, f := range fs {
			want := SettlementCount(f.Type, land, pop)
			if len(f.Settlements) != want || len(f.SettlementNames) != want {
				t.Errorf("pop %d %s: %d settlements / %d names, want %d", pop, f.Name, len(f.Settlements), len(f.SettlementNames), want)
			}
			if len(f.Settlements) == 0 {
				t.Errorf("pop %d: %s has no capital", pop, f.Name)
			}
			for k, tile := range f.Settlements {
				if !g.Land(tile) {
					t.Errorf("settlement %d in the ocean", tile)
				}
				if b := g.Biome(tile); b == biome.IceSheet || b == biome.SeaIce || b == biome.ExtremeDesert {
					t.Errorf("settlement %d in excluded biome %d", tile, b)
				}
				for _, o := range all {
					if o == tile || g.Hops(tile, o, spacing) < spacing {
						t.Errorf("pop %d: settlements %d and %d closer than %d tiles", pop, tile, o, spacing)
					}
				}
				all = append(all, tile)
				n := f.SettlementNames[k]
				if n == "" || names[n] {
					t.Errorf("settlement name %q empty or repeated", n)
				}
				names[n] = true
				if p, bad := Blocked(n); bad {
					t.Errorf("settlement name %q hits blocklist %q", n, p)
				}
			}
		}
		if pop == 100 {
			// Clans prefer forest, Settlers temperate/arid, Freebooters
			// hills. The map has room, so every capital is in its band.
			for _, f := range fs {
				capital := f.Settlements[0]
				switch f.Type {
				case Clans:
					if b := g.Biome(capital); b != biome.BorealForest && b != biome.TemperateForest {
						t.Errorf("clan capital in biome %d", b)
					}
				case Settlers:
					if b := g.Biome(capital); b != biome.TemperateForest && b != biome.AridShrubland {
						t.Errorf("settler capital in biome %d", b)
					}
				case Freebooters:
					if g.Hills(capital) < 2 {
						t.Errorf("freebooter capital is not on hills")
					}
				}
			}
		}
	}
	// More people, more settlements.
	count := func(pop int) int {
		_, fs := place(t, pop, 7)
		n := 0
		for _, f := range fs {
			n += len(f.Settlements)
		}
		return n
	}
	if sparse, normal, crowded := count(50), count(100), count(160); !(sparse < normal && normal < crowded) {
		t.Errorf("sparse %d, normal %d, crowded %d should rise", sparse, normal, crowded)
	}
}

func TestPlaceSettlementsDeterministicAndEdgeCases(t *testing.T) {
	t.Parallel()
	_, a := place(t, 100, 21)
	_, b := place(t, 100, 21)
	for i := range a {
		if len(a[i].Settlements) != len(b[i].Settlements) {
			t.Fatal("placement differs between runs")
		}
		for k := range a[i].Settlements {
			if a[i].Settlements[k] != b[i].Settlements[k] || a[i].SettlementNames[k] != b[i].SettlementNames[k] {
				t.Fatal("placement differs between runs")
			}
		}
	}
	// A world of only ice has nowhere to settle.
	ice := &fakeGlobe{w: 30, h: 5, seaCols: 0} // y < 6 is ice everywhere
	fs := Generate(DefaultSlots(), randx.New(1))
	PlaceSettlements(ice, fs, 100, randx.New(1))
	for _, f := range fs {
		if len(f.Settlements) != 0 {
			t.Errorf("%s settled on ice", f.Name)
		}
	}
	// A tiny world fills up and stops instead of looping.
	tiny := &fakeGlobe{w: 14, h: 14, seaCols: 0}
	fs = Generate([]Slot{{Clans, 4}, {Settlers, 4}}, randx.New(1))
	PlaceSettlements(tiny, fs, 160, randx.New(1))
	// Existing settlements are kept and respected on a second call.
	before := len(fs[0].Settlements)
	PlaceSettlements(tiny, fs, 160, randx.New(2))
	if len(fs[0].Settlements) < before {
		t.Error("existing settlements were dropped")
	}
	// Swarm gets none.
	sw := Generate([]Slot{{Swarm, 1}}, randx.New(1))
	PlaceSettlements(&fakeGlobe{w: 240, h: 130, seaCols: 20}, sw, 100, randx.New(1))
	if len(sw[0].Settlements) != 0 {
		t.Error("the swarm has no settlements")
	}
}
