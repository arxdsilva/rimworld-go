package water

import "rimworld-go/internal/domain/world/mapgen"

// River is the "river" pass (MAPGEN-2): writes River, RiverDist, RiverLine, Water; cuts Rock.
// Stub: owned by its Phase 1 agent.
type River struct{}

// Name is the step's stream and label key.
func (River) Name() string { return "river" }

// Run is the pass.
func (River) Run(c *mapgen.Ctx) error { return nil }
