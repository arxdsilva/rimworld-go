package body

import "rimworld-go/internal/domain/economy/item"

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
	// MED-05 appends the inner organs (Missing is a uint32: at most 32
	// parts). An older save has no wounds and no Missing bit on them, so
	// they load whole.
	KidneyL
	KidneyR
	Liver
	Spine
	NumParts
)

// KillDamage is the Damage reported when a vital part is gone. It equals
// health.MaxHP (100) so the caller's existing death check fires.
const KillDamage = 100

// Def describes one part of a body plan (the humanlike table below, the
// animal plans in anatomy_plan.go).
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

// defs: the torso gave up 84 of its coverage to the MED-05 organs, so
// the total stays 1000.
var defs = [NumParts]Def{
	Brain:   {"brain", Torso, 25, 40, true, false, Consciousness, true},
	EyeL:    {"left eye", Brain, 12, 15, false, true, Sight, true},
	EyeR:    {"right eye", Brain, 12, 15, false, true, Sight, true},
	EarL:    {"left ear", Brain, 10, 10, false, true, Hearing, true},
	EarR:    {"right ear", Brain, 10, 10, false, true, Hearing, true},
	Nose:    {"nose", Brain, 10, 10, false, true, 0, false},
	Jaw:     {"jaw", Brain, 20, 20, false, true, Talking, true},
	Torso:   {"torso", Torso, 60, 296, true, false, 0, false},
	Heart:   {"heart", Torso, 25, 20, true, false, BloodPumping, true},
	LungL:   {"left lung", Torso, 25, 30, false, false, Breathing, true},
	LungR:   {"right lung", Torso, 25, 30, false, false, Breathing, true},
	Stomach: {"stomach", Torso, 25, 30, false, false, Digestion, true},
	ArmL:    {"left arm", Torso, 35, 60, false, false, Manipulation, true},
	ArmR:    {"right arm", Torso, 35, 60, false, false, Manipulation, true},
	HandL:   {"left hand", ArmL, 20, 25, false, false, Manipulation, true},
	HandR:   {"right hand", ArmR, 20, 25, false, false, Manipulation, true},
	LegL:    {"left leg", Torso, 40, 80, false, false, Moving, true},
	LegR:    {"right leg", Torso, 40, 80, false, false, Moving, true},
	FootL:   {"left foot", LegL, 20, 30, false, false, Moving, true},
	FootR:   {"right foot", LegR, 20, 30, false, false, Moving, true},
	// MED-05 organs: not vital by themselves; losing their capacity is
	// (OrganFailure, capacity.go). wiki: Kidney (50 HP), Liver (50 HP),
	// Spine (40 HP). Coverage is original tuning.
	KidneyL: {"left kidney", Torso, 50, 17, false, false, BloodFiltration, true},
	KidneyR: {"right kidney", Torso, 50, 17, false, false, BloodFiltration, true},
	Liver:   {"liver", Torso, 50, 25, false, false, BloodFiltration, true},
	Spine:   {"spine", Torso, 40, 25, false, false, Moving, true},
}

// DefOf returns the humanlike definition of p (the zero Def for an
// unknown part); Body.PartDef reads a body's own plan.
func DefOf(p Part) Def {
	if p >= NumParts {
		return Def{}
	}
	return defs[p]
}

// LossKills reports that losing p now would kill: p is vital, or it is
// the last of a capacity the body cannot live without (OrganFailure: the
// liver, the stomach, the last kidney or lung). A hit never destroys such
// a part (the game spills the damage on), so death from wounds still
// comes at KillDamage.
//
// MED-13: on an animal plan, a part with a vital part under it (a head
// over the brain) counts as vital too.
func (b *Body) LossKills(p Part) bool {
	n, t := b.numParts(), b.table()
	if p >= n || t[p].Vital {
		return p < n
	}
	if b.Anatomy != Humanlike {
		for q := Part(0); q < n; q++ {
			if t[q].Vital && b.under(q, p) {
				return true
			}
		}
	}
	c := Body{Wounds: b.Wounds, Missing: b.Missing, Conds: b.Conds, Anatomy: b.Anatomy, Scale: b.Scale}
	if len(b.Fitted) > 0 {
		c.Fitted = append([]item.Kind(nil), b.Fitted...)
	}
	for q := Part(0); q < n; q++ {
		if b.under(q, p) {
			c.Missing |= bit(q)
			c.SetImplant(q, 0)
		}
	}
	return !b.OrganFailure() && c.OrganFailure()
}

// Label is the plain-English name of humanlike part p (Body.PartLabel
// names a part of any plan).
func (p Part) Label() string { return DefOf(p).Label }

func bit(p Part) uint32 { return 1 << p }

// isMissing reports p gone (destroyed, amputated or replaced).
func (b *Body) isMissing(p Part) bool { return b.Missing&bit(p) != 0 }

// under reports p equal to or below ancestor in the body's part tree.
func (b *Body) under(p, ancestor Part) bool {
	t := b.table()
	if p >= Part(len(t)) {
		return false
	}
	for range t {
		if p == ancestor {
			return true
		}
		if t[p].Parent == p {
			return false
		}
		p = t[p].Parent
	}
	return false
}

// PickPart chooses a non-missing part by coverage with roll, which
// returns a value in [0, n). It returns false when nothing is left.
func (b *Body) PickPart(roll func(int) int) (Part, bool) {
	t := b.table()
	total := 0
	for p := range t {
		if !b.isMissing(Part(p)) {
			total += t[p].Coverage
		}
	}
	if total == 0 {
		return b.Root(), false
	}
	r := roll(total)
	if r < 0 {
		r = 0
	}
	for p := range t {
		if b.isMissing(Part(p)) {
			continue
		}
		if r < t[p].Coverage {
			return Part(p), true
		}
		r -= t[p].Coverage
	}
	return b.Root(), false
}

// Inner reports an internal organ: harvested or transplanted (MED-06),
// never amputated.
func Inner(p Part) bool {
	switch p {
	case Heart, LungL, LungR, Stomach, KidneyL, KidneyR, Liver, Spine:
		return true
	}
	return false
}
