package modding

// ParseOps reads p's patches files into operations (PLG-030, PLG-031).
// Owner: W1-patch.
func ParseOps(p Plugin) ([]Op, []Problem) { return nil, nil }

// Apply runs ops in order over defs (PLG-030…033). loads reports whether
// a plugin id is in the load order (for If and Missing). Owner: W1-patch.
func Apply(defs []Def, ops []Op, loads func(id string) bool) ([]Def, []Problem) {
	return defs, nil
}
