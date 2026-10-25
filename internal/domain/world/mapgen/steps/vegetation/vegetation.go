package vegetation

import "rimworld-go/internal/domain/world/mapgen"

// Vegetation is the "vegetation" pass (MAPGEN-2): writes VegDensity, Plants, Grass.
// Stub: owned by its Phase 1 agent.
type Vegetation struct{}

// Name is the step's stream and label key.
func (Vegetation) Name() string { return "vegetation" }

// Run is the pass.
func (Vegetation) Run(c *mapgen.Ctx) error { return nil }
