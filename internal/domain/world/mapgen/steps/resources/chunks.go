package resources

import "rimworld-go/internal/domain/world/mapgen"

// Chunks is the "chunks" pass (MAPGEN-2): writes ChunkDensity, Chunks, Blocked.
// Stub: owned by its Phase 1 agent.
type Chunks struct{}

// Name is the step's stream and label key.
func (Chunks) Name() string { return "chunks" }

// Run is the pass.
func (Chunks) Run(c *mapgen.Ctx) error { return nil }
