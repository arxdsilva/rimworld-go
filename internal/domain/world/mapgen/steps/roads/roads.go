package roads

import "rimworld-go/internal/domain/world/mapgen"

// Roads is the "roads" pass (MAPGEN-2): writes Road, Roads, Track terrain, Blocked.
// Stub: owned by its Phase 1 agent.
type Roads struct{}

// Name is the step's stream and label key.
func (Roads) Name() string { return "roads" }

// Run is the pass.
func (Roads) Run(c *mapgen.Ctx) error { return nil }
