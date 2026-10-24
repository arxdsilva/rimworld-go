package drug

import "rimworld-go/internal/domain/economy/item"

// Preset is a drug policy (append only, saved).
type Preset uint8

const (
	NoDrugs Preset = iota
	SocialDrinking
	Unrestricted
)

// Allows reports whether the policy lets a colonist take drug k now.
func (p Preset) Allows(k item.Kind, joyLow bool, s *State) bool {
	return false // TODO(M2)
}
