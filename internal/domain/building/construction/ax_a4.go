package construction

// plannedA4 is A4's kinds not live yet (power and industry; AX-0
// contract). AX-4 made every one real: the waterproof conduit, chemfuel
// generator, watermill, deep drill, ground-penetrating scanner, moisture
// pump and electric crematorium. This file is A4's, and so are any
// predicates it adds here for its own kinds.
func plannedA4(Kind) bool { return false }

// ExtraNeed is how many of its Def.Extra[i] secondary material frame f
// takes (AX-4 owns the delivery): none for a reinstall or an in-place
// material swap (the piece already holds them), none past the slots.
func (f Frame) ExtraNeed(i int) int {
	if i < 0 || i >= len(f.ExtraGot) || f.Reinstall != 0 || f.Swaps != 0 {
		return 0
	}
	return DefOf(f.Kind).Extra[i].N
}

// ExtrasIn reports every secondary material of frame f delivered.
func (f Frame) ExtrasIn() bool {
	for i, got := range f.ExtraGot {
		if got < f.ExtraNeed(i) {
			return false
		}
	}
	return true
}

// WheelCell reports footprint cell i (Footprint order) of kind k is a
// watermill's wheel: the row on its facing side, which must sit in a
// river's moving water (wiki: Watermill generator).
func WheelCell(k Kind, i int) bool {
	if k != Watermill {
		return false
	}
	d := DefOf(k)
	return i >= d.Size[0]*(d.Size[1]-1) && i < d.Size[0]*d.Size[1]
}
