// Package mentalbreak is what a colonist does when its mood gives way:
// for a while it stops taking orders or doing work and acts out instead.
// Afterwards it feels the relief of catharsis.
package mentalbreak

// Kind is a mental break.
type Kind uint8

const (
	None       Kind = iota
	SadWander       // wanders aimlessly in a daze
	FoodBinge       // eats far past full
	HideInRoom      // shuts itself away in its room
	// SocialFight is a brawl two colonists fall into after an insult
	// (ENG-003). It is not a mood break: Kinds never offers it.
	SocialFight
	// Appended for SIM-066: kinds are saved as uint8, so order is fixed.
	InsultingSpree // insults everyone nearby
	TargetedInsult // insults one colonist over and over
	Berserk        // attacks anyone nearby
	MurderousRage  // hunts one colonist
	Catatonic      // collapses and stays down until it recovers
	// Tantrum (CBT-4, appended): a major break; it punches and smashes
	// the buildings and items around it.
	Tantrum
	numKinds
)

// Def is a break's player-facing texts, how long it lasts and how likely
// it is among the breaks of its tier.
type Def struct {
	Label    string // inspector/alert: "sad wander"
	Message  string // "%s is wandering around in a daze."
	Duration uint64 // ticks; the fixed or minimum duration
	Weight   int    // relative chance among the kinds of its Tier
	// MinDuration..MaxDuration is the range DurationFor draws from.
	MinDuration, MaxDuration uint64
	Tier                     Tier
}

const hour = 2500

// def builds a Def. dur is the legacy fixed/minimum Duration current
// callers use; lo..hi is the wiki range DurationFor draws from; weight is
// the commonality times ten (0.5 -> 5).
func def(label, msg string, dur, lo, hi uint64, weight int, tier Tier) Def {
	return Def{label, msg, dur, weight, lo, hi, tier}
}

// Per-kind tables: one line per kind, so swapping a value is a one-line
// change. Ranges, weights and tiers are from the RimWorld wiki.
var defs = [numKinds]Def{
	// wiki: Mental_break
	SadWander: def("sad wander", "%s is wandering around in a daze.", 8*hour, 40000, 60000, 5, Minor),
	// wiki: Mental_break
	FoodBinge: def("food binge", "%s is binge eating.", 4*hour, 25000, 45000, 8, Minor),
	// wiki: Mental_break
	HideInRoom: def("hiding in room", "%s has locked themselves away.", 10*hour, 40000, 60000, 5, Minor),
	// the minimum: a fight usually ends sooner, when one side goes down
	SocialFight: def("social fight", "%s is in a fight.", 420, 420, 420, 0, TierNone),
	// wiki: Mental_break (needs 2+ other colonists; ends via a social fight)
	InsultingSpree: def("insulting spree", "%s is going around insulting people.", 25000, 25000, 45000, 5, Minor),
	// wiki: Mental_break (one colonist)
	TargetedInsult: def("targeted insults", "%s is hurling insults at one colonist.", 25000, 25000, 45000, 5, Minor),
	// wiki: Mental_break
	Berserk: def("berserk", "%s has gone berserk!", 40000, 40000, 60000, 10, Extreme),
	// wiki: Mental_break (fixed length)
	MurderousRage: def("murderous rage", "%s is out for someone's blood!", 100000, 100000, 100000, 10, Extreme),
	// wiki: Mental_break
	Catatonic: def("catatonic", "%s has collapsed into a catatonic state.", 100000, 100000, 300000, 10, Extreme),
	// wiki: Mental_break (Tantrum: 8000..12000 ticks, commonality 0.333)
	Tantrum: def("tantrum", "%s is throwing a tantrum and smashing things!", 10000, 8000, 12000, 3, Major),
}

// DefOf returns the break definition.
func DefOf(k Kind) Def {
	if k >= numKinds {
		return Def{}
	}
	return defs[k]
}

func (k Kind) String() string { return DefOf(k).Label }

// Kinds lists the breaks a colonist can have.
func Kinds() []Kind { return []Kind{SadWander, FoodBinge, HideInRoom} }

// Threshold is the mood below which breaks become possible.
const Threshold = 20

// RiskPerHour is the chance (permille) of a break starting in an hour at
// mood m: none at or above Threshold, then 2% per point below it (about 4
// in 10 per hour for a colonist at rock bottom).
func RiskPerHour(m int) int {
	if m >= Threshold {
		return 0
	}
	return (Threshold - m) * 20
}

// AllMoodKinds lists every kind a low mood can cause, across all tiers
// (Kinds is the legacy subset the simulation uses today).
func AllMoodKinds() []Kind {
	return []Kind{SadWander, FoodBinge, InsultingSpree, HideInRoom, TargetedInsult, Berserk, MurderousRage, Catatonic, Tantrum}
}

// Tier is a break's severity, set by how far mood has fallen.
type Tier uint8

const (
	TierNone Tier = iota
	Minor
	Major
	Extreme
)

// Thresholds are the moods (0-100) below which each tier opens up.
type Thresholds struct{ Minor, Major, Extreme int }

// DefaultThresholds returns the standard tier thresholds.
func DefaultThresholds() Thresholds {
	// wiki: Mental_break (0.35/0.20/0.05 of the 0-100 mood scale)
	return Thresholds{Minor: 35, Major: 20, Extreme: 5}
}

// TierOf is the severity tier of a kind; TierNone for None and SocialFight.
func TierOf(k Kind) Tier { return DefOf(k).Tier }

// TierAt is the highest tier open at mood m: a tier opens strictly below
// its threshold (like Threshold/RiskPerHour: at the threshold, no break).
func TierAt(m int, t Thresholds) Tier {
	switch {
	case m < t.Extreme:
		return Extreme
	case m < t.Major:
		return Major
	case m < t.Minor:
		return Minor
	}
	return TierNone
}

// Pick chooses a break of the given tier, weighted by Def.Weight, from the
// kinds allowed (nil allows all). A tier with no allowed kind falls back to
// the next lower tier (the Major tier has no kinds today, so it picks from
// Minor). The roll is the only source of randomness, so the same inputs
// give the same kind. It returns None when no tier down to Minor has an
// allowed kind.
func Pick(tier Tier, roll uint32, allowed func(Kind) bool) Kind {
	for ; tier > TierNone; tier-- {
		if k := pickIn(tier, roll, allowed); k != None {
			return k
		}
	}
	return None
}

func pickIn(tier Tier, roll uint32, allowed func(Kind) bool) Kind {
	total := 0
	for k := Kind(1); k < numKinds; k++ {
		if candidate(k, tier, allowed) {
			total += defs[k].Weight
		}
	}
	if total == 0 {
		return None
	}
	r := int(roll % uint32(total))
	for k := Kind(1); k < numKinds; k++ {
		if !candidate(k, tier, allowed) {
			continue
		}
		if r < defs[k].Weight {
			return k
		}
		r -= defs[k].Weight
	}
	return None
}

func candidate(k Kind, tier Tier, allowed func(Kind) bool) bool {
	d := defs[k]
	return tier != TierNone && d.Tier == tier && d.Weight > 0 && (allowed == nil || allowed(k))
}

// DurationFor is a duration in MinDuration..MaxDuration chosen by roll.
func DurationFor(k Kind, roll uint32) uint64 {
	d := DefOf(k)
	if d.MaxDuration <= d.MinDuration {
		return d.MinDuration
	}
	return d.MinDuration + uint64(roll)%(d.MaxDuration-d.MinDuration+1)
}

// TargetsOthers reports whether the break is aimed at other colonists.
func TargetsOthers(k Kind) bool {
	return k == InsultingSpree || k == TargetedInsult || k == Berserk || k == MurderousRage
}

// Downs reports whether the break leaves the colonist downed.
func Downs(k Kind) bool { return k == Catatonic }

// MTBTicks is the mean time between breaks at a tier (wiki: Mental_break):
// minor 4 days, major 0.8 days, extreme 0.5 days. A day is 60000 ticks.
func MTBTicks(t Tier) uint64 {
	switch t {
	case Minor:
		return 240000
	case Major:
		return 48000
	case Extreme:
		return 30000
	}
	return 0
}

// Trait offsets to the Minor threshold, in mood points (wiki: Traits). The
// caller adds the ones its colonist has and calls ShiftThresholds.
const (
	OffsetIronWilled = -18
	OffsetSteadfast  = -9
	OffsetNervous    = 8
	OffsetNeurotic   = 8
	OffsetVolatile   = 15
)

// ShiftThresholds moves the Minor threshold by offset, clamped to 1..50,
// and derives Major = Minor*4/7 and Extreme = Minor/7 (wiki: Mental_break).
func ShiftThresholds(t Thresholds, offset int) Thresholds {
	m := min(max(t.Minor+offset, 1), 50)
	return Thresholds{Minor: m, Major: m * 4 / 7, Extreme: m / 7}
}
