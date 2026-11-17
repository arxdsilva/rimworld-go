package modding

// CheckManifest reports what is wrong with one manifest (PLG-001): a
// missing or malformed id, the reserved id "core", a missing name or
// version, and malformed entries in requires/after/before/conflicts.
// Owner: W1-order.
func CheckManifest(m Manifest) []Problem { return nil }

// Resolve orders the enabled plugins among found (PLG-010, PLG-011).
// gameVersion is "major.minor[.patch]". Core is always Loaded[0].
// Owner: W1-order.
func Resolve(found []Manifest, enabled []string, gameVersion string) (Order, []Problem) {
	return Order{Loaded: []Placed{{ID: Core, Reason: "the game itself"}}}, nil
}
