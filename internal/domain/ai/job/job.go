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
	Bill
	Hunt
	Attack
	Equip
	Rescue
	Tend
	// Refuel carries wood to a campfire.
	Refuel
	// Warm walks a freezing (or overheating) pawn somewhere bearable.
	Warm
	// Idle behaviours: free time with nothing designated to do.
	Wander
	Chat
	Relax    // sit on a chair
	FireSide // warm hands by a burning campfire
	Horseshoes
	// Hide: a mental break spent shut away in a room.
	Hide
	// Breach: a raider bashing through a door or wall.
	Breach
	// Repair: a builder mending a damaged building.
	Repair
	// Research: studying at a research bench.
	Research
	// Tame: coaxing a wild animal into the colony.
	Tame
	// Bury: carrying a body to an empty grave and filling it in.
	Bury
	// Roof: building the roof over a planned tile (SIM-018).
	Roof
	// Unroof: taking down the roof over a tile planned for removal.
	Unroof
	// Clean: sweeping dirt and stains off a Home-area tile (SIM-021).
	Clean
	// Milk and Shear collect a tame animal's product (SIM-026; ember oil
	// is collected with Milk).
	Milk
	Shear
	// Smooth: working a rock wall or stone floor smooth (SIM-029).
	Smooth
	// Social and host systems (ENG-003, append only, saved): surgery,
	// taking a drug, intimacy, joining a gathering, a social fight,
	// capturing a downed pawn, wardening, carrying food to a prisoner,
	// socialising, training an animal, arresting, a raider kidnapping,
	// walking off the map, tending an animal and a visit.
	Surgery
	Ingest
	Lovin
	Gather
	Brawl
	Capture
	Warden
	DeliverFood
	SocialRelax
	Train
	Arrest
	Kidnap
	ExitMap
	TendAnimal
	Visit
	// PLAY-015: lead a penned animal to its pen; beat out a fire; shovel
	// snow off a cell of the snow clearing area.
	Rope
	FightFire
	ClearSnow
)

// IsIdle reports a free-time behaviour that any real work may replace.
func (k Kind) IsIdle() bool {
	return k == Wander || k == Chat || k == Relax || k == FireSide || k == Horseshoes || k == SocialRelax || k == Gather
}

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
	case Smooth:
		return "smooth"
	case Sow:
		return "sow"
	case Bill:
		return "work at bench"
	case Hunt:
		return "hunt"
	case Attack:
		return "attack"
	case Equip:
		return "equip"
	case Rescue:
		return "rescue"
	case Tend:
		return "tend"
	case Refuel:
		return "refuel"
	case Warm:
		return "seek warmth"
	case Wander:
		return "wander"
	case Chat:
		return "chat"
	case Relax:
		return "relax"
	case FireSide:
		return "warm up by the fire"
	case Horseshoes:
		return "play horseshoes"
	case Hide:
		return "hide in room"
	case Breach:
		return "break through"
	case Repair:
		return "repair"
	case Research:
		return "research"
	case Tame:
		return "tame"
	case Bury:
		return "bury"
	case Roof:
		return "build roof"
	case Unroof:
		return "remove roof"
	case Clean:
		return "clean"
	case Milk:
		return "milk"
	case Shear:
		return "shear"
	case Surgery:
		return "operate"
	case Ingest:
		return "take drug"
	case Lovin:
		return "make love"
	case Gather:
		return "join gathering"
	case Brawl:
		return "brawl"
	case Capture:
		return "capture"
	case Warden:
		return "warden"
	case DeliverFood:
		return "deliver food"
	case SocialRelax:
		return "socialise"
	case Train:
		return "train"
	case Arrest:
		return "arrest"
	case Kidnap:
		return "kidnap"
	case ExitMap:
		return "leave the map"
	case TendAnimal:
		return "tend animal"
	case Visit:
		return "visit"
	case Rope:
		return "rope animal"
	case FightFire:
		return "fight fire"
	case ClearSnow:
		return "clear snow"
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
	// Blocked: other creatures kept the way shut for too long.
	Blocked
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
	case Blocked:
		return "blocked"
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
	// Count is how many items the job fetches (refuelling).
	Count int
	// Dest is the entity a haul delivers into (a frame); 0 = drop on tile.
	Dest entity.EntityID
	// Forced marks a player-prioritized job: tiredness does not
	// interrupt it.
	Forced bool
	// BillIndex is the bench bill a Bill job works on.
	BillIndex int
	// Route (HAUL-001) is a delivery's targets in order (frames, or a fuel
	// user), RouteI the next one; SpareX/SpareY the stockpile cell that
	// takes what's left (HasSpare).
	Route          []entity.EntityID
	RouteI         int
	SpareX, SpareY int
	HasSpare       bool
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
