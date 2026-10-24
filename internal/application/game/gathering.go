package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Gatherings (ENG-003, M5): weddings and parties. One runs at a time, as
// the domain's save-safe machine (social.Gathering): the hosts walk to the
// spot, guests drift in on job.Gather (chairs nearby, else cells around
// it) and enjoy the company; a wedding's ceremony ends in marry(), a party
// ends with its thought. Every step is a function of saved state and the
// clock (staggered cadences, the social stream), so a save inside any
// phase loads into the same run: the marriage happens exactly once.
//
// An engaged couple is looked at hourly (staggered by the lower id); the
// wedding needs both of them fit, no threat on the map and a spot: the
// marriage spot, else a party spot, a campfire or the colony's home. A
// threat, a host drafted, downed, gone or breaking, or the couple splitting
// calls it off, with a day before the next try.

const (
	gatherPeriod      = 60 // gatherings advance every gatherPeriod ticks
	weddingStagger    = 29
	weddingEngagedMin = social.Day // engaged a day before the wedding
	gatherPullPeriod  = 60         // a free colonist is asked to join this often
	gatherPullStagger = 11
	hostNear          = 2 // a host this close to the spot is present
	attendRadius      = 6 // a guest this close counts as attending
	callingTimeout    = 6 * social.Hour
	partyMinColonists = 3
	partyMinMood      = 45
)

// gatherState is the gatherings and lovin' bookkeeping, saved in Social.
type gatherState struct {
	active  []social.Gathering
	retry   []gatherRetry
	lovinAt map[entity.EntityID]uint64 // next tick a pawn may make love
	// once-per-tick guard for gatherTick (never saved: loads happen
	// between ticks)
	tickAt uint64
	ticked bool
}

// gatherRetry blocks a gathering for its hosts until At (after an abort).
type gatherRetry struct {
	Kind  social.GatherKind
	Hosts [2]social.ID
	At    uint64
}

// gatherSave is gatherState as saved.
type gatherSave struct {
	Active []social.Gathering
	Retry  []gatherRetry
	Lovin  []tickSave
}

// gatherSt is the gathering state, created on first use.
func (g *Game) gatherSt() *gatherState {
	s := g.socState()
	if s.gath == nil {
		s.gath = &gatherState{lovinAt: map[entity.EntityID]uint64{}}
	}
	return s.gath
}

// running is the gathering under way (nil: none).
func (g *Game) running() *social.Gathering {
	if g.soc == nil || g.soc.gath == nil || len(g.soc.gath.active) == 0 {
		return nil
	}
	return &g.soc.gath.active[0]
}

// socialDuty is called in phaseAI right after mentalBreakTick: true means
// pawn id is taken up by lovin' or a gathering this tick. It also drives
// the staggered wedding and lovin' checks and, once per tick, the running
// gathering.
func (g *Game) socialDuty(id entity.EntityID, j job.Record) bool {
	now := uint64(g.clock.Now())
	g.gatherTick(now)
	if g.soc != nil {
		if (now+uint64(id)*weddingStagger)%social.Hour == 0 {
			g.weddingCheck(id, now)
		}
		if (now+uint64(id)*lovinStagger)%lovinPeriod == 0 {
			g.lovinCheck(id, now) // beds.go
		}
		j, _ = g.jobs.Get(id)
	}
	if j.Kind == job.Lovin && j.Active() {
		return true
	}
	return g.gatherDuty(id, j, now)
}

// gatherDuty keeps pawn id at the running gathering, or sends it there
// (on its staggered turn) when it is free to come.
func (g *Game) gatherDuty(id entity.EntityID, j job.Record, now uint64) bool {
	ga := g.running()
	in := j.Kind == job.Gather && j.Active()
	if ga == nil || !g.gatherJoins(id, j) {
		if in {
			j.State = job.Complete
			g.jobs.Set(id, j)
		}
		return false
	}
	if in {
		return true
	}
	if (now+uint64(id)*gatherPullStagger)%gatherPullPeriod != 0 {
		return false
	}
	x, y, ok := g.guestCell(id, ga)
	if h := hostIndex(ga, id); h >= 0 {
		x, y, ok = g.reachableSpot(id, ga.SpotX+h, ga.SpotY)
	}
	if !ok {
		return false
	}
	if j.Kind != job.None && j.State != job.Idle {
		g.interrupt(id, job.Interrupted)
	}
	if !g.startJob(id, job.Record{Kind: job.Gather, TX: x, TY: y, DestX: ga.SpotX, DestY: ga.SpotY, WorkLeft: 1}) {
		return false
	}
	g.reserve.TryTile(id, x, y)
	return true
}

func hostIndex(ga *social.Gathering, id entity.EntityID) int {
	for i, h := range ga.Hosts {
		if h != 0 && h == social.ID(id) {
			return i
		}
	}
	return -1
}

// gatherReady: a colonist on its feet, on the map, not drafted or breaking.
func (g *Game) gatherReady(id entity.EntityID) bool {
	return g.alive(id) && g.romanceable(id) && g.onMap(id) && !g.drafted(id) && !g.breaking(id)
}

// gatherJoins reports id comes (or stays) to the gathering now: ready, not
// on an order or an urgent job, not hungry or tired, not bedtime.
func (g *Game) gatherJoins(id entity.EntityID, j job.Record) bool {
	if !g.gatherReady(id) || j.Forced && j.Active() {
		return false
	}
	switch j.Kind {
	case job.Sleep, job.Eat, job.Rescue, job.Tend, job.Surgery, job.Warm, job.Attack, job.Hunt, job.Brawl,
		job.Lovin, job.Capture, job.Arrest, job.Kidnap, job.ExitMap, job.DeliverFood, job.Hide, job.Ingest:
		if j.Active() {
			return false
		}
	}
	n, _ := g.need.Get(id)
	if n.Tired() || n.Hungry() && g.seekFood(id) {
		return false
	}
	return !(needs.SleepScheduled(g.activityOf(id)) && !g.restedFree[id])
}

// guestCell is where guest id waits: the free chair nearest the spot
// (within attendRadius), else the first free cell two or three tiles off it.
func (g *Game) guestCell(id entity.EntityID, ga *social.Gathering) (int, int, bool) {
	best, bx, by := 1<<30, 0, 0
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Chair {
			continue
		}
		d := max(abs(b.X-ga.SpotX), abs(b.Y-ga.SpotY))
		if d > attendRadius || d >= best {
			continue
		}
		if _, taken := g.reserve.TileOwner(b.X, b.Y); taken || !g.reachable(id, b.X, b.Y) {
			continue
		}
		best, bx, by = d, b.X, b.Y
	}
	if best < 1<<30 {
		return bx, by, true
	}
	for r := 2; r <= 3; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				x, y := ga.SpotX+dx, ga.SpotY+dy
				if !g.grid.Walkable(x, y) || g.bedCovering(x, y) {
					continue
				}
				if _, taken := g.reserve.TileOwner(x, y); taken || !g.reachable(id, x, y) {
					continue
				}
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// doGather runs a gathering job: company and joy while it lasts.
func (g *Game) doGather(id entity.EntityID, j job.Record) {
	if g.running() == nil {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if uint64(g.clock.Now())%recreation.GainPeriod == 0 {
		g.joy[id] = recreation.Clamp(g.joy[id] + recreation.Gain(recreation.Chat))
	}
	g.jobs.Set(id, j)
}

// gatherTick advances the running gathering (once per tick, every
// gatherPeriod ticks) and settles a finished or aborted one.
func (g *Game) gatherTick(now uint64) {
	ga := g.running()
	if ga == nil {
		return
	}
	gs := g.soc.gath
	if gs.ticked && gs.tickAt == now {
		return
	}
	gs.ticked, gs.tickAt = true, now
	if now%gatherPeriod != 0 {
		return
	}
	prev := ga.Phase
	if !ga.Advance(now, g.hostsPresent(ga), g.gatherAbort(ga, now)) {
		return
	}
	h0, h1 := entity.EntityID(ga.Hosts[0]), entity.EntityID(ga.Hosts[1])
	switch {
	case ga.Kind == social.Wedding && prev == social.PhaseCeremony && ga.Phase == social.PhaseReception:
		if g.marry(h0, h1, now) {
			for _, a := range g.gatherAttendees(ga) {
				g.memories[a] = g.memories[a].AddFor(mood.AttendedWedding, uint64(h0), now)
			}
		}
	case ga.Kind == social.Wedding && ga.Phase == social.PhaseCeremony:
		g.gatherNote(ga, fmt.Sprintf("The wedding ceremony of %s and %s has begun.", g.personName(h0), g.personName(h1)))
	case ga.Phase == social.PhaseOver:
		if ga.Kind == social.Party {
			for _, a := range append(g.gatherAttendees(ga), h0) {
				g.memories[a] = g.memories[a].AddFor(mood.AttendedParty, uint64(h0), now)
			}
			g.gatherNote(ga, "The party is over.")
		}
		gs.active = gs.active[1:]
	case ga.Phase == social.PhaseAborted:
		text := fmt.Sprintf("%s's party was called off.", g.personName(h0))
		if ga.Kind == social.Wedding {
			text = fmt.Sprintf("The wedding of %s and %s was called off.", g.personName(h0), g.personName(h1))
		}
		g.gatherNote(ga, text)
		gs.retry = append(gs.retry, gatherRetry{Kind: ga.Kind, Hosts: ga.Hosts, At: ga.RetryAt()})
		gs.active = gs.active[1:]
	}
	if len(gs.active) == 0 {
		gs.active = nil
	}
}

// hostsPresent reports every host stands near the spot.
func (g *Game) hostsPresent(ga *social.Gathering) bool {
	for _, h := range ga.Hosts {
		if h == 0 {
			continue
		}
		p, ok := g.positions.Get(entity.EntityID(h))
		if !ok || max(abs(p.X-ga.SpotX), abs(p.Y-ga.SpotY)) > hostNear {
			return false
		}
	}
	return true
}

// gatherAbort reports the gathering must end now: a threat, a host unfit,
// a couple no longer engaged, or hosts who never came.
func (g *Game) gatherAbort(ga *social.Gathering, now uint64) bool {
	if g.threatActive(now) {
		return true
	}
	for _, h := range ga.Hosts {
		if h != 0 && !g.gatherReady(entity.EntityID(h)) {
			return true
		}
	}
	if ga.Kind == social.Wedding {
		k := g.pairOf(entity.EntityID(ga.Hosts[0]), entity.EntityID(ga.Hosts[1])).Kind
		if k != social.PairFiance && !(k == social.PairSpouse && ga.Phase == social.PhaseReception) {
			return true
		}
	}
	return ga.Phase == social.PhaseCalling && now >= ga.PhaseAt+callingTimeout
}

// gatherAttendees are the guests at the spot (hosts excluded).
func (g *Game) gatherAttendees(ga *social.Gathering) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.pawnIDs {
		if hostIndex(ga, id) >= 0 || !g.alive(id) {
			continue
		}
		j, _ := g.jobs.Get(id)
		p, ok := g.positions.Get(id)
		if j.Kind == job.Gather && j.State == job.Do && ok && max(abs(p.X-ga.SpotX), abs(p.Y-ga.SpotY)) <= attendRadius {
			out = append(out, id)
		}
	}
	return out
}

// gatherNote posts a message about the gathering.
func (g *Game) gatherNote(ga *social.Gathering, text string) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: ga.SpotX, Y: ga.SpotY, Focus: entity.EntityID(ga.Hosts[0])})
}

// retryBlocked reports a gathering of kind for hosts a, b waits after an
// abort (expired entries are dropped).
func (g *Game) retryBlocked(kind social.GatherKind, a, b entity.EntityID, now uint64) bool {
	gs := g.soc.gath
	if gs == nil {
		return false
	}
	gs.retry = slices.DeleteFunc(gs.retry, func(r gatherRetry) bool { return r.At <= now })
	if len(gs.retry) == 0 {
		gs.retry = nil
	}
	for _, r := range gs.retry {
		if r.Kind == kind && (r.Hosts == [2]social.ID{social.ID(a), social.ID(b)} || r.Hosts == [2]social.ID{social.ID(b), social.ID(a)}) {
			return true
		}
	}
	return false
}

// weddingCheck is id's hourly look at its engagements (as the lower id of
// the couple): a wedding starts when they are fit, nothing threatens and
// a spot is free.
func (g *Game) weddingCheck(id entity.EntityID, now uint64) {
	if g.running() != nil || !g.gatherReady(id) {
		return
	}
	for _, o := range g.soc.adj[id] {
		if o < id {
			continue
		}
		p := g.pairOf(id, o)
		if p.Kind != social.PairFiance || now < p.LastChange+weddingEngagedMin || !g.gatherReady(o) {
			continue
		}
		if g.retryBlocked(social.Wedding, id, o, now) {
			continue
		}
		if g.threatActive(now) {
			return
		}
		x, y, ok := g.gatherSpot(true)
		if !ok {
			return
		}
		g.startGathering(social.Gathering{Kind: social.Wedding, SpotX: x, SpotY: y,
			Hosts: [2]social.ID{social.ID(id), social.ID(o)}, Phase: social.PhaseCalling, PhaseAt: now})
		text := fmt.Sprintf("%s and %s are getting married! The colony gathers for the wedding.", g.personName(id), g.personName(o))
		g.post(MessageView{Tick: now, Text: text, X: x, Y: y, Focus: id, Letter: true, Title: "Wedding"})
		return
	}
}

func (g *Game) startGathering(ga social.Gathering) {
	gs := g.gatherSt()
	gs.active = append(gs.active, ga)
}

// gatherSpot is where a gathering is held: for a wedding the marriage
// spot first; then a party spot, a campfire, and (weddings only) the
// colony's home.
func (g *Game) gatherSpot(wedding bool) (int, int, bool) {
	var marriage, party, fire *construction.Building
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok {
			continue
		}
		switch {
		case b.Kind == construction.MarriageSpot && marriage == nil:
			marriage = &b
		case b.Kind == construction.PartySpot && party == nil:
			party = &b
		case b.Kind == construction.Campfire && fire == nil:
			fire = &b
		}
	}
	if wedding && marriage != nil {
		return marriage.X, marriage.Y, true
	}
	if party != nil {
		return party.X, party.Y, true
	}
	if fire != nil {
		x, y := fire.X, fire.Y+1 // in front of the fire, not in it
		if !g.grid.Walkable(x, y) {
			y = fire.Y - 1
		}
		return x, y, true
	}
	if wedding {
		x, y := g.home() // handling.go
		return x, y, true
	}
	return 0, 0, false
}

// marry weds a and b at the end of their ceremony: spouses, the mood, a
// shared surname (sometimes), the letter and the history line. It does
// nothing for a couple already married, so it can never pay out twice.
func (g *Game) marry(a, b entity.EntityID, now uint64) bool {
	p := g.pairOf(a, b)
	if p.Kind == social.PairSpouse || !p.Transition(social.PairSpouse, now) {
		return false
	}
	g.setPair(a, b, p)
	g.memories[a] = g.memories[a].AddFor(mood.GotMarried, uint64(b), now)
	g.memories[b] = g.memories[b].AddFor(mood.GotMarried, uint64(a), now)
	if g.stream(streamSocial).IntN(2) == 0 {
		g.shareSurname(a, b)
	}
	text := fmt.Sprintf("%s and %s are now married.", g.personName(a), g.personName(b))
	g.romanceLetter("Married", text, a) // social_romance.go
	g.addHistory("wedding", text, a, b)
	return true
}

// shareSurname has one spouse take the other's surname: the wife the
// husband's, else the higher id the lower's.
func (g *Game) shareSurname(a, b entity.EntityID) {
	ba, okA := g.personBio(a)
	bb, okB := g.personBio(b)
	if !okA || !okB || ba.Last == bb.Last || ba.Last == "" || bb.Last == "" {
		return
	}
	taker, from := b, ba.Last
	switch {
	case ba.Look.Sex != bb.Look.Sex && ba.Look.Sex == look.Female:
		taker, from = a, bb.Last
	case ba.Look.Sex == bb.Look.Sex && a < b:
		taker, from = b, ba.Last
	case ba.Look.Sex == bb.Look.Sex:
		taker, from = a, bb.Last
	}
	g.setSurname(taker, from)
}

// setSurname changes id's Bio.Last only (colonist bio or registry bio).
func (g *Game) setSurname(id entity.EntityID, last string) {
	if p, ok := g.sk.bios[id]; ok {
		p.Last = last
		g.sk.bios[id] = p
		return
	}
	if p := g.personOf(id); p != nil && p.HasBio {
		p.Bio.Last = last
	}
}

// partyFits reports whether a party can be thrown now: none running, no
// threat, enough fit colonists in a decent mood, and a party spot or
// campfire.
func (g *Game) partyFits(now uint64) bool {
	if g.running() != nil || g.threatActive(now) {
		return false
	}
	ready := g.partyGuests()
	if len(ready) < partyMinColonists {
		return false
	}
	sum := 0
	for _, id := range ready {
		sum += g.moodOf(id)
	}
	if sum < partyMinMood*len(ready) {
		return false
	}
	_, _, ok := g.gatherSpot(false)
	return ok
}

// partyGuests are the colonists fit for a party, in id order.
func (g *Game) partyGuests() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.pawnIDs {
		if g.colonist(id) && g.gatherReady(id) {
			out = append(out, id)
		}
	}
	return out
}

// throwParty starts a party gathering hosted by a fit colonist.
func (g *Game) throwParty(now uint64) bool {
	if !g.partyFits(now) {
		return false
	}
	x, y, _ := g.gatherSpot(false)
	guests := g.partyGuests()
	host := guests[g.stream(streamSocial).IntN(len(guests))]
	g.startGathering(social.Gathering{Kind: social.Party, SpotX: x, SpotY: y,
		Hosts: [2]social.ID{social.ID(host), 0}, Phase: social.PhaseCalling, PhaseAt: now})
	g.post(MessageView{Tick: now, Text: fmt.Sprintf("%s is throwing a party! Everyone is welcome.", g.personName(host)),
		X: x, Y: y, Focus: host, Letter: true, Title: "Party"})
	return true
}

// GatheringView is a running party or wedding for the HUD banner.
type GatheringView struct {
	Kind  string // "wedding", "party"
	Phase string // "gathering", "ceremony", "reception", "under way"
	Hosts []string
	X, Y  int
}

// Gatherings lists the running gatherings (read only).
func (g *Game) Gatherings() []GatheringView {
	ga := g.running()
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

// export is the gathering block of the Social save (nil: nothing).
func (s *gatherState) export() *gatherSave {
	if s == nil {
		return nil
	}
	out := &gatherSave{Active: slices.Clone(s.active), Retry: slices.Clone(s.retry)}
	for _, id := range sortedIDs(s.lovinAt) {
		out.Lovin = append(out.Lovin, tickSave{id, s.lovinAt[id]})
	}
	if len(out.Active) == 0 && len(out.Retry) == 0 && len(out.Lovin) == 0 {
		return nil
	}
	return out
}

// importGather restores gatherState.export's state into the social state.
func (g *Game) importGather(in *gatherSave) {
	if in == nil {
		return
	}
	s := g.gatherSt()
	s.active = slices.Clone(in.Active)
	s.retry = slices.Clone(in.Retry)
	for _, x := range in.Lovin {
		s.lovinAt[x.ID] = x.Tick
	}
}
