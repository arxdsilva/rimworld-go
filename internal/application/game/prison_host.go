package game

import (
	"rimworld-go/internal/application/game/prison"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// prisonHost implements prison.Host (prison/deps.go) over the game: the
// care adapters it shares (care_host*.go) and four small ones of its own,
// so no method is added to Game (.cursor/SOLID_GO.md §10).
type prisonHost struct {
	careBodies
	careRoster
	careReach
	careJobs
	careBeds
	careEvents
	carePrison
	prisonRoster
	prisonLife
	prisonWarden
	prisonTalk
	prisonEscapes
}

var _ prison.Host = prisonHost{}

func newPrisonHost(g *Game) prisonHost {
	return prisonHost{careBodies{g}, careRoster{g}, careReach{g}, careJobs{g}, careBeds{g}, careEvents{g}, carePrison{g},
		prisonRoster{g}, prisonLife{g}, prisonWarden{g}, prisonTalk{g}, prisonEscapes{g}}
}

type prisonRoster struct{ g *Game }

func (h prisonRoster) Held(id entity.EntityID) prison.Held {
	r := h.g.raiders[id]
	return prison.Held{Leaving: r.Leaving, Hungry: r.Hungry, FoodRetry: r.FoodRetry, Idle: r.Idle, RetryAt: r.RetryAt, Warden: r.Warden}
}
func (h prisonRoster) SetHeld(id entity.EntityID, v prison.Held) {
	r := h.g.raiders[id]
	r.Leaving, r.Hungry, r.FoodRetry, r.Idle, r.RetryAt, r.Warden = v.Leaving, v.Hungry, v.FoodRetry, v.Idle, v.RetryAt, v.Warden
	h.g.raiders[id] = r
}
func (h prisonRoster) Captured(id entity.EntityID) bool   { return h.g.care.BeingCaptured(id) }
func (h prisonRoster) ReleaseOwner(owner entity.EntityID) { h.g.reserve.ReleaseOwner(owner) }
func (h prisonRoster) Roll(n int) int                     { return h.g.stream(streamPrison).IntN(n) }
func (h prisonRoster) SleepSpot(id entity.EntityID) (entity.EntityID, int, int, bool) {
	bid, ok := h.g.prisonerBed(id)
	if !ok {
		return 0, 0, 0, false
	}
	b, _ := h.g.bldgs.Get(bid)
	x, y := h.g.sleepCellFor(id, b)
	return bid, x, y, true
}
func (h prisonRoster) BedAt(id entity.EntityID) (int, int, int, bool) {
	bid, ok := h.g.prisonerBed(id)
	if !ok {
		return 0, 0, 0, false
	}
	b, _ := h.g.bldgs.Get(bid)
	return b.X, b.Y, h.g.roomAt(b.X, b.Y), true
}
func (h prisonRoster) FreeInRoom(x, y, room int) bool {
	g := h.g
	if !g.grid.InBounds(x, y) || !g.dryWalkable(x, y) || g.roomAt(x, y) != room {
		return false
	}
	_, taken := g.reserve.TileOwner(x, y)
	return !taken
}

type prisonLife struct{ g *Game }

func (h prisonLife) Needs(id entity.EntityID) needs.Pair       { n, _ := h.g.need.Get(id); return n }
func (h prisonLife) SetNeeds(id entity.EntityID, n needs.Pair) { h.g.need.Set(id, n) }
func (h prisonLife) Heal(id entity.EntityID, v health.Record, now uint64) health.Record {
	h.g.heal(id, &v, now) // combat.go
	return v
}
func (h prisonLife) Decay(id entity.EntityID, n needs.Pair, now uint64) needs.Pair {
	return h.g.decayNeeds(id, n, now) // skills.go
}
func (h prisonLife) DownedHunger(id entity.EntityID, v health.Record, now uint64) (needs.Pair, health.Record) {
	n := h.g.downedHunger(id, &v, now, false)
	return n, v
}
func (h prisonLife) Down(id entity.EntityID, c health.Cause) { h.g.down(id, c) }
func (h prisonLife) Die(id entity.EntityID, c health.Cause)  { h.g.die(id, c) }

type prisonWarden struct{ g *Game }

func (h prisonWarden) BedBack(id entity.EntityID) bool { return h.g.care.PrisonBeds.Assign(id) } // care/prisonbed.go
func (h prisonWarden) FeedRun(id, p entity.EntityID) bool {
	return h.g.care.Feed.Start(id, p, job.DeliverFood, false) // care/feed.go
}
func (h prisonWarden) Chase(id, p entity.EntityID, j job.Record, every int) {
	h.g.care.Feed.Chase(id, p, j, every)
}
func (h prisonWarden) SocialWork(id entity.EntityID) int {
	return h.g.workProgress(id, skSocial, true) // skills.go: Social trains
}
func (h prisonWarden) ConsciousEnough(id entity.EntityID) bool { return h.g.consciousEnough(id) }
func (h prisonWarden) Recruit(p, w entity.EntityID) {
	g, now := h.g, uint64(h.g.clock.Now())
	g.enlist(p)
	g.addOpinionMemory(p, w, social.MemRecruitedMe, now)
	pos, _ := g.positions.Get(p)
	text := g.personName(p) + " has joined the colony, won over by " + g.personName(w) + "."
	g.post(MessageView{Tick: now, Text: text, X: pos.X, Y: pos.Y, Focus: p, Letter: true, Title: "Recruited", Kind: letterGood})
	g.addHistory("recruit", text, p, w)
}

// Execute: its relatives hold it against w (socialOnDeath).
func (h prisonWarden) Execute(p, w entity.EntityID) {
	g := h.g
	name := g.personName(p)
	pos, _ := g.positions.Get(p)
	g.dieBy(p, health.CauseExecution, w) // combat.go
	g.message(pos.X, pos.Y, "%s executed %s.", g.personName(w), name)
	g.addHistory("execution", name+" was executed by "+g.personName(w)+".", p, w)
}

type prisonTalk struct{ g *Game }

func (h prisonTalk) Meet(a, b entity.EntityID) {
	h.g.ensurePerson(a)
	h.g.ensurePerson(b)
}
func (h prisonTalk) Opinion(from, to entity.EntityID) int32 { return h.g.opinion(from, to) }
func (h prisonTalk) Mood(id entity.EntityID) int            { return h.g.prisonerMood(id) }
func (h prisonTalk) Negotiation(id entity.EntityID) int32   { return h.g.negotiation(id) }
func (h prisonTalk) SocialLevel(id entity.EntityID) int     { return int(h.g.skillOf(id, skSocial).Level) }
func (h prisonTalk) Chat(a, b entity.EntityID, now uint64) {
	h.g.applyInteraction(a, b, social.Chitchat, now) // social.go: memories, log
}

// enlist makes p, a prisoner or a guest, a colonist: it leaves the
// raid roster with its own bio, every relation it had and the default
// work priorities (incapable work off).
func (g *Game) enlist(p entity.EntityID) {
	g.interrupt(p, job.Cancelled)
	g.releaseBed(p)
	delete(g.raiders, p)
	g.raiderIDs, g.pawnIDs = withoutID(g.raiderIDs, p), withID(g.pawnIDs, p)
	if per := g.personOf(p); per != nil {
		if _, has := g.sk.bios[p]; !has && per.HasBio {
			g.giveBio(p, per.Bio) // skills.go: its skills and stats
		}
		per.Bio, per.HasBio, per.Where, per.Faction = g.sk.bios[p], false, whereColony, 0
	}
	if _, ok := g.prio[p]; !ok {
		g.prio[p] = work.DefaultPriorities()
		g.prio[p] = g.prioOf(p) // priorities.go: incapable work off
	}
	g.labor.Set(p, g.prio[p].Mask())
	if _, ok := g.scheds[p]; !ok {
		g.scheds[p] = g.sched
	}
	g.refreshSig(p)
}

// prisonEscapes is a prison break (prison/escape.go, CBT-4).
type prisonEscapes struct{ g *Game }

func (h prisonEscapes) RoomOf(id entity.EntityID) int {
	p, _ := h.g.positions.Get(id)
	return h.g.roomAt(p.X, p.Y) + 1
}
func (h prisonEscapes) Escape(id entity.EntityID) {
	r := h.g.raiders[id]
	r.Mode, r.Escaping, r.Leaving = raid.Fleeing, true, false // raid/tactics.go: fights its way out
	h.g.raiders[id] = r
}
func (h prisonEscapes) Breakout(id entity.EntityID, n int) {
	p, _ := h.g.positions.Get(id)
	h.g.threat("Prison break", p.X, p.Y, "%s started a prison break: %d prisoners are fighting their way out!", h.g.name(id), n)
}
func (h prisonEscapes) Exit(x, y int) (int, int, bool, bool) {
	w, ht := h.g.grid.Width(), h.g.grid.Height()
	ex, ey := raid.NearestEdge(x, y, w, ht)
	tx, ty, ok := h.g.combat.Tiles.WalkableNear(ex, ey, 3)
	return tx, ty, x == 0 || y == 0 || x == w-1 || y == ht-1, ok
}
func (h prisonEscapes) Depart(id entity.EntityID) { h.g.departReleased(id) } // prisoners.go
