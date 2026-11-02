package construction

// plannedA4 is A4's kinds not live yet (power and industry; AX-0 contract): delete a
// case when its kind works end to end. This file is A4's, and so are any
// predicates it adds here for its own kinds.
func plannedA4(k Kind) bool {
	switch k {
	case WaterproofConduit, ChemfuelGenerator, Watermill, DeepDrill, GroundScanner, MoisturePump, ElectricCrematorium:
		return true
	}
	return false
}
