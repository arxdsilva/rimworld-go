package landform

import "rimworld-go/internal/domain/world/mapgen"

// Elevation is the "elevation" pass (MAPGEN-2): writes Elevation, MountainBias.
// Stub: owned by its Phase 1 agent.
type Elevation struct{}

// Name is the step's stream and label key.
func (Elevation) Name() string { return "elevation" }

// Run is the pass.
func (Elevation) Run(c *mapgen.Ctx) error { return nil }
