package ground

import "rimworld-go/internal/domain/world/mapgen"

// Fertility is the "fertility" pass (MAPGEN-2): writes Fertility.
// Stub: owned by its Phase 1 agent.
type Fertility struct{}

// Name is the step's stream and label key.
func (Fertility) Name() string { return "fertility" }

// Run is the pass.
func (Fertility) Run(c *mapgen.Ctx) error { return nil }
