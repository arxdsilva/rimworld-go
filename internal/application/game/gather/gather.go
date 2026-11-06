// Package gather is the colony's weddings and parties (ENG-003, M5). One
// runs at a time, as the domain's save-safe machine (social.Gathering):
// the hosts walk to the spot, guests drift in on job.Gather (chairs nearby,
// else cells around it) and enjoy the company; a wedding's ceremony ends in
// Marry, a party ends with its thought. Every step is a function of saved
// state and the clock (staggered cadences, the social stream), so a save
// inside any phase loads into the same run: the marriage happens exactly
// once.
//
// An engaged couple is looked at hourly (staggered by the lower id); the
// wedding needs both of them fit, no threat on the map and a spot: the
// marriage spot, else a party spot, a campfire or the colony's home. A
// threat, a host drafted, downed, gone or breaking, or the couple splitting
// calls it off, with a day before the next try. The game keeps and saves
// the state; only the first colony holds gatherings (MAP-0).
package gather

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

const (
	// Period: gatherings advance every Period ticks.
	Period            = 60
	weddingStagger    = 29
	weddingEngagedMin = social.Day // engaged a day before the wedding
	pullPeriod        = 60         // a free colonist is asked to join this often
	pullStagger       = 11
	hostNear          = 2 // a host this close to the spot is present
	attendRadius      = 6 // a guest this close counts as attending
	callingTimeout    = 6 * social.Hour
	partyMinColonists = 3
	partyMinMood      = 45
)

// State is the gatherings and lovin' bookkeeping, saved in Social.
type State struct {
	Active  []social.Gathering
	Retry   []Retry
	LovinAt map[entity.EntityID]uint64 // next tick a pawn may make love
	// once-per-tick guard for Tick (never saved: loads happen between
	// ticks)
	tickAt uint64
	ticked bool
}

// NewState is an empty state.
func NewState() *State { return &State{LovinAt: map[entity.EntityID]uint64{}} }

// Retry blocks a gathering for its hosts until At (after an abort).
type Retry struct {
	Kind  social.GatherKind
	Hosts [2]social.ID
	At    uint64
}

// Tick is one pawn's lovin' cooldown in a save.
type Tick struct {
	ID   entity.EntityID
	Tick uint64
}

// Save is State as saved.
type Save struct {
	Active []social.Gathering
	Retry  []Retry
	Lovin  []Tick
}

// Gatherings is the gatherings over the host.
type Gatherings struct{ h Host }

// Of is the gatherings over h.
func Of(h Host) Gatherings { return Gatherings{h} }

// Running is the gathering under way (nil: none).
func (g Gatherings) Running() *social.Gathering {
	if s := g.h.State(false); s != nil && len(s.Active) > 0 {
		return &s.Active[0]
	}
	return nil
}

// Duty is called in phaseAI right after the mental break check: true means
// pawn id is taken up by lovin' or a gathering this tick. It also drives
// the staggered wedding and lovin' checks and, once per tick, the running
// gathering.
func (g Gatherings) Duty(id entity.EntityID, j job.Record, now uint64) bool {
	g.Tick(now)
	if (now+uint64(id)*weddingStagger)%social.Hour == 0 {
		g.WeddingCheck(id, now)
	}
	if (now+uint64(id)*rest.LovinStagger)%rest.LovinPeriod == 0 {
		g.h.LovinCheck(id, now) // rest/lovin.go
	}
	j = g.h.Job(id)
	if j.Kind == job.Lovin && j.Active() {
		return true
	}
	return g.duty(id, j, now)
}

// duty keeps pawn id at the running gathering, or sends it there (on its
// staggered turn) when it is free to come.
func (g Gatherings) duty(id entity.EntityID, j job.Record, now uint64) bool {
	ga := g.Running()
	in := j.Kind == job.Gather && j.Active()
	if ga == nil || !g.joins(id, j) {
		if in {
			j.State = job.Complete
			g.h.SetJob(id, j)
		}
		return false
	}
	if in {
		return true
	}
	if (now+uint64(id)*pullStagger)%pullPeriod != 0 {
		return false
	}
	x, y, ok := g.guestCell(id, ga)
	if h := hostIndex(ga, id); h >= 0 {
		x, y, ok = g.h.ReachableSpot(id, ga.SpotX+h, ga.SpotY)
	}
	if !ok {
		return false
	}
	if j.Kind != job.None && j.State != job.Idle {
		g.h.Interrupt(id)
	}
	if !g.h.StartJob(id, job.Record{Kind: job.Gather, TX: x, TY: y, DestX: ga.SpotX, DestY: ga.SpotY, WorkLeft: 1}) {
		return false
	}
	g.h.ClaimTile(id, x, y)
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

// ready: a colonist on its feet, on the map, not drafted or breaking.
func (g Gatherings) ready(id entity.EntityID) bool {
	return g.h.Alive(id) && g.h.Romanceable(id) && g.h.OnMap(id) && !g.h.Drafted(id) && !g.h.Breaking(id)
}

// joins reports id comes (or stays) to the gathering now: ready, not on an
// order or an urgent job, not hungry or tired, not bedtime.
func (g Gatherings) joins(id entity.EntityID, j job.Record) bool {
	if !g.ready(id) || j.Forced && j.Active() {
		return false
	}
	switch j.Kind {
	case job.Sleep, job.Eat, job.Rescue, job.Tend, job.Surgery, job.Warm, job.Attack, job.Hunt, job.Brawl,
		job.Lovin, job.Capture, job.Arrest, job.Kidnap, job.ExitMap, job.DeliverFood, job.Hide, job.Ingest:
		if j.Active() {
			return false
		}
	}
	return !g.h.Needy(id) && !g.h.Bedtime(id)
}

// guestCell is where guest id waits: the free chair nearest the spot
// (within attendRadius), else the first free cell two or three tiles off it.
func (g Gatherings) guestCell(id entity.EntityID, ga *social.Gathering) (int, int, bool) {
	best, bx, by := 1<<30, 0, 0
	for _, bid := range g.h.BuildingIDs() {
		b, ok := g.h.Building(bid)
		if !ok || b.Kind != construction.Chair {
			continue
		}
		d := max(mathx.Abs(b.X-ga.SpotX), mathx.Abs(b.Y-ga.SpotY))
		if d > attendRadius || d >= best {
			continue
		}
		if g.h.TileTaken(b.X, b.Y) || !g.h.Reachable(id, b.X, b.Y) {
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
				if max(mathx.Abs(dx), mathx.Abs(dy)) != r {
					continue
				}
				x, y := ga.SpotX+dx, ga.SpotY+dy
				if !g.h.Walkable(x, y) || g.h.BedCovering(x, y) {
					continue
				}
				if g.h.TileTaken(x, y) || !g.h.Reachable(id, x, y) {
					continue
				}
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// Do runs a gathering job: company and joy while it lasts.
func (g Gatherings) Do(id entity.EntityID, j job.Record, now uint64) {
	if g.Running() == nil {
		j.State = job.Complete
		g.h.SetJob(id, j)
		return
	}
	if now%recreation.GainPeriod == 0 {
		g.h.AddJoy(id, recreation.Gain(recreation.Chat))
	}
	g.h.SetJob(id, j)
}

// Tick advances the running gathering (once per tick, every Period ticks)
// and settles a finished or aborted one.
func (g Gatherings) Tick(now uint64) {
	ga := g.Running()
	if ga == nil {
		return
	}
	gs := g.h.State(false)
	if gs.ticked && gs.tickAt == now {
		return
	}
	gs.ticked, gs.tickAt = true, now
	if now%Period != 0 {
		return
	}
	prev := ga.Phase
	if !ga.Advance(now, g.hostsPresent(ga), g.abort(ga, now)) {
		return
	}
	h0, h1 := entity.EntityID(ga.Hosts[0]), entity.EntityID(ga.Hosts[1])
	switch {
	case ga.Kind == social.Wedding && prev == social.PhaseCeremony && ga.Phase == social.PhaseReception:
		if g.Marry(h0, h1, now) {
			for _, a := range g.attendees(ga) {
				g.h.Remember(a, mood.AttendedWedding, h0, now)
			}
		}
	case ga.Kind == social.Wedding && ga.Phase == social.PhaseCeremony:
		g.note(ga, fmt.Sprintf("The wedding ceremony of %s and %s has begun.", g.h.Name(h0), g.h.Name(h1)))
	case ga.Phase == social.PhaseOver:
		if ga.Kind == social.Party {
			for _, a := range append(g.attendees(ga), h0) {
				g.h.Remember(a, mood.AttendedParty, h0, now)
			}
			g.note(ga, "The party is over.")
		}
		gs.Active = gs.Active[1:]
	case ga.Phase == social.PhaseAborted:
		text := fmt.Sprintf("%s's party was called off.", g.h.Name(h0))
		if ga.Kind == social.Wedding {
			text = fmt.Sprintf("The wedding of %s and %s was called off.", g.h.Name(h0), g.h.Name(h1))
		}
		g.note(ga, text)
		gs.Retry = append(gs.Retry, Retry{Kind: ga.Kind, Hosts: ga.Hosts, At: ga.RetryAt()})
		gs.Active = gs.Active[1:]
	}
	if len(gs.Active) == 0 {
		gs.Active = nil
	}
}

// hostsPresent reports every host stands near the spot.
func (g Gatherings) hostsPresent(ga *social.Gathering) bool {
	for _, h := range ga.Hosts {
		if h == 0 {
			continue
		}
		x, y, ok := g.h.Pos(entity.EntityID(h))
		if !ok || max(mathx.Abs(x-ga.SpotX), mathx.Abs(y-ga.SpotY)) > hostNear {
			return false
		}
	}
	return true
}

// abort reports the gathering must end now: a threat, a host unfit, a
// couple no longer engaged, or hosts who never came.
func (g Gatherings) abort(ga *social.Gathering, now uint64) bool {
	if g.h.ThreatActive(now) {
		return true
	}
	for _, h := range ga.Hosts {
		if h != 0 && !g.ready(entity.EntityID(h)) {
			return true
		}
	}
	if ga.Kind == social.Wedding {
		k := g.h.Pair(entity.EntityID(ga.Hosts[0]), entity.EntityID(ga.Hosts[1])).Kind
		if k != social.PairFiance && !(k == social.PairSpouse && ga.Phase == social.PhaseReception) {
			return true
		}
	}
	return ga.Phase == social.PhaseCalling && now >= ga.PhaseAt+callingTimeout
}

// attendees are the guests at the spot (hosts excluded).
func (g Gatherings) attendees(ga *social.Gathering) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.h.PawnIDs() {
		if hostIndex(ga, id) >= 0 || !g.h.Alive(id) {
			continue
		}
		j := g.h.Job(id)
		x, y, ok := g.h.Pos(id)
		if j.Kind == job.Gather && j.State == job.Do && ok && max(mathx.Abs(x-ga.SpotX), mathx.Abs(y-ga.SpotY)) <= attendRadius {
			out = append(out, id)
		}
	}
	return out
}

// note posts a message about the gathering.
func (g Gatherings) note(ga *social.Gathering, text string) {
	g.h.Note(ga.SpotX, ga.SpotY, entity.EntityID(ga.Hosts[0]), "%s", text)
}

// retryBlocked reports a gathering of kind for hosts a, b waits after an
// abort (expired entries are dropped).
func (g Gatherings) retryBlocked(kind social.GatherKind, a, b entity.EntityID, now uint64) bool {
	gs := g.h.State(false)
	if gs == nil {
		return false
	}
	gs.Retry = slices.DeleteFunc(gs.Retry, func(r Retry) bool { return r.At <= now })
	if len(gs.Retry) == 0 {
		gs.Retry = nil
	}
	for _, r := range gs.Retry {
		if r.Kind == kind && (r.Hosts == [2]social.ID{social.ID(a), social.ID(b)} || r.Hosts == [2]social.ID{social.ID(b), social.ID(a)}) {
			return true
		}
	}
	return false
}

// WeddingCheck is id's hourly look at its engagements (as the lower id of
// the couple): a wedding starts when they are fit, nothing threatens and a
// spot is free.
func (g Gatherings) WeddingCheck(id entity.EntityID, now uint64) {
	if g.Running() != nil || !g.ready(id) {
		return
	}
	for _, o := range g.h.Partners(id) {
		if o < id {
			continue
		}
		p := g.h.Pair(id, o)
		if p.Kind != social.PairFiance || now < p.LastChange+weddingEngagedMin || !g.ready(o) {
			continue
		}
		if g.retryBlocked(social.Wedding, id, o, now) {
			continue
		}
		if g.h.ThreatActive(now) {
			return
		}
		x, y, ok := g.spot(true)
		if !ok {
			return
		}
		g.start(social.Gathering{Kind: social.Wedding, SpotX: x, SpotY: y,
			Hosts: [2]social.ID{social.ID(id), social.ID(o)}, Phase: social.PhaseCalling, PhaseAt: now})
		g.h.GoodLetter("Wedding", x, y, id, "%s and %s are getting married! The colony gathers for the wedding.", g.h.Name(id), g.h.Name(o))
		return
	}
}

func (g Gatherings) start(ga social.Gathering) {
	gs := g.h.State(true)
	gs.Active = append(gs.Active, ga)
}

// spot is where a gathering is held: for a wedding the marriage spot
// first; then a party spot, a campfire, and (weddings only) the colony's
// home.
func (g Gatherings) spot(wedding bool) (int, int, bool) {
	var marriage, party, fire *construction.Building
	for _, bid := range g.h.BuildingIDs() {
		b, ok := g.h.Building(bid)
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
		if !g.h.Walkable(x, y) {
			y = fire.Y - 1
		}
		return x, y, true
	}
	if wedding {
		x, y := g.h.Home()
		return x, y, true
	}
	return 0, 0, false
}

// Marry weds a and b at the end of their ceremony: spouses, the mood, a
// shared surname (sometimes), the letter and the history line. It does
// nothing for a couple already married, so it can never pay out twice.
func (g Gatherings) Marry(a, b entity.EntityID, now uint64) bool {
	p := g.h.Pair(a, b)
	if p.Kind == social.PairSpouse || !p.Transition(social.PairSpouse, now) {
		return false
	}
	g.h.SetPair(a, b, p)
	g.h.Remember(a, mood.GotMarried, b, now)
	g.h.Remember(b, mood.GotMarried, a, now)
	if g.h.Roll(2) == 0 {
		g.ShareSurname(a, b)
	}
	text := fmt.Sprintf("%s and %s are now married.", g.h.Name(a), g.h.Name(b))
	g.h.RomanceLetter("Married", text, a)
	g.h.History("wedding", text, a, b)
	return true
}

// ShareSurname has one spouse take the other's surname: the wife the
// husband's, else the higher id the lower's.
func (g Gatherings) ShareSurname(a, b entity.EntityID) {
	ba, okA := g.h.Bio(a)
	bb, okB := g.h.Bio(b)
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
	g.h.SetSurname(taker, from)
}

// PartyFits reports whether a party can be thrown now: none running, no
// threat, enough fit colonists in a decent mood, and a party spot or
// campfire.
func (g Gatherings) PartyFits(now uint64) bool {
	if g.Running() != nil || g.h.ThreatActive(now) {
		return false
	}
	ready := g.guests()
	if len(ready) < partyMinColonists {
		return false
	}
	sum := 0
	for _, id := range ready {
		sum += g.h.Mood(id)
	}
	if sum < partyMinMood*len(ready) {
		return false
	}
	_, _, ok := g.spot(false)
	return ok
}

// guests are the colonists fit for a party, in id order.
func (g Gatherings) guests() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.h.PawnIDs() {
		if g.h.Colonist(id) && g.ready(id) {
			out = append(out, id)
		}
	}
	return out
}

// ThrowParty starts a party gathering hosted by a fit colonist.
func (g Gatherings) ThrowParty(now uint64) bool {
	if !g.PartyFits(now) {
		return false
	}
	x, y, _ := g.spot(false)
	guests := g.guests()
	host := guests[g.h.Roll(len(guests))]
	g.start(social.Gathering{Kind: social.Party, SpotX: x, SpotY: y,
		Hosts: [2]social.ID{social.ID(host), 0}, Phase: social.PhaseCalling, PhaseAt: now})
	g.h.GoodLetter("Party", x, y, host, "%s is throwing a party! Everyone is welcome.", g.h.Name(host))
	return true
}

// Export is the gathering block of the Social save (nil: nothing).
func Export(s *State) *Save {
	if s == nil {
		return nil
	}
	out := &Save{Active: slices.Clone(s.Active), Retry: slices.Clone(s.Retry)}
	ids := make([]entity.EntityID, 0, len(s.LovinAt))
	for id := range s.LovinAt {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		out.Lovin = append(out.Lovin, Tick{id, s.LovinAt[id]})
	}
	if len(out.Active) == 0 && len(out.Retry) == 0 && len(out.Lovin) == 0 {
		return nil
	}
	return out
}

// Import restores Export's state into s.
func Import(s *State, in *Save) {
	s.Active = slices.Clone(in.Active)
	s.Retry = slices.Clone(in.Retry)
	for _, x := range in.Lovin {
		s.LovinAt[x.ID] = x.Tick
	}
}
