package drug

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/trait"
)

// Preset is a drug policy (append only, saved).
type Preset uint8

const (
	NoDrugs Preset = iota
	SocialDrinking
	Unrestricted
)

// Minimum gap between recreational doses of one drug.
const (
	baseGap     = 12 * hourTicks
	indulgeGap  = 6 * hourTicks
	hedonistGap = 3 * hourTicks
)

// Label is the policy's UI name.
func (p Preset) Label() string {
	switch p {
	case NoDrugs:
		return "no drugs"
	case SocialDrinking:
		return "social drinking"
	}
	return "unrestricted"
}

// Allows reports whether the policy lets a colonist take drug k now. joyLow
// is whether the joy need is low; traits are the colonist's traits.
func (p Preset) Allows(k item.Kind, joyLow bool, s *State, traits []trait.ID, now uint64) bool {
	i := index(k)
	if i < 0 || s == nil || p == NoDrugs {
		return false
	}
	if c, ok := s.Craving(); ok && c == k {
		return true // an addict's relief
	}
	gap, hedonist := uint64(baseGap), false
	for _, t := range traits {
		switch t {
		case trait.Abstainer:
			return false
		case trait.Indulgent:
			gap = min(gap, indulgeGap)
		case trait.Hedonist:
			gap, hedonist = hedonistGap, true
		}
	}
	if p == SocialDrinking && k != item.Beer {
		return false
	}
	if !joyLow && !hedonist {
		return false
	}
	if k == item.Beer && s.Band() >= Drunk {
		return false
	}
	if last := slot(s.LastDose, i); last != 0 && now < last+gap {
		return false
	}
	return true
}
