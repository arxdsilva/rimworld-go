package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 power toggles and switches (POWERSIM owns this file). The
// player asks for a device or switch on or off (SetPowerWanted); a
// colonist doing Construct work walks there and flicks it (job.Flick, 30
// work), before any construction. A pending flick is any building whose
// wanted and actual state differ; the index of them is a sorted id list,
// rebuilt on load. A flick moves the topology version (net cache key) and
// balances the grid at once.

// flickWork is the work a flick takes.
const flickWork = 30

// cmdSetPowerWanted asks for building target's power on (on) or off; only
// a switch or a device with a power toggle takes it.
func (g *Game) cmdSetPowerWanted(target entity.EntityID, on bool) {
	b := g.bldgs.Ref(target)
	if b == nil || !power.SpecOf(b.Kind).Toggle {
		return
	}
	if g.powerWantOn(target) == on {
		return
	}
	pw := g.ensurePower(uint64(g.clock.Now()))
	if on {
		delete(pw.wantOff, target)
	} else {
		pw.wantOff[target] = true
	}
	g.syncFlick(target)
	g.bumpPower()
}

// syncFlick puts building id in the flick index, or takes it out, as its
// wanted and actual state differ or match.
func (g *Game) syncFlick(id entity.EntityID) {
	pw := g.pw
	i, in := slices.BinarySearch(pw.flicks, id)
	switch pending := pw.wantOff[id] != pw.isOff[id]; {
	case pending && !in:
		pw.flicks = slices.Insert(pw.flicks, i, id)
	case !pending && in:
		pw.flicks = slices.Delete(pw.flicks, i, i+1)
	}
}

// flickPending reports building id waiting for a colonist to flick it.
func (g *Game) flickPending(id entity.EntityID) bool {
	if g.pw == nil {
		return false
	}
	_, in := slices.BinarySearch(g.pw.flicks, id)
	return in && g.bldgs.Ref(id) != nil
}

// flickAt is the building on (x, y) waiting for a flick (0: none), for
// the order menu.
func (g *Game) flickAt(x, y int) entity.EntityID {
	if g.pw == nil || len(g.pw.flicks) == 0 || !g.grid.InBounds(x, y) {
		return 0
	}
	if bid := g.layerBldgAt(x, y, construction.LayerObject); g.flickPending(bid) {
		return bid
	}
	return 0
}

// assignFlick gives colonist id the nearest pending flick (Construct work,
// ahead of assignConstruct); false when there is none it can do.
func (g *Game) assignFlick(id entity.EntityID) bool {
	if g.pw == nil || len(g.pw.flicks) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var tried []entity.EntityID
	for range workTries { // SIM-027: the next nearest when one can't be started
		var best entity.EntityID
		bestD := 1 << 30
		for _, bid := range g.pw.flicks {
			b := g.bldgs.Ref(bid) // read only
			if b == nil || b.Deconstruct || g.fogged(b.X, b.Y) || !g.areaAllows(id, b.X, b.Y) {
				continue
			}
			if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
				continue
			}
			if slices.Contains(tried, bid) || g.postponedFor(id, job.Flick, bid, 0, 0) {
				continue
			}
			if d := pathing.Dist(pos.X, pos.Y, b.X, b.Y); d < bestD {
				best, bestD = bid, d // ascending ids: ties keep the lowest
			}
		}
		if best == 0 {
			return false
		}
		if rec, ok := g.flickRecord(id, best); ok && g.startJob(id, rec) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// flickRecord is colonist id's flick job for building bid: walk beside
// it, then flickWork of work.
func (g *Game) flickRecord(id, bid entity.EntityID) (job.Record, bool) {
	b := g.bldgs.Ref(bid)
	if b == nil {
		return job.Record{}, false
	}
	tx, ty, ok := g.besideSpot(id, b.X, b.Y)
	if !ok {
		return job.Record{}, false
	}
	return job.Record{Kind: job.Flick, Target: bid, TX: tx, TY: ty, WorkLeft: flickWork}, true
}

// doFlick works a flick job: when the work is done the building's actual
// state takes the wanted one and the grid balances at once.
func (g *Game) doFlick(id entity.EntityID, j job.Record) {
	if g.bldgs.Ref(j.Target) == nil {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if !g.flickPending(j.Target) {
		j.State = job.Complete // flicked by someone else, or wanted back
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skNone, false) // skills.go
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.flick(j.Target)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// flick sets building bid's actual state to the wanted one; a switch
// changes the nets. The grid balances at once.
func (g *Game) flick(bid entity.EntityID) {
	pw := g.pw
	if pw.wantOff[bid] {
		pw.isOff[bid] = true
	} else {
		delete(pw.isOff, bid)
	}
	g.syncFlick(bid)
	if b := g.bldgs.Ref(bid); b != nil && power.SpecOf(b.Kind).Role == power.Switch {
		g.pwc.topo++
	}
	g.bumpPower()
	g.balancePower(uint64(g.clock.Now()))
}

// flickJob is colonist id's forced flick of the switch or toggle at (x, y)
// (command.OrderFlick), or why it can't.
func (g *Game) flickJob(id entity.EntityID, x, y int) (job.Record, string) {
	bid := g.flickAt(x, y)
	if bid == 0 {
		return job.Record{}, ReasonGone
	}
	if m, _ := g.labor.Get(id); !m.Enabled(work.Construct) {
		return job.Record{}, ReasonNoConstruct
	}
	if r := g.busyBy(id, bid); r != "" {
		return job.Record{}, r
	}
	rec, ok := g.flickRecord(id, bid)
	if !ok {
		return job.Record{}, ReasonUnreachable
	}
	return rec, ""
}
