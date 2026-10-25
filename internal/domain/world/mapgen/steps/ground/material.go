package ground

import "rimworld-go/internal/domain/world/mapgen"

// Material is the "material" pass (MAPGEN-2): writes Stone.
// Stub: owned by its Phase 1 agent.
type Material struct{}

// Name is the step's stream and label key.
func (Material) Name() string { return "material" }

// Run is the pass.
func (Material) Run(c *mapgen.Ctx) error { return nil }
