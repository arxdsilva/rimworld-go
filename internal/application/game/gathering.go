package game

import (
	"rimworld-go/internal/application/game/gather"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

// Weddings and parties live in game/gather; here are its entry points,
// read model and host adapters.

// gatherOf is game/gather over the game (its host built once per social
// state; a read before the social state exists builds a throwaway one).
func gatherOf(g *Game) gather.Gatherings {
	if s := g.soc; s != nil {
		if s.gathH == nil {
			s.gathH = newGatherHost(g)
		}
		return gather.Of(s.gathH)
	}
	return gather.Of(newGatherHost(g))
}

// gatherSt is the gathering state, created on first use.
func (g *Game) gatherSt() *gather.State {
	s := g.socState()
	if s.gath == nil {
		s.gath = gather.NewState()
	}
	return s.gath
}

// socialDuty is called in phaseAI right after mentalBreakTick: true means
// pawn id is taken up by lovin' or a gathering this tick.
func (g *Game) socialDuty(id entity.EntityID, j job.Record) bool {
	now := uint64(g.clock.Now())
	if g.soc != nil {
		return gatherOf(g).Duty(id, j, now)
	}
	if j.Kind == job.Gather && j.Active() { // no social state, so no gathering: this job is over
		j.State = job.Complete
		g.jobs.Set(id, j)
	}
	return j.Kind == job.Lovin && j.Active()
}

// doGather runs a gathering job: company and joy while it lasts.
func (g *Game) doGather(id entity.EntityID, j job.Record) {
	gatherOf(g).Do(id, j, uint64(g.clock.Now()))
}

// gatherTick advances the running gathering (phaseSocial).
func (g *Game) gatherTick(now uint64) {
	if g.soc != nil {
		gatherOf(g).Tick(now)
	}
}

// partyFits reports whether a party can be thrown now (incidents.go).
func (g *Game) partyFits(now uint64) bool { return gatherOf(g).PartyFits(now) }

// throwParty starts a party gathering hosted by a fit colonist.
func (g *Game) throwParty(now uint64) bool { return gatherOf(g).ThrowParty(now) }

// GatheringView is a running party or wedding for the HUD banner.
type GatheringView struct {
	Kind  string // "wedding", "party"
	Phase string // "gathering", "ceremony", "reception", "under way"
	Hosts []string
	X, Y  int
}

// Gatherings lists the running gatherings (read only).
func (g *Game) Gatherings() []GatheringView {
	ga := gatherOf(g).Running()
	if ga == nil {
		return nil
	}
	v := GatheringView{Kind: "party", X: ga.SpotX, Y: ga.SpotY}
	if ga.Kind == social.Wedding {
		v.Kind = "wedding"
	}
	switch ga.Phase {
	case social.PhaseCalling:
		v.Phase = "gathering"
	case social.PhaseUnderway:
		v.Phase = "under way"
		if ga.Kind == social.Wedding {
			v.Phase = "guests arriving"
		}
	case social.PhaseCeremony:
		v.Phase = "ceremony"
	case social.PhaseReception:
		v.Phase = "reception"
	}
	for _, h := range ga.Hosts {
		if h != 0 {
			v.Hosts = append(v.Hosts, g.personName(entity.EntityID(h)))
		}
	}
	return []GatheringView{v}
}

// importGather restores a saved gathering block into the social state.
func (g *Game) importGather(in *gather.Save) {
	if in != nil {
		gather.Import(g.gatherSt(), in)
	}
}

// gatherHost is game/gather's roles (gather/deps.go) over small adapters.
type gatherHost struct {
	gatherFolk
	gatherWalks
	gatherGrounds
	gatherHearts
	gatherTidings
}

func newGatherHost(g *Game) gatherHost {
	return gatherHost{gatherFolk{g}, gatherWalks{g}, gatherGrounds{g}, gatherHearts{g}, gatherTidings{g}}
}

type gatherFolk struct{ g *Game }

func (h gatherFolk) Alive(id entity.EntityID) bool       { return h.g.alive(id) }
func (h gatherFolk) Romanceable(id entity.EntityID) bool { return h.g.romanceable(id) }
func (h gatherFolk) OnMap(id entity.EntityID) bool       { return primary(h.g) && h.g.onMap(id) } // MAP-0: the first colony's
func (h gatherFolk) Drafted(id entity.EntityID) bool     { return h.g.drafted(id) }
func (h gatherFolk) Breaking(id entity.EntityID) bool    { return h.g.breaking(id) }
func (h gatherFolk) Colonist(id entity.EntityID) bool    { return h.g.colonist(id) }
func (h gatherFolk) PawnIDs() []entity.EntityID          { return h.g.pawnIDs }
func (h gatherFolk) Mood(id entity.EntityID) int         { return h.g.moodOf(id) }

type gatherWalks struct{ g *Game }

func (h gatherWalks) Job(id entity.EntityID) job.Record              { j, _ := h.g.jobs.Get(id); return j }
func (h gatherWalks) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h gatherWalks) Interrupt(id entity.EntityID)                   { h.g.interrupt(id, job.Interrupted) }
func (h gatherWalks) StartJob(id entity.EntityID, r job.Record) bool { return h.g.startJob(id, r) }
func (h gatherWalks) ClaimTile(id entity.EntityID, x, y int)         { h.g.reserve.TryTile(id, x, y) }
func (h gatherWalks) TileTaken(x, y int) bool                        { _, t := h.g.reserve.TileOwner(x, y); return t }
func (h gatherWalks) Reachable(id entity.EntityID, x, y int) bool    { return h.g.reachable(id, x, y) }
func (h gatherWalks) ReachableSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.reachableSpot(id, x, y)
}

type gatherGrounds struct{ g *Game }

func (h gatherGrounds) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h gatherGrounds) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h gatherGrounds) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h gatherGrounds) Walkable(x, y int) bool       { return h.g.grid.Walkable(x, y) }
func (h gatherGrounds) BedCovering(x, y int) bool    { return h.g.bedCovering(x, y) }
func (h gatherGrounds) Home() (int, int)             { return h.g.home() }
func (h gatherGrounds) ThreatActive(now uint64) bool { return h.g.threatActive(now) }

type gatherHearts struct{ g *Game }

func (h gatherHearts) Pair(a, b entity.EntityID) social.Pair         { return h.g.pairOf(a, b) }
func (h gatherHearts) SetPair(a, b entity.EntityID, p social.Pair)   { h.g.setPair(a, b, p) }
func (h gatherHearts) Partners(id entity.EntityID) []entity.EntityID { return h.g.soc.adj[id] }
func (h gatherHearts) Needy(id entity.EntityID) bool {
	n, _ := h.g.need.Get(id)
	return n.Tired() || n.Hungry() && h.g.seekFood(id)
}
func (h gatherHearts) Bedtime(id entity.EntityID) bool {
	return needs.SleepScheduled(h.g.activityOf(id)) && !h.g.rest.RestedFree(id)
}
func (h gatherHearts) LovinCheck(id entity.EntityID, now uint64) { h.g.rest.Lovin.Check(id, now) }
func (h gatherHearts) AddJoy(id entity.EntityID, n int) {
	h.g.joy[id] = recreation.Clamp(h.g.joy[id] + n)
}
func (h gatherHearts) Remember(id entity.EntityID, t mood.Kind, other entity.EntityID, now uint64) {
	h.g.memories[id] = h.g.memories[id].AddFor(t, uint64(other), now)
}

type gatherTidings struct{ g *Game }

func (h gatherTidings) State(create bool) *gather.State {
	if create {
		return h.g.gatherSt()
	}
	if h.g.soc == nil {
		return nil
	}
	return h.g.soc.gath
}
func (h gatherTidings) Name(id entity.EntityID) string              { return h.g.personName(id) }
func (h gatherTidings) Bio(id entity.EntityID) (pawngen.Pawn, bool) { return h.g.personBio(id) }
func (h gatherTidings) SetSurname(id entity.EntityID, last string) { // colonist bio or registry bio
	g := h.g
	if p, ok := g.sk.bios[id]; ok {
		p.Last = last
		g.sk.bios[id] = p
		return
	}
	if p := g.personOf(id); p != nil && p.HasBio {
		p.Bio.Last = last
	}
}
func (h gatherTidings) Note(x, y int, focus entity.EntityID, format string, args ...any) {
	h.g.notice(x, y, focus, format, args...) // incidents.go
}
func (h gatherTidings) GoodLetter(title string, x, y int, focus entity.EntityID, format string, args ...any) {
	h.g.post(MessageView{Tick: uint64(h.g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y, Focus: focus, Letter: true, Title: title, Kind: letterGood})
}
func (h gatherTidings) RomanceLetter(title, text string, focus entity.EntityID) {
	pawnLetterf(h.g, letterGood, title, focus, msgfmt.Plain(text)) // social_romance.go
}
func (h gatherTidings) History(kind, text string, a, b entity.EntityID) {
	h.g.addHistory(kind, text, a, b)
}
func (h gatherTidings) Roll(n int) int { return h.g.stream(streamSocial).IntN(n) }
