package game

import (
	"sort"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/world/biome"
)

// BiomeInfo is what the new-game landing picker shows of one biome: a
// read-only card built from the biome definition, usable before any
// colony exists.
type BiomeInfo struct {
	ID         biome.ID
	Key        string // "boreal_forest"
	Name       string
	Blurb      string
	Icon       string // sprite key of the picker icon
	Difficulty string // "", "hard" or "extreme"
	// WinterLow and SummerHigh are the year's coldest and warmest outdoor
	// air in whole degrees Celsius.
	WinterLow, SummerHigh int
	// GrowingDays counts the days a year with a daily mean above 3 C.
	GrowingDays int
	Ground      []string // soft ground kinds, most common first
	Plants      []string // wild plants, most common first
	Wildlife    []string // top species by commonality, at most 5
}

// Biomes lists every biome card in picker order.
func Biomes() []BiomeInfo {
	all := biome.All()
	out := make([]BiomeInfo, len(all))
	for i, d := range all {
		out[i] = biomeInfo(d)
	}
	return out
}

// BiomeInfoOf is biome id's picker card.
func BiomeInfoOf(id biome.ID) BiomeInfo { return biomeInfo(biome.DefOf(id)) }

func biomeInfo(d biome.Def) BiomeInfo {
	lo, hi := d.Climate.Range()
	info := BiomeInfo{
		ID: d.ID, Key: d.Key, Name: d.Name, Blurb: d.Blurb, Icon: d.Icon(), Difficulty: d.Difficulty,
		WinterLow: tenthsToC(int(lo)), SummerHigh: tenthsToC(int(hi)),
		GrowingDays: d.Climate.GrowingDays(30),
	}
	g := d.Ground
	if d.Legacy {
		g = biome.Ground{Soil: 800, Rich: 100, Gravel: 100}
	}
	type part struct {
		name string
		n    int
	}
	var ground []part
	for _, p := range []part{{"soil", g.Soil}, {"rich soil", g.Rich}, {"gravel", g.Gravel}, {"sand", g.Sand}, {"marsh", g.Marsh}, {"ice", g.Ice}} {
		if p.n > 0 {
			ground = append(ground, p)
		}
	}
	sort.SliceStable(ground, func(i, j int) bool { return ground[i].n > ground[j].n })
	for _, p := range ground {
		info.Ground = append(info.Ground, p.name)
	}
	plants := append([]biome.PlantDensity(nil), d.Plants...)
	sort.SliceStable(plants, func(i, j int) bool { return plants[i].Permil > plants[j].Permil })
	for _, p := range plants {
		info.Plants = append(info.Plants, plantLabel(p.Species)) // orders.go: "ground moss", "desert rosette"
	}
	type sp struct {
		label string
		w     int
	}
	var wild []sp
	for _, s := range animal.All() {
		if w := animal.Weight(s, d.ID); w > 0 {
			wild = append(wild, sp{s.Label(), w})
		}
	}
	sort.SliceStable(wild, func(i, j int) bool { return wild[i].w > wild[j].w })
	for i := 0; i < len(wild) && i < 5; i++ {
		info.Wildlife = append(info.Wildlife, wild[i].label)
	}
	return info
}

// tenthsToC rounds tenths of a degree to whole degrees.
func tenthsToC(t int) int {
	if t < 0 {
		return -((-t + 5) / 10)
	}
	return (t + 5) / 10
}
