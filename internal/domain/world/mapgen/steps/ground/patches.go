package ground

import (
	"strconv"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// patch is one data-driven biome ground patch (plan §2.6 rule 6): Terrain
// covers the cells whose Fertility and Wetness lie in the ranges and whose
// patch noise (lattice cell NoiseCell at a 250×250 map, scaled; 0 = no
// noise) is at least Threshold. Connected pieces smaller than MinSize are
// dropped and fall through to the ground mix.
type patch struct {
	Terrain          terrain.Type
	NoiseCell        float64
	Threshold        float64
	FertMin, FertMax float32
	WetMin, WetMax   float32
	MinSize          int
}

// patchNoise: three octaves; FBm sits around 0.5, so a threshold of 0.5
// keeps about half the cells, 0.6 about a fifth, 0.65 about a tenth.
const (
	patchOctaves = 3
	patchGain    = 0.5
	patchMinCell = 4.0
)

// patches lists every biome's ground patches, first match wins. Patch
// types that are in the biome's Ground mix are taken out of the mix's
// share, so a biome keeps its mix overall; others (mud, a temperate
// forest's marsh) stay small.
var patches = [biome.NumBiomes][]patch{
	biome.TemperateForest: {
		{Terrain: terrain.Marsh, NoiseCell: 22, Threshold: 0.56, FertMax: 1, WetMin: 0.64, WetMax: 1, MinSize: 10}, // wet hollows
		{Terrain: terrain.Gravel, NoiseCell: 14, Threshold: 0.6, FertMax: 0.12, WetMax: 1, MinSize: 6},             // stony poor ground
	},
	biome.BorealForest: {
		{Terrain: terrain.Marsh, NoiseCell: 24, Threshold: 0.55, FertMax: 1, WetMin: 0.6, WetMax: 1, MinSize: 10},
		{Terrain: terrain.Gravel, NoiseCell: 16, Threshold: 0.58, FertMax: 0.35, WetMax: 0.6, MinSize: 8}, // scree
	},
	biome.Tundra: {
		{Terrain: terrain.Gravel, NoiseCell: 18, Threshold: 0.54, FertMax: 0.5, WetMax: 1, MinSize: 10}, // scree fields
		{Terrain: terrain.Marsh, NoiseCell: 20, Threshold: 0.6, FertMax: 1, WetMin: 0.68, WetMax: 1, MinSize: 10},
	},
	biome.ColdBog: {
		{Terrain: terrain.Marsh, NoiseCell: 22, Threshold: 0.5, FertMax: 1, WetMin: 0.52, WetMax: 1, MinSize: 10},
		{Terrain: terrain.Mud, NoiseCell: 12, Threshold: 0.6, FertMax: 0.5, WetMin: 0.42, WetMax: 1, MinSize: 6},
	},
	biome.IceSheet: {
		{Terrain: terrain.Ice, FertMax: 1, WetMax: 1, MinSize: 1}, // all ice
	},
	biome.SeaIce: {
		{Terrain: terrain.Ice, FertMax: 1, WetMax: 1, MinSize: 1},
	},
	biome.TemperateSwamp: {
		{Terrain: terrain.Marsh, NoiseCell: 26, Threshold: 0.5, FertMax: 1, WetMin: 0.55, WetMax: 1, MinSize: 12},
		{Terrain: terrain.Mud, NoiseCell: 12, Threshold: 0.62, FertMax: 1, WetMin: 0.45, WetMax: 0.7, MinSize: 6},
	},
	biome.TropicalSwamp: {
		{Terrain: terrain.Marsh, NoiseCell: 26, Threshold: 0.5, FertMax: 1, WetMin: 0.55, WetMax: 1, MinSize: 12},
		{Terrain: terrain.RichSoil, NoiseCell: 30, Threshold: 0.52, FertMin: 0.55, FertMax: 1, WetMax: 0.6, MinSize: 10},
		{Terrain: terrain.Mud, NoiseCell: 12, Threshold: 0.64, FertMax: 1, WetMin: 0.45, WetMax: 0.7, MinSize: 6},
	},
	biome.TropicalRainforest: {
		{Terrain: terrain.RichSoil, NoiseCell: 34, Threshold: 0.52, FertMin: 0.4, FertMax: 1, WetMin: 0.4, WetMax: 1, MinSize: 12},
		{Terrain: terrain.Mud, NoiseCell: 12, Threshold: 0.64, FertMax: 1, WetMin: 0.66, WetMax: 1, MinSize: 6},
	},
	biome.AridShrubland: {
		{Terrain: terrain.Sand, NoiseCell: 30, Threshold: 0.52, FertMax: 0.4, WetMax: 0.6, MinSize: 12}, // dunes
		{Terrain: terrain.Gravel, NoiseCell: 16, Threshold: 0.6, FertMax: 1, WetMax: 0.5, MinSize: 6},
	},
	biome.Desert: {
		{Terrain: terrain.Sand, NoiseCell: 36, Threshold: 0.48, FertMax: 0.6, WetMax: 1, MinSize: 14}, // dune seas
		{Terrain: terrain.Gravel, NoiseCell: 18, Threshold: 0.62, FertMax: 1, WetMax: 1, MinSize: 6},  // desert pavement
	},
	biome.ExtremeDesert: {
		{Terrain: terrain.Sand, NoiseCell: 40, Threshold: 0.44, FertMax: 0.85, WetMax: 1, MinSize: 16},
		{Terrain: terrain.Gravel, NoiseCell: 18, Threshold: 0.64, FertMax: 1, WetMax: 1, MinSize: 6},
	},
}

// patchesOf is biome b's patch list (nil for an unknown biome).
func patchesOf(b biome.ID) []patch {
	if b >= biome.NumBiomes {
		return nil
	}
	return patches[b]
}

// applyPatches lays patches ps over the rest cells (in order, first match
// wins), writes their terrain and returns the cells left for the ground mix
// and the patch cell count per type.
func (e *env) applyPatches(ps []patch, rest []int32) (left []int32, counts [typeSlots]int) {
	c := e.c
	f, t := c.F, c.R.Terrain.Cells
	taken := make([]bool, c.W*c.H)
	in := make([]bool, c.W*c.H)
	for k, p := range ps {
		clear(in)
		cell := max(patchMinCell, p.NoiseCell*c.Scale())
		seed := c.NoiseSeed("terrain/patch/" + strconv.Itoa(int(c.Cfg.Biome.ID)) + "/" + strconv.Itoa(k))
		hit := false
		for _, i := range rest {
			if taken[i] {
				continue
			}
			fe, we := f.Fertility[i], f.Wetness[i]
			if fe < p.FertMin || fe > p.FertMax || we < p.WetMin || we > p.WetMax {
				continue
			}
			if p.NoiseCell > 0 {
				x, y := c.XY(int(i))
				if noise2.FBm(seed, float64(x)+0.5, float64(y)+0.5, patchOctaves, cell, patchGain) < p.Threshold {
					continue
				}
			}
			in[i], hit = true, true
		}
		if !hit {
			continue
		}
		label, sizes := mapgen.Components(c.W, c.H, in, false)
		for _, i := range rest {
			if in[i] && sizes[label[i]] >= p.MinSize {
				t[i], taken[i] = p.Terrain, true
				counts[p.Terrain]++
			}
		}
	}
	left = make([]int32, 0, len(rest))
	for _, i := range rest {
		if !taken[i] {
			left = append(left, i)
		}
	}
	return left, counts
}
