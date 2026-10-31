package animal

import (
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
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
