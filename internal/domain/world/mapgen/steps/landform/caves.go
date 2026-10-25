package landform

import "rimworld-go/internal/domain/world/mapgen"

// Caves is the "caves" pass (MAPGEN-2): writes Cave.
// Stub: owned by its Phase 1 agent.
type Caves struct{}

// Name is the step's stream and label key.
func (Caves) Name() string { return "caves" }

// Run is the pass.
func (Caves) Run(c *mapgen.Ctx) error { return nil }
