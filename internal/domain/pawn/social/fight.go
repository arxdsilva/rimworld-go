package social

// FightChance is the permille chance an interaction starts a social
// fight, scaled by each multiplier (permille).
func FightChance(trigger Interaction, mults ...int32) int32 {
	var c int64
	switch trigger {
	case Insult:
		c = FightChanceInsult
	case Slight:
		c = FightChanceSlight
	default:
		return 0
	}
	for _, m := range mults {
		c = c * int64(m) / 1000
	}
	if c > FightChanceMax {
		c = FightChanceMax
	}
	if c < 0 {
		c = 0
	}
	return int32(c)
}

// MalnutritionMult is the fight multiplier for a hunger band: 0 fed,
// 1 hungry .. 4 starving.
func MalnutritionMult(band int) int32 {
	switch {
	case band <= 0:
		return 1000
	case band >= 4:
		return 3000
	}
	return int32(1000 + 500*band)
}

// FightOutcome is how a fighter feels afterwards.
type FightOutcome uint8

const (
	FightAngering FightOutcome = iota
	FightCathartic
)

// RollOutcome picks an outcome (about half each).
func RollOutcome(roll func(int) int) FightOutcome {
	if roll(1000) < FightAngeringShare {
		return FightAngering
	}
	return FightCathartic
}

// Mood is the outcome's mood points and duration in days.
func (o FightOutcome) Mood() (points, days int) {
	if o == FightAngering {
		return FightAngeringMood, FightOutcomeDays
	}
	return FightCatharticMood, FightOutcomeDays
}

// Memory is the opinion memory the outcome leaves about the other fighter.
func (o FightOutcome) Memory() MemKind {
	if o == FightAngering {
		return MemAngeringFight
	}
	return MemCatharticFight
}
