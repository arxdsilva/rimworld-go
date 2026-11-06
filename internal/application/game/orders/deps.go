package orders

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything the orders need from the simulation: value-only
// roles the game implements with one small adapter (game/move.go,
// ordersHost), each at most eight methods (.cursor/SOLID_GO.md §10).
type Host interface {
	Buildings
	Hands
	Crates
	Blueprints
	Containers
}

// Buildings reads and marks the map's buildings.
type Buildings interface {
	// BuildingIDs is every building, ascending.
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	SetBuilding(id entity.EntityID, b construction.Building)
	// Seen reports (x, y) explored (not under the fog).
	Seen(x, y int) bool
	// Claimed: building id became the colony's (rooms, links, the Home
	// area follow it).
	Claimed(id entity.EntityID)
	// Message posts a quiet message at (x, y), a format and args (the
	// catalog's key, OPT-001).
	Message(x, y int, format string, args ...any)
	// Defending counts a settlement's defenders on their feet on this map
	// (Claim: its doors wait for the defeat, MAP-17).
	Defending() int
}

// Hands is a worker's job, its claims and its work.
type Hands interface {
	Pos(id entity.EntityID) (x, y int)
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	// Work is id's work done this tick (ticks; 0 while it pauses), with
	// its Construction skill when skilled.
	Work(id entity.EntityID, skilled bool) int
	// Spot is the walkable, reachable cell id works target cell (x, y)
	// from (ok false: none).
	Spot(id entity.EntityID, x, y int) (sx, sy int, ok bool)
	// Free reports target held by no one but id (a claim, RES-1) and not
	// put off for id's job kind k (SIM-027).
	Free(id, target entity.EntityID, k job.Kind) bool
	// Claim holds target for id (false: someone else holds it).
	Claim(id, target entity.EntityID) bool
	// Walk sends id, on job j, to (x, y) (false: no way there).
	Walk(id entity.EntityID, j job.Record, x, y int) bool
}

// Crates is the crated buildings.
type Crates interface {
	Stack(id entity.EntityID) (item.Stack, bool)
	// Pack takes building id off the map (no refund, its bills and
	// owners dropped) and lays crate st where it stood; the crate's id.
	Pack(id entity.EntityID, st item.Stack) entity.EntityID
	// Carry has pawn by pick crate id up.
	Carry(id, by entity.EntityID)
	// Fits reports kind of stuff placeable at (x, y) facing f (the
	// Architect's footprint rule, research aside), and why not.
	Fits(kind construction.Kind, stuff item.Kind, f construction.Facing, x, y int) (bool, string)
}

// Blueprints is the reinstall blueprints (frames whose Reinstall is a
// crate).
type Blueprints interface {
	// FrameIDs is every blueprint, ascending.
	FrameIDs() []entity.EntityID
	Frame(id entity.EntityID) (construction.Frame, bool)
	SetFrame(id entity.EntityID, fr construction.Frame)
	// AddFrame lays blueprint fr; its id.
	AddFrame(fr construction.Frame) entity.EntityID
	// DropFrame removes blueprint id (nothing to refund).
	DropFrame(id entity.EntityID)
	// Install finishes reinstall frame id with crate (the building takes
	// the crate's stuff, quality and wear; the crate is gone).
	Install(id, crate entity.EntityID)
}

// Containers is what opening and stripping act on.
type Containers interface {
	// Interred reports a grave or sarcophagus with someone in it.
	Interred(id entity.EntityID) bool
	// Exhume lifts the body out of grave id beside it.
	Exhume(id entity.EntityID)
	// OpenCasket marks sealed sleeper casket id to be opened (on) or
	// takes the mark off; Sealed reports one.
	OpenCasket(id entity.EntityID, on bool)
	Sealed(id entity.EntityID) bool
	// PeopleIn is every pawn standing or lying in the rectangle,
	// ascending.
	PeopleIn(minX, minY, maxX, maxY int) []entity.EntityID
	// Strippable reports a downed person (seen) wearing or holding
	// something.
	Strippable(id entity.EntityID) bool
}
