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

// State is one prisoner's state (saved).
type State struct {
	Mode       Mode
	Resistance int32
	NextVisit  uint64
	Captor     uint64
}

// ResistanceFactor scales resistance reduction by the prisoner's opinion of
// the warden (centi-points) and mood, permille.
func ResistanceFactor(opinionCenti int32, mood int) int32 {
	return 0 // TODO(M8)
}

// Reduction is the resistance one visit removes at negotiation ability and
// factor (both permille).
func Reduction(negotiation int32, factor int32) int32 {
	return 0 // TODO(M8)
}

// RecruitChance is the permille chance a visit recruits a prisoner whose
// resistance is gone, for a warden of the given social skill.
func RecruitChance(social int) int32 {
	return 0 // TODO(M8)
}
