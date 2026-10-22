package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// Mental breaks: once an hour a miserable colonist (mood under 20) may
// break. For the break's length it takes no orders and does no work - it
// wanders in a daze, binge-eats, or shuts itself away - and afterwards
// it feels a day of catharsis. Breaks never starve or exhaust anyone: a
// starving or exhausted colonist still eats and sleeps.

type breakState struct {
	kind  mentalbreak.Kind
	until uint64
}

// mentalBreakTick handles breaks for one colonist in phaseAI: it may start
// one, runs one in progress, and ends it. It reports whether the colonist
// is breaking (so phaseAI leaves it alone).
func (g *Game) mentalBreakTick(id entity.EntityID, j job.Record) bool {
	now := uint64(g.clock.Now())
	b, breaking := g.breaks[id]
	if !breaking {
		if (now+uint64(id)*977)%clock.TicksPerHour != 0 {
			return false
		}
		risk := mentalbreak.RiskPerHour(g.moodOf(id))
		if risk == 0 || g.rng.IntN(1000) >= risk {
			return false
		}
		g.startBreak(id)
		return true
	}
	if now >= b.until {
		delete(g.breaks, id)
		g.logEvent(id, logEntry{Kind: logRecovered})
		g.remember(id, mood.Catharsis)
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s has calmed down and feels a little better.", g.name(id))
		g.interrupt(id, job.Cancelled)
		return false
	}
	if j.Active() && (j.Kind == job.Eat || j.Kind == mentalJob[b.kind]) {
		return true
	}
	switch b.kind {
	case mentalbreak.SadWander:
		if !g.idleWander(id, 200+g.rng.IntN(300)) {
			return true
		}
	case mentalbreak.FoodBinge:
		if best, bx, by := g.bestFood(id); best != 0 {
			g.startJob(id, job.Record{Kind: job.Eat, Target: best, TX: bx, TY: by, WorkLeft: needs.EatWorkTicks, Count: bingeEat})
		} else if !g.idleWander(id, 300) {
			return true
		}
	case mentalbreak.HideInRoom:
		g.hide(id, int(b.until-now))
	}
	return true
}

// bingeEat marks an Eat job that ignores fullness (j.Count).
const bingeEat = 3

var mentalJob = map[mentalbreak.Kind]job.Kind{
	mentalbreak.SadWander:  job.Wander,
	mentalbreak.FoodBinge:  job.Eat,
	mentalbreak.HideInRoom: job.Hide,
}

// startBreak picks a break that makes sense here and starts it.
func (g *Game) startBreak(id entity.EntityID) {
	kinds := []mentalbreak.Kind{mentalbreak.SadWander, mentalbreak.SadWander}
	if best, _, _ := g.bestFood(id); best != 0 {
		kinds = append(kinds, mentalbreak.FoodBinge)
	}
	if g.hideSpot(id) != nil {
		kinds = append(kinds, mentalbreak.HideInRoom)
	}
	k := kinds[g.rng.IntN(len(kinds))]
	g.interrupt(id, job.Cancelled)
	delete(g.draft, id)
	delete(g.orderQueue, id)
	g.breaks[id] = breakState{kind: k, until: uint64(g.clock.Now()) + mentalbreak.DefOf(k).Duration}
	g.logEvent(id, logEntry{Kind: logMental, Name: mentalbreak.DefOf(k).Label})
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "Mental break: "+mentalbreak.DefOf(k).Message, g.name(id))
}

// hideSpot is where a colonist would shut itself away: its own bed if it
// has one, else the nearest cell of any room.
func (g *Game) hideSpot(id entity.EntityID) *Position {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && b.SlotOf(uint64(id)) >= 0 {
			x, y := g.sleepCellFor(id, b)
			return &Position{x, y}
		}
	}
	g.ensureRooms()
	pos, _ := g.positions.Get(id)
	w := g.grid.Width()
	var best *Position
	bestD := 1 << 30
	for i, r := range g.roomGrid {
		if r < 0 || r == room.Wall {
			continue
		}
		x, y := i%w, i/w
		if !g.grid.Walkable(x, y) {
			continue
		}
		if d := pathing.Dist(pos.X, pos.Y, x, y); d < bestD {
			best, bestD = &Position{x, y}, d
		}
	}
	return best
}

func (g *Game) hide(id entity.EntityID, ticks int) {
	p := g.hideSpot(id)
	if p == nil || !g.reachable(id, p.X, p.Y) {
		g.idleWander(id, 300)
		return
	}
	g.startJob(id, job.Record{Kind: job.Hide, TX: p.X, TY: p.Y, WorkLeft: ticks})
}

// Breaking names a colonist's mental break ("" when none).
func (g *Game) Breaking(id entity.EntityID) string {
	if b, ok := g.breaks[id]; ok {
		return b.kind.String()
	}
	return ""
}

// ForceBreak starts a mental break now (dev scenarios and fixtures).
func (g *Game) ForceBreak(id entity.EntityID, k mentalbreak.Kind) {
	if v, ok := g.vital.Get(id); !ok || v.Status != health.Alive {
		return
	}
	g.interrupt(id, job.Cancelled)
	delete(g.draft, id)
	g.breaks[id] = breakState{kind: k, until: uint64(g.clock.Now()) + mentalbreak.DefOf(k).Duration}
}
