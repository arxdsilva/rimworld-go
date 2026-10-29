package needs

// Beauty (BED-3; wiki: Beauty, Thoughts). A pawn looks around now and
// then; the average tile beauty it sees sets the level the need drifts
// toward. The wiki publishes the thought ladder but not the need's rates:
// the target and the drift are original tuning.
const (
	// BeautySampleTicks is how often an awake pawn looks around
	// (original tuning).
	BeautySampleTicks = 250
	// BeautyPerPoint is how far one point of average beauty moves the
	// target from half full: 10 % (original tuning, so the wiki's neutral
	// band of 35–65 % is about -1.5..+1.5 beauty).
	BeautyPerPoint = 10 * LevelPct
	// BeautyDrift is how far the need moves toward its target per look:
	// 10 % (original tuning).
	BeautyDrift = 10 * LevelPct
	// BeautyStart is a new pawn's beauty need (half full).
	BeautyStart = LevelFull / 2
)

// BeautyTarget is the level average beauty avgTenths (tenths of a point)
// pulls the need toward.
func BeautyTarget(avgTenths int) int32 {
	t := int64(LevelFull/2) + int64(avgTenths)*int64(BeautyPerPoint)/10
	return int32(max(0, min(int64(LevelFull), t)))
}

// BeautyStep is one look: level moves BeautyDrift toward target.
func BeautyStep(level, target int32) int32 {
	if level < target {
		return min(level+BeautyDrift, target)
	}
	return max(level-BeautyDrift, target)
}

// BeautyLadder is the beauty thoughts: below 15 % -15, below 30 % -10,
// below 42 % -5, none to 58 %, then +5, +10 and +15 above 58, 70 and 85 %
// (wiki: Thoughts; the two ugliest values from the Beauty page; the
// labels are original).
var BeautyLadder = Ladder{
	Up:    []int32{15 * LevelPct, 30 * LevelPct, 42 * LevelPct, 58 * LevelPct, 70 * LevelPct, 85 * LevelPct},
	Value: []int{-15, -10, -5, 0, 5, 10, 15},
	Label: []string{"hideous surroundings", "ugly surroundings", "unsightly surroundings", "", "pretty surroundings", "beautiful surroundings", "gorgeous surroundings"},
}
