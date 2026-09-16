// Package health is pawn vitality: alive, downed, or dead.
package health

// Status is a pawn's life state.
type Status uint8

const (
	Alive Status = iota
	Downed
	Dead
)

func (s Status) String() string {
	switch s {
	case Downed:
		return "downed"
	case Dead:
		return "dead"
	default:
		return "alive"
	}
}

// Cause is why a pawn is downed or dead (player-facing inspector text).
type Cause uint8

const (
	CauseNone Cause = iota
	CauseNoFood
	CauseNoBed
)

func (c Cause) String() string {
	switch c {
	case CauseNoFood:
		return "no food"
	case CauseNoBed:
		return "no bed"
	default:
		return ""
	}
}

// Record is vitality plus a typed cause.
type Record struct {
	Status     Status
	Cause      Cause
	DownedTick uint64
}

const (
	// StarvationTicks after downed-from-hunger until death.
	StarvationTicks = 2500
)
