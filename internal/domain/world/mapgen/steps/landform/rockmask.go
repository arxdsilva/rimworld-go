package landform

import "rimworld-go/internal/domain/world/mapgen"

// RockMask is the "rockmask" pass (MAPGEN-2): writes RockTau/RoughTau/GravelTau, provisional Rock.
// Stub: owned by its Phase 1 agent.
type RockMask struct{}

// Name is the step's stream and label key.
func (RockMask) Name() string { return "rockmask" }

// Run is the pass.
func (RockMask) Run(c *mapgen.Ctx) error { return nil }
