package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// Host is everything travel needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10).
type Host interface {
	Sites
	People
	Carriers
	Movers
	Goods
	Beasts
	Globe
	Colony
}

// Sites is the buildings, in building order (the id slice is the host's
// own, read only).
type Sites interface {
	// StructVer moves whenever a building is added, removed or changed.
	StructVer() uint64
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	Powered(id entity.EntityID) bool
	// Fuel and SetFuel are a launcher's chemfuel, in whole units.
	Fuel(id entity.EntityID) int
	SetFuel(id entity.EntityID, units int)
	// Remove takes a building away with no refund (a launched pod).
	Remove(id entity.EntityID)
	// Explode bursts d at (x, y) (a stasis casket blowing up).
	Explode(x, y int, d explosion.Def)
}

// People is the colony's pawns and its off-map stash.
type People interface {
	PawnIDs() []entity.EntityID
	Pos(id entity.EntityID) (x, y int)
	// Standing reports a pawn up and about; Downed one lying down alive.
	Standing(id entity.EntityID) bool
	Downed(id entity.EntityID) bool
	Name(id entity.EntityID) string
	// Stash takes pawn id off the map for holder (a casket, a pod), as
	// what it is (a prisoner stays one); Unstash brings it back at (x, y).
	Stash(id, holder entity.EntityID) bool
	Unstash(id entity.EntityID, x, y int) bool
}

// Carriers is carrying a downed pawn into a casket: the carry itself,
// the prisoners (listed among the raiders) who may be carried in, and the
// escape ship's caskets (game/ship, AX-8; LayInShip posts its message).
type Carriers interface {
	// Pick has carrier pick patient up; PutDown sets what it carries down
	// where it stands.
	Pick(carrier, patient entity.EntityID)
	PutDown(carrier entity.EntityID)
	RaiderIDs() []entity.EntityID
	IsPrisoner(id entity.EntityID) bool
	ShipCasketFree(id entity.EntityID) bool
	LayInShip(pawn, casket entity.EntityID) bool
	// Sicken gives a person out of a casket cryptosleep sickness.
	Sicken(id entity.EntityID)
}

// Movers is the pawns' jobs and walking.
type Movers interface {
	Job(id entity.EntityID) (job.Record, bool)
	// StartJob starts rec (pathing to TX, TY and claiming its Target).
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, rec job.Record)
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// BesideSpot is a reachable walkable cell beside (x, y) for id.
	BesideSpot(id entity.EntityID, x, y int) (int, int, bool)
	WalkableNear(x, y, r int) (int, int, bool)
	// Move walks drafted pawn id to (x, y).
	Move(id entity.EntityID, x, y int)
	Drafted(id entity.EntityID) bool
}

// Goods is the colony's item stacks.
type Goods interface {
	// Stock is the nearest usable stack of k for id, with its cell.
	Stock(id entity.EntityID, k item.Kind, qty int) (entity.EntityID, int, int, bool)
	// PickUp has id carry qty of stack (split off as needed): the carried
	// stack, its kind and how many.
	PickUp(id, stack entity.EntityID, qty int) (entity.EntityID, item.Kind, int, bool)
	Stack(id entity.EntityID) (item.Stack, bool)
	// Spend deletes a carried stack (it went into a pod).
	Spend(carry entity.EntityID)
	// DropStack sets a copy of st down at (x, y), or beside it.
	DropStack(st item.Stack, x, y int)
}

// Beasts is the map's animals.
type Beasts interface {
	AnimalIDs() []entity.EntityID
	Animal(id entity.EntityID) (animal.State, bool)
	// RemoveAnimal lifts animal id off the map (into a pod); PutAnimal sets
	// it back down at (x, y) under the same id.
	RemoveAnimal(id entity.EntityID)
	PutAnimal(id entity.EntityID, st animal.State, x, y int)
}

// Globe is the world beyond the map.
type Globe interface {
	Landings
	// Planet is the colony's planet (nil: no world map); ColonyTile its
	// tile (-1 none); Home the colony's drop spot on the map.
	Planet() *planet.Planet
	ColonyTile() int
	Home() (int, int)
	// Faction is the faction whose settlement stands on tile t (nil none)
	// and the tile's name.
	Faction(t int) (*faction.Faction, string)
	// Land sets members (stashed passengers) down on world tile t as a
	// caravan arrived there (animals ride with it): its id.
	Land(members, animals []uint64, t int) uint64
	// Assault has caravan cid (pod passengers just landed) attack the
	// settlement on its tile, entering at its map's edge or centre
	// (game/maps, MAP-05).
	Assault(cid uint64, centre bool)
	// Caravan reports caravan id still out on the world.
	Caravan(id uint64) bool
}

// Landings is pods coming down on the open maps and founding colonies
// (game/maps, MAP-10).
type Landings interface {
	// MapOpen reports a map open on tile t (the first colony's included).
	MapOpen(t int) bool
	// LandOn sets pod passengers (stashed) and animals down on the open map
	// at tile t (game/maps, MAP-10): at its centre, at spot (x, y), else its
	// edge. The carrier caravan's id (0: no map there).
	LandOn(members, animals []uint64, t int, centre, spot bool, x, y int) uint64
	// FoundReason is why a colony cannot be founded on tile t ("" it can);
	// Found has caravan cid found it (game/maps, MAP-06).
	FoundReason(t int) string
	Found(cid uint64) string
}

// Colony is the colony-wide inputs and outputs.
type Colony interface {
	Now() uint64
	Seed() uint64
	// Letter posts a letter (and its message) at (x, y); Message a plain
	// message.
	Letter(title, text string, x, y int)
	Message(x, y int, text string)
	// Changed is called after a change the views show.
	Changed()
	// Work is one working tick of id: work units done (Intellectual).
	Work(id entity.EntityID, mining bool) int
	// Free reports worker may take bench at its spot (sx, sy) now; Start
	// starts a sitting of kind k there for ticks.
	Free(worker, bench entity.EntityID, k job.Kind, sx, sy int) bool
	Start(worker, bench entity.EntityID, k job.Kind, sx, sy, ticks int) bool
}
