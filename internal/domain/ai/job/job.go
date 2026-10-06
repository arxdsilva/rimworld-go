// Package job is the pawn job record and finite state machine kinds.
package job

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// State is the job FSM. Zero is Idle (no job).
type State uint8

const (
	Idle State = iota
	Goto
	Do
	Complete
	Fail
)

func (s State) String() string {
	switch s {
	case Goto:
		return "goto"
	case Do:
		return "do"
	case Complete:
		return "complete"
	case Fail:
		return "fail"
	default:
		return "idle"
	}
}

// Kind is what the pawn is trying to accomplish.
type Kind uint8

const (
	None Kind = iota
	Move
	Eat
	Sleep
	Haul
	Construct
	Harvest
	Deconstruct
	Cut
	Mine
	Sow
)

func (k Kind) String() string {
	switch k {
	case Move:
		return "goto"
	case Eat:
		return "eat"
	case Sleep:
		return "sleep"
	case Haul:
		return "haul"
	case Construct:
		return "construct"
	case Harvest:
		return "harvest"
	case Deconstruct:
		return "deconstruct"
	case Cut:
		return "cut"
	case Mine:
		return "mine"
	case Sow:
		return "sow"
	default:
		return "idle"
	}
}

// FailReason is a typed gameplay outcome, not a Go error.
type FailReason uint8

const (
	FailNone FailReason = iota
	Unreachable
	TargetGone
	Interrupted
	Cancelled
	Reserved
)

func (r FailReason) String() string {
	switch r {
	case Unreachable:
		return "unreachable"
	case TargetGone:
		return "target gone"
	case Interrupted:
		return "interrupted"
	case Cancelled:
		return "cancelled"
	case Reserved:
		return "reserved"
	default:
		return ""
	}
}

// Record is the single current job on a pawn.
type Record struct {
	Kind     Kind
	State    State
	Target   entity.EntityID
	TX, TY   int
	Carry    entity.EntityID
	DestX    int
	DestY    int
	Path     []pathing.Point
	PathI    int
	WorkLeft int
	Fail     FailReason
	// Dest is the entity a haul delivers into (a frame); 0 = drop on tile.
	Dest entity.EntityID
	// Forced marks a player-prioritized job: tiredness does not
	// interrupt it.
	Forced bool
}

// Line is the inspector job summary.
func (r Record) Line() string {
	if r.Kind == None || r.State == Idle {
		return "idle"
	}
	s := r.Kind.String()
	if r.State == Fail && r.Fail != FailNone {
		return s + " (" + r.Fail.String() + ")"
	}
	if r.State == Goto {
		return s + " " + r.State.String()
	}
	return s + " " + r.State.String()
}

// Active reports the pawn still has work in Goto or Do.
func (r Record) Active() bool {
	return r.Kind != None && (r.State == Goto || r.State == Do)
}
