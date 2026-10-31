package item

import (
	"reflect"
	"testing"
)

// categoryNames is every category Category may return, in list order.
var categoryNames = []string{"Food", "Materials", "Medicine", "Apparel", "Weapons", "Manufactured", "Corpses", "Other"}

func TestCategoriesInListOrder(t *testing.T) {
	t.Parallel()
	if got := Categories(); !reflect.DeepEqual(got, categoryNames) {
		t.Fatalf("Categories() = %v, want %v", got, categoryNames)
	}
	for i, c := range categoryNames {
		if CategoryRank(c) != i {
			t.Errorf("CategoryRank(%q) = %d, want %d", c, CategoryRank(c), i)
		}
	}
	if CategoryRank("Bogus") != len(categoryNames) {
		t.Errorf("an unknown category ranks %d", CategoryRank("Bogus"))
	}
}

func TestCategoryCoversEveryKind(t *testing.T) {
	t.Parallel()
	known := map[string]bool{}
	for _, c := range categoryNames {
		known[c] = true
	}
	for k := Kind(0); k < numKinds; k++ {
		if c := k.Category(); !known[c] {
			t.Errorf("%s: category %q is not one of %v", k, c, categoryNames)
		}
	}
}

// Only kinds listed here may land in "Other": a new kind that reaches it
// by falling through the switch fails this test until it is placed on
// purpose.
func TestCategoryOtherIsDeliberate(t *testing.T) {
	t.Parallel()
	deliberate := map[Kind]bool{} // FOOD-3: fertile eggs are food now
	for k := Kind(0); k < numKinds; k++ {
		if k.Category() == "Other" && !deliberate[k] {
			t.Errorf("%s falls through to Other", k)
		}
	}
	for k := range deliberate {
		if k.Category() != "Other" {
			t.Errorf("%s should be Other, is %s", k, k.Category())
		}
	}
	if got := (numKinds + 3).Category(); got != "Other" {
		t.Errorf("an unknown kind is %q, want Other", got)
	}
}

func TestCategoryGroups(t *testing.T) {
	t.Parallel()
	cases := map[string][]Kind{
		"Food":         {Food, Meal, Meat, Berries, Potatoes, Corn, Milk, Egg, Brewgrain},
		"Materials":    {Wood, Stone, GraniteChunk, MarbleBlocks, Steel, Silver, Gold, Leather, Cloth, Straw, Wool, EmberOil, AshleafLeaf},
		"Medicine":     {HerbalMedicine, IndustrialMedicine, Beer, AshleafTea, SparkTablet},
		"Apparel":      {Parka, Tuque, Duster, WoolParka},
		"Weapons":      {Club, Rifle, Knife, Spear},
		"Manufactured": {Components, CarpetStraw, CarpetWool, PegLeg, ProstheticArm, WoodenNose},
		"Corpses":      {CorpseHuman, CorpseAnimal, CorpseDustbuck, CorpseRidgewolf, Bones},
	}
	for want, kinds := range cases {
		for _, k := range kinds {
			if got := k.Category(); got != want {
				t.Errorf("%s: category %q, want %q", k, got, want)
			}
		}
	}
}
