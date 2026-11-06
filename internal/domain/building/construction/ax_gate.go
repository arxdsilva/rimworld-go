package construction

// Planned reports an AX-0 contract kind its owner has not made real yet:
// it cannot be designated (game placement.go placeFrameStuff refuses it)
// and has no Architect tool. Each owner's ax_<owner>.go lists its own
// planned kinds and deletes a case when that kind goes live (AX-0
// contract, .agents/handoffs/AX-0-contract.md); this file stays the lead's.
func (k Kind) Planned() bool {
	return plannedA2(k) || plannedA3(k) || plannedA4(k) || plannedA5(k) || plannedA6(k) || plannedA7(k) ||
		plannedA8(k) || plannedA9(k)
}
