package body

// Part is a body part (append only, saved).
type Part uint8

const (
	Brain Part = iota
	EyeL
	EyeR
	EarL
	EarR
	Nose
	Jaw
	Torso
	Heart
	LungL
	LungR
	Stomach
	ArmL
	ArmR
	HandL
	HandR
	LegL
	LegR
	FootL
	FootR
	NumParts
)

// KillDamage is the Damage reported when a vital part is gone. It equals
// health.MaxHP (100) so the caller's existing death check fires.
const KillDamage = 100

// Def describes one part of the humanlike body.
type Def struct {
	Label    string
	Parent   Part // Torso is the root and is its own parent
	MaxHP    int
	Coverage int // hit weight in permille; all parts sum to 1000
	Vital    bool
	Face     bool
	Cap      Capacity // the capacity the part mainly serves
	HasCap   bool
}

var defs = [NumParts]Def{
	Brain:   {"brain", Torso, 25, 40, true, false, Consciousness, true},
	EyeL:    {"left eye", Brain, 12, 15, false, true, Sight, true},
	EyeR:    {"right eye", Brain, 12, 15, false, true, Sight, true},
	EarL:    {"left ear", Brain, 10, 10, false, true, Hearing, true},
	EarR:    {"right ear", Brain, 10, 10, false, true, Hearing, true},
	Nose:    {"nose", Brain, 10, 10, false, true, 0, false},
	Jaw:     {"jaw", Brain, 20, 20, false, true, Talking, true},
	Torso:   {"torso", Torso, 60, 380, true, false, 0, false},
	Heart:   {"heart", Torso, 25, 20, true, false, 0, false},
	LungL:   {"left lung", Torso, 25, 30, false, false, Breathing, true},
	LungR:   {"right lung", Torso, 25, 30, false, false, Breathing, true},
	Stomach: {"stomach", Torso, 25, 30, false, false, 0, false},
	ArmL:    {"left arm", Torso, 35, 60, false, false, Manipulation, true},
	ArmR:    {"right arm", Torso, 35, 60, false, false, Manipulation, true},
	HandL:   {"left hand", ArmL, 20, 25, false, false, Manipulation, true},
	HandR:   {"right hand", ArmR, 20, 25, false, false, Manipulation, true},
	LegL:    {"left leg", Torso, 40, 80, false, false, Moving, true},
	LegR:    {"right leg", Torso, 40, 80, false, false, Moving, true},
	FootL:   {"left foot", LegL, 20, 30, false, false, Moving, true},
	FootR:   {"right foot", LegR, 20, 30, false, false, Moving, true},
}

// DefOf returns the definition of p (the zero Def for an unknown part).
func DefOf(p Part) Def {
	if p >= NumParts {
		return Def{}
	}
	return defs[p]
}

// Label is the plain-English name of p.
func (p Part) Label() string { return DefOf(p).Label }

func bit(p Part) uint32 { return 1 << p }

// isMissing reports p gone (destroyed, amputated or replaced).
func (b *Body) isMissing(p Part) bool { return b.Missing&bit(p) != 0 }

// under reports p equal to or below ancestor in the part tree.
func under(p, ancestor Part) bool {
	for i := 0; i < int(NumParts); i++ {
		if p == ancestor {
			return true
		}
		if defs[p].Parent == p {
			return false
		}
		p = defs[p].Parent
	}
	return false
}

// PickPart chooses a non-missing part by coverage with roll, which
// returns a value in [0, n). It returns false when nothing is left.
func (b *Body) PickPart(roll func(int) int) (Part, bool) {
	total := 0
	for p := Part(0); p < NumParts; p++ {
		if !b.isMissing(p) {
			total += defs[p].Coverage
		}
	}
	if total == 0 {
		return Torso, false
	}
	r := roll(total)
	if r < 0 {
		r = 0
	}
	for p := Part(0); p < NumParts; p++ {
		if b.isMissing(p) {
			continue
		}
		if r < defs[p].Coverage {
			return p, true
		}
		r -= defs[p].Coverage
	}
	return Torso, false
}
