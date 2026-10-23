package ebiten

import (
	"strings"
	"sync"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
)

// Locked buildings and component costs on the Architect cards (NG-1 §2.12,
// §4.8). The scenario forbids some buildings (BuildableView.Locked, from
// Simulation.Buildables); the card is disabled with that reason. A card
// that needs components says so in its tooltip.

// lockCache remembers one simulation's locked buildings by kind for the
// tick it was read at: the Architect asks per card, several times a frame,
// and the catalog is rebuilt on every call.
type lockCache struct {
	mu     sync.Mutex
	sim    Simulation
	tick   clock.Tick
	locked map[string]string
}

var archLocks lockCache

// get is the scenario's reason kind is locked ("" when it is allowed).
func (c *lockCache) get(sim Simulation, kind string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t := sim.Tick(); c.locked == nil || c.sim != sim || c.tick != t {
		c.sim, c.tick, c.locked = sim, t, map[string]string{}
		for _, b := range sim.Buildables() {
			if b.Locked != "" {
				c.locked[b.Kind] = b.Locked
			}
		}
	}
	return c.locked[kind]
}

// toolLock is why the Architect card of t cannot be used: the scenario
// forbids the building, or its research is not done ("" when it is free).
func (a *App) toolLock(t archTool) string {
	if t.build == "" {
		return ""
	}
	if a.renderer != nil {
		if why := archLocks.get(a.renderer.game, t.build); why != "" {
			return why
		}
	}
	if p := a.lockedBy("build:" + t.build); p != "" {
		return "Needs research: " + p
	}
	return ""
}

// lockWord is the card's second line for a locked tool.
func lockWord(reason string) string {
	if strings.HasPrefix(reason, "Needs research") {
		return "needs research"
	}
	return "locked"
}

// componentsOf is how many components building bv needs besides its
// material (the read model carries the material only).
func componentsOf(bv game.BuildableView) int {
	return construction.DefOf(construction.Kind(bv.ID)).Components
}

func componentWord(n int) string {
	if n == 1 {
		return "component"
	}
	return "components"
}
