package drug

import "rimworld-go/internal/domain/economy/item"

// Craving hysteresis: start seeking below craveStart, stop above craveStop.
const (
	craveStart = 300
	craveStop  = 700
)

// Craving is the drug the colonist needs now, with hysteresis.
func (s *State) Craving() (item.Kind, bool) {
	for _, a := range s.Addictions {
		if a.Seeking {
			return a.Drug, true
		}
	}
	return 0, false
}
