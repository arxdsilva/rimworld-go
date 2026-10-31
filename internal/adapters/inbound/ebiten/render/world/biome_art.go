package world

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
)

// Biome presentation (plan part 6): the ground plants, the biome plants'
// fallbacks and the feeding trough, each with a fallback so the game
// draws before (or without) the art for it. The terrains' fallbacks are
// terrain.go's.

// GroundVariants is how many art variants each ground plant has.
const GroundVariants = 4

// youngBelow is the growth percent under which a ground plant draws its
// young stage.
const youngBelow = 55

// GroundKeys is one ground plant's sprite keys per variant.
type GroundKeys struct {
	Young, Mature, Dormant, harvest [GroundVariants]string
}

// groundPlants lists the ground plant species.
var groundPlants = []string{
	"bush", "berry_bush", "shrub", "reeds", "cactus", "bramble", "moss", "agave",
	"ball_cactus", "healroot_wild", "elephant_ear", "fire_lily", "dandelion", "duskmoss",
}

// PlantKeyTable is render/env's plant keys (crops, trees, fallbacks,
// seasons) with the ground plants' stages.
type PlantKeyTable struct {
	*env.PlantKeyTable
	Ground map[string]*GroundKeys
}

// PlantKeys is the plant key table: built at declaration, never assigned
// after.
var PlantKeys = buildPlantKeys()

func buildPlantKeys() PlantKeyTable {
	t := PlantKeyTable{PlantKeyTable: env.NewPlantKeys()}
	AddBiomePlantFallbacks(t.PlantKeyTable)
	t.Ground = AddGroundPlantKeys(t.PlantKeyTable)
	return t
}

// groundPick is the sprite key of ground plant p (hash h): dormant when
// wilted, harvestable for ripe produce, else young or mature by growth.
func groundPick(g *GroundKeys, p game.PlantView, h uint64) string {
	v := h % GroundVariants
	switch {
	case p.Wilted:
		return g.Dormant[v]
	case p.Harvestable && g.harvest[v] != "":
		return g.harvest[v]
	case p.Growth < youngBelow:
		return g.Young[v]
	}
	return g.Mature[v]
}

// NewPlantKey is the sprite of a biome or ground plant species (pine,
// jungle_tree, bush, berry bush, shrub, reeds, cactus and the PLAY-010
// wild plants) at tile hash h, with the growth scale; the fallback chain
// in PlantKeys resolves to the mature stage or variant 0.
func NewPlantKey(p game.PlantView, h uint64) (string, float64, bool) {
	v := int(h % 3)
	switch p.Species {
	case "pine":
		return PlantKeys.Trees[2][v], 0.45 + 0.55*float64(p.Growth)/100, true
	case "jungle_tree":
		return env.ArtKey("plant", "tree_jungle", v), 0.45 + 0.55*float64(p.Growth)/100, true
	}
	if p.Species == "healroot" && p.Zone == "" { // wild healroot, not a sown crop
		return groundPick(PlantKeys.Ground["healroot_wild"], p, h), bushScale(p), true
	}
	if g, ok := PlantKeys.Ground[p.Species]; ok {
		return groundPick(g, p, h), bushScale(p), true
	}
	return "", 0, false
}

// AddGroundPlantKeys builds the stage keys of every ground plant into
// t's fallbacks (variant 0, then the mature stage) and the winter dormant
// look, and returns them by species.
func AddGroundPlantKeys(t *env.PlantKeyTable) map[string]*GroundKeys {
	ground := map[string]*GroundKeys{}
	for _, name := range groundPlants {
		g := &GroundKeys{}
		ground[name] = g
		berry := name == "berry_bush" || name == "agave" // FOOD-2: agave bears fruit too
		for v := 0; v < GroundVariants; v++ {
			key := func(stage string) string { return fmt.Sprintf("plant/%s_%s_v%d", name, stage, v) }
			v0 := func(stage string) []string {
				if v == 0 {
					return nil
				}
				return []string{fmt.Sprintf("plant/%s_%s_v0", name, stage)}
			}
			g.Young[v], g.Mature[v], g.Dormant[v] = key("young"), key("mature"), key("dormant")
			t.Fallback[g.Young[v]] = append(v0("young"), g.Mature[v])
			t.Fallback[g.Mature[v]] = v0("mature")
			t.Fallback[g.Dormant[v]] = append(v0("dormant"), g.Mature[v])
			winter := g.Dormant[v]
			t.Seasonal[g.Young[v]] = [2]string{g.Young[v], winter}
			t.Seasonal[g.Mature[v]] = [2]string{g.Mature[v], winter}
			if berry {
				g.harvest[v] = key("harvestable")
				t.Fallback[g.harvest[v]] = append(v0("harvestable"), g.Mature[v])
				t.Seasonal[g.harvest[v]] = [2]string{g.harvest[v], winter}
			}
		}
	}
	return ground
}

// AddBiomePlantFallbacks registers the fallback art of the biome plants:
// the jungle tree falls back to the oak.
func AddBiomePlantFallbacks(t *env.PlantKeyTable) {
	oak := env.ArtKey("plant", "tree_oak", 0)
	for v := 0; v < 3; v++ {
		jt := env.ArtKey("plant", "tree_jungle", v)
		t.Fallback[jt] = []string{env.ArtKey("plant", "tree_oak", v), oak}
		// pine stays green through the year but wears snow in winter
		pk := t.Trees[2][v]
		t.Seasonal[pk] = [2]string{pk, pk + "_snow"}
		t.Fallback[pk+"_snow"] = []string{pk}
	}
}

// TroughKind is the feeding trough's building kind.
const TroughKind = "feeding trough"

// TroughKey is the feeding trough's sprite: the full or empty box.
func TroughKey(b game.BuildingView) string {
	state := "empty"
	if b.TroughFill > 0 {
		state = "full"
	}
	return "furniture/feeding_trough_" + state + "_" + FacingOr(b.Facing)
}

// drawTroughFallback draws a feeding trough as a plain wooden box (full
// when it holds feed) while its sprite is missing.
func (o *Objects) drawTroughFallback(screen *ebiten.Image, b game.BuildingView, tile float64) {
	sx, sy := o.worldToScreenF(float64(b.X), float64(b.Y))
	w, h := float32(max(b.W, 1))*float32(tile), float32(max(b.H, 1))*float32(tile)
	pad := float32(tile) * 0.15
	vector.FillRect(screen, sx+pad, sy+pad*1.8, w-2*pad, h-pad*3.6, color.RGBA{110, 80, 48, 255}, false)
	inner := color.RGBA{60, 44, 28, 255}
	if b.TroughFill > 0 {
		inner = color.RGBA{150, 170, 70, 255}
	}
	vector.FillRect(screen, sx+pad*1.8, sy+pad*2.6, w-3.6*pad, h-pad*5.2, inner, false)
}
