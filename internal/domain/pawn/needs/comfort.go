package needs

// Comfort (BED-3, ENG-003 wiki numbers; wiki: Comfort). It rises only
// while a pawn uses furniture (sits on a seat, lies in a bed), up to that
// piece's comfort, and falls otherwise.
const (
	// StepTicks is how often the comfort and outdoors needs step: the
	// wiki gives their rates per 150 ticks.
	StepTicks = 150
	// ComfortGain is 3.6 % a step (60 % an hour) while using furniture
	// whose comfort is above the level.
	ComfortGain int32 = 36_000
	// ComfortFall is 4 % an hour, 0.24 % a step (the page also says about
	// 5 %; 4 is its stated rate).
	ComfortFall int32 = 2_400
	// ComfortStart is a new pawn's comfort (original tuning: neutral).
	ComfortStart = 50 * LevelPct
)

// ComfortStep is one step of comfort at level: using furniture of comfort
// seat (permille, 1000 = 1.0) it climbs to the furniture's comfort, and
// above it falls back to it; using none it falls toward 0.
func ComfortStep(level int32, seat int, using bool) int32 {
	if !using {
		return clampLevel(level - ComfortFall)
	}
	limit := clampLevel(int32(seat) * 1000)
	if level < limit {
		return min(level+ComfortGain, limit)
	}
	return max(level-ComfortFall, limit)
}

// ComfortLadder is the comfort thoughts: below 10 % -3, 10–59 % none,
// then +4, +6, +8 and +10 from 60, 70, 80 and 90 % (wiki: Comfort).
var ComfortLadder = Ladder{
	Up:    []int32{10 * LevelPct, 60 * LevelPct, 70 * LevelPct, 80 * LevelPct, 90 * LevelPct},
	Value: []int{-3, 0, 4, 6, 8, 10},
	Label: []string{"uncomfortable", "", "comfortable", "very comfortable", "extremely comfortable", "luxuriously comfortable"},
}
