package game

import (
	"slices"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
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
	// dementiaWanderTicks is how long a confused wander lasts (original tuning).
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
	for _, id := range ownPatients(g) { // patients.go: this map's, a copy (a death touches the index)
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
			g.dieMedical(id, cause) // caravan_medical.go
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
	if !ok || v.Status == health.Dead || g.prison.Is(id) || g.isRaider(id) || g.visitor(id) {
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
		g.post(MessageView{Tick: now, X: pos.X, Y: pos.Y, Focus: id, Letter: true, Title: "Heart attack", Kind: letters.Threat,
			Msg: msgfmt.Of("%s is having a heart attack. Untreated it is usually fatal within hours: get a doctor to tend %s every hour, each tend may ease it. A pump or valve heart prevents heart attacks.",
				g.personName(id), g.personName(id))})
	case body.CondCarcinoma:
		g.post(MessageView{Tick: now, X: pos.X, Y: pos.Y, Focus: id, Letter: true, Title: "Carcinoma", Kind: letters.Caution,
			Msg: msgfmt.Of("%s has a carcinoma in the %s. If it grows to full size it destroys the %s. A doctor with Medicine 10 can excise it with 4 industrial medicine or better; a failed excision kills one time in four.",
				g.personName(id), part.Label(), part.Label())})
	default:
		g.message(pos.X, pos.Y, "%s has developed %s.", g.name(id), body.AgingOnsetText(k, part))
	}
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
