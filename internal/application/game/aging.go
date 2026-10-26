package game

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Aging (MED-08). Once a day at 00:00 every living colonist rolls, on the
// medical stream and in id order, for each chronic condition its age
// allows (body.OnsetPerDay: none before the wiki onset age, then a chance
// rising with age): bad back, cataract and hearing loss (per eye, per
// ear), frailty, dementia, artery blockage and carcinoma (on a part picked
// by coverage). The same roll brings on a heart attack from an artery
// blockage (its stage's mean time between attacks) and sends a demented
// colonist wandering off confused about once in five days.
//
// Each hour, in the medical phase after the diseases, the patients with
// an aging condition step (body.StepAging): artery blockage and
// carcinomas grow, a heart attack rises and falls and is eased by tends,
// a replaced part takes its conditions along. Artery blockage and heart
// attacks kill with health.CauseHeartAttack; a carcinoma destroys its
// part, killing with health.CauseCarcinoma when the part is vital. A
// carcinoma is excised by surgery (body.ExciseCarcinoma).

const (
	// dementiaWanderDays is the mean days between a demented colonist's
	// confused wanders. wiki: Ailments (MTB 5 days).
	dementiaWanderDays = 5
	// dementiaWanderTicks is how long a confused wander lasts (original
	// tuning).
	dementiaWanderTicks = 4 * clock.TicksPerHour
)

// stepAging is the medical phase's aging work at tick now (on the hour):
// the hourly step, and at 00:00 the daily rolls.
func (g *Game) stepAging(now uint64) {
	g.stepAgingConds(now)
	if now > 0 && now%clock.TicksPerDay == 0 {
		g.rollAging(now)
	}
}

// agingSalt is pawn id's salt for its conditions' growth rates
// (body.AgingRate): derived from the game seed, so nothing is saved.
func (g *Game) agingSalt(id entity.EntityID) uint64 {
	return g.streams.seed ^ uint64(id)*0x9e3779b97f4a7c15
}

// stepAgingConds advances an hour of aging on every patient with an
// aging condition, in id order.
func (g *Game) stepAgingConds(now uint64) {
	if len(g.med.patients) == 0 {
		return
	}
	for _, id := range slices.Clone(g.med.patients) { // a death touches the index
		b := g.bodies[id]
		if !b.HasAging() {
			continue
		}
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead {
			continue
		}
		st := b.StepAging(now, g.agingSalt(id), g.stream(streamMedical).IntN)
		pos, _ := g.positions.Get(id)
		if st.Died != body.CondNone {
			cause := health.CauseHeartAttack
			switch st.Died {
			case body.CondCarcinoma:
				cause = health.CauseCarcinoma
				g.message(pos.X, pos.Y, "%s died of a carcinoma in the %s.", g.name(id), st.DiedOn.Label())
			case body.CondArteryBlockage:
				g.message(pos.X, pos.Y, "%s's blocked arteries stopped the heart.", g.name(id))
			default:
				g.message(pos.X, pos.Y, "%s died of a heart attack.", g.name(id))
			}
			g.die(id, cause)
			g.clearPatient(id)
			continue
		}
		for _, k := range st.Ended {
			if k == body.CondHeartAttack {
				g.message(pos.X, pos.Y, "%s has come through the heart attack.", g.name(id))
			} else {
				g.message(pos.X, pos.Y, "%s's carcinoma has faded away.", g.name(id))
			}
		}
		for _, p := range st.Lost {
			g.message(pos.X, pos.Y, "A carcinoma destroyed %s's %s.", g.name(id), p.Label())
		}
		if st.Changed {
			g.bodyChanged(id, b, &v) // body.go: stats follow the capacities
			g.updateDowned(id, b, &v)
		}
	}
}

// agingCandidate reports id one of the colony's living colonists (no
// prisoner, raider or visitor) with an age.
func (g *Game) agingCandidate(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead || g.isPrisoner(id) || g.isRaider(id) || g.visitor(id) {
		return false
	}
	_, aged := g.sk.bios[id]
	return aged
}

// rollAging is the daily aging roll over the colonists, in id order.
func (g *Game) rollAging(now uint64) {
	ids := slices.Clone(g.pawnIDs)
	slices.Sort(ids)
	r := g.stream(streamMedical)
	for _, id := range ids {
		if !g.agingCandidate(id) {
			continue
		}
		age := g.sk.bios[id].AgeYears()
		for _, ch := range body.Chronics() {
			chance := body.OnsetPerDay(ch.Kind, age)
			if chance <= 0 {
				continue
			}
			if ch.Parts == nil { // a carcinoma: one roll, then a part by coverage
				if int32(r.IntN(body.OnsetScale)) < chance {
					g.developAging(id, ch.Kind, -1, now)
				}
				continue
			}
			for _, p := range ch.Parts {
				if !g.bodies[id].CanDevelop(ch.Kind, p) {
					continue
				}
				if int32(r.IntN(body.OnsetScale)) < chance {
					g.developAging(id, ch.Kind, int(p), now)
				}
			}
		}
		if b := g.bodies[id]; b != nil {
			if mtb := b.HeartAttackMTB(); mtb > 0 && r.IntN(int(mtb)) == 0 {
				g.developAging(id, body.CondHeartAttack, int(body.Heart), now)
			}
			if _, dem := b.Cond(body.CondDementia); dem && r.IntN(dementiaWanderDays) == 0 {
				g.dementiaWander(id, now)
			}
		}
	}
}

// developAging starts aging kind k on id's part p (p < 0: a part picked
// by coverage on the medical stream) and tells the player.
func (g *Game) developAging(id entity.EntityID, k body.CondKind, p int, now uint64) {
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	if b == nil {
		return
	}
	part := body.Part(p)
	if p < 0 {
		var ok bool
		if part, ok = b.PickPart(g.stream(streamMedical).IntN); !ok {
			return
		}
	}
	if !b.Develop(k, part, now) {
		return
	}
	g.bodyChanged(id, b, &v)
	g.updateDowned(id, b, &v)
	pos, _ := g.positions.Get(id)
	switch k {
	case body.CondHeartAttack:
		g.post(MessageView{Tick: now, X: pos.X, Y: pos.Y, Focus: id, Letter: true, Title: "Heart attack", Kind: letterThreat,
			Text: fmt.Sprintf("%s is having a heart attack. Untreated it is usually fatal within hours: get a doctor to tend %s every hour, each tend may ease it. A pump or valve heart prevents heart attacks.",
				g.personName(id), g.personName(id))})
	case body.CondCarcinoma:
		g.post(MessageView{Tick: now, X: pos.X, Y: pos.Y, Focus: id, Letter: true, Title: "Carcinoma", Kind: letterCaution,
			Text: fmt.Sprintf("%s has a carcinoma in the %s. If it grows to full size it destroys the %s. A doctor with Medicine 10 can excise it with 4 industrial medicine or better; a failed excision kills one time in four.",
				g.personName(id), part.Label(), part.Label())})
	default:
		g.message(pos.X, pos.Y, "%s has developed %s.", g.name(id), agingOnsetText(k, part))
	}
}

// agingOnsetText names a new chronic condition for the message ("a bad
// back", "a cataract in the left eye").
func agingOnsetText(k body.CondKind, p body.Part) string {
	switch k {
	case body.CondBadBack:
		return "a bad back"
	case body.CondCataract:
		return "a cataract in the " + p.Label()
	case body.CondFrail:
		return "frailty"
	case body.CondHearingLoss:
		return "hearing loss in the " + p.Label()
	case body.CondArteryBlockage:
		return "an artery blockage"
	}
	return body.DefOfCond(k).Label
}

// dementiaWander sends demented colonist id wandering off confused (the
// mental break's wander hook), unless it is down, drafted or already in
// a break.
func (g *Game) dementiaWander(id entity.EntityID, now uint64) {
	if v, ok := g.vital.Get(id); !ok || v.Status != health.Alive {
		return
	}
	if _, breaking := g.breaks[id]; breaking || g.draft[id] {
		return
	}
	g.interrupt(id, job.Cancelled)
	delete(g.orderQueue, id)
	g.breaks[id] = breakState{kind: mentalbreak.SadWander, until: now + dementiaWanderTicks}
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s is wandering around, confused (dementia).", g.name(id))
}

// --- Health tab -------------------------------------------------------

// agingLabel is an aging condition's row name: "Bad back", "Cataract
// (left eye)", "Artery blockage (major)", "Carcinoma (stomach), growing".
func (g *Game) agingLabel(id entity.EntityID, c body.Condition, d body.CondDef) string {
	l := strings.ToUpper(d.Label[:1]) + d.Label[1:]
	switch c.Kind {
	case body.CondCataract, body.CondHearingLoss:
		return l + " (" + c.Part.Label() + ")"
	case body.CondCarcinoma:
		return l + " (" + c.Part.Label() + "), " + body.CarcinomaGrowth(c, g.agingSalt(id)).String()
	case body.CondArteryBlockage, body.CondHeartAttack:
		if int(c.Stage) < len(d.Stages) {
			return l + " (" + d.Stages[c.Stage].Label + ")"
		}
	}
	return l
}

// agingTip is an aging condition's hover card: what it does and how it
// ends.
func (g *Game) agingTip(id entity.EntityID, c body.Condition, label string) string {
	switch c.Kind {
	case body.CondBadBack:
		return label + "\nMoving −30 %, manipulation −10 %. It comes with age and stays; a spinal brace in place of the spine ends it."
	case body.CondCataract:
		return label + "\nThe " + c.Part.Label() + " sees half as well (sight −25 %). It stays; replacing the eye (an optic implant) ends it."
	case body.CondFrail:
		return label + "\nMoving and manipulation −30 %. It comes with age and stays."
	case body.CondDementia:
		return label + "\nConsciousness −15 %, talking and hearing −25 %; now and then wanders off confused. It stays."
	case body.CondHearingLoss:
		return label + "\nThe " + c.Part.Label() + " hears half as well (hearing −25 %). It stays; a cochlear implant in its place ends it."
	case body.CondArteryBlockage:
		mtb := heartAttackMTBAt(c.Stage)
		return fmt.Sprintf("%s\nSeverity %d%%; it slowly grows and stops the heart at 100%%. A heart attack about once in %d days at this stage. A pump or valve heart, or a new heart, ends it.",
			label, int(c.Sev)/10, mtb)
	case body.CondCarcinoma:
		d := body.DefOfCond(c.Kind)
		pain := int32(0)
		if int(c.Stage) < len(d.Stages) {
			pain = d.Stages[c.Stage].PainAdd
		}
		t := fmt.Sprintf("%s\nSeverity %d%%, pain +%d%%. ", label, int(c.Sev)/10, pain/10)
		switch body.CarcinomaGrowth(c, g.agingSalt(id)) {
		case body.Growing:
			t += "Growing: at 100 % it destroys the " + c.Part.Label() + "."
		case body.Stable:
			t += "Stable for now."
		default:
			t += "In remission: shrinking, it will fade away."
		}
		return t + "\nExcise it: a doctor with Medicine 10 and 4 industrial medicine or better; a failed excision kills one time in four."
	case body.CondHeartAttack:
		return fmt.Sprintf("%s\nSeverity %d%%: at 60 %% the patient blacks out, at 100 %% the heart stops. Each tend (one an hour) has a 65 %% × quality chance to ease it by 30 %%; below 0 it is over.",
			label, int(c.Sev)/10)
	}
	return label
}

// heartAttackMTBAt is the mean days between heart attacks at artery
// blockage stage s (for the tooltip).
func heartAttackMTBAt(s uint8) int32 {
	b := body.Body{Conds: []body.Condition{{Kind: body.CondArteryBlockage, Part: body.Heart, Sev: 1, Stage: s}}}
	return b.HeartAttackMTB()
}

// agingHeadline is the Health tab's headline when the body's conditions
// are all aging ones or a heart attack is under way: the heart attack
// first, then a carcinoma and an artery blockage; the permanent kinds
// keep the plain state ("Healthy", "Injured").
func agingHeadline(b *body.Body, state string, now uint64) string {
	if c, ok := b.Cond(body.CondHeartAttack); ok {
		if now >= c.TendUntil {
			return "Heart attack: needs tending"
		}
		return "Heart attack: tended"
	}
	if state != "Healthy" {
		return state
	}
	for _, k := range []body.CondKind{body.CondCarcinoma, body.CondArteryBlockage} {
		if _, ok := b.Cond(k); ok {
			d := body.DefOfCond(k)
			return strings.ToUpper(d.Label[:1]) + d.Label[1:]
		}
	}
	return state
}

// agingHeads reports b's headline as agingHeadline's: a heart attack, or
// nothing but aging conditions.
func agingHeads(b *body.Body) bool {
	if _, ok := b.Cond(body.CondHeartAttack); ok {
		return true
	}
	for _, c := range b.Conds {
		if !body.IsAging(c.Kind) {
			return false
		}
	}
	return true
}
