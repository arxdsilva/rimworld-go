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
	if i < 0 || s == nil || p == NoDrugs || !defs[i].Recreational {
		return false // medical drugs: AllowsMedical
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

// AllowsMedical reports whether the policy has a colonist take medical
// drug k now (MED-07). Every policy but "no drugs" allows both: the
// painkiller when pain (permille) is at least PainkillerAt and no dose is
// still acting (numbed), so it is retaken only once the last one wore off
// and the pain is back; a scheduled drug (prophyl) every ScheduleDays
// days from its last dose. An abstainer takes medical drugs too.
func (p Preset) AllowsMedical(k item.Kind, pain int32, numbed bool, s *State, now uint64) bool {
	i := index(k)
	if i < 0 || s == nil || p == NoDrugs {
		return false
	}
	d := defs[i]
	switch {
	case d.Painkiller:
		return !numbed && pain >= PainkillerAt
	case d.ScheduleDays > 0:
		last := slot(s.LastDose, i)
		return last == 0 || now >= last+uint64(d.ScheduleDays)*dayTicks
	}
	return false
}

// Medical lists the medical drugs in definition order.
func Medical() []Def {
	var out []Def
	for _, d := range defs {
		if !d.Recreational {
			out = append(out, d)
		}
	}
	return out
}
