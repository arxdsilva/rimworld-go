package training

// Skill is a trainable behaviour (append only, saved).
type Skill uint8

const (
	Obedience Skill = iota
	Guard
	Release
	NumSkills
)

// Steps is how many successful steps it takes to learn each skill.
var Steps = [NumSkills]uint8{Obedience: 3, Guard: 3, Release: 2}

// Bond chances, permille per event (Proximity per BondCheckTicks check).
const (
	BondTame       int32 = 10
	BondTrain      int32 = 7
	BondTend       int32 = 4
	BondProximity  int32 = 1
	BondCheckTicks       = 2500
	// DecayTicks is how long a skill goes unpracticed before it loses a
	// step (10 days).
	DecayTicks uint64 = 10 * 60000
	// BondedFactor multiplies the step chance for a bonded trainer, permille.
	BondedFactor int32 = 5000
)

// Needed returns the steps to learn s (0 for an invalid skill).
func Needed(s Skill) uint8 {
	if s >= NumSkills {
		return 0
	}
	return Steps[s]
}

// Learned reports whether steps complete skill s.
func Learned(s Skill, steps uint8) bool {
	n := Needed(s)
	return n > 0 && steps >= n
}

// StepChance is the permille chance one training session succeeds for a
// trainer of the given Animals skill (0..20); a bonded trainer gets x5,
// capped at 1000.
func StepChance(animalsSkill int, bonded bool) int32 {
	s := int32(min(max(animalsSkill, 0), 20))
	c := 100 + s*25
	if bonded {
		c = c * BondedFactor / 1000
	}
	return min(c, 1000)
}

// Decay returns steps after sinceTrained ticks without practice: one step
// is lost per DecayTicks; a learned Obedience never drops below 1.
func Decay(s Skill, steps uint8, sinceTrained uint64) uint8 {
	if s >= NumSkills || steps == 0 {
		return steps
	}
	lost := sinceTrained / DecayTicks
	floor := uint8(0)
	if Learned(s, steps) && s == Obedience {
		floor = 1
	}
	if lost >= uint64(steps) || uint8(lost) > steps-floor {
		return floor
	}
	return steps - uint8(lost)
}
