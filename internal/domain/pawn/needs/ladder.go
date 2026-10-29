package needs

// The comfort, beauty and outdoors needs (BED-3) are levels in millionths
// of the need, fine enough to hold the wiki's per-150-tick rates exactly
// (outdoors under a mountain loses 0.1125 % a step, 0.0225 % in bed).
const (
	// LevelFull is a full comfort, beauty or outdoors need.
	LevelFull int32 = 1_000_000
	// LevelPct is one percent of a need.
	LevelPct int32 = LevelFull / 100
	// StageMargin is how far past a boundary a level must climb before
	// its thought moves up a stage (it falls back as soon as it drops
	// below): the hysteresis that keeps a level wobbling on a boundary
	// from flipping the thought (original tuning).
	StageMargin = 2 * LevelPct
)

// LevelPercent is level as a whole percent, 0..100.
func LevelPercent(level int32) int { return int(clampLevel(level) / LevelPct) }

func clampLevel(v int32) int32 { return max(0, min(LevelFull, v)) }

// Ladder is a need's mood thoughts by level: stage i runs from Up[i-1]
// (inclusive) to Up[i]; Value[i] and Label[i] are its thought (Value 0:
// none). The wiki's "above x" boundaries are read as "at least x", a
// millionth of a need apart.
type Ladder struct {
	Up    []int32
	Value []int
	Label []string
}

// Stages is how many stages the ladder has.
func (l *Ladder) Stages() int { return len(l.Up) + 1 }

// raw is the stage level falls in, with no hysteresis.
func (l *Ladder) raw(level int32) int {
	s := 0
	for s < len(l.Up) && level >= l.Up[s] {
		s++
	}
	return s
}

// Stage is the stage for level coming from stage cur: it falls as soon as
// level drops below cur's lower boundary, but climbs only once level
// clears a boundary by StageMargin, so a level wobbling on a boundary
// keeps its thought. A cur outside the ladder (no stage yet) takes the
// raw stage.
func (l *Ladder) Stage(cur int, level int32) int {
	r := l.raw(level)
	if cur < 0 || cur >= l.Stages() || r <= cur {
		return r
	}
	for cur < len(l.Up) && level >= min(l.Up[cur]+StageMargin, LevelFull) {
		cur++
	}
	return cur
}
