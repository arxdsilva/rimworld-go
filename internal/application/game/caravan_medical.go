package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Medicine on the road (WS-M). A colonist away on a caravan keeps its
// body: the patients index steps its wounds, bleeding and conditions
// hourly like anyone's, and outbreaks can strike it (diseaseCandidates).
// No doctor at home can reach it, so the caravan's best medic (the member
// with the highest Medicine skill who can doctor and stands) tends what is
// due on its fellow members and itself, once an hour on the road: the
// conditions once per tend window, each open wound once (body.TendAll).
// It works without a bed and without medicine (caravans carry none), at
// the self-tend factor on itself; a member whose care level is "No care"
// (or with self-tend off, for the medic) is left alone. Any medical death
// goes through dieMedical: off the map that is dieAway (world_state.go):
// no corpse, the caravan loses the member, a letter.

// dieMedical kills id of a medical cause: away on a caravan it dies there
// (dieAway), anyone else where it is (die).
func (g *Game) dieMedical(id entity.EntityID, cause health.Cause) {
	if g.isStashed(id) {
		dieAway(g, id, cause)
		return
	}
	g.die(id, cause)
}

// caravanMembers are the living colonists away on caravans, in id order.
func (g *Game) caravanMembers() []entity.EntityID {
	if g.worldSt == nil {
		return nil
	}
	var out []entity.EntityID
	for _, r := range g.worldSt.stashed { // sorted by id
		if fleetOf(g).At(r.Holder) == nil {
			continue
		}
		if v, ok := g.vital.Get(r.ID); ok && v.Status != health.Dead {
			out = append(out, r.ID)
		}
	}
	return out
}

// colonistsHomeAndAway are the colonists at home (homeColonists) and away
// on caravans, in id order. MAP-0: the caravans' members belong to the
// first map's pass (owns), so another map's outbreak strikes only its own.
func (g *Game) colonistsHomeAndAway() []entity.EntityID {
	out := g.homeColonists() // relative_world.go: alive, on the map
	if !primary(g) {
		return out
	}
	away := g.caravanMembers()
	if len(away) == 0 {
		return out
	}
	out = append(out, away...)
	slices.Sort(out)
	return slices.Compact(out)
}

// caravanName names the caravan id travels with ("the caravan to Oakhold";
// "" when it is on none).
func (g *Game) caravanName(id entity.EntityID) string {
	c := fleetOf(g).Of(id)
	if c == nil {
		return ""
	}
	if c.Back {
		return "the caravan returning home"
	}
	return "the caravan to " + g.tileName(c.C.Dest)
}

// awayNote is a letter's note on which victims are away on caravans
// (" Ada is away with the caravan to Oakhold." ; "" when none are).
func (g *Game) awayNote(victims []entity.EntityID) string {
	var parts []string
	for _, id := range victims {
		if n := g.caravanName(id); n != "" {
			parts = append(parts, g.personName(id)+" is away with "+n+"; only a medic among them can tend it.")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// MedicLevel is id's Medicine level when it can doctor on the road
// (alive, able to doctor, hands that work), else 0 (game/maps picks the
// caravan's medic).
func (h mapsRoad) MedicLevel(id entity.EntityID) int {
	g := h.g
	if !g.alive(id) || g.prison.Is(id) || g.incapableWork(id).Enabled(work.Doctor) || g.capacity(id, body.Manipulation) <= 0 {
		return 0
	}
	return int(g.skillOf(id, skill.Medicine).Level)
}

// TendDue reports member id alive with a tend due.
func (h mapsRoad) TendDue(id entity.EntityID, now uint64) bool {
	b := h.g.bodies[id]
	v, ok := h.g.vital.Get(id)
	return b != nil && b.TendDue(now) && ok && v.Status != health.Dead
}

// MayTend reports medic may tend id: its care level, or self-tend.
func (h mapsRoad) MayTend(medic, id entity.EntityID) bool {
	if id == medic {
		return h.g.care.SelfTendAllowed(id)
	}
	return h.g.care.Level(id).Tends()
}

// Tend has medic tend what is due on id on the road: no bed, no
// medicine, its manipulation and sight, x0.7 on itself.
func (h mapsRoad) Tend(medic, id entity.EntityID, now uint64) {
	g, b := h.g, h.g.bodies[id]
	v, _ := g.vital.Get(id)
	in := body.TendInput{Skill: int(g.skillOf(medic, skill.Medicine).Level), Manip: 1000, Sight: 1000,
		Medicine: noMedicine, Self: medic == id, Roll: g.stream(streamMedical).IntN}
	if mb := g.bodies[medic]; mb != nil {
		in.Manip, in.Sight = mb.Capacity(body.Manipulation), mb.Capacity(body.Sight)
	}
	if b.TendAll(body.TendQuality(in), now) > 0 {
		g.bodyChanged(id, b, &v) // body.go: bleeding stops, stats follow
		g.updateDowned(id, b, &v)
	}
}

// caravanMemberStatus is a member's short health note for the caravan
// list: "dead", "bleeding", "down", its diseases, "food poisoning",
// "injured", or "" when well.
func (g *Game) caravanMemberStatus(id entity.EntityID) string {
	v, ok := g.vital.Get(id)
	if !ok {
		return ""
	}
	if v.Status == health.Dead {
		return "dead"
	}
	var out []string
	b := g.bodies[id]
	if b != nil && b.BleedPerDay() > 0 {
		out = append(out, "bleeding")
	}
	if v.Status == health.Downed {
		out = append(out, "down")
	}
	if d := g.diseaseLabel(id); d != "" {
		out = append(out, d)
	}
	if _, ok := b.Poisoned(); ok {
		out = append(out, "food poisoning")
	}
	if _, ok := g.worstInfection(id); ok {
		out = append(out, "infection")
	}
	if len(out) == 0 && b != nil && hasOpenWound(b) {
		out = append(out, "injured")
	}
	return strings.Join(out, ", ")
}
