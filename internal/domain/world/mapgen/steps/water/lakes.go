package water

import "rimworld-go/internal/domain/world/mapgen"

// Lakes is the "lakes" pass (MAPGEN-2): writes Lake, Water.
// Stub: owned by its Phase 1 agent.
type Lakes struct{}

// Name is the step's stream and label key.
func (Lakes) Name() string { return "lakes" }

// Run is the pass.
func (Lakes) Run(c *mapgen.Ctx) error { return nil }
