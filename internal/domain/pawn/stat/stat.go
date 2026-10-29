// Package stat is a pawn's derived numbers (work speed, movement, hunger,
// comfort, accuracy...) built in a fixed order from body, age, conditions,
// traits and difficulty (NG-1 §4.6).
package stat

import "strconv"

// ID is one stat (append only).
type ID uint8

const (
	WorkSpeed      ID = iota // permille
	MoveSpeed                // permille
	HungerRate               // permille
	RestRate                 // permille
	MoodOffset               // mood points
	BreakThreshold           // mood points added to the break threshold
	ColdComfort              // tenths of a degree C added to the comfort floor
	HeatComfort              // tenths of a degree C added to the comfort ceiling
	ShootAcc                 // percentage points
	MeleeAcc                 // percentage points
	AimCooldown              // permille
	PainTolerance            // percent
	LearnRate                // permille
	RecreationRate           // permille: how fast recreation drains
	// Social and host systems (ENG-003), all permille.
	SocialImpact
	NegotiationAbility
	TradePriceImprovement
	ArrestSuccess
	// Beds and rooms (BED-1): furniture stats, all permille.
	Comfort           // sitting/lying comfort of furniture
	RestEffectiveness // how fast sleeping on it restores rest
	Beauty            // tile beauty contribution, thousandths of a beauty point
	NumStats
)

// Sheet is every stat's value.
type Sheet [NumStats]int32

// Mod changes one stat: Add for offsets, Mul (permille, 0 = none) for rates.
type Mod struct {
	Stat ID
	Add  int32
	Mul  int32
}

// unit is how a stat's value reads.
type unit uint8

const (
	unitRate    unit = iota // permille, shown as a percentage
	unitPoints              // plain points
	unitTenthsC             // tenths of a degree C
	unitPercent             // percent
)

var meta = [NumStats]struct {
	label string
	unit  unit
}{
	WorkSpeed:             {"Work speed", unitRate},
	MoveSpeed:             {"Move speed", unitRate},
	HungerRate:            {"Hunger rate", unitRate},
	RestRate:              {"Rest rate", unitRate},
	MoodOffset:            {"Mood", unitPoints},
	BreakThreshold:        {"Break threshold", unitPoints},
	ColdComfort:           {"Cold comfort", unitTenthsC},
	HeatComfort:           {"Heat comfort", unitTenthsC},
	ShootAcc:              {"Shooting accuracy", unitPoints},
	MeleeAcc:              {"Melee accuracy", unitPoints},
	AimCooldown:           {"Aim time", unitRate},
	PainTolerance:         {"Pain tolerance", unitPercent},
	LearnRate:             {"Learning rate", unitRate},
	RecreationRate:        {"Recreation drain", unitRate},
	SocialImpact:          {"Social impact", unitRate},
	NegotiationAbility:    {"Negotiation ability", unitRate},
	TradePriceImprovement: {"Trade price improvement", unitRate},
	ArrestSuccess:         {"Arrest success chance", unitRate},
	Comfort:               {"Comfort", unitRate},
	RestEffectiveness:     {"Rest effectiveness", unitRate},
	Beauty:                {"Beauty", unitRate},
}

// Label is the display name of a stat.
func (id ID) Label() string {
	if id >= NumStats {
		return ""
	}
	return meta[id].label
}

// Base is the neutral sheet: rates 1000, offsets 0.
func Base() Sheet {
	var s Sheet
	for _, id := range []ID{WorkSpeed, MoveSpeed, HungerRate, RestRate, AimCooldown, LearnRate, RecreationRate,
		SocialImpact, NegotiationAbility, TradePriceImprovement, ArrestSuccess, RestEffectiveness} {
		s[id] = 1000
	}
	return s
}

// Apply applies mods in order (multiplications then additions per mod).
// Multiplication truncates toward zero.
func (s Sheet) Apply(mods ...Mod) Sheet {
	for _, m := range mods {
		if m.Stat >= NumStats {
			continue
		}
		v := int64(s[m.Stat])
		if m.Mul != 0 {
			v = v * int64(m.Mul) / 1000
		}
		s[m.Stat] = int32(v + int64(m.Add))
	}
	return s
}

// Build applies layers to base in the order given. Callers pass the
// layers in the fixed NG-1 order: body, age, conditions, traits,
// difficulty (pawngen.StatLayers gives the first four).
func Build(base Sheet, layers ...[]Mod) Sheet {
	for _, l := range layers {
		base = base.Apply(l...)
	}
	return base
}

// Format reads a mod for people: "Work speed +20%", "Cold comfort -6 °C".
func Format(m Mod) string {
	if m.Stat >= NumStats {
		return ""
	}
	md := meta[m.Stat]
	out := md.label
	if m.Mul != 0 {
		out += " " + signed(int64(m.Mul)-1000, 10) + "%"
	}
	if m.Add != 0 {
		switch md.unit {
		case unitTenthsC:
			out += " " + signed(int64(m.Add), 10) + " °C"
		case unitPercent:
			out += " " + signed(int64(m.Add), 1) + "%"
		case unitRate:
			out += " " + signed(int64(m.Add), 10) + "%"
		default:
			out += " " + signed(int64(m.Add), 1)
		}
	}
	return out
}

// signed formats v/div with a sign and at most one decimal.
func signed(v, div int64) string {
	sign := "+"
	if v < 0 {
		sign, v = "-", -v
	}
	whole := strconv.FormatInt(v/div, 10)
	if rem := v % div; rem != 0 {
		// one decimal is enough for every stat we show (div is 1 or 10)
		whole += "." + strconv.FormatInt(rem*10/div, 10)
	}
	return sign + whole
}
