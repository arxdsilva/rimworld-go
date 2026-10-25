package start

import "rimworld-go/internal/domain/world/mapgen"

// Validate is the "validate" pass (MAPGEN-2): writes repairs, Metrics.
// Stub: owned by its Phase 1 agent.
type Validate struct{}

// Name is the step's stream and label key.
func (Validate) Name() string { return "validate" }

// Run is the pass.
func (Validate) Run(c *mapgen.Ctx) error { return nil }
