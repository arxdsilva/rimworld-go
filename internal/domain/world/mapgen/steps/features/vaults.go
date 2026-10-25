package features

import "rimworld-go/internal/domain/world/mapgen"

// Vaults is the "vaults" pass (MAPGEN-3): sealed chambers cut into rock.
// Stub: owned by its Phase 1 agent (VAULT-GEN).
type Vaults struct{}

// Name is the step's stream and label key.
func (Vaults) Name() string { return "vaults" }

// Run is the pass.
func (Vaults) Run(c *mapgen.Ctx) error { return nil }
