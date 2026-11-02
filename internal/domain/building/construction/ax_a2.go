package construction

// plannedA2 is A2's kinds not live yet (structure, furniture and decor; AX-0 contract): delete a
// case when its kind works end to end. This file is A2's, and so are any
// predicates it adds here for its own kinds.
func plannedA2(k Kind) bool {
	switch k {
	case OrnateDoor, Column, AnimalFlap, DoubleSleepingSpot, AnimalSleepingBox, Table2x2, Table2x4, Table3x3, Couch,
		PlantPot, SmallShelf, WallTorch, WallLamp, FloodLight, Urn, LargeStele, GrandStele, EggBox:
		return true
	}
	return false
}
