package features

import "rimworld-go/internal/domain/world/mapgen"

// Ruins is the "ruins" pass (MAPGEN-2): writes Ruins, Blocked.
// Stub: owned by its Phase 1 agent.
type Ruins struct{}

// Name is the step's stream and label key.
func (Ruins) Name() string { return "ruins" }

// Run is the pass.
func (Ruins) Run(c *mapgen.Ctx) error { return nil }
