package construction

// plannedA9 is A9's kinds not live yet (caskets, pods, scanner, hitching spot; AX-0 contract): delete a
// case when its kind works end to end. This file is A9's, and so are any
// predicates it adds here for its own kinds.
func plannedA9(k Kind) bool {
	switch k {
	case StasisCasket, TransportPod, PodLauncher, MineralScanner, HitchingSpot:
		return true
	}
	return false
}
