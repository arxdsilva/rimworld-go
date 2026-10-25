package ground

import "rimworld-go/internal/domain/world/mapgen"

// Terrain is the "terrain" pass (MAPGEN-2): writes Terrain, Geology.Stone.
// Stub: owned by its Phase 1 agent.
type Terrain struct{}

// Name is the step's stream and label key.
func (Terrain) Name() string { return "terrain" }

// Run is the pass.
func (Terrain) Run(c *mapgen.Ctx) error { return nil }
