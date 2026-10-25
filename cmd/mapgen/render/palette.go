// Package render turns a generated map's fields into PNG-ready images for
// the cmd/mapgen debug tool. It is pure Go (image only), with its own
// palettes; nothing here is shared with the game's renderer.
package render

import (
	"image/color"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Background is drawn where a value does not exist (no river, no coast).
var Background = color.RGBA{24, 24, 30, 255}

var terrainColors = [...]color.RGBA{
	terrain.Soil:         {122, 96, 64, 255},
	terrain.RichSoil:     {84, 62, 40, 255},
	terrain.Gravel:       {140, 134, 124, 255},
	terrain.Stone:        {110, 110, 116, 255},
	terrain.Rock:         {52, 54, 60, 255},
	terrain.Sand:         {212, 192, 128, 255},
	terrain.Marsh:        {72, 100, 76, 255},
	terrain.Ice:          {200, 226, 240, 255},
	terrain.RockSmooth:   {70, 72, 80, 255},
	terrain.StoneSmooth:  {150, 150, 158, 255},
	terrain.ShallowWater: {88, 150, 190, 255},
	terrain.DeepWater:    {30, 70, 130, 255},
	terrain.Mud:          {96, 76, 60, 255},
	terrain.Track:        {170, 140, 100, 255},
}

// TerrainColor is the debug colour of a ground type; unknown types are magenta.
func TerrainColor(t terrain.Type) color.RGBA {
	if int(t) >= len(terrainColors) {
		return color.RGBA{255, 0, 255, 255}
	}
	return terrainColors[t]
}

var stoneColors = [...]color.RGBA{
	geology.None:      {0, 0, 0, 255},
	geology.Sandstone: {214, 170, 110, 255},
	geology.Granite:   {150, 110, 120, 255},
	geology.Limestone: {222, 218, 190, 255},
	geology.Slate:     {80, 100, 130, 255},
	geology.Marble:    {240, 240, 250, 255},
}

// StoneColor is the debug colour of a stone type; None is black.
func StoneColor(s geology.Stone) color.RGBA {
	if int(s) >= len(stoneColors) {
		return color.RGBA{255, 0, 255, 255}
	}
	return stoneColors[s]
}

// OreColor is the colour of an ore vein by its geology.Ore code.
func OreColor(o geology.Ore) color.RGBA {
	switch o {
	case geology.Steel:
		return color.RGBA{150, 190, 230, 255}
	case geology.Silver:
		return color.RGBA{225, 225, 235, 255}
	case geology.Gold:
		return color.RGBA{240, 200, 50, 255}
	case geology.Machinery:
		return color.RGBA{60, 200, 120, 255}
	}
	return color.RGBA{255, 0, 255, 255}
}

// PlantColor groups a species: trees dark green, shrubs olive, the rest
// (ground plants, crops) yellow.
func PlantColor(s plant.Species) color.RGBA {
	switch s {
	case plant.Tree, plant.Pine, plant.JungleTree:
		return color.RGBA{16, 78, 30, 255}
	case plant.Shrub, plant.Bramble, plant.BerryBush, plant.Cactus:
		return color.RGBA{120, 130, 40, 255}
	}
	return color.RGBA{230, 210, 60, 255}
}

// Feature colours.
var (
	ChunkColor = color.RGBA{255, 255, 255, 255}
	VentColor  = color.RGBA{255, 0, 255, 255}
	RuinColor  = color.RGBA{130, 80, 40, 255}
	RoadColor  = color.RGBA{210, 180, 120, 255}
	CaveColor  = color.RGBA{240, 140, 30, 255}
	RiverLine  = color.RGBA{255, 60, 60, 255}
)

// Grey is a grey level for v in [0, 1] (clamped).
func Grey(v float32) color.RGBA {
	g := uint8(clamp01(v)*255 + 0.5)
	return color.RGBA{g, g, g, 255}
}

// WetRamp runs white (dry) to deep blue (wet).
func WetRamp(v float32) color.RGBA {
	v = clamp01(v)
	return color.RGBA{uint8(255 - 225*v), uint8(255 - 175*v), uint8(255 - 40*v), 255}
}

// Heat runs dark blue, green, yellow, red for v in [0, 1].
func Heat(v float32) color.RGBA {
	v = clamp01(v)
	switch {
	case v < 1.0/3:
		t := v * 3
		return color.RGBA{0, uint8(200 * t), uint8(120 + 80*(1-t)), 255}
	case v < 2.0/3:
		t := (v - 1.0/3) * 3
		return color.RGBA{uint8(240 * t), uint8(200 + 40*t), 0, 255}
	}
	t := (v - 2.0/3) * 3
	return color.RGBA{240, uint8(240 * (1 - t)), 0, 255}
}

// DistRamp colours a distance in cells: bright near, dark far. Far values
// (no feature, +Inf, NaN) are the Background.
func DistRamp(d float32) color.RGBA {
	if !(d >= 0) || d > 1e6 {
		return Background
	}
	v := 1 - clamp01(d/64)
	return color.RGBA{uint8(20 + 40*v), uint8(40 + 190*v), uint8(60 + 190*v), 255}
}

// RegionColor is a distinct colour per component index.
func RegionColor(k int) color.RGBA {
	if k < 0 {
		return color.RGBA{0, 0, 0, 255}
	}
	h := uint32(k)*2654435761 + 12345
	return color.RGBA{uint8(80 + h>>8&0x7f), uint8(80 + h>>16&0x7f), uint8(80 + h>>24&0x7f), 255}
}

func clamp01(v float32) float32 {
	if !(v > 0) { // also NaN
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Dim scales a colour's brightness by f in [0, 1].
func Dim(c color.RGBA, f float32) color.RGBA {
	return color.RGBA{uint8(float32(c.R) * f), uint8(float32(c.G) * f), uint8(float32(c.B) * f), 255}
}
