package item

// Category is the heading the resource list files k under (PLAY-018):
// one of "Food", "Materials", "Medicine" (drugs too), "Apparel",
// "Weapons", "Manufactured" (components, carpets, prosthetics and
// implants, natural organs, mortar shells, the reinforced barrel),
// "Corpses" (bones too) or "Other". Every kind is placed on purpose; a
// kind added later that is not listed here lands in "Other" and fails
// TestCategoryOtherIsDeliberate until it is.
func (k Kind) Category() string {
	switch {
	case k.IsFood():
		return "Food"
	case k.IsCorpse(), k == Bones:
		return "Corpses"
	case k.IsApparel(), k.IsBelt():
		return "Apparel"
	case k.IsWeapon(), k.IsContractWeapon():
		return "Weapons"
	case k.IsMedicine(), k.IsDrug():
		return "Medicine"
	case k == Components, k.IsCarpet(), k.IsImplant(), k.IsOrgan(), k.IsShell(), k == AdvancedComponents, k == ReinforcedBarrel:
		return "Manufactured"
	case k == Wood, k == Stone, IsChunk(k), IsStoneBlocks(k), IsMetal(k),
		k == Leather, k == Cloth, k == Straw, k == Wool, k == EmberOil, k == AshleafLeaf,
		k == Chemfuel, k == Wort, k == Reagent, k == Uranium, k == Plasteel:
		return "Materials"
	}
	return "Other"
}

// categoryOrder is the order the resource list shows categories in.
var categoryOrder = [...]string{"Food", "Materials", "Medicine", "Apparel", "Weapons", "Manufactured", "Corpses", "Other"}

// Categories lists every category Category returns, in list order.
func Categories() []string { return append([]string(nil), categoryOrder[:]...) }

// CategoryRank is the position of category c in Categories (an unknown
// name sorts after them all).
func CategoryRank(c string) int {
	for i, n := range categoryOrder {
		if n == c {
			return i
		}
	}
	return len(categoryOrder)
}
