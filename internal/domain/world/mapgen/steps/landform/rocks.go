package landform

import "rimworld-go/internal/domain/world/mapgen"

// Rocks is the "rocks" pass (MAPGEN-2): writes final Rock, RockDepth, Roof.
// Stub: owned by its Phase 1 agent.
type Rocks struct{}

// Name is the step's stream and label key.
func (Rocks) Name() string { return "rocks" }

// Run is the pass.
func (Rocks) Run(c *mapgen.Ctx) error { return nil }
