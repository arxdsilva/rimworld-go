// Package stat is a pawn's derived numbers (work speed, movement, hunger,
// comfort, accuracy...) built in a fixed order from body, age, conditions,
// traits and difficulty (NG-1 §4.6).
package stat

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

// Base is the neutral sheet: rates 1000, offsets 0.
func Base() Sheet {
	var s Sheet
	for _, id := range []ID{WorkSpeed, MoveSpeed, HungerRate, RestRate, AimCooldown, LearnRate} {
		s[id] = 1000
	}
	return s
}

// Apply applies mods in order (multiplications then additions per mod).
func (s Sheet) Apply(mods ...Mod) Sheet { return s } // NG-1 stub (PAWN-D)
