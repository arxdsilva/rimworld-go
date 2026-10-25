package resources

import "rimworld-go/internal/domain/world/mapgen"

// Ores is the "ores" pass (MAPGEN-2): writes Geology.Ore.
// Stub: owned by its Phase 1 agent.
type Ores struct{}

// Name is the step's stream and label key.
func (Ores) Name() string { return "ores" }

// Run is the pass.
func (Ores) Run(c *mapgen.Ctx) error { return nil }
