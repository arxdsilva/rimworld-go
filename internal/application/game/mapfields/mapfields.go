// Package mapfields is the developer read model of the fields the map
// generator built a map from (MAPGEN-2 §6.2): debug layers regenerated from
// the saved Setup on demand, plus the ones read off the live map. It never
// touches the live simulation or its random streams; the game package caches
// the generated layers and passes in what the live ones read (mapfields.go).
package mapfields

import (
	"errors"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// LegendEntry is one class of a class layer.
type LegendEntry struct {
	Label string
	RGBA  [4]uint8
}

// Layer is one debug view over the whole map (row-major W×H). A scalar layer
// fills Scalar with values in [0, 1]; a class layer fills Classes (each
// indexing Legend). Marks are cells to highlight.
type Layer struct {
	Name    string
	W, H    int
	Scalar  []float32
	Classes []uint8
	Legend  []LegendEntry
	Marks   [][2]int
}

// Generate regenerates the fields with a private pipeline run: the layers of
// a w×h map from seed at this site.
func Generate(seed uint64, w, h int, at site.Site) ([]Layer, error) {
	res, err := pipeline.Generate(mapgen.Config{Seed: seed, W: w, H: h, Site: at,
		Biome: biome.DefOf(at.Biome), Debug: true})
	if err != nil {
		return nil, err
	}
	f := res.Fields
	if f == nil {
		return nil, errors.New("generator kept no fields")
	}
	scalar := func(name string, v []float32) Layer {
		return Layer{Name: name, W: w, H: h, Scalar: clamp01All(v)}
	}
	river := make([]float32, len(f.RiverDist))
	for i, d := range f.RiverDist { // near = bright, none = 0
		if d < riverFade {
			river[i] = 1 - d/riverFade
		}
	}
	rock := make([]uint8, len(f.Stone))
	caves := make([]uint8, len(f.Stone))
	for i, st := range f.Stone {
		rock[i] = uint8(st)
		if f.Rock[i] {
			caves[i] = 1
		} else if f.Cave[i] {
			caves[i] = 2
		}
	}
	terr := make([]uint8, w*h)
	for i, t := range res.Terrain.Cells {
		terr[i] = uint8(t)
	}
	// MAPGEN-3 features over the ground: road bridges and sealed vaults
	// (classes after the terrain types, see TerrainLegend).
	nt := uint8(len(terrain.Names()))
	for _, v := range res.Vaults {
		for y := v.Y0; y <= v.Y1; y++ {
			for x := v.X0; x <= v.X1; x++ {
				if x >= 0 && y >= 0 && x < w && y < h {
					terr[y*w+x] = nt + 1
				}
			}
		}
	}
	for _, b := range res.Bridges {
		if b.X >= 0 && b.Y >= 0 && b.X < w && b.Y < h {
			terr[b.Y*w+b.X] = nt
		}
	}
	start := scalar("Start score", f.StartScore)
	start.Marks = [][2]int{{res.Start.X, res.Start.Y}}
	return []Layer{
		scalar("Elevation", f.Elevation), scalar("Fertility", f.Fertility), scalar("Wetness", f.Wetness),
		{Name: "Rock types", W: w, H: h, Classes: rock, Legend: stoneLegend()},
		{Name: "Caves", W: w, H: h, Classes: caves, Legend: []LegendEntry{
			{"open ground", [4]uint8{60, 70, 60, 255}}, {"rock", [4]uint8{140, 140, 150, 255}}, {"cave", [4]uint8{250, 190, 60, 255}}}},
		scalar("River distance", river),
		{Name: "Terrain classes", W: w, H: h, Classes: terr, Legend: TerrainLegend()},
		scalar("Vegetation density", f.VegDensity),
		start,
	}, nil
}

// riverFade is how far (cells) the river-distance view shades.
const riverFade = 40

func clamp01All(v []float32) []float32 {
	out := make([]float32, len(v))
	for i, x := range v {
		switch {
		case x < 0 || x != x:
			out[i] = 0
		case x > 1:
			out[i] = 1
		default:
			out[i] = x
		}
	}
	return out
}

var stonePalette = [...][4]uint8{{40, 40, 40, 255}, {214, 180, 120, 255}, {130, 130, 140, 255}, {225, 220, 190, 255},
	{70, 90, 120, 255}, {235, 235, 245, 255}}

func stoneLegend() []LegendEntry {
	out := []LegendEntry{{"none", stonePalette[0]}}
	for _, s := range geology.Stones() {
		out = append(out, LegendEntry{s.Label(), stonePalette[s]})
	}
	return out
}

// TerrainLegend names the terrain classes, then the features over them.
func TerrainLegend() []LegendEntry {
	names := terrain.Names()
	out := make([]LegendEntry, len(names))
	for i, n := range names {
		h := uint32(i+1) * 2654435761
		out[i] = LegendEntry{n, [4]uint8{uint8(h >> 8), uint8(h >> 16), uint8(h >> 24), 255}}
	}
	return append(out, LegendEntry{"bridge", [4]uint8{170, 120, 70, 255}},
		LegendEntry{"sealed vault", [4]uint8{150, 90, 200, 255}})
}

// Roofs is the live roof layer over the w×h map's cells.
func Roofs(w, h int, roofs []roof.Cell) Layer {
	cl := make([]uint8, w*h)
	for i, c := range roofs {
		switch {
		case i >= len(cl) || !c.Roofed():
		case c.Has(roof.Natural | roof.NaturalThick):
			cl[i] = 2
		default:
			cl[i] = 1
		}
	}
	return Layer{Name: "Roofs", W: w, H: h, Classes: cl, Legend: []LegendEntry{
		{"none", [4]uint8{50, 60, 50, 255}}, {"thin", [4]uint8{210, 190, 90, 255}}, {"thick", [4]uint8{150, 70, 60, 255}}}}
}

// Fog is the live fog layer.
func Fog(w, h int, fogged func(x, y int) bool) Layer {
	cl := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if fogged(x, y) {
				cl[y*w+x] = 1
			}
		}
	}
	return Layer{Name: "Fog", W: w, H: h, Classes: cl, Legend: []LegendEntry{
		{"explored", [4]uint8{60, 90, 60, 255}}, {"unexplored", [4]uint8{20, 20, 30, 255}}}}
}

// regionPalette colours the largest walkable components; the rest share
// the last entry.
var regionPalette = [...][4]uint8{{40, 40, 40, 255}, {90, 200, 90, 255}, {90, 150, 230, 255}, {230, 200, 80, 255},
	{220, 100, 100, 255}, {190, 110, 220, 255}, {90, 210, 210, 255}, {230, 140, 60, 255}, {160, 160, 160, 255}}

// Regions is the live region layer: the walkable components of the w×h map,
// largest first.
func Regions(w, h int, walkable func(x, y int) bool) Layer {
	walk := make([]bool, w*h)
	for i := range walk {
		walk[i] = walkable(i%w, i/w)
	}
	label, sizes := mapgen.Components(w, h, walk, false)
	rank := make([]int, len(sizes)) // component -> class by size, largest = 1
	order := make([]int, len(sizes))
	for i := range order {
		order[i] = i
	}
	for i := 1; i < len(order); i++ { // stable insertion sort, size descending
		for j := i; j > 0 && sizes[order[j]] > sizes[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	last := len(regionPalette) - 1
	for r, c := range order {
		rank[c] = min(r+1, last)
	}
	cl := make([]uint8, w*h)
	for i, l := range label {
		if l >= 0 {
			cl[i] = uint8(rank[l])
		}
	}
	legend := []LegendEntry{{"blocked", regionPalette[0]}}
	for k := 1; k <= last; k++ {
		name := "region"
		if k == last {
			name = "smaller regions"
		}
		legend = append(legend, LegendEntry{name, regionPalette[k]})
	}
	if len(sizes) < last { // keep the legend to the classes in use
		legend = legend[:len(sizes)+1]
	}
	return Layer{Name: "Regions", W: w, H: h, Classes: cl, Legend: legend}
}
