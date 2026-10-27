package body

// Disfigured reports a scar on, or the loss of, a face part (eye, ear,
// nose, jaw).
func (b *Body) Disfigured() bool {
	t := b.table()
	for _, w := range b.Wounds {
		if w.Permanent && w.Part < Part(len(t)) && t[w.Part].Face {
			return true
		}
	}
	for p := range t {
		if t[p].Face && b.isMissing(Part(p)) {
			return true
		}
	}
	return false
}
