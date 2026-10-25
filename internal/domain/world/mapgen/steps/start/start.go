package start

import "rimworld-go/internal/domain/world/mapgen"

// Start is the "start" pass (MAPGEN-2): writes StartScore, Start (local repair).
// Stub: owned by its Phase 1 agent.
type Start struct{}

// Name is the step's stream and label key.
func (Start) Name() string { return "start" }

// Run is the pass.
func (Start) Run(c *mapgen.Ctx) error { return nil }
