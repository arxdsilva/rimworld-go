package body

// Disfigured reports a scar on, or the loss of, a face part (eye, ear,
// nose, jaw).
func (b *Body) Disfigured() bool {
	for _, w := range b.Wounds {
		if w.Permanent && defs[w.Part].Face {
			return true
		}
	}
	for p := Part(0); p < NumParts; p++ {
		if defs[p].Face && b.isMissing(p) {
			return true
		}
	}
	return false
}
