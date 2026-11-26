// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package aging is MED-08: deterministic daily onset and hourly condition
// progression of chronic ailments, heart attacks and dementia. The colony
// owns the pawns; Host is what the rules read and write.
package aging

import (
	"slices"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

const (
	// dementiaWanderDays is the mean days between a demented colonist's
	// confused wanders. wiki: Ailments (MTB 5 days).
	dementiaWanderDays = mentalbreak.DementiaWanderDays
	// dementiaWanderTicks is how long a confused wander lasts (original tuning).
	dementiaWanderTicks = mentalbreak.DementiaWanderTicks
)

// Host is the colony the ageing rules read and write.
type Host interface {
	Bodies
	Rolls
	Voice
	Mind
}

// Bodies are the pawns and their bodies.
type Bodies interface {
	Pawns() []entity.EntityID
	// Patients lists the pawns the medical pass owns, ascending.
	Patients() []entity.EntityID
	// Eligible is a living colonist with a bio (not a prisoner, raider or
	// visitor).
	Eligible(id entity.EntityID) bool
	Bio(id entity.EntityID) pawngen.Pawn
	Vital(id entity.EntityID) (health.Record, bool)
	Body(id entity.EntityID) *body.Body
	EnsureBody(id entity.EntityID, v *health.Record) *body.Body
}

// Rolls are the deterministic draws: Salt fixes a pawn's disease
// progression, Roll is the medical stream.
type Rolls interface {
	Salt(id entity.EntityID) uint64
	Roll(n int) int
}

// Voice is what the colony is told.
type Voice interface {
	Pos(id entity.EntityID) (x, y int)
	Name(id entity.EntityID) string
	PersonName(id entity.EntityID) string
	Message(x, y int, format string, args ...any)
	// Letter posts a letter about id (threat: a threat letter, else a caution).
	Letter(now uint64, x, y int, id entity.EntityID, title string, threat bool, msg msgfmt.Text)
}

// Mind is what an ailment does to a pawn's state.
type Mind interface {
	DieMedical(id entity.EntityID, cause health.Cause)
	ClearPatient(id entity.EntityID)
	BodyChanged(id entity.EntityID, b *body.Body, v *health.Record)
	UpdateDowned(id entity.EntityID, b *body.Body, v *health.Record)
	Memories(id entity.EntityID) mood.Memories
	SetMemories(id entity.EntityID, m mood.Memories)
	Drafted(id entity.EntityID) bool
	// Wander cancels id's job and orders and sends it on a sad wander
	// until the given tick (ok false: it is breaking or drafted).
	Wander(id entity.EntityID, kind mentalbreak.Kind, until uint64) (ok bool)
}

// Step runs chronic disease progression and onset.
func Step(h Host, now uint64, policy ...clock.CalendarPolicy) {
	StepIDs(h, now, h.Patients())
	if clock.Policy(policy...).BiologicalDayDue(now) && cadence.Due(now, clock.TicksPerDay, cadence.OffMedical, 0) {
		RollDaily(h, now, policy...)
	}
}

// StepIDs is the shared per-pawn chronic progression hook. World pawns use
// it from the world-pawn owner so the medical pass has one owner for each
// off-map pawn rather than a second hidden sweep.
func StepIDs(h Host, now uint64, ids []entity.EntityID) {
	for _, id := range ids {
		b := h.Body(id)
		if b == nil || !b.HasAging() {
			continue
		}
		v, ok := h.Vital(id)
		if !ok || v.Status == health.Dead {
			continue
		}
		st := b.StepAging(now, h.Salt(id), h.Roll)
		x, y := h.Pos(id)
		if st.Died != body.CondNone {
			cause := health.CauseHeartAttack
			switch st.Died {
			case body.CondCarcinoma:
				cause = health.CauseCarcinoma
				h.Message(x, y, "%s died of a carcinoma in the %s.", h.Name(id), st.DiedOn.Label())
			case body.CondArteryBlockage:
				h.Message(x, y, "%s's blocked arteries stopped the heart.", h.Name(id))
			default:
				h.Message(x, y, "%s died of a heart attack.", h.Name(id))
			}
			h.DieMedical(id, cause)
			h.ClearPatient(id)
			continue
		}
		for _, k := range st.Ended {
			if k == body.CondHeartAttack {
				h.Message(x, y, "%s has come through the heart attack.", h.Name(id))
			} else {
				h.Message(x, y, "%s's carcinoma has faded away.", h.Name(id))
			}
		}
		for _, p := range st.Lost {
			h.Message(x, y, "A carcinoma destroyed %s's %s.", h.Name(id), p.Label())
		}
		if st.Changed {
			h.BodyChanged(id, b, &v)
			h.UpdateDowned(id, b, &v)
		}
		if c, ok := b.Cond(body.CondAlzheimers); ok {
			wander, forget := body.AlzheimerEvents(c.Stage, h.Roll)
			if wander {
				ConfusedWander(h, id, now, "alzheimers")
			}
			if forget {
				h.SetMemories(id, mood.ForgetOne(h.Memories(id), now, h.Roll))
			}
		}
	}
}

// RollDaily performs deterministic daily onset checks.
func RollDaily(h Host, now uint64, policy ...clock.CalendarPolicy) {
	ids := slices.Clone(h.Pawns())
	slices.Sort(ids)
	for _, id := range ids {
		if !h.Eligible(id) {
			continue
		}
		age := h.Bio(id).AgeYears()
		for _, ch := range body.Chronics() {
			// Asthma is checked by the actual biological birthday owner.
			if ch.Kind == body.CondAsthma {
				continue
			}
			chance := body.OnsetPerDay(ch.Kind, age)
			if chance <= 0 {
				continue
			}
			if ch.Parts == nil { // a carcinoma: one roll, then a part by coverage
				if int32(h.Roll(body.OnsetScale)) < chance {
					Develop(h, id, ch.Kind, -1, now)
				}
				continue
			}
			for _, p := range ch.Parts {
				if !h.Body(id).CanDevelop(ch.Kind, p) {
					continue
				}
				if int32(h.Roll(body.OnsetScale)) < chance {
					Develop(h, id, ch.Kind, int(p), now)
				}
			}
		}
		if b := h.Body(id); b != nil {
			if mtb := b.HeartAttackMTB(); mtb > 0 && h.Roll(int(mtb)) == 0 {
				Develop(h, id, body.CondHeartAttack, int(body.Heart), now)
			}
			if _, dem := b.Cond(body.CondDementia); dem && h.Roll(dementiaWanderDays) == 0 {
				ConfusedWander(h, id, now, "dementia")
			}
		}
	}
}

// Birthday checks asthma when the native age owner crosses a birthday.
func Birthday(h Host, id entity.EntityID, now uint64) {
	if !h.Eligible(id) || !body.AsthmaBirthday(int(h.Bio(id).AgeDays), 80*clock.DaysPerYear, h.Roll) {
		return
	}
	v, _ := h.Vital(id)
	b := h.EnsureBody(id, &v)
	if b != nil && body.DevelopAsthma(b, now) {
		h.BodyChanged(id, b, &v)
		h.UpdateDowned(id, b, &v)
		x, y := h.Pos(id)
		h.Message(x, y, "%s has developed asthma.", h.Name(id))
	}
}

// Develop records an ailment and reports its onset.
func Develop(h Host, id entity.EntityID, k body.CondKind, p int, now uint64) {
	v, _ := h.Vital(id)
	b := h.EnsureBody(id, &v)
	if b == nil {
		return
	}
	part := body.Part(p)
	if p < 0 {
		var ok bool
		if part, ok = b.PickPart(h.Roll); !ok {
			return
		}
	}
	if !b.Develop(k, part, now) {
		return
	}
	if k == body.CondCarcinoma && h.Roll(100) < 30 {
		c, _ := b.Cond(body.CondCarcinoma)
		c.Rate = -1 // wiki: Carcinoma; 30% are painless, persisted in Rate
	}
	h.BodyChanged(id, b, &v)
	h.UpdateDowned(id, b, &v)
	x, y := h.Pos(id)
	switch k {
	case body.CondHeartAttack:
		h.Letter(now, x, y, id, "Heart attack", true,
			msgfmt.Of("%s is having a heart attack. Untreated it is usually fatal within hours: get a doctor to tend %s every hour, each tend may ease it. A pump or valve heart prevents heart attacks.",
				h.PersonName(id), h.PersonName(id)))
	case body.CondCarcinoma:
		h.Letter(now, x, y, id, "Carcinoma", false,
			msgfmt.Of("%s has a carcinoma in the %s. If it grows to full size it destroys the %s. A doctor with Medicine 10 can excise it with 4 industrial medicine or better; a failed excision kills one time in four.",
				h.PersonName(id), part.Label(), part.Label()))
	default:
		h.Message(x, y, "%s has developed %s.", h.Name(id), body.AgingOnsetText(k, part))
	}
}

// ConfusedWander sends id on a sad wander, cancelling its job and orders.
func ConfusedWander(h Host, id entity.EntityID, now uint64, reason string) {
	if v, ok := h.Vital(id); !ok || v.Status != health.Alive {
		return
	}
	if h.Drafted(id) || !h.Wander(id, mentalbreak.SadWander, now+dementiaWanderTicks) {
		return
	}
	x, y := h.Pos(id)
	h.Message(x, y, "%s is wandering around, confused (%s).", h.Name(id), reason)
}
