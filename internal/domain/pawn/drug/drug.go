package drug

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
)

// numDrugs is how many drugs there are (beer, ashleaf tea, spark tablet).
const numDrugs = 3

// High is a drug's effect wearing off.
type High struct {
	Drug     item.Kind
	Severity int32 // permille
	Until    uint64
}

// Addiction is a dependence on a drug and its unmet need.
type Addiction struct {
	Drug     item.Kind
	Severity int32 // permille
	Need     int32 // permille
}

// State is one colonist's drug state (saved).
type State struct {
	Alcohol    int32 // permille
	Highs      []High
	Addictions []Addiction
	Tolerance  [numDrugs]int32
	Hangover   uint64 // tick the hangover ends (0: none)
}

// Ingest takes one dose of drug k at tick now.
func (s *State) Ingest(k item.Kind, roll func(int) int, now uint64) {
	// TODO(M2)
}

// Tick advances the state by hours.
func (s *State) Tick(hours int, now uint64) {
	// TODO(M2)
}

// FightMultiplier scales social-fight chance (alcohol band x withdrawal),
// permille.
func (s *State) FightMultiplier() int32 {
	return 1000 // TODO(M2)
}

// Thoughts are the state thoughts the drugs give (tipsy, high, withdrawal).
func (s *State) Thoughts() []mood.Kind {
	return nil // TODO(M2)
}
