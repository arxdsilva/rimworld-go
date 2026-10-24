package drug

import "rimworld-go/internal/domain/economy/item"

// Craving is the drug the colonist needs now, with hysteresis.
func (s *State) Craving() (item.Kind, bool) {
	return 0, false // TODO(M2)
}
