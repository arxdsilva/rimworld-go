package water

import "rimworld-go/internal/domain/world/mapgen"

// Coast is the "coast" pass (MAPGEN-2): writes Coast, CoastDist, Water.
// Stub: owned by its Phase 1 agent.
type Coast struct{}

// Name is the step's stream and label key.
func (Coast) Name() string { return "coast" }

// Run is the pass.
func (Coast) Run(c *mapgen.Ctx) error { return nil }
