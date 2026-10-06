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
	CauseInjury
)

func (c Cause) String() string {
	switch c {
	case CauseNoFood:
		return "no food"
	case CauseNoBed:
		return "no bed"
	case CauseInjury:
		return "injured"
	default:
		return ""
	}
}

// Record is vitality plus a typed cause.
type Record struct {
	Status     Status
	Cause      Cause
	DownedTick uint64
	// Injury is damage taken; the zero value is full health.
	Injury int
}

const (
	// StarvationTicks after downed-from-hunger until death.
	StarvationTicks = 2500
)

// Injury thresholds for pawns (hit points out of MaxHP).
const (
	MaxHP = 100
	// DownedInjury downs a pawn; RecoverInjury lets it stand again.
	DownedInjury  = 70
	RecoverInjury = 55
	// HealTicks is how often one point of injury heals (awake, up);
	// resting in bed heals twice as fast, lying downed half as fast.
	HealTicks = 300
)

// HP is remaining hit points.
func (r Record) HP() int {
	hp := MaxHP - r.Injury
	if hp < 0 {
		return 0
	}
	return hp
}
