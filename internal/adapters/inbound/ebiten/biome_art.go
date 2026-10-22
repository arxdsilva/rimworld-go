package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Biome presentation (plan part 6): grass tint per biome, the new
// terrains, plants, troughs and items, each with a fallback so the game
// draws before (or without) the art for it.

// grassTints colours the grass tufts per biome: lush green, boreal dark
// green, moss olive, dry gold, jungle saturated. Temperate forest keeps
// the original art untinted.
var grassTints = map[string]colorScale{
	"temperate_forest":    noCS,
	"temperate_swamp":     {0.95, 1.1, 0.9, 1},   // lush
	"boreal_forest":       {0.68, 0.9, 0.78, 1},  // boreal dark green
	"cold_bog":            {0.86, 0.9, 0.62, 1},  // moss
	"tundra":              {0.88, 0.9, 0.66, 1},  // moss
	"ice_sheet":           {0.8, 0.95, 0.9, 1},   // pale
	"sea_ice":             {0.8, 0.95, 0.9, 1},   // pale
	"arid_shrubland":      {1.22, 1.05, 0.58, 1}, // dry gold
	"desert":              {1.28, 1.02, 0.5, 1},  // dry gold
	"extreme_desert":      {1.3, 1.0, 0.48, 1},   // dry gold
	"tropical_swamp":      {0.78, 1.18, 0.7, 1},  // jungle
	"tropical_rainforest": {0.72, 1.25, 0.62, 1}, // jungle
}

// grassTint is the biome's grass colour scale (none for an unknown key).
func grassTint(biomeKey string) colorScale {
	if cs, ok := grassTints[biomeKey]; ok {
		return cs
	}
	return noCS
}

// terrainTile is how a terrain is drawn: the sprite family name, and the
// fallback (a tinted soil or gravel tile) when its art is missing.
type terrainFallback struct {
	base string
	cs   colorScale
}

var terrainFallbacks = map[string]terrainFallback{
	"sand":  {"gravel", colorScale{1.35, 1.15, 0.7, 1}},
	"marsh": {"soil", colorScale{0.55, 0.78, 0.66, 1}},
	"ice":   {"gravel", colorScale{0.78, 0.95, 1.3, 1}},
}

// terrainTileKey is the ground tile of terrain t at variant v and the
// colour scale to draw it with: its own art, else the tinted fallback.
func (r *Renderer) terrainTileKey(t string, v int) (string, colorScale) {
	base := terrainSprite[t]
	if base == "" {
		base = "soil" // a terrain this client has never heard of
	}
	k := artKey("terrain", base, v)
	if r.art.has(k) || r.art == nil {
		return k, noCS
	}
	if fb, ok := terrainFallbacks[t]; ok {
		return artKey("terrain", terrainSprite[fb.base], v), fb.cs
	}
	return k, noCS
}

// isTreePlant reports species that are trees: they shake in the wind and
// get the tree lean.
func isTreePlant(species string) bool {
	return species == "tree" || species == "pine" || species == "jungle_tree"
}

// newPlantKey is the sprite of a biome plant species (pine, jungle_tree,
// cactus, shrub, reeds) at tile hash h, with the growth scale; the
// fallback chain in plantKeys resolves to tree and bush art when the
// species art is missing.
func newPlantKey(p game.PlantView, h uint64) (string, float64, bool) {
	v := int(h % 3)
	switch p.Species {
	case "pine":
		return plantKeys.trees[2][v], 0.45 + 0.55*float64(p.Growth)/100, true
	case "jungle_tree":
		return artKey("plant", "tree_jungle", v), 0.45 + 0.55*float64(p.Growth)/100, true
	case "cactus":
		k := artKey("plant", "cactus", v)
		if p.Wilted {
			k += "_dry"
		}
		return k, bushScale(p), true
	case "shrub":
		k := artKey("plant", "shrub", v)
		if p.Wilted {
			k += "_dry"
		}
		return k, bushScale(p), true
	case "reeds":
		return artKey("plant", "reeds", v), bushScale(p), true
	}
	return "", 0, false
}

// addBiomePlantFallbacks registers the fallback art of the biome plants:
// trees fall back to the oak, the rest to bushes.
func addBiomePlantFallbacks(t *plantKeyTable) {
	oak := artKey("plant", "tree_oak", 0)
	bush := artKey("plant", "bush", 0)
	for v := 0; v < 3; v++ {
		jt := artKey("plant", "tree_jungle", v)
		t.fallback[jt] = []string{artKey("plant", "tree_oak", v), oak, artKey("plant", "tree", v)}
		for _, n := range []string{"cactus", "shrub", "reeds"} {
			k := artKey("plant", n, v)
			t.fallback[k] = []string{artKey("plant", "bush", v%2), bush}
			t.fallback[k+"_dry"] = []string{k, artKey("plant", "bush", v%2), bush}
		}
		// pine stays green through the year but wears snow in winter
		pk := t.trees[2][v]
		t.seasonal[pk] = [2]string{pk, pk + "_snow"}
		t.fallback[pk+"_snow"] = []string{pk, artKey("plant", "tree", v)}
	}
}

// troughKey is the feeding trough's sprite: the full or empty box.
func troughKey(b game.BuildingView) string {
	state := "empty"
	if b.TroughFill > 0 {
		state = "full"
	}
	return "furniture/feeding_trough_" + state + "_" + facingOr(b.Facing)
}

// biomeItemSprite is the sprite of the new products, "" for other kinds.
func biomeItemSprite(kind string) string {
	switch kind {
	case "fertile egg":
		return "item/fertile_egg"
	case "ember oil":
		return "item/ember_oil"
	case "wool parka":
		return "item/parka_wool"
	case "wool carpet":
		return "item/carpet_wool_rolled"
	}
	return ""
}

// drawTroughFallback draws a feeding trough as a plain wooden box (full
// when it holds feed) while its sprite is missing.
func (r *Renderer) drawTroughFallback(screen *ebiten.Image, b game.BuildingView, tile float64) {
	sx, sy := r.worldToScreenF(float64(b.X), float64(b.Y))
	w, h := float32(max(b.W, 1))*float32(tile), float32(max(b.H, 1))*float32(tile)
	pad := float32(tile) * 0.15
	vector.FillRect(screen, sx+pad, sy+pad*1.8, w-2*pad, h-pad*3.6, color.RGBA{110, 80, 48, 255}, false)
	inner := color.RGBA{60, 44, 28, 255}
	if b.TroughFill > 0 {
		inner = color.RGBA{150, 170, 70, 255}
	}
	vector.FillRect(screen, sx+pad*1.8, sy+pad*2.6, w-3.6*pad, h-pad*5.2, inner, false)
}
