package game

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/mapgen"
)

// PLAY-021 FUELGEN: steam vents live on the foundation layer, so a
// geothermal generator can stand on top of one (vents.go).

func TestSteamVentDefIsAFoundationLayerFeature(t *testing.T) {
	t.Parallel()
	d := construction.DefOf(construction.SteamVent)
	if d.Layer != construction.LayerFoundation || !d.Natural || d.Size != [2]int{2, 2} || d.BlocksMove {
		t.Fatalf("steam vent def: %+v", d)
	}
	if d.Foundation {
		t.Fatal("a vent is not a bridge deck: Foundation must stay false")
	}
}

func TestVentSitsOnTheFoundationLayer(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	vid := bldgIDAt(g, construction.SteamVent, 70, 30)
	if vid == 0 {
		t.Fatal("no vent")
	}
	for _, c := range [][2]int{{70, 30}, {71, 30}, {70, 31}, {71, 31}} {
		if g.layerBldgAt(c[0], c[1], construction.LayerFoundation) != vid {
			t.Errorf("(%d,%d): the foundation layer does not hold the vent", c[0], c[1])
		}
		if g.layerBldgAt(c[0], c[1], construction.LayerObject) != 0 {
			t.Errorf("(%d,%d): the object layer still holds something", c[0], c[1])
		}
		if !g.naturalAt(c[0], c[1]) {
			t.Errorf("(%d,%d): naturalAt misses the vent", c[0], c[1])
		}
		if g.ventAt(c[0], c[1]) != vid {
			t.Errorf("(%d,%d): ventAt = %d, want %d", c[0], c[1], g.ventAt(c[0], c[1]), vid)
		}
		if !g.grid.Walkable(c[0], c[1]) {
			t.Errorf("(%d,%d): a vent never blocks the way", c[0], c[1])
		}
		if g.bridged(c[0], c[1]) {
			t.Errorf("(%d,%d): a vent must not read as a bridge", c[0], c[1])
		}
	}
	for _, c := range [][2]int{{69, 30}, {72, 30}, {70, 29}, {70, 32}} {
		if g.naturalAt(c[0], c[1]) || g.ventAt(c[0], c[1]) != 0 {
			t.Errorf("(%d,%d) is beside the vent, not on it", c[0], c[1])
		}
	}
}

func TestVentUnderIsExactTopLeft(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	if !g.ventUnder(70, 30) {
		t.Fatal("a 2x2 piece at the vent's origin sits exactly over it")
	}
	for _, c := range [][2]int{{71, 30}, {70, 31}, {71, 31}, {69, 30}, {70, 29}, {5, 5}, {-1, -1}} {
		if g.ventUnder(c[0], c[1]) {
			t.Errorf("(%d,%d): a 2x2 piece there is not exactly over the vent", c[0], c[1])
		}
	}
}

func TestGeothermalOverAVentKeepsTheVentAndItsWarmth(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	g.ensureRooms()
	ctrl := g.temperatureAt(90, 10)
	before := g.temperatureAt(70, 30)
	gen := g.placeGenerated(construction.Building{Kind: construction.GeothermalGenerator, Stuff: item.Steel, X: 70, Y: 30})
	if gen == 0 {
		t.Fatal("a generator cannot stand over the vent's foundation layer")
	}
	if vid := bldgIDAt(g, construction.SteamVent, 70, 30); vid == 0 || g.layerBldgAt(70, 30, construction.LayerFoundation) != vid {
		t.Fatal("the vent must stay under the generator")
	}
	if g.layerBldgAt(71, 31, construction.LayerObject) != gen {
		t.Fatal("the generator holds the object layer")
	}
	if got := g.temperatureAt(70, 30); got != before || got-ctrl != ventOutdoorHeat {
		t.Fatalf("vent warmth changed: %d -> %d (control %d)", before, got, ctrl)
	}
	if got := g.genOutput(gen); got != 1800 {
		t.Fatalf("geothermal output %d", got)
	}
}

// ventFingerprints are the vents map generation left in whole colonies
// before the vents moved to the foundation layer (steam vents in
// LayerObject), by seed: the move must not change where they stand.
var ventFingerprints = map[uint64][][2]int{
	1:      nil,
	7:      {{41, 93}, {91, 58}},
	99:     nil,
	4242:   {{20, 85}, {32, 62}},
	31337:  {{26, 79}},
	123456: {{28, 44}},
}

func TestMapGenerationPlacesTheSameVents(t *testing.T) {
	t.Parallel()
	seeds := make([]uint64, 0, len(ventFingerprints))
	for s := range ventFingerprints {
		seeds = append(seeds, s)
	}
	slices.Sort(seeds)
	for _, seed := range seeds {
		s := setupFor(t, "broken_hull", func(s *Setup) { s.Seed = seed })
		b := NewColonyBuilder(Config{Setup: s})
		for i := 0; i <= buildSteps() && b.Game() == nil; i++ {
			b.Step(1)
		}
		g := b.Game()
		if g == nil {
			t.Fatalf("seed %d: the builder never finished", seed)
		}
		var got [][2]int
		for _, id := range g.bldgIDs {
			if v, ok := g.bldgs.Get(id); ok && v.Kind == construction.SteamVent {
				got = append(got, [2]int{v.X, v.Y})
				if g.layerBldgAt(v.X, v.Y, construction.LayerFoundation) != id {
					t.Errorf("seed %d: vent at %d,%d is not on the foundation layer", seed, v.X, v.Y)
				}
			}
		}
		sort.Slice(got, func(i, j int) bool { return got[i][0] < got[j][0] || got[i][0] == got[j][0] && got[i][1] < got[j][1] })
		want := ventFingerprints[seed]
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("seed %d: vents %v, want %v", seed, got, want)
		}
		// every vent the generator asked for was placed (none refused)
		if len(got) != len(b.res.Vents) {
			t.Errorf("seed %d: %d vents placed of %d generated", seed, len(got), len(b.res.Vents))
		}
	}
}

func TestVentOverARuinPieceIsStillRefused(t *testing.T) {
	t.Parallel()
	// siteFixture's shed has walls at (40,20) and (41,20): a vent over them
	// was refused when both shared the object layer, and still is
	g := siteGame(t, "broken_hull", func(res *mapgen.Result) {
		res.Vents = append(res.Vents, mapgen.Point{X: 40, Y: 20})
	})
	if bldgIDAt(g, construction.SteamVent, 40, 20) != 0 {
		t.Fatal("a vent was placed over a ruin wall")
	}
	if bldgIDAt(g, construction.SteamVent, 70, 30) == 0 {
		t.Fatal("the fixture's own vent is gone")
	}
}

func TestOverlappingVentsAreStillRefused(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 40, Height: 40})
	if g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: item.GraniteBlocks, X: 10, Y: 10}) == 0 {
		t.Fatal("first vent")
	}
	if g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: item.GraniteBlocks, X: 11, Y: 11}) != 0 {
		t.Fatal("a second vent overlapping the first was placed")
	}
}
