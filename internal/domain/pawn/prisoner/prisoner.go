package prisoner

// Mode is what the colony wants done with a prisoner (append only, saved).
type Mode uint8

const (
	None Mode = iota
	Recruit
	Reduce
	Release
	Execute
)

// State is one prisoner's state (saved). Resistance is in milli-points
// (1000 = 1.0).
type State struct {
	Mode       Mode
	Resistance int32
	NextVisit  uint64
	Captor     uint64
}

// Tuning (wiki-derived curves per the ENG-003 balance override).
const (
	// VisitInterval is the ticks between warden visits (6 hours).
	VisitInterval uint64 = 6 * 2500
	// MinInitialResistance and MaxInitialResistance bound a new prisoner's
	// resistance, milli-points (10 to 30 points).
	MinInitialResistance int32 = 10000
	MaxInitialResistance int32 = 30000
	// ResistanceUnit is 1.0 resistance in milli.
	ResistanceUnit int32 = 1000
)

func lerp(x, x0, x1, y0, y1 int32) int32 {
	return y0 + (y1-y0)*(x-x0)/(x1-x0)
}

// ResistanceFactor scales resistance reduction by the prisoner's opinion of
// the warden (centi-points, -10000..10000) and mood (0..100), permille.
// Opinion curve: -100 -> 500, 0 -> 1000, +100 -> 1500. Mood curve: 0 -> 200,
// 50 -> 1000, 100 -> 1500.
func ResistanceFactor(opinionCenti int32, mood int) int32 {
	op := min(max(opinionCenti, -10000), 10000)
	opF := 1000 + op*500/10000
	md := int32(min(max(mood, 0), 100))
	var mdF int32
	if md <= 50 {
		mdF = lerp(md, 0, 50, 200, 1000)
	} else {
		mdF = lerp(md, 50, 100, 1000, 1500)
	}
	return opF * mdF / 1000
}

// Reduction is the resistance (milli-points) one visit removes at
// negotiation ability and factor (both permille): 1.0 x negotiation x factor.
func Reduction(negotiation int32, factor int32) int32 {
	return ResistanceUnit * negotiation / 1000 * factor / 1000
}

// InitialResistance maps a roll (permille, 0..1000) to a new prisoner's
// resistance in milli-points.
func InitialResistance(roll int32) int32 {
	r := min(max(roll, 0), 1000)
	return MinInitialResistance + (MaxInitialResistance-MinInitialResistance)*r/1000
}

// RecruitChance is the permille chance a visit recruits a prisoner whose
// resistance is gone, for a warden of the given social skill (0..20).
func RecruitChance(social int) int32 {
	s := int32(min(max(social, 0), 20))
	return min(150+s*45, 1000)
}
