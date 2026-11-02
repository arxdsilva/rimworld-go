package construction

// plannedA3 is A3's kinds not live yet (floors; AX-0 contract): delete a
// case when its kind works end to end. This file is A3's, and so are any
// predicates it adds here for its own kinds.
func plannedA3(k Kind) bool {
	switch k {
	case StrawMatting, Concrete, PavedTile, SteelTile, SilverTile, GoldTile, SterileTile, StoneTile, Flagstone,
		CarpetFloor:
		return true
	}
	return false
}
