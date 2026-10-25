package fire

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/plant"
)

// Flammability tables, permille (our numbers). Terrain itself never burns.

// Grass is the flammability of a grassy cell.
const Grass = 500

// How many fire steps each fuel lasts before it is burnt away (our
// numbers): grass and loose things go first, trees last. Buildings burn
// by hit points instead (BuildingDamage).
const (
	GrassBurnSteps = 25
	ItemBurnSteps  = 40
	plantBurnSteps = 35
	cropBurnSteps  = 30
	treeBurnSteps  = 80
)

func isTree(s plant.Species) bool {
	return s == plant.Tree || s == plant.Pine || s == plant.JungleTree
}

// PlantFlammability is how readily a plant of species s burns: trees 0.7,
// duskmoss 0.9, crops and other wild plants 0.6.
func PlantFlammability(s plant.Species) int {
	switch {
	case s == plant.Duskmoss:
		return 900
	case isTree(s):
		return 700
	}
	return 600
}

// PlantBurnSteps is how many fire steps a plant of species s burns before
// nothing of it is left.
func PlantBurnSteps(s plant.Species) int {
	switch {
	case isTree(s):
		return treeBurnSteps
	case plant.DefOf(s).Crop:
		return cropBurnSteps
	}
	return plantBurnSteps
}

// BuildingFlammability is how readily a building of kind k made of stuff
// burns: wooden ones (bridges too) 0.7, carpets 0.8, stone and steel never.
// Fire pits, graves, spots and natural features don't burn.
func BuildingFlammability(k construction.Kind, stuff item.Kind) int {
	d := construction.DefOf(k)
	switch {
	case d.Natural, d.Dug, k == construction.Campfire, k == construction.Pyre, k == construction.Sandbags:
		return 0
	case d.Wood == 0 && !d.Placed: // a spot painted on the ground
		return 0
	}
	if stuff == item.Food { // legacy zero value: the kind's default material
		stuff = construction.DefaultStuff(k)
	}
	switch {
	case stuff.IsCarpet():
		return 800
	case stuff == item.Wood:
		return 700
	}
	return 0
}

// ItemFlammability is how readily a loose stack of kind k burns: wood,
// textiles, leather and corpses 0.7..0.9, food and drugs 0.3, metal and
// stone never.
func ItemFlammability(k item.Kind) int {
	switch {
	case k == item.Cloth || k == item.Straw || k == item.Wool:
		return 900
	case k == item.Wood || k.IsCarpet() || k.IsApparel() || k == item.Spear || k == item.Club ||
		k == item.PegLeg || k == item.WoodenHand || k == item.WoodenNose || k == item.EmberOil:
		return 800
	case k == item.Leather || k.IsCorpse() || k == item.HerbalMedicine || k == item.AshleafLeaf:
		return 700
	case k.IsFood() || k.IsDrug() || k == item.Bones:
		return 300
	}
	return 0
}
