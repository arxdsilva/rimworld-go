package mapview

import (
	"strings"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// The ground's read models (SIM-029): terrain, stone and ore names and the
// hover labels; the game hands them out as game.Soils, game.SoilLabel,
// game.StoneNames and game.OreNames.

// MinedOres is every ore a rock cell can hold, in code order: the metal
// veins of geology.Ores() and the buried machinery (game.WorldView.Ore
// codes, OreNames, game.SetGround).
func MinedOres() []geology.Ore { return append(geology.Ores(), geology.Machinery) }

// StoneNames maps game.WorldView.StoneType codes to names ("" is none).
func StoneNames() []string {
	out := []string{""}
	for _, s := range geology.Stones() {
		out = append(out, s.String())
	}
	return out
}

// OreNames maps game.WorldView.Ore codes to names ("" is none).
func OreNames() []string {
	out := []string{""}
	for _, o := range MinedOres() {
		out = append(out, o.String())
	}
	return out
}

// soilLabels are the plain ground labels (game.Soils).
var soilLabels = map[string]string{
	"soil": "Soil", "rich_soil": "Rich soil", "gravel": "Gravel",
	"stone": "Rough stone", "rock": "Rock wall",
	"sand": "Sand", "marsh": "Marsh", "ice": "Ice",
	"rock_smooth": "Smoothed rock wall", "stone_smooth": "Smooth stone floor",
	"shallow_water": "Shallow water", "deep_water": "Deep water", "mud": "Mud", "track": "Dirt track",
}

// Soils lists every ground type, indexed by the game.WorldView.Terrain code.
func Soils() []Soil {
	names := terrain.Names()
	out := make([]Soil, len(names))
	for i, n := range names {
		t := terrain.Type(i)
		out[i] = Soil{Name: n, Label: soilLabels[n], Fertility: t.Fertility(),
			Farmable: t.Fertility() > 0, Walkable: t.Walkable(), WalkCost: t.WalkCost(),
			Smoothable: t == terrain.Rock || t == terrain.Stone}
	}
	return out
}

// SoilLabel is the hover name of a tile's ground (SIM-029): ground, stone
// and ore are names as in game.TerrainNames(), StoneNames() and OreNames(), ""
// meaning none. Natural stone names its stone ("Granite rock wall",
// "Smooth granite floor", "Compacted steel (in granite)"); without a
// stone it is the plain Soils() label; an unknown ground is "".
func SoilLabel(ground, stone, ore string) string {
	s, ok := geology.ParseStone(stone)
	if !ok {
		return soilLabels[ground]
	}
	name := strings.ToLower(s.Label())
	switch ground {
	case "rock":
		for _, o := range MinedOres() {
			if o.String() == ore {
				return o.Label() + " (in " + name + ")"
			}
		}
		return s.Label() + " rock wall"
	case "rock_smooth":
		return "Smoothed " + name + " wall"
	case "stone":
		return "Rough " + name + " ground"
	case "stone_smooth":
		return "Smooth " + name + " floor"
	}
	return soilLabels[ground]
}
