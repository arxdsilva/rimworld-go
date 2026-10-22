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
// may go negative (decay band, see DecayDay).
type Skill struct {
	Level   uint8
	XP      int32
	Passion Passion
}

// Set is every skill of one pawn.
type Set [NumSkills]Skill

// Key is the stable name ("shooting"); Label the display name.
func (id ID) Key() string   { return "" } // NG-1 stub (PAWN-D)
func (id ID) Label() string { return "" } // NG-1 stub (PAWN-D)

// ForWork is the skill that drives work type t (Hunt -> Shooting; callers
// switch to Melee for melee weapons). ok=false: general work speed only.
func ForWork(t work.Type) (ID, bool) { return 0, false } // NG-1 stub (PAWN-D)

// SpeedPermil is work speed at level l: 1000 + 60*(l-7) (580..1780).
func SpeedPermil(l uint8) int { return 1000 } // NG-1 stub (PAWN-D)

// Cost is the experience from level l to l+1: 20000 + 6000*l.
func Cost(l uint8) int32 { return 0 } // NG-1 stub (PAWN-D)

// Learn adds xp (already scaled by the caller's learn rate) times the
// passion multiplier (35/100/150 %), levelling up as needed.
func Learn(s *Skill, xp int) {} // NG-1 stub (PAWN-D)

// DecayDay applies one day of decay above level 10 with the no-flap band.
func DecayDay(s *Skill) {} // NG-1 stub (PAWN-D)

// Label names a level band: Novice, Capable, Skilled, Expert, Master.
func LevelLabel(l uint8) string { return "" } // NG-1 stub (PAWN-D)

// Neutralset is a set at Neutral with no passions (legacy pawns).
func NeutralSet() Set {
	var s Set
	for i := range s {
		s[i].Level = Neutral
	}
	return s
}
