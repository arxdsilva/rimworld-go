package construction

// plannedA6 is A6's kinds not live yet (recreation; AX-0 contract): delete a
// case when its kind works end to end. This file is A6's, and so are any
// predicates it adds here for its own kinds.
func plannedA6(k Kind) bool {
	switch k {
	case RingToss, UrBoard, ChessTable, BilliardsTable, PokerTable, TubeTV, FlatscreenTV, BigScreenTV, Telescope:
		return true
	}
	return false
}
