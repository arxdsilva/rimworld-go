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
		g.dieAway(id, cause)
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
		if g.caravanAt(r.Holder) == nil {
			continue
		}
		if v, ok := g.vital.Get(r.ID); ok && v.Status != health.Dead {
			out = append(out, r.ID)
		}
	}
	return out
}

// colonistsHomeAndAway are the colonists at home (homeColonists) and away
// on caravans, in id order.
func (g *Game) colonistsHomeAndAway() []entity.EntityID {
	out := g.homeColonists() // relative_world.go: alive, on the map
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
	c := g.caravanOf(id)
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

// caravanMedic is c's best medic: a standing member able to doctor with
// the highest Medicine level (at least 1; ties to the lowest id); 0 when
// there is none.
func (g *Game) caravanMedic(c *caravanRec) entity.EntityID {
	var best entity.EntityID
	bestLvl := 0
	for _, m := range c.C.Members {
		id := entity.EntityID(m)
		if !g.alive(id) || g.incapableWork(id).Enabled(work.Doctor) || g.capacity(id, body.Manipulation) <= 0 {
			continue
		}
		lvl := int(g.skillOf(id, skill.Medicine).Level)
		if lvl < 1 {
			continue
		}
		if lvl > bestLvl || lvl == bestLvl && id < best {
			best, bestLvl = id, lvl
		}
	}
	return best
}

// caravanTendQuality rolls medic's tend on patient on the road: no bed,
// no medicine, its manipulation and sight, ×0.7 on itself.
func (g *Game) caravanTendQuality(medic, patient entity.EntityID) int32 {
	in := body.TendInput{Skill: int(g.skillOf(medic, skill.Medicine).Level), Manip: 1000, Sight: 1000,
		Medicine: noMedicine, Self: medic == patient, Roll: g.stream(streamMedical).IntN}
	if b := g.bodies[medic]; b != nil {
		in.Manip, in.Sight = b.Capacity(body.Manipulation), b.Capacity(body.Sight)
	}
	return body.TendQuality(in)
}

// caravanCare has c's medic tend whatever is due on its members, in
// member order (hourly on the road, stepCaravans).
func (g *Game) caravanCare(c *caravanRec, now uint64) {
	medic := g.caravanMedic(c)
	if medic == 0 {
		return
	}
	for _, m := range slices.Clone(c.C.Members) {
		id := entity.EntityID(m)
		b := g.bodies[id]
		if b == nil || !b.TendDue(now) {
			continue
		}
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead {
			continue
		}
		if id == medic && !g.selfTendAllowed(id) || id != medic && !g.careOf(id).Tends() {
			continue
		}
		if b.TendAll(g.caravanTendQuality(medic, id), now) > 0 {
			g.bodyChanged(id, b, &v) // body.go: bleeding stops, stats follow
			g.updateDowned(id, b, &v)
		}
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
