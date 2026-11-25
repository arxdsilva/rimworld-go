// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
)

// An animal's medical body (MED-09, MED-13; moved here from the game's
// animal_health.go in TEMP-3): its anatomy scaled by its size, and its
// blood loss scaled the same way. RimWorld divides blood loss by body
// size; ours have none, so hit points against a human's (health.MaxHP)
// stand in.

// Animal health scale bounds (permille of a human's hit points): the
// smallest species bleeds at most 10x a human's rate, the largest at
// least a tenth of it (original tuning).
const (
	healthScaleMin = 100
	healthScaleMax = 10000
)

// HealthScale is species s's body size against a human's, from its
// hit points (permille).
func HealthScale(s Species) int32 {
	return min(max(int32(DefOf(s).MaxHP)*1000/health.MaxHP, healthScaleMin), healthScaleMax)
}

// BleedPerDay is how fast an animal of species s with body b bleeds
// (permille of its blood per day): the body's bleeding scaled by its size.
func BleedPerDay(b *body.Body, s Species) int32 {
	if b == nil {
		return 0
	}
	n := b.BleedPerDay()
	if n <= 0 {
		return 0
	}
	return max(n*1000/HealthScale(s), 1)
}

// hourShare is a daily permille rate's share of one hour, rounded to
// nearest, never below 1 for a positive rate (body.StepBlood's rounding).
func hourShare(perDay int32) int32 {
	if perDay <= 0 {
		return 0
	}
	return max((perDay+12)/24, 1)
}

// StepBlood advances an hour of blood loss on body b of species s:
// the scaled bleeding, or the regeneration while nothing bleeds; the
// stage follows with its hysteresis (body.BloodStage).
func StepBlood(b *body.Body, s Species) {
	if bleed := BleedPerDay(b, s); bleed > 0 {
		b.BloodLoss = min(b.BloodLoss+hourShare(bleed), body.BloodLossDeath)
	} else if b.BloodLoss > 0 {
		b.BloodLoss = max(b.BloodLoss-hourShare(body.BloodRegenPerDay), 0)
	}
	b.BloodStage = body.BloodStage(b.BloodLoss, b.BloodStage)
}

// BleedDeathHours is how many hours an animal of species s with
// body b has before bleeding out at its current rate (-1: not bleeding).
func BleedDeathHours(b *body.Body, s Species) int {
	bleed := BleedPerDay(b, s)
	if bleed <= 0 {
		return -1
	}
	left := max(body.BloodLossDeath-b.BloodLoss, 0)
	h := hourShare(bleed)
	return int((left + h - 1) / h)
}

// NewBody is a whole body of species s: its anatomy, its parts
// scaled to its hit points (MED-13).
func NewBody(s Species) *body.Body {
	return &body.Body{Anatomy: AnatomyOf(s), Scale: HealthScale(s)}
}

// ReconcileBody bounds open wounds by scalar injury and drops fully
// healed bodies unless a part is permanently missing.
func ReconcileBody(a *State) {
	b := a.Body
	if b == nil {
		return
	}
	if d := body.OpenWoundDamage(b); d > a.Injury {
		body.TrimOpen(b, d-max(a.Injury, 0))
	}
	if !body.NeedsCare(b) && b.Missing == 0 {
		a.Body = nil
	}
}

// MigrateBody upgrades a pre-anatomy live body's virtual part into the
// species anatomy while preserving wounds and conditions.
func MigrateBody(a *Live) {
	old := a.Body
	nb := NewBody(a.Species)
	root := nb.Root()
	nb.Wounds, nb.Conds = old.Wounds, old.Conds
	nb.BloodLoss, nb.BloodStage = old.BloodLoss, old.BloodStage
	for i := range nb.Wounds {
		nb.Wounds[i].Part = root
	}
	for i := range nb.Conds {
		nb.Conds[i].Part = root
	}
	a.Body = nb
}

// RollAsthmaBirthday is checked on the animal's existing daily cadence.
// Unknown legacy birth dates are not fabricated. Species-life normalization
// is native tuning; the public Ailments article establishes the human curve
// and susceptibility of both wild and domestic animals.
func RollAsthmaBirthday(a *State, now uint64, roll func(int) int, policy ...clock.CalendarPolicy) bool {
	if a == nil || a.Born == 0 || IsMech(a.Species) {
		return false
	}
	ageDays := AgeDays(a.Born, now, a.Species, policy...)
	if ageDays == AgeDays(a.Born, now-clock.TicksPerDay, a.Species, policy...) {
		return false
	}
	if !body.AsthmaBirthday(ageDays, LifeOf(a.Species).LifeDays, roll) {
		return false
	}
	return DevelopAsthma(a, now)
}

// DevelopAsthma places one incident on every eligible lung of this animal.
// Common Body conditions own the treatment and progression rules.
func DevelopAsthma(a *State, now uint64) bool {
	if a == nil || IsMech(a.Species) {
		return false
	}
	if a.Body == nil {
		next := NewBody(a.Species)
		if !body.DevelopAsthma(next, now) {
			return false
		}
		a.Body = next
		return true
	}
	return body.DevelopAsthma(a.Body, now)
}
