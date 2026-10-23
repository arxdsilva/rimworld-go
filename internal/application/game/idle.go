package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// Free time. A colonist with nothing designated to do doesn't stand
// still: it wanders, chats with someone, sits down or warms up by a fire,
// in proportions set by its temperament. Any real work replaces these at
// once (phaseAI re-checks for work every idleRecheck ticks).

// Temperament is a colonist's free-time leaning.
type Temperament uint8

const (
	Wanderer Temperament = iota // roams farther and more often
	Social                      // seeks company
	Homebody                    // sits down, stays by the fire
	numTemperaments
)

func (t Temperament) String() string {
	switch t {
	case Social:
		return "social"
	case Homebody:
		return "homebody"
	default:
		return "wanderer"
	}
}

// temperamentOf derives a stable temperament from the colonist's name, so
// each colonist keeps the same character across sessions.
func (g *Game) temperamentOf(id entity.EntityID) Temperament {
	if p, ok := g.bioFor(id); ok { // skills.go: from the bio (legacy bios: this same name hash)
		return Temperament(p.Temperament)
	}
	n := g.nameOf(id)
	h := uint32(2166136261)
	for i := 0; i < len(n); i++ {
		h = (h ^ uint32(n[i])) * 16777619
	}
	return Temperament(h % uint32(numTemperaments))
}

// idle behaviour weights per temperament: wander, chat, relax, fireside.
var idleWeights = [numTemperaments][4]int{
	Wanderer: {6, 2, 1, 1},
	Social:   {2, 6, 1, 2},
	Homebody: {1, 2, 4, 4},
}

const (
	// idleRecheck: how often an idling colonist looks for real work.
	idleRecheck  = 30
	idleMinTicks = 240
	idleMaxTicks = 720
)

// assignIdle starts a free-time behaviour. It tries the colonist's
// weighted pick first and falls back to wandering.
func (g *Game) assignIdle(id entity.EntityID) bool {
	w := idleWeights[g.temperamentOf(id)]
	total := w[0] + w[1] + w[2] + w[3]
	roll := g.rng.IntN(total)
	pick := 0
	for roll >= w[pick] {
		roll -= w[pick]
		pick++
	}
	linger := idleMinTicks + g.rng.IntN(idleMaxTicks-idleMinTicks)
	if g.temperamentOf(id) != Wanderer || g.rng.IntN(2) == 0 {
		if g.joy[id] < recreation.Entertained && g.rng.IntN(3) > 0 && g.idleHorseshoes(id, linger) {
			return true // the best fun around, when there's a pin
		}
	}
	switch pick {
	case 1:
		if g.idleChat(id, linger) {
			return true
		}
	case 2:
		if g.idleRelax(id, linger) {
			return true
		}
	case 3:
		if g.idleFire(id, linger) {
			return true
		}
	}
	return g.idleWander(id, linger)
}

// idleWander strolls to a random reachable tile nearby.
func (g *Game) idleWander(id entity.EntityID, linger int) bool {
	pos, _ := g.positions.Get(id)
	r := 5
	if g.temperamentOf(id) == Wanderer {
		r = 9
	}
	for try := 0; try < 8; try++ {
		x := pos.X + g.rng.IntN(2*r+1) - r
		y := pos.Y + g.rng.IntN(2*r+1) - r
		if x == pos.X && y == pos.Y || !g.grid.Walkable(x, y) || g.bedCovering(x, y) {
			continue
		}
		if _, taken := g.reserve.TileOwner(x, y); taken {
			continue
		}
		if !g.reachable(id, x, y) {
			continue
		}
		if g.startJob(id, job.Record{Kind: job.Wander, TX: x, TY: y, WorkLeft: linger}) {
			return true
		}
	}
	return false
}

// idleChat walks over to another free colonist nearby for a chat.
func (g *Game) idleChat(id entity.EntityID, linger int) bool {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 16
	for _, oid := range g.pawnIDs {
		if oid == id || !g.alive(oid) || g.drafted(oid) {
			continue
		}
		if v, _ := g.vital.Get(oid); v.Status != health.Alive {
			continue
		}
		oj, _ := g.jobs.Get(oid)
		if oj.Kind == job.Sleep {
			continue
		}
		op, _ := g.positions.Get(oid)
		if d := pathing.Dist(pos.X, pos.Y, op.X, op.Y); d < bestD {
			best, bestD = oid, d
		}
	}
	if best == 0 {
		return false
	}
	op, _ := g.positions.Get(best)
	sx, sy, ok := g.besideSpot(id, op.X, op.Y)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Chat, Target: best, TX: sx, TY: sy, WorkLeft: linger})
}

// idleRelax sits on the nearest free chair.
func (g *Game) idleRelax(id entity.EntityID, linger int) bool {
	return g.idleAt(id, linger, job.Relax, func(b construction.Building, bid entity.EntityID) bool {
		return b.Kind == construction.Chair
	}, false)
}

// idleFire warms up beside the nearest burning campfire.
func (g *Game) idleFire(id entity.EntityID, linger int) bool {
	return g.idleAt(id, linger, job.FireSide, func(b construction.Building, bid entity.EntityID) bool {
		return b.Kind == construction.Campfire && g.burning(bid, b)
	}, true)
}

// idleAt goes to the nearest building matching want: onto it, or beside it
// when beside is set.
func (g *Game) idleAt(id entity.EntityID, linger int, kind job.Kind,
	want func(construction.Building, entity.EntityID) bool, beside bool) bool {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !want(b, bid) {
			continue
		}
		if !beside {
			if _, taken := g.reserve.TileOwner(b.X, b.Y); taken {
				continue
			}
		}
		if d := pathing.Dist(pos.X, pos.Y, b.X, b.Y); d < bestD {
			best, bestD = bid, d
		}
	}
	if best == 0 || bestD > 30 {
		return false
	}
	b, _ := g.bldgs.Get(best)
	x, y := b.X, b.Y
	if beside {
		var ok bool
		if x, y, ok = g.besideSpot(id, b.X, b.Y); !ok {
			return false
		}
	} else if !g.reachable(id, x, y) {
		return false
	}
	if !g.startJob(id, job.Record{Kind: kind, TX: x, TY: y, WorkLeft: linger}) {
		return false
	}
	if !beside {
		g.reserve.TryTile(id, x, y) // one sitter per chair
	}
	return true
}

// doIdle lingers at the spot until the time is up (or the chat partner
// has left).
func (g *Game) doIdle(id entity.EntityID, j job.Record) {
	j.WorkLeft--
	if _, breaking := g.breaks[id]; !breaking && uint64(g.clock.Now())%recreation.GainPeriod == 0 {
		g.joy[id] = recreation.Clamp(g.joy[id] + recreation.Gain(activityOf(j.Kind)))
	}
	if j.Kind == job.Chat {
		pos, _ := g.positions.Get(id)
		op, _ := g.positions.Get(j.Target)
		if !g.alive(j.Target) || pathing.Dist(pos.X, pos.Y, op.X, op.Y) > 2 {
			j.WorkLeft = 0
		}
	}
	if j.WorkLeft <= 0 {
		j.State = job.Complete
		if j.Kind == job.Chat {
			g.learn(id, skSocial, chatXP) // skills.go
		}
	}
	g.jobs.Set(id, j)
}

// TemperamentOf names a colonist's free-time leaning (inspector).
func (g *Game) TemperamentOf(id entity.EntityID) string { return g.temperamentOf(id).String() }

// activityOf maps a free-time job to its recreation activity.
func activityOf(k job.Kind) recreation.Activity {
	switch k {
	case job.Wander:
		return recreation.Wander
	case job.Chat:
		return recreation.Chat
	case job.Relax:
		return recreation.Relax
	case job.FireSide:
		return recreation.FireSide
	case job.Horseshoes:
		return recreation.Horseshoes
	}
	return recreation.None
}

// idleHorseshoes pitches horseshoes at the nearest pin from a spot three
// tiles off it.
func (g *Game) idleHorseshoes(id entity.EntityID, linger int) bool {
	pos, _ := g.positions.Get(id)
	var best construction.Building
	found, bestD := false, 31
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.HorseshoePin {
			continue
		}
		if d := pathing.Dist(pos.X, pos.Y, b.X, b.Y); d < bestD {
			best, bestD, found = b, d, true
		}
	}
	if !found {
		return false
	}
	for _, d := range [][2]int{{0, 3}, {3, 0}, {0, -3}, {-3, 0}, {2, 2}, {-2, 2}, {2, -2}, {-2, -2}} {
		x, y := best.X+d[0], best.Y+d[1]
		if !g.grid.Walkable(x, y) || g.bedCovering(x, y) {
			continue
		}
		if _, taken := g.reserve.TileOwner(x, y); taken {
			continue
		}
		if !g.reachable(id, x, y) {
			continue
		}
		if g.startJob(id, job.Record{Kind: job.Horseshoes, TX: x, TY: y, DestX: best.X, DestY: best.Y, WorkLeft: linger}) {
			g.reserve.TryTile(id, x, y)
			return true
		}
	}
	return false
}
