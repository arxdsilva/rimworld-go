package construction

// plannedA8 is A8's kinds not live yet (the escape ship; AX-0 contract): delete a
// case when its kind works end to end. This file is A8's, and so are any
// predicates it adds here for its own kinds.
func plannedA8(k Kind) bool {
	switch k {
	case ShipBeam, ShipCasket, ShipCore, ShipReactor, ShipEngine, ShipSensor:
		return true
	}
	return false
}
