package construction

// plannedA5 is A5's kinds not live yet (fire foam and security; AX-0 contract): delete a
// case when its kind works end to end. This file is A5's, and so are any
// predicates it adds here for its own kinds.
func plannedA5(k Kind) bool {
	switch k {
	case FoamPopper, IEDFoam, FoamTurret, IEDWarhead, RocketBattery:
		return true
	}
	return false
}
