package modding

// Inherit fills each def from its parent chain and drops abstract defs
// (PLG-022). Owner: W1-patch.
func Inherit(defs []Def) ([]Def, []Problem) { return defs, nil }
