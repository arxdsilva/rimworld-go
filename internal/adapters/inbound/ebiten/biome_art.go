package ebiten

import (
	"fmt"
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
	"sand": {"gravel", colorScale{1.35, 1.15, 0.7, 1}},
	// NG-1 ground before its art: water as a cool tint, mud and a track as soil
	"shallow_water": {"gravel", colorScale{0.45, 0.78, 1.25, 1}},
	"deep_water":    {"gravel", colorScale{0.18, 0.38, 0.8, 1}},
	"mud":           {"soil", colorScale{0.62, 0.5, 0.38, 1}},
	"track":         {"gravel", colorScale{1.1, 1.0, 0.85, 1}},
	"marsh":         {"soil", colorScale{0.55, 0.78, 0.66, 1}},
	"ice":           {"gravel", colorScale{0.78, 0.95, 1.3, 1}},
	// smooth stone before its art: rough stone, a touch lighter
	"stone_smooth": {"stone", colorScale{1.12, 1.12, 1.12, 1}},
	"rock_smooth":  {"stone", colorScale{1.12, 1.12, 1.12, 1}},
}

// terrainTileKey is the ground tile of terrain t at variant v and the
// colour scale to draw it with: its own art, else the tinted fallback.
func (r *Renderer) terrainTileKey(t string, v int) (string, colorScale) {
	base := terrainSprite[t]
	if base == "" {
		base = "soil" // a terrain this client has never heard of
	}
	k := artKey("terrain", base, v)
	if r.art.Has(k) || r.art == nil {
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

// groundVariants is how many art variants each ground plant has.
const groundVariants = 4

// youngBelow is the growth percent under which a ground plant draws its
// young stage.
const youngBelow = 55

// groundKeys is one ground plant's sprite keys per variant.
type groundKeys struct {
	young, mature, dormant, harvest [groundVariants]string
}

// groundPlants lists the ground plant species.
var groundPlants = []string{
	"bush", "berry_bush", "shrub", "reeds", "cactus", "bramble", "moss", "agave",
	"ball_cactus", "healroot_wild", "elephant_ear", "fire_lily", "dandelion", "duskmoss",
}

// groundPick is the sprite key of ground plant p (hash h): dormant when
// wilted, harvestable for ripe produce, else young or mature by growth.
func groundPick(g *groundKeys, p game.PlantView, h uint64) string {
	v := h % groundVariants
	switch {
	case p.Wilted:
		return g.dormant[v]
	case p.Harvestable && g.harvest[v] != "":
		return g.harvest[v]
	case p.Growth < youngBelow:
		return g.young[v]
	}
	return g.mature[v]
}

// newPlantKey is the sprite of a biome or ground plant species (pine,
// jungle_tree, bush, berry bush, shrub, reeds, cactus and the PLAY-010
// wild plants) at tile hash h, with the growth scale; the fallback chain
// in plantKeys resolves to the mature stage or variant 0.
func newPlantKey(p game.PlantView, h uint64) (string, float64, bool) {
	v := int(h % 3)
	switch p.Species {
	case "pine":
		return plantKeys.trees[2][v], 0.45 + 0.55*float64(p.Growth)/100, true
	case "jungle_tree":
		return artKey("plant", "tree_jungle", v), 0.45 + 0.55*float64(p.Growth)/100, true
	}
	if p.Species == "healroot" && p.Zone == "" { // wild healroot, not a sown crop
		return groundPick(plantKeys.ground["healroot_wild"], p, h), bushScale(p), true
	}
	if g, ok := plantKeys.ground[p.Species]; ok {
		return groundPick(g, p, h), bushScale(p), true
	}
	return "", 0, false
}

// addGroundPlantKeys builds the stage keys of every ground plant, their
// fallbacks (variant 0, then the mature stage) and the winter dormant look.
func addGroundPlantKeys(t *plantKeyTable) {
	t.ground = map[string]*groundKeys{}
	for _, name := range groundPlants {
		g := &groundKeys{}
		t.ground[name] = g
		berry := name == "berry_bush"
		for v := 0; v < groundVariants; v++ {
			key := func(stage string) string { return fmt.Sprintf("plant/%s_%s_v%d", name, stage, v) }
			v0 := func(stage string) []string {
				if v == 0 {
					return nil
				}
				return []string{fmt.Sprintf("plant/%s_%s_v0", name, stage)}
			}
			g.young[v], g.mature[v], g.dormant[v] = key("young"), key("mature"), key("dormant")
			t.fallback[g.young[v]] = append(v0("young"), g.mature[v])
			t.fallback[g.mature[v]] = v0("mature")
			t.fallback[g.dormant[v]] = append(v0("dormant"), g.mature[v])
			winter := g.dormant[v]
			t.seasonal[g.young[v]] = [2]string{g.young[v], winter}
			t.seasonal[g.mature[v]] = [2]string{g.mature[v], winter}
			if berry {
				g.harvest[v] = key("harvestable")
				t.fallback[g.harvest[v]] = append(v0("harvestable"), g.mature[v])
				t.seasonal[g.harvest[v]] = [2]string{g.harvest[v], winter}
			}
		}
	}
}

// addBiomePlantFallbacks registers the fallback art of the biome plants:
// the jungle tree falls back to the oak.
func addBiomePlantFallbacks(t *plantKeyTable) {
	oak := artKey("plant", "tree_oak", 0)
	for v := 0; v < 3; v++ {
		jt := artKey("plant", "tree_jungle", v)
		t.fallback[jt] = []string{artKey("plant", "tree_oak", v), oak}
		// pine stays green through the year but wears snow in winter
		pk := t.trees[2][v]
		t.seasonal[pk] = [2]string{pk, pk + "_snow"}
		t.fallback[pk+"_snow"] = []string{pk}
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
