// Package skill is a colonist's skills: levels 0..20, experience, passions,
// learning and decay, and how a level turns into work speed (NG-1 §4.6).
package skill

import "rimworld-go/internal/domain/ai/work"

// ID is one skill (append only, saved).
type ID uint8

const (
	Shooting ID = iota
	Melee
	Construction
	Mining
	Cooking
	Plants
	Animals
	Crafting
	Medicine
	Social
	Intellectual
	NumSkills
)

// Passion is how much a colonist loves a skill.
type Passion uint8

const (
	NoPassion Passion = iota
	Minor
	Major
)

// MaxLevel is the highest skill level.
const MaxLevel = 20

// Neutral is the level of a legacy/neutral pawn: exactly 1000‰ speed.
const Neutral = 7

// Skill is one skill's state. XP is progress toward the next level and
// may go negative (decay band, see DecayDay). Carry holds the hundredths
// of an XP point that passion scaling left over, so single units of work
// still add up for a pawn without passion.
type Skill struct {
	Level   uint8
	XP      int32
	Passion Passion
	Carry   int16
}

// Set is every skill of one pawn.
type Set [NumSkills]Skill

var names = [NumSkills]struct{ key, label string }{
	Shooting:     {"shooting", "Shooting"},
	Melee:        {"melee", "Melee"},
	Construction: {"construction", "Construction"},
	Mining:       {"mining", "Mining"},
	Cooking:      {"cooking", "Cooking"},
	Plants:       {"plants", "Plants"},
	Animals:      {"animals", "Animals"},
	Crafting:     {"crafting", "Crafting"},
	Medicine:     {"medicine", "Medicine"},
	Social:       {"social", "Social"},
	Intellectual: {"intellectual", "Intellectual"},
}

// Key is the stable name ("shooting"); Label the display name.
func (id ID) Key() string {
	if id >= NumSkills {
		return ""
	}
	return names[id].key
}

// Label is the display name ("Shooting").
func (id ID) Label() string {
	if id >= NumSkills {
		return ""
	}
	return names[id].label
}

// Parse finds a skill by its key.
func Parse(key string) (ID, bool) {
	for i, n := range names {
		if n.key == key {
			return ID(i), true
		}
	}
	return 0, false
}

// ForWork is the skill that drives work type t (Hunt -> Shooting; callers
// switch to Melee for melee weapons). ok=false: general work speed only.
func ForWork(t work.Type) (ID, bool) {
	switch t {
	case work.Hunt:
		return Shooting, true
	case work.Construct:
		return Construction, true
	case work.Mine:
		return Mining, true
	case work.Cook:
		return Cooking, true
	case work.Grow, work.PlantCut:
		return Plants, true
	case work.Handle:
		return Animals, true
	case work.Craft:
		return Crafting, true
	case work.Doctor:
		return Medicine, true
	case work.Research:
		return Intellectual, true
	case work.Warden:
		return Social, true
	}
	return 0, false
}

// SpeedPermil is work speed at level l: 1000 + 60*(l-7) (580..1780).
func SpeedPermil(l uint8) int {
	if l > MaxLevel {
		l = MaxLevel
	}
	return 1000 + 60*(int(l)-Neutral)
}

// Cost is the experience from level l to l+1: 20000 + 6000*l.
func Cost(l uint8) int32 { return 20000 + 6000*int32(l) }

// passionPct is the learning multiplier per passion, in percent.
var passionPct = [...]int{NoPassion: 35, Minor: 100, Major: 150}

// Learn adds xp (already scaled by the caller's learn rate) times the
// passion multiplier (35/100/150 %), levelling up as needed. Fractions of
// a point carry over to the next call. At MaxLevel experience banks up to
// one Cost(MaxLevel), a cushion against decay. Callers must not call it
// for skills the pawn is incapable of.
func Learn(s *Skill, xp int) {
	if xp <= 0 {
		return
	}
	pct := 100
	if int(s.Passion) < len(passionPct) {
		pct = passionPct[s.Passion]
	}
	total := int64(xp)*int64(pct) + int64(s.Carry)
	gain := total / 100
	s.Carry = int16(total % 100)
	xpNow := int64(s.XP) + gain
	for s.Level < MaxLevel && xpNow >= int64(Cost(s.Level)) {
		xpNow -= int64(Cost(s.Level))
		s.Level++
	}
	if s.Level >= MaxLevel {
		s.Level = MaxLevel
		xpNow = min(xpNow, int64(Cost(MaxLevel)))
	}
	s.XP = int32(xpNow)
}

// DecayFloor is the level at and below which skills never decay.
const DecayFloor = 10

// DecayDay applies one day of decay above level 10: 500*(L-9) XP. The
// no-flap band: XP may go negative and the level only drops once XP is
// below -Cost(L-1)/2; after a drop XP restarts at 75% of Cost(L-1), so a
// pawn hovering at a boundary needs a whole quarter level of learning to
// climb back and half a level of neglect to fall again.
func DecayDay(s *Skill) {
	if s.Level <= DecayFloor {
		return
	}
	s.XP -= 500 * int32(s.Level-9)
	below := Cost(s.Level - 1)
	if s.XP < -below/2 {
		s.Level--
		s.XP = below * 3 / 4
	}
}

// LevelLabel names a level band: Novice 0-3, Capable 4-7, Skilled 8-11,
// Expert 12-15, Master 16-20.
func LevelLabel(l uint8) string {
	switch {
	case l <= 3:
		return "Novice"
	case l <= 7:
		return "Capable"
	case l <= 11:
		return "Skilled"
	case l <= 15:
		return "Expert"
	default:
		return "Master"
	}
}

// Label names a passion: "", "Minor", "Major".
func (p Passion) Label() string {
	switch p {
	case Minor:
		return "Minor"
	case Major:
		return "Major"
	}
	return ""
}

// NeutralSet is a set at Neutral with no passions (legacy pawns).
func NeutralSet() Set {
	var s Set
	for i := range s {
		s[i].Level = Neutral
	}
	return s
}
