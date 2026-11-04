// Package gameplay is the colony's habit rules (CSE-*): small switches and
// limits on how colonists go about their day. They are authoritative game
// state, carried in the save and changed only by typed commands, so a replay
// gives the same result on any machine. The simulation never reads a player
// settings file for them.
package gameplay

// Toggle is one habit. Saved by number through Settings.Flipped: append only.
type Toggle uint8

const (
	OutdoorsJoy         Toggle = iota // CSE-001
	JoyTopOff                         // CSE-002
	SpareMeal                         // CSE-005
	SickGifts                         // CSE-007
	RelaxDrink                        // CSE-008
	DrugPotentialMood                 // CSE-010
	AnyDrugFromPack                   // CSE-011
	SeparateMeals                     // CSE-012
	OddMeatNormal                     // CSE-013
	FlavourIngredients                // CSE-014
	SpoilingMeals                     // CSE-015
	CleanBeforeWork                   // CSE-017, CSE-022
	CleanBeforeBill                   // CSE-018
	BenchCleanToggle                  // CSE-019
	CleanBeforeMeal                   // CSE-020
	CleanAfterTend                    // CSE-021
	FullPathCheck                     // CSE-027
	SnowWaitsForSky                   // CSE-028
	HaulAllIngredients                // CSE-029
	RespectCapacity                   // CSE-030
	SpoilingIngredients               // CSE-033
	StackToStockpile                  // CSE-035
	PutBack                           // CSE-037
	ManualUnload                      // CSE-038
	ShelterWander                     // CSE-041
	PoliteWander                      // CSE-042
	FullRecipeList                    // CSE-046
	NumToggles
)

// offByDefault is the set of toggles that start off, as bits.
const offByDefault = 1<<OddMeatNormal | 1<<FlavourIngredients | 1<<BenchCleanToggle |
	1<<CleanBeforeMeal | 1<<ShelterWander | 1<<PoliteWander

// ToggleDefault is whether a toggle starts on.
func ToggleDefault(t Toggle) bool { return offByDefault&(uint64(1)<<t) == 0 }

// AllToggles lists every toggle in order.
func AllToggles() []Toggle {
	out := make([]Toggle, NumToggles)
	for i := range out {
		out[i] = Toggle(i)
	}
	return out
}

// Limit is one numeric habit. Saved by position: append only.
type Limit uint8

const (
	TidyStation Limit = iota // CSE-023: cells cleaned before work
	TidyBench                // cells cleaned between gathering and crafting
	TidyTend                 // cells cleaned after tending, 0 unlimited
	NumLimits
)

// MaxLimit is the largest value a limit takes.
const MaxLimit = 50

var limitDefaults = [NumLimits]int{TidyStation: 5, TidyBench: 5, TidyTend: 0}

// LimitDefault is a limit's starting value.
func LimitDefault(l Limit) int { return limitDefaults[l] }

// Settings holds differences from the defaults: the zero value is the
// defaults, so an older save decodes to it and it adds nothing to the state
// hash until the player changes something.
type Settings struct {
	Flipped uint64           // bit t: toggle t differs from its default
	Limits  [NumLimits]uint8 // 0: default; else the value plus one
}

// On reports whether toggle t is on.
func (s Settings) On(t Toggle) bool {
	return ToggleDefault(t) != (s.Flipped&(uint64(1)<<t) != 0)
}

// With returns s with toggle t set to on.
func (s Settings) With(t Toggle, on bool) Settings {
	if t >= NumToggles {
		return s
	}
	if on == ToggleDefault(t) {
		s.Flipped &^= uint64(1) << t
	} else {
		s.Flipped |= uint64(1) << t
	}
	return s
}

// LimitOf is the value of limit l.
func (s Settings) LimitOf(l Limit) int {
	if l >= NumLimits {
		return 0
	}
	if v := s.Limits[l]; v != 0 {
		return int(v) - 1
	}
	return limitDefaults[l]
}

// WithLimit returns s with limit l set to n, clamped to 0..MaxLimit.
func (s Settings) WithLimit(l Limit, n int) Settings {
	if l >= NumLimits {
		return s
	}
	n = max(0, min(n, MaxLimit))
	if n == limitDefaults[l] {
		s.Limits[l] = 0
	} else {
		s.Limits[l] = uint8(n + 1)
	}
	return s
}
