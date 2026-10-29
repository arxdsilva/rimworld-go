package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Guests in the colony's care (SIM-063). A visitor, trader, ally or
// faction-less stranger that goes down is rescued like a colonist
// (Doctor.rescueEligible asks Rescuable) to a free non-prison bed and tended
// under the guest care group (the host's CareGroup). Once laid in a bed it
// stays there while it heals; its host loop calls Step every tick it is up.
//
// The lifecycle has two thresholds far apart, so a guest never flaps
// between them:
//   - rescue starts only when it is down, out of bed;
//   - it leaves only when it stands and no longer needs bed rest (the
//     colonist's restOver: healing injury at most RestWellInjury, nothing to
//     tend, not sick, no operation due).
//
// Between the two it lies in its bed. Leaving gives the bed back at once,
// so a guest walking home is never "in care" again; only going down again
// makes it rescuable.
//
// On recovery a faction's guest raises its faction's goodwill by
// GoodwillHealed and leaves (Guests.Leave). A stranger (FactionOf 0) is
// handed to Guests.JoinOffer instead: the host rolls whether it asks to
// join (its RNG, not care's) and leaves it itself when it does not ask. A
// guest that dies while down on the map costs GoodwillDiedDowned (Died).
const (
	// GoodwillHealed: a downed faction member recovers and leaves (wiki:
	// Guest, +12; ENG-003).
	GoodwillHealed = 12
	// GoodwillDiedDowned: a guest dies down on the map. The wiki publishes
	// no number; the published "non-hostile member killed" value, -5, is
	// used (gap analysis §4 decision; wiki: Goodwill).
	GoodwillDiedDowned = -5
	// JoinAskSafe and JoinAskUnsafe are the permille chance a recovered
	// stranger asks to join: 0.5 when leaving is safe, 1.0 when not (wiki:
	// Guest, tagged Verify; ENG-003). The host decides "safe" and rolls.
	JoinAskSafe   = 500
	JoinAskUnsafe = 1000
	// ReasonHealed and ReasonDiedInCare are the goodwill event texts.
	ReasonHealed     = "rescued and healed"
	ReasonDiedInCare = "member died in your care"
)

// Hospitality is the guest half of care: who of the guests is rescued,
// keeping a rescued guest in bed while it heals, and its recovery or death.
// It keeps no state of its own: "in care" is "lies in a bed".
type Hospitality struct{ c *Care }

// NewHospitality is c's guest component.
func newHospitality(c *Care) *Hospitality { return &Hospitality{c: c} }

// Rescuable: guest pid is one the colony takes in when down (the caller
// checks down and out of bed): a non-hostile humanlike on the map that is
// neither colonist nor prisoner. One already leaving qualifies again only
// by going down, which the caller requires anyway.
func (g *Hospitality) Rescuable(pid entity.EntityID) bool {
	return g.c.gu != nil && g.c.gu.IsGuest(pid)
}

// Step is guest id's care for one tick while it is up (the host's visitor
// loop): true while it stays in its bed to heal, so the loop leaves it
// lying; false when it is not in care (or just recovered and set off).
func (g *Hospitality) Step(id entity.EntityID) bool {
	c := g.c
	bid := c.h.BedUnder(id)
	if bid == 0 || !g.Rescuable(id) {
		return false
	}
	switch v, _ := c.h.Vital(id); v.Status {
	case health.Downed:
		return true
	case health.Dead:
		return false
	}
	if !c.Rest.restOver(id) {
		g.lieIn(id, bid)
		return true
	}
	g.recovered(id)
	return false
}

// lieIn keeps guest id lying in bed bid (a sleep job where it lies, the
// prisoner's way of resting in bed).
func (g *Hospitality) lieIn(id, bid entity.EntityID) {
	h := g.c.h
	if j, _ := h.Job(id); j.Kind == job.Sleep && j.Target == bid && j.Active() {
		return
	}
	x, y, _ := h.Pos(id)
	h.SetJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: x, TY: y, State: job.Do})
}

// recovered: guest id is well; it gives its bed back and leaves, raising
// its faction's goodwill, or, a stranger, may ask to join.
func (g *Hospitality) recovered(id entity.EntityID) {
	c := g.c
	c.h.ReleaseBed(id)
	c.Rest.Clear(id)
	if j, _ := c.h.Job(id); j.Kind == job.Sleep {
		c.h.SetJob(id, job.Record{})
	}
	f := c.gu.FactionOf(id)
	if f == 0 {
		c.gu.JoinOffer(id)
		return
	}
	c.gu.Goodwill(f, GoodwillHealed, ReasonHealed)
	c.gu.Leave(id)
}

// Died: guest id died while down on the map; its faction thinks the worse
// of the colony for it (a stranger has no faction to mind).
func (g *Hospitality) Died(id entity.EntityID) {
	c := g.c
	c.Rest.Clear(id)
	if c.gu == nil {
		return
	}
	if f := c.gu.FactionOf(id); f != 0 {
		c.gu.Goodwill(f, GoodwillDiedDowned, ReasonDiedInCare)
	}
}
