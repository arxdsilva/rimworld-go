package game

import "rimworld-go/internal/domain/world/vault"

// vaultSave is one sealed vault as saved (MAPGEN-3). Stub: owned by
// VAULT-GAME, which fills in the lifecycle.
type vaultSave struct {
	ID             int
	X0, Y0, X1, Y1 int
	State          vault.State
	Defenders      []uint64 // entity ids of its dormant (or woken) defenders
}
