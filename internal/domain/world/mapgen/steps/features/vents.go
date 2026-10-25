package features

import "rimworld-go/internal/domain/world/mapgen"

// Vents is the "vents" pass (MAPGEN-2): writes Vents, Blocked.
// Stub: owned by its Phase 1 agent.
type Vents struct{}

// Name is the step's stream and label key.
func (Vents) Name() string { return "vents" }

// Run is the pass.
func (Vents) Run(c *mapgen.Ctx) error { return nil }
