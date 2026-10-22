package faction

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/randx"
)

// Settlement placement (NG-1 §3.4): Poisson-disc over land tiles.

const (
	spacingSparse  = 8
	spacingNormal  = 6
	spacingCrowded = 4
	// hillsPreferred is the Globe.Hills value from which hills count as
	// hills (SmallHills; 0 is the legacy default, 1 is flat).
	hillsPreferred = 2
	// nearRadius bounds how far from its capital a faction's other
	// settlements are first tried.
	nearRadius = 18
	nearTries  = 120
	anyTries   = 400
)

// Spacing is the minimum tile distance between settlements for popPct
// (sparse 50, normal 100, crowded 160).
func Spacing(popPct int) int {
	switch {
	case popPct <= 60:
		return spacingSparse
	case popPct >= 140:
		return spacingCrowded
	}
	return spacingNormal
}

// excluded biomes never hold a settlement.
var excluded = biome.Of(biome.IceSheet, biome.SeaIce, biome.ExtremeDesert)

var preferredBiomes = map[Type]biome.Set{
	Clans: biome.Of(biome.BorealForest, biome.TemperateForest, biome.TemperateSwamp,
		biome.TropicalSwamp, biome.ColdBog, biome.Tundra),
	Settlers: biome.Of(biome.TemperateForest, biome.AridShrubland, biome.Desert),
}

// preferred reports tile i suits faction type t.
func preferred(g Globe, t Type, i int) bool {
	if t == Freebooters {
		return g.Hills(i) >= hillsPreferred
	}
	if set, ok := preferredBiomes[t]; ok {
		return set.Has(g.Biome(i))
	}
	return true
}

// SettlementCount is how many settlements a faction of type t gets on a
// planet with land land tiles at popPct: round(land x density x pop /
// 10,000), at least one for any type with a density.
func SettlementCount(t Type, land, popPct int) int {
	d := DefOf(t)
	if d.PerTenK <= 0 {
		return 0
	}
	const den = 10 * 100 * 10_000 // PerTenK is x10, popPct is a percent
	return max(1, (land*d.PerTenK*popPct+den/2)/den)
}

// PlaceSettlements places settlements on g for fs (popPct is the
// population percent: 50 sparse, 100 normal, 160 crowded). Each faction's
// first settlement is its capital; every settlement keeps at least
// Spacing(popPct) tiles from every other. Existing settlements in fs are
// kept and respected. Names come from the place-name generator.
func PlaceSettlements(g Globe, fs []Faction, popPct int, rng *randx.Rand) {
	n := g.TileCount()
	land := 0
	var eligible []int
	for i := 0; i < n; i++ {
		if !g.Land(i) {
			continue
		}
		land++
		if !excluded.Has(g.Biome(i)) {
			eligible = append(eligible, i)
		}
	}
	if len(eligible) == 0 {
		return
	}
	spacing := Spacing(popPct)
	var placed []int
	usedNames := map[string]bool{}
	for _, f := range fs {
		placed = append(placed, f.Settlements...)
		for _, s := range f.SettlementNames {
			usedNames[s] = true
		}
	}
	prefLists := map[Type][]int{}
	prefOf := func(t Type) []int {
		if l, ok := prefLists[t]; ok {
			return l
		}
		var l []int
		for _, i := range eligible {
			if preferred(g, t, i) {
				l = append(l, i)
			}
		}
		prefLists[t] = l
		return l
	}
	free := func(t int) bool {
		for _, p := range placed {
			if p == t || g.Hops(t, p, spacing) < spacing {
				return false
			}
		}
		return true
	}
	for fi := range fs {
		f := &fs[fi]
		want := SettlementCount(f.Type, land, popPct)
		pref := prefOf(f.Type)
		for len(f.Settlements) < want {
			tile, ok := pickTile(g, f, pref, eligible, free, rng)
			if !ok {
				break
			}
			placed = append(placed, tile)
			f.Settlements = append(f.Settlements, tile)
			f.SettlementNames = append(f.SettlementNames, uniquePlace(rng, usedNames))
		}
	}
}

// pickTile finds a free tile for f: near its capital when it has one,
// preferring its biomes, then anywhere eligible.
func pickTile(g Globe, f *Faction, pref, eligible []int, free func(int) bool, rng *randx.Rand) (int, bool) {
	try := func(list []int, tries int, near bool) (int, bool) {
		if len(list) == 0 {
			return 0, false
		}
		for k := 0; k < tries; k++ {
			t := list[rng.IntN(len(list))]
			if near && g.Hops(f.Settlements[0], t, nearRadius) >= nearRadius {
				continue
			}
			if free(t) {
				return t, true
			}
		}
		return 0, false
	}
	if len(f.Settlements) > 0 {
		if t, ok := try(pref, nearTries, true); ok {
			return t, true
		}
	}
	if t, ok := try(pref, anyTries, false); ok {
		return t, true
	}
	return try(eligible, anyTries, false)
}
