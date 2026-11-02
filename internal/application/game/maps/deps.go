package maps

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// Host is everything the caravans and the map lifecycle need from the
// simulation, as consumer-owned roles (.cursor/SOLID_GO.md §7, §10). The
// game package implements each role with a small adapter type. Calls act
// on the map the simulation is stepping when they are made.
type Host interface {
	Travellers
	Surroundings
	Cartography
}

// Travellers is the people on the road.
type Travellers interface {
	Crew
	Walkers
	Road
}

// Surroundings is where they go and what the player hears.
type Surroundings interface {
	Ground
	Globe
	Post
}

// Cartography is opening, emptying and changing maps.
type Cartography interface {
	Opener
	Populator
	Settler
	Founder
	Waylayer
}

// Crew is the colony's people as caravan members.
type Crew interface {
	// Candidate reports a colonist standing on the current map, not in a
	// mental break.
	Candidate(id entity.EntityID) bool
	Standing(id entity.EntityID) bool
	Breaking(id entity.EntityID) bool
	Alive(id entity.EntityID) bool
	Capacity(id entity.EntityID, c body.Capacity) int32
	Name(id entity.EntityID) string
	Drafted(id entity.EntityID) bool
	SetDraft(id entity.EntityID, on bool)
}

// Walkers is leaving and entering a map.
type Walkers interface {
	Job(id entity.EntityID) (job.Record, bool)
	// StartExit sends id to the nearest map edge (job.ExitMap).
	StartExit(id entity.EntityID) bool
	// Stash takes id off the map for holder; Unstash brings it back at
	// (x, y); Rehold hands a stashed id to another holder (a caravan).
	Stash(id, holder entity.EntityID) bool
	Unstash(id entity.EntityID, x, y int) bool
	IsStashed(id entity.EntityID) bool
	Rehold(id, holder entity.EntityID)
	// Move walks a pawn to (x, y).
	Move(id entity.EntityID, x, y int)
}

// Ground is where people walk in.
type Ground interface {
	// Edge is a walkable edge cell from which the colony can be reached.
	Edge() (x, y int, ok bool)
	// Home is the colony's centre.
	Home() (x, y int)
	// FreeLand is a free land cell within r of (x, y), avoiding used.
	FreeLand(x, y, r int, used [][2]int) (int, int, bool)
	WalkableNear(x, y, r int) (int, int, bool)
	// HomeSpot is the i-th walk-in spot around the colony's centre.
	HomeSpot(i int) (x, y int, ok bool)
	// Landing is the nearest cell to (x, y) a pod can come down on (x < 0:
	// the map's centre): walkable and not under overhead mountain (MAP-10).
	Landing(x, y int) (int, int, bool)
}

// Globe is the world beyond the map.
type Globe interface {
	// Planet is nil without a world map; ColonyTile is -1 then.
	Planet() *planet.Planet
	ColonyTile() int
	TileName(t int) string
	// Faction is the faction whose settlement stands on t (nil: none).
	Faction(t int) *faction.Faction
	// NewID hands out a fresh entity id (a caravan's).
	NewID() uint64
	Now() uint64
	// Cargo is game/travel's goods, pack animals, hitching spots and
	// mineral sites.
	Cargo() Freight
}

// Freight is what game/travel does for a caravan (AX-9).
type Freight interface {
	Pack(cid uint64, list string, members int)
	Hitch(cid uint64, member entity.EntityID, i int) bool
	Gather(cid uint64, members []entity.EntityID, formed, now uint64) bool
	Pace(cid uint64, members int) int
	// Goods is what caravan cid carries; Yield gives qty of k up (false:
	// it has less).
	Goods(cid uint64) []item.Stack
	Yield(cid uint64, k item.Kind, qty int) bool
	// Claim takes the mineral site on t off the world: its lump.
	Claim(t int) (item.Kind, int, bool)
	Home(cid uint64, x, y int)
}

// Post is the letters and messages.
type Post interface {
	// Letter posts a letter at the colony (caution: a bad one).
	Letter(caution bool, title, text string, focus entity.EntityID)
	// Note posts a plain message at the colony.
	Note(text string)
}

// Road is life on the road: hunger, talk and the medic.
type Road interface {
	// Starve drains id's hunger by an hour.
	Starve(id entity.EntityID)
	// Talk has a talk to b (k); each side gains joy recreation.
	Talk(a, b entity.EntityID, k social.Interaction, joy int, now uint64)
	// Roll draws from the world stream.
	Roll(n int) int
	// MedicLevel is id's Medicine level when it can doctor on the road,
	// else 0.
	MedicLevel(id entity.EntityID) int
	// TendDue reports id alive with care due; MayTend that medic may tend
	// it (care levels, self-tend); Tend does it.
	TendDue(id entity.EntityID, now uint64) bool
	MayTend(medic, id entity.EntityID) bool
	Tend(medic, id entity.EntityID, now uint64)
	// Species is a stashed tame animal's species (a mount, MAP-09).
	Species(id entity.EntityID) (animal.Species, bool)
}

// Opened is what Open made: the lump's ore cells and the generator's
// landing spot.
type Opened struct {
	Cells   []int
	Land    [2]int
	HasLand bool
}

// Opener opens, switches between and closes maps. Open and Close are
// called only from Flush, at the end of a tick.
type Opener interface {
	// Open makes map id (w×h, seed) on world tile and lays qty of the
	// lump on it as ore near the centre; false when it cannot. The current
	// map is unchanged.
	Open(id uint32, tile, w, h int, seed uint64, lump item.Kind, qty int) (Opened, bool)
	// Close removes map id and forgets everything left on it.
	Close(id uint32)
	// Current is the map being stepped; Switch makes map id current
	// (false: it is not open).
	Current() uint32
	Switch(id uint32) bool
}

// Populator reads and empties the current map.
type Populator interface {
	// Census counts the current map; cells are a lump's ore cells.
	Census(cells []int) Census
	// Aboard is our people (colonists, prisoners) and tame animals on the
	// current map, in id order.
	Aboard() (people, animals []entity.EntityID)
	// PackUp loads the current map's unforbidden goods onto caravan cid.
	PackUp(cid uint64)
	// Raid sends the storyteller's raid onto the current map.
	Raid()
	Seed() uint64
}
