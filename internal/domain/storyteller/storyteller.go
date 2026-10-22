// Package storyteller paces the colony's story: when threats come and how
// big, with three original personalities and a hidden legacy profile that
// reproduces the pre-NG-1 constants (NG-1 §3.2, §4.5).
package storyteller

import "rimworld-go/pkg/randx"

// Mode is a pacing model.
type Mode uint8

const (
	Legacy Mode = iota
	Cycle
	Calm
	Dice
)

// Category is a threat category the scheduler may fire.
type Category uint8

const (
	None Category = iota
	Raid
	Pack
	Manhunters
	Hive
	Mad
)

// Profile is a storyteller.
type Profile struct {
	Key, Name, Title, Bio, Portrait string
	Mode                            Mode
	FirstThreatDay                  int
	GapMin, GapMax                  int
	CycleOn, CycleOff               int
	DiceP0, DiceStep                int // permille
	PointsMin, PointsMax            int // percent
	RampPerCycle, RampCap           int // percent
	PositiveBias                    int // percent
	LossEaseDays, LossEasePct       int
	Hidden                          bool
}

// State is the scheduler's memory (saved).
type State struct {
	LastThreatDay, CycleDay, QuietDays, LastLossDay, Cycles int
	Pressure                                                bool
}

// Decision is the next threat.
type Decision struct {
	AtTick    uint64
	Category  Category // the caller picks the concrete threat by its rules
	PointsPct int
}

// Profiles lists the visible storytellers; ByKey finds any (incl. legacy).
func Profiles() []Profile              { return nil }              // NG-1 stub (RULES)
func ByKey(key string) (Profile, bool) { return Profile{}, false } // NG-1 stub (RULES)

// Plan decides the next threat after the one that just fired (or at start).
func Plan(p Profile, st *State, now uint64, rng *randx.Rand) Decision { return Decision{} } // NG-1 stub (RULES)

// Spark is an example pacing for the UI: threat strength per day.
func Spark(p Profile, days int, seed uint64) []int { return nil } // NG-1 stub (RULES)
