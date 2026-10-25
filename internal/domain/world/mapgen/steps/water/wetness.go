package water

import "rimworld-go/internal/domain/world/mapgen"

// Wetness is the "wetness" pass (MAPGEN-2): writes Wetness.
// Stub: owned by its Phase 1 agent.
type Wetness struct{}

// Name is the step's stream and label key.
func (Wetness) Name() string { return "wetness" }

// Run is the pass.
func (Wetness) Run(c *mapgen.Ctx) error { return nil }
