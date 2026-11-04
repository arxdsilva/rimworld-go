package construction

// firstNeat..lastNeat are the 27 Neat Storage kinds, appended in the order of
// NEAT_STORAGE_GAP_ANALYSIS.md; NeatProfile counts from firstNeat.
const (
	firstNeat = DisplayCase
	lastNeat  = CorpseDumpSpot
)

// IsNeatStorage reports whether k is one of the 27 Neat Storage pieces.
func (k Kind) IsNeatStorage() bool { return k >= firstNeat && k <= lastNeat }

// NeatProfile is the zone profile number (1..27, enum order) of a Neat
// Storage kind; ok is false for any other kind. The zone package turns it
// into its own Profile type.
func NeatProfile(k Kind) (p int, ok bool) {
	if !k.IsNeatStorage() {
		return 0, false
	}
	return int(k-firstNeat) + 1, true
}
