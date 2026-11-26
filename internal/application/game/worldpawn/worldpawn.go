// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package worldpawn ticks the pawns that are stashed off the map (in a
// transport pod, a casket or a caravan's hold) and are not a caravan's own
// walkers: health, needs, auto-tend and the mothball cadence. The colony
// owns the pawn state; Host is the colony the pass reads and writes.
package worldpawn

import (
	"slices"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	tendInterval     = uint64(7500)
	mothballInterval = uint64(15000)
	tendSkill        = 0
)

// Mothballed is the authoritative runtime set of pawns on the lightweight
// cadence. Save/restore preserves membership between interval boundaries.
type Mothballed map[entity.EntityID]bool

// Host is the colony the pass ticks.
type Host interface {
	Stash
	Records
	Medicine
	Care
	Recovery
	Needs
}

// Stash is who is off the map and how.
type Stash interface {
	// Stashed lists every stashed id in ascending order; Owned reports the
	// ones this pass ticks (not beasts, caravan members or escapees).
	Stashed() []entity.EntityID
	Owned(id entity.EntityID) bool
	Pod(id entity.EntityID) bool
	Asleep(id entity.EntityID) bool
	// Prepare readies the travel state a direct caller may not have built.
	Prepare()
}

// Records are the pawn's stored state.
type Records interface {
	Vital(id entity.EntityID) (health.Record, bool)
	Body(id entity.EntityID) *body.Body
	EnsureBody(id entity.EntityID, v *health.Record) *body.Body
	Need(id entity.EntityID) (needs.Pair, bool)
	SetNeed(id entity.EntityID, n needs.Pair)
	Bio(id entity.EntityID) (pawngen.Pawn, bool)
	SetBio(id entity.EntityID, p pawngen.Pawn)
	RebuildStats(id entity.EntityID)
}

// Medicine is the medical rules and random streams.
type Medicine interface {
	BodyRoll(n int) int
	MedicalRoll(n int) int
	NoMedicine() item.Kind
	NeedsCare(b *body.Body) bool
	HasDisease(b *body.Body) bool
	Poisoned(id entity.EntityID) bool
	LogScar(id entity.EntityID, part string)
}

// Care is what happens to the body.
type Care interface {
	DieMedical(id entity.EntityID, cause health.Cause)
	ClearPatient(id entity.EntityID)
	BodyChanged(id entity.EntityID, b *body.Body, v *health.Record)
	UpdateDowned(id entity.EntityID, b *body.Body, v *health.Record)
	RollInfections(id entity.EntityID, b *body.Body, now uint64)
	StepConds(id entity.EntityID, b *body.Body, now uint64) bool
	StepDisease(id entity.EntityID, b *body.Body, v *health.Record, now uint64)
	StepFoodPoison(id entity.EntityID, b *body.Body, v *health.Record, now uint64)
}

// Recovery is ageing, healing and drugs.
type Recovery interface {
	StepAging(id entity.EntityID, now uint64)
	DevelopAging(id entity.EntityID, k body.CondKind, part int, now uint64)
	Heal(id entity.EntityID, v *health.Record, now uint64)
	WakeFromAnaesthetic(id entity.EntityID, v *health.Record, now uint64)
	TickDrugs(id entity.EntityID, now uint64)
}

// Needs are the hourly needs and skill passes.
type Needs interface {
	DownedHunger(id entity.EntityID, now uint64)
	DecayNeeds(id entity.EntityID, n needs.Pair, now uint64) needs.Pair
	DrainJoy(id entity.EntityID, now uint64)
	SkillsDaily(id entity.EntityID, now uint64)
	Down(id entity.EntityID, cause health.Cause)
}

// Step owns ticking for every non-caravan pawn that is currently stashed
// off the map. Caravan members remain on the caravan path. The stashed set
// is canonical; transport/casket state only changes how the same pawn is
// treated.
func Step(h Host, m *Mothballed, now uint64) {
	if len(h.Stashed()) == 0 {
		clear(*m)
		return
	}
	h.Prepare()
	ids := tickIDs(h)
	if len(ids) == 0 {
		clear(*m)
		return
	}

	prepareMothballs(h, m, ids)
	mothballTick := now%mothballInterval == 0
	if mothballTick {
		for _, id := range ids {
			if (*m)[id] {
				stepMothballed(h, id, now)
			}
		}
	}

	medicalDue := cadence.Due(now, clock.TicksPerHour, cadence.OffMedical, 0)
	for _, id := range ids {
		if (*m)[id] {
			continue
		}

		stepTick(h, id, now, medicalDue)
	}

	if mothballTick {
		finishMothballs(h, m, ids)
	}
}

// live is a non-dead world pawn, including a downed pawn. The map-only
// alive helper intentionally is not used for this off-map owner.
func live(h Host, id entity.EntityID) bool {
	v, ok := h.Vital(id)
	return ok && v.Status != health.Dead
}

// Candidate reports the non-caravan off-map pawns owned by this pass.
// Prisoners are deliberately included (original design): every living
// off-map pawn the colony owns is ticked and tended. Animals, caravan members, and pawns
// that have left the planet are separate owners/terminal states.
func Candidate(h Host, id entity.EntityID) bool {
	return live(h, id) && h.Owned(id)
}

func tickIDs(h Host) []entity.EntityID {
	stashed := h.Stashed()
	ids := make([]entity.EntityID, 0, len(stashed))
	for _, id := range stashed {
		if Candidate(h, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// prepareMothballs removes obsolete holders. A newly stashed pawn stays
// active until the next interval; restored membership is kept until then.
// Older saves without this field start with no mothballed pawns.
func prepareMothballs(h Host, m *Mothballed, ids []entity.EntityID) {
	if *m == nil {
		*m = make(Mothballed)
	}
	alive := make(map[entity.EntityID]struct{}, len(ids))
	for _, id := range ids {
		alive[id] = struct{}{}
		if h.Pod(id) {
			delete(*m, id)
		}
	}
	for id := range *m {
		if _, ok := alive[id]; !ok {
			delete(*m, id)
		}
	}
}

// finishMothballs keeps the interval boundary: existing mothballed pawns
// have already received their lightweight interval tick; active safe pawns
// are moved into the mothballed set afterward.
func finishMothballs(h Host, m *Mothballed, ids []entity.EntityID) {
	for _, id := range ids {
		if !Candidate(h, id) || h.Pod(id) {
			delete(*m, id)
			continue
		}
		if ShouldMothball(h, id) {
			(*m)[id] = true
		} else {
			delete(*m, id)
		}
	}
}

// ShouldMothball reports that an off-map pawn's body will not change while it
// is away, so it can move to the slow cadence (original design): nothing
// bleeds, no wound is still healing, no replacement part is damaged, and only
// chronic conditions remain.
func ShouldMothball(h Host, id entity.EntityID) bool {
	if !Candidate(h, id) || h.Pod(id) {
		return false
	}
	b := h.Body(id)
	if b == nil {
		return true
	}
	if b.BloodLoss > 0 || b.BleedPerDay() > 0 || body.HasDamagedReplacement(b) {
		return false
	}
	for _, w := range b.Wounds {
		if w.Dmg > 0 && !w.Permanent {
			return false
		}
	}
	for _, c := range b.Conds {
		if c.Sev <= 0 {
			continue
		}
		if _, chronic := body.ChronicOf(c.Kind); chronic {
			continue
		}
		return false
	}
	return true
}

func stepTick(h Host, id entity.EntityID, now uint64, medicalDue bool) {
	if !live(h, id) {
		return
	}
	if medicalDue && !h.Asleep(id) {
		stepHealth(h, id, now)
		if !live(h, id) {
			return
		}
	}
	if h.Asleep(id) {
		return
	}
	stepNeeds(h, id, now)
	if !live(h, id) {
		return
	}
	if cadence.Due(now, tendInterval, 0, id) && !h.Pod(id) {
		stepTend(h, id, now)
	}
}

func dead(v health.Record, ok bool) bool { return !ok || v.Status == health.Dead }

func stepHealth(h Host, id entity.EntityID, now uint64) {
	if h.Asleep(id) {
		return
	}
	v, ok := h.Vital(id)
	b := h.Body(id)
	if dead(v, ok) || b == nil {
		return
	}

	if b.BleedPerDay() != 0 || b.BloodLoss != 0 || b.BloodStage != 0 {
		b.StepBlood(1)
		if b.BloodLoss >= body.BloodLossDeath {
			h.DieMedical(id, health.CauseBloodLoss)
			h.ClearPatient(id)
			return
		}
		h.BodyChanged(id, b, &v)
		h.UpdateDowned(id, b, &v)
		if v, ok = h.Vital(id); dead(v, ok) {
			return
		}
	}

	if h.NeedsCare(b) {
		h.RollInfections(id, b, now)
		if h.StepConds(id, b, now) {
			return
		}
		before := len(b.Wounds)
		scarred := b.HealHour(h.BodyRoll, 0)
		for _, p := range scarred {
			h.LogScar(id, p.Label())
		}
		if len(scarred) > 0 || len(b.Wounds) != before || b.Damage() != v.Injury {
			h.BodyChanged(id, b, &v)
		}
		h.UpdateDowned(id, b, &v)
	}

	b = h.Body(id)
	if b == nil {
		return
	}
	v, ok = h.Vital(id)
	if dead(v, ok) {
		return
	}
	if h.HasDisease(b) || h.Poisoned(id) {
		if h.HasDisease(b) {
			h.StepDisease(id, b, &v, now)
			if v, ok = h.Vital(id); dead(v, ok) {
				return
			}
		}
		if h.Poisoned(id) {
			h.StepFoodPoison(id, b, &v, now)
			if v, ok = h.Vital(id); dead(v, ok) {
				return
			}
		}
	}

	if body.ToxicBuildup(b) > 0 {
		st := body.StepToxic(b, false, 0, now/clock.TicksPerHour, h.MedicalRoll)
		if st.Dead {
			h.DieMedical(id, health.CauseDisease)
			h.ClearPatient(id)
			return
		}
		if st.Changed {
			h.BodyChanged(id, b, &v)
			h.UpdateDowned(id, b, &v)
		}
		if st.Dementia {
			h.DevelopAging(id, body.CondDementia, int(body.Brain), now)
		}
		if st.Carcinoma {
			h.DevelopAging(id, body.CondCarcinoma, -1, now)
		}
	}

	if n, ok := h.Need(id); ok && (n.Starving() || body.Malnutrition(b) > 0) {
		v, _ = h.Vital(id)
		b = h.EnsureBody(id, &v)
		if b != nil {
			changed, died := body.StepMalnutrition(b, n.Starving(), now/clock.TicksPerHour, h.MedicalRoll)
			if died {
				h.DieMedical(id, health.CauseNoFood)
				h.ClearPatient(id)
				return
			}
			if changed {
				h.BodyChanged(id, b, &v)
				h.UpdateDowned(id, b, &v)
			}
		}
	}

	if b = h.Body(id); b != nil && b.OrganFailure() {
		h.DieMedical(id, health.CauseOrganFailure)
		h.ClearPatient(id)
		return
	}

	if h.Body(id) != nil {
		h.StepAging(id, now)
	}
}

// stepNeeds is the off-map counterpart of phaseNeeds. Map-only
// comfort/beauty sampling is intentionally skipped; hunger/rest/recreation,
// overdose, downed hunger, and the daily age/skill hook still advance.
func stepNeeds(h Host, id entity.EntityID, now uint64) {
	v, ok := h.Vital(id)
	if dead(v, ok) {
		return
	}
	h.WakeFromAnaesthetic(id, &v, now)
	h.Heal(id, &v, now)
	h.TickDrugs(id, now)
	if v, _ = h.Vital(id); v.Status == health.Dead {
		return
	}
	if v.Status == health.Downed {
		h.DownedHunger(id, now)
		h.SkillsDaily(id, now)
		return
	}

	n, _ := h.Need(id)
	n = h.DecayNeeds(id, n, now)
	h.SetNeed(id, n)
	h.DrainJoy(id, now)
	h.SkillsDaily(id, now)
	if n.Exhausted() {
		h.Down(id, health.CauseNoBed)
	}
}

// stepTend is the vanilla doctorless world-pawn auto-tend. Caravan members
// and traveling transport-pod pawns are excluded by the owner before this
// function is reached.
func stepTend(h Host, id entity.EntityID, now uint64) {
	b := h.Body(id)
	v, ok := h.Vital(id)
	if b == nil || dead(v, ok) || !b.TendDue(now) {
		return
	}
	in := body.TendInput{
		Skill: tendSkill, Manip: 1000, Sight: 1000,
		Medicine: h.NoMedicine(), Roll: h.MedicalRoll,
	}
	if b.TendAll(body.TendQuality(in), now) > 0 {
		h.BodyChanged(id, b, &v)
		h.UpdateDowned(id, b, &v)
	}
}

// stepMothballed is the core's lightweight equivalent of
// TickMothballed(15000): the current save model stores whole days for age,
// so only the deterministic daily age bucket is advanced here. Health,
// needs, tend and chronic progression remain frozen.
func stepMothballed(h Host, id entity.EntityID, now uint64) {
	if !live(h, id) || h.Asleep(id) {
		return
	}
	p, ok := h.Bio(id)
	if !ok {
		return
	}
	day := uint64(clock.TicksPerDay)
	if (now+day-uint64(id%24)*clock.TicksPerHour)%day != 0 {
		return
	}
	if c, ok := h.(interface{ Calendar() clock.CalendarPolicy }); ok && !c.Calendar().BiologicalDayDue(now) {
		return
	}
	p.AgeDays++
	h.SetBio(id, p)
	if p.AgeDays%pawngen.DaysPerYear == 0 {
		h.RebuildStats(id)
	}
}

// Export is the sorted authoritative membership at this checkpoint.
func (m Mothballed) Export() []entity.EntityID {
	var ids []entity.EntityID
	for id, active := range m {
		if active {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

func ImportMothballed(ids []entity.EntityID) Mothballed {
	if len(ids) == 0 {
		return nil
	}
	m := make(Mothballed, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
