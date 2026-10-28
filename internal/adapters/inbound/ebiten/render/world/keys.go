package world

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// The item and plant art keys: a ground stack's sprite by kind, a plant's
// by species, stage and season (PlantKeys), the stone tint of chunks and
// blocks.

// ItemSprite is the sprite key of an item kind.
func ItemSprite(kind string) string {
	switch kind {
	case "wood":
		return "item/wood_pile"
	case "food":
		return "item/ration_pack"
	case "berries":
		return "item/berries_basket"
	case "potatoes":
		return "item/potatoes"
	case "corn":
		return "item/corn_pile"
	case "wheat":
		return "item/wheat_sheaf"
	case "beans":
		return "item/beans_sack"
	case "lettuce":
		return "item/lettuce_heads"
	case "radishes":
		return "item/radish_bunch"
	case "pumpkins":
		return "item/pumpkins"
	case "herbal medicine":
		return "item/herbal_medicine"
	case "components":
		return "item/components_m" // SizedItemKey picks the pile by quantity
	case "industrial medicine":
		return "item/industrial_medicine_m"
	case "meat":
		return "item/meat_pile"
	case "leather":
		return "item/leather_hides"
	case "parka":
		return "item/parka_folded"
	case "tuque":
		return "item/tuque"
	case "duster":
		return "item/duster_folded"
	case "meal":
		return "item/meal_simple"
	case "steel":
		return "item/steel_bars"
	case "club":
		return "item/club_wood"
	case "rifle":
		return "item/rifle_ground"
	case "dustbuck corpse":
		return "item/corpse_dustbuck"
	case "ridgewolf corpse":
		return "item/corpse_ridgewolf"
	case "stone", "stone_blocks":
		return "item/stone_blocks"
	case "silver":
		return "item/silver"
	case "gold":
		return "item/gold"
	case "cloth":
		return "item/cloth_bolt"
	case "straw":
		return "item/straw_bundle"
	case "straw carpet", "cloth carpet", "leather carpet", "wood carpet", "wool carpet":
		return "item/carpet_" + strings.TrimSuffix(kind, " carpet") + "_rolled"
	case "human corpse":
		return creature.CorpseCarrySprite
	case "bones":
		return creature.BonesSprite(0, 0)
	}
	switch {
	case strings.HasSuffix(kind, " chunk"):
		return "item/stone_chunk"
	case strings.HasSuffix(kind, " blocks"):
		return "item/stone_blocks"
	}
	if k := sprite.BiomeItemSprite(kind); k != "" {
		return k
	}
	return "item/" + strings.ReplaceAll(kind, " ", "_") // sprite names use underscores ("servo arm" -> item/servo_arm)
}

// ItemSpriteSpecies is ItemSprite with an animal corpse's species art.
func ItemSpriteSpecies(kind, species string, x, y int) string {
	if k, ok := creature.CorpseSprite(kind, species, x, y); ok {
		return k
	}
	return ItemSprite(kind)
}

// StoneItem is the key and colour scale of a ground stack: chunks and
// blocks take their stone's tint; a chunk without art draws as blocks and
// precious metals without art as steel bars.
func (b *base) StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	if fb, ok := itemFallback[key]; ok && !b.art().Has(key) {
		key = fb
	}
	stone := ""
	switch {
	case kind == "stone":
		stone = sprite.DefaultStone
	case strings.HasSuffix(kind, " chunk"), strings.HasSuffix(kind, " blocks"):
		stone = kind[:strings.LastIndexByte(kind, ' ')]
	}
	if stone == "" {
		return key, cs
	}
	return key, sprite.MulCS(sprite.TintCS(b.stuffTint(stone+" blocks")), cs)
}

var itemFallback = map[string]string{
	"item/stone_chunk": "item/stone_blocks",
	"item/silver":      "item/steel_bars",
	"item/gold":        "item/steel_bars",
}

// PlantSprite picks the sprite and growth scale for a plant: tree
// species, bush and crop variants by tile hash, crops by drawn stage
// (ART §9, §28). Keys come from a prebuilt table; Layer.SeasonKey then
// picks the seasonal look and falls back to older art.
func PlantSprite(p game.PlantView) (string, float64) {
	h := sprite.TileHash(p.X, p.Y)
	if p.Species == "healroot" && p.Zone == "" {
		k, sc, _ := NewPlantKey(p, h)
		return k, sc
	}
	if ks, ok := PlantKeys.Crops[p.Species]; ok {
		return ks[env.CropStage(p)][h%env.CropVariants], 1 // the inspector's icon and the shadow; DrawCropObj composes the units
	}
	if k, sc, ok := NewPlantKey(p, h); ok {
		return k, sc
	}
	switch p.Species {
	case "tree":
		k := 0.45 + 0.55*float64(p.Growth)/100
		return PlantKeys.Trees[env.TreeSpeciesAt(h)][h%3], k
	default:
		return groundPick(PlantKeys.Ground["bush"], p, h), bushScale(p)
	}
}

func bushScale(p game.PlantView) float64 {
	return min(0.6+0.4*float64(p.Growth)/100, 1)
}

// ClampStage clamps a crop stage to 0..3.
func ClampStage(s int) int {
	if s < 0 {
		return 0
	}
	if s > 3 {
		return 3
	}
	return s
}

// OnFloor reports a building kind drawn flat under pawns and furniture
// (the floor-marking, foundation or conduit pass), not in the object pass.
func OnFloor(kind string) bool {
	return kind == "sleeping spot" || kind == "carpet" || kind == overlay.BridgeKind || // a bridge: the foundation pass
		overlay.IsConduitKind(kind) // conduits: the conduit pass (render/overlay)
}
