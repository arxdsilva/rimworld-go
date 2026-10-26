package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 power toggles and switches (POWERSIM owns this file). The
// player asks for a device or switch on or off (SetPowerWanted); a
// colonist doing Construct work walks there and flicks it (job.Flick, 30
// work), before any construction. A pending flick is any building whose
// wanted and actual state differ; the index of them is a sorted id list,
// rebuilt on load. A flick moves the topology version (net cache key).

// cmdSetPowerWanted asks for building target's power on (on) or off.
func (g *Game) cmdSetPowerWanted(target entity.EntityID, on bool) {
	// TODO(POWERSIM): wanted state, flick index
}

// assignFlick gives colonist id a pending flick (Construct work, ahead of
// assignConstruct); false when there is none it can do.
func (g *Game) assignFlick(id entity.EntityID) bool {
	return false // TODO(POWERSIM)
}

// doFlick works a flick job: when the work is done the building's actual
// state takes the wanted one.
func (g *Game) doFlick(id entity.EntityID, j job.Record) {
	j.State = job.Complete // TODO(POWERSIM): count the work down, then flick
	g.jobs.Set(id, j)
}

// flickJob is colonist id's forced flick of the switch or toggle at (x, y)
// (command.OrderFlick), or why it can't.
func (g *Game) flickJob(id entity.EntityID, x, y int) (job.Record, string) {
	return job.Record{}, ReasonGone // TODO(POWERSIM)
}
