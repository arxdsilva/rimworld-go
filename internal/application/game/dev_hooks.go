package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Dev hooks for set-piece scenarios (cmd/game -scenario). They change the
// colony the same way gameplay would, only at once.

// Arm puts weapon k in colonist id's hands (k must be a weapon).
func (g *Game) Arm(id entity.EntityID, k item.Kind) {
	if k.IsWeapon() {
		g.equip[id] = k
	}
}

// RaidArmed sends an assault raid carrying exactly these weapons.
func (g *Game) RaidArmed(kit ...item.Kind) { g.raidWith(kit, planAssault) }
