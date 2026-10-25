package game

import (
	"rimworld-go/internal/domain/world/terrain"
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
)

// fieldsGame is a Setup colony stamped with the current generator version.
func fieldsGame(t *testing.T) *Game {
	t.Helper()
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 64 }))
	g.ng.setup.GenVersion = mapgen.Version
	return g
}

func TestMapFieldLayersAllLayersInRange(t *testing.T) {
	g := fieldsGame(t)
	layers, err := g.MapFieldLayers()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Elevation", "Fertility", "Wetness", "Rock types", "Caves", "River distance",
		"Terrain classes", "Vegetation density", "Start score", "Roofs", "Fog", "Regions"}
	if len(layers) != len(want) {
		t.Fatalf("%d layers, want %d", len(layers), len(want))
	}
	n := g.grid.Width() * g.grid.Height()
	for i, l := range layers {
		if l.Name != want[i] || l.W != g.grid.Width() || l.H != g.grid.Height() {
			t.Fatalf("layer %d = %q %dx%d", i, l.Name, l.W, l.H)
		}
		switch {
		case l.Scalar != nil:
			if len(l.Scalar) != n {
				t.Fatalf("%s: %d scalars", l.Name, len(l.Scalar))
			}
			for _, v := range l.Scalar {
				if v < 0 || v > 1 {
					t.Fatalf("%s: value %v out of [0,1]", l.Name, v)
				}
			}
		default:
			if len(l.Classes) != n || len(l.Legend) == 0 {
				t.Fatalf("%s: %d classes, %d legend", l.Name, len(l.Classes), len(l.Legend))
			}
			for _, c := range l.Classes {
				if int(c) >= len(l.Legend) {
					t.Fatalf("%s: class %d has no legend entry", l.Name, c)
				}
			}
		}
	}
}

func TestMapFieldLayersOldOrLegacyColonyErrors(t *testing.T) {
	g := fieldsGame(t)
	g.ng.setup.GenVersion = 0
	if _, err := g.MapFieldLayers(); err == nil {
		t.Fatal("old generator version must error")
	}
	if _, err := New(Config{Seed: 3, Width: 30, Height: 30}).MapFieldLayers(); err == nil {
		t.Fatal("legacy colony must error")
	}
}

func TestMapFieldLayersCachedAndSideEffectFree(t *testing.T) {
	g := fieldsGame(t)
	h := g.StateHash()
	a, err := g.MapFieldLayers()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := g.MapFieldLayers()
	if &a[0].Scalar[0] != &b[0].Scalar[0] {
		t.Fatal("generated layers must be cached")
	}
	if g.StateHash() != h {
		t.Fatal("reading the layers changed the state")
	}
}

func TestMapFieldLayersLiveLayersFollowTheGame(t *testing.T) {
	g := fieldsGame(t)
	g.fog.hidden = make([]uint64, (g.grid.Width()*g.grid.Height()+63)/64)
	g.fog.hidden[0] |= 1
	layers, _ := g.MapFieldLayers()
	var fog MapFieldLayer
	for _, l := range layers {
		if l.Name == "Fog" {
			fog = l
		}
	}
	if fog.Classes[0] != 1 || fog.Classes[1] != 0 {
		t.Fatalf("fog classes %v", fog.Classes[:2])
	}
}

// MAPGEN-3: the terrain layer marks road bridges and sealed vaults over the
// ground, after the terrain types.
func TestTerrainLegendHasBridgesAndVaults(t *testing.T) {
	l := terrainLegend()
	n := len(terrain.Names())
	if len(l) != n+2 || l[n].Label != "bridge" || l[n+1].Label != "sealed vault" {
		t.Fatalf("legend tail %+v", l[n:])
	}
}
