package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Military aid (PLAY-024 Phase 4) lives in game/visit (aid.go): the calls
// in the trade block, the fighters among the visitors. Here are its
// read model and the entry points the radio, the raiders and the
// prisoners use.

// aidCall is a military aid call (game/visit), saved in the trade block.
type aidCall = visit.Call

// AllyGroupView is one aid group on the map for the alerts: its faction,
// how many of its fighters stand and are on the map, whether it is
// leaving, the hours left at most, and where its standing fighters are.
type AllyGroupView struct {
	Faction   string
	Standing  int
	Total     int
	Leaving   bool
	HoursLeft int
	Spots     [][2]int
}

// aidOf is game/visit's military aid over the game.
func aidOf(g *Game) visit.Aid { return visitsOf(g).Aid(&g.trade.aid) }

// Allies lists the aid groups on the map (requests not yet here are not
// listed), in request order.
func (g *Game) Allies() []AllyGroupView {
	var out []AllyGroupView
	now, a := uint64(g.clock.Now()), aidOf(g)
	for _, c := range g.trade.aid {
		if !c.Here {
			continue
		}
		v := AllyGroupView{Leaving: c.Leaving, HoursLeft: hoursLeft(c.LeaveAt, now)}
		if f := g.factionByID(c.Faction); f != nil {
			v.Faction = f.Name
		}
		for _, id := range a.AllyIDs(c.Faction) {
			v.Total++
			if st, _ := g.vital.Get(id); st.Status == health.Alive {
				p, _ := g.positions.Get(id)
				v.Standing++
				v.Spots = append(v.Spots, [2]int{p.X, p.Y})
			}
		}
		if v.Total > 0 {
			out = append(out, v)
		}
	}
	return out
}

// aidRequestOK reports whether faction f sends military aid if asked
// now, or why not (comms.go).
func (g *Game) aidRequestOK(f faction.Faction, now uint64) (bool, string) {
	return aidOf(g).RequestOK(f, now)
}

// requestAid charges faction f and queues its fighters: hours to arrive.
func (g *Game) requestAid(f *faction.Faction, now uint64) int { return aidOf(g).Request(f, now) }

// aidTick runs the aid calls once a tick (tradeTick).
func (g *Game) aidTick(now uint64) { aidOf(g).Tick(now) }

// isAlly reports id is an ally on the map.
func (g *Game) isAlly(id entity.EntityID) bool { return g.worldSt != nil && aidOf(g).IsAlly(id) }

// raidTarget is what a raider goes for from p: the nearest standing
// colonist or ally (ties: the colonist).
func (g *Game) raidTarget(p Position) (entity.EntityID, Position, bool) {
	id, x, y, ok := aidOf(g).RaidTarget(p.X, p.Y)
	return id, Position{X: x, Y: y}, ok
}

// Engage sends ally id at (x, y) after the nearest foe it can see within
// r (the creature index: a standing raider or a hostile animal, as a
// turret picks them); it reports whether it went.
func (h visitFighters) Engage(id entity.EntityID, x, y, r int, now uint64) bool {
	g := h.g
	foe, fp, ok := g.nearestWhere(x, y, r, ckRaider|ckAnimal, func(t entity.EntityID, p Position) bool {
		return g.combat.Turrets.Targetable(t, now) && g.combat.Sight.Line(x, y, p.X, p.Y) // combat/turret.go
	})
	if !ok {
		return false
	}
	g.reserve.ReleaseOwner(id)
	d := weaponDefOf(g, id)
	rec := job.Record{Kind: job.Attack, Target: foe, TX: fp.X, TY: fp.Y, WorkLeft: combat.AimTicks(d)}
	if g.combat.Sight.CanHit(x, y, d, fp.X, fp.Y) { // already in reach: no walking needed
		rec.State = job.Do
		g.jobs.Set(id, rec)
		return true
	}
	return g.startJob(id, rec)
}
