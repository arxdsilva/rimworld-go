package research

import (
	"rimworld-go/internal/domain/building/construction"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything research needs from the simulation, as
// consumer-owned roles (.cursor/SOLID_GO.md §7, §10).
type Host interface {
	Sites
	Rooms
	Workers
	Colony
}

// Sites is the buildings, in building order (the id slice is the host's
// own, read only).
type Sites interface {
	// StructVer moves whenever a building is added, removed or changed.
	StructVer() uint64
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	// Powered reports an electric building on power now.
	Powered(id entity.EntityID) bool
}

// Rooms is what a bench stands in.
type Rooms interface {
	// Room is the room at (x, y) and whether it is indoors (false:
	// outdoors, or a room that feels outdoors).
	Room(x, y int) (r int, indoors bool)
	// Cleanliness is indoor room r's cleanliness, hundredths.
	Cleanliness(r int) int
	// Contents is indoor room r's beds and bill benches.
	Contents(r int) (beds, benches int)
	// TemperatureAt is the air at (x, y), tenths of a degree C.
	TemperatureAt(x, y int) int
	// ReadingBonus is indoor room r's reading bonus, permille over 1000
	// (AX-7, game/books: books shelved in its bookcases).
	ReadingBonus(r int) int
}

// Workers is the researchers and their job.
type Workers interface {
	Pos(id entity.EntityID) (x, y int)
	// Free reports worker may take bench now: not held by anyone else,
	// not put off (SIM-027), its spot (sx, sy) walkable and not held.
	Free(worker, bench entity.EntityID, sx, sy int) bool
	// Start starts a research sitting of ticks at bench, the worker
	// standing on (sx, sy), and holds the spot.
	Start(worker, bench entity.EntityID, sx, sy, ticks int) bool
	// Work is one working tick of research by id: false on a tick a
	// low mood idles; it trains Intellectual.
	Work(id entity.EntityID) bool
	// Speed is id's research speed inputs.
	Speed(id entity.EntityID) Pawn
	// Researcher reports a free colonist who can and may research.
	Researcher() bool
}

// Colony is the colony-wide settings and outputs.
type Colony interface {
	// Tech is the colony's tech level (its scenario faction's).
	Tech() proj.TechLevel
	// CostPct is the scenario's research cost parts, percent.
	CostPct() int
	// Difficulty is the research difficulty factor, percent.
	Difficulty() int
	Now() uint64
	// Letter posts a letter (and its message) at (x, y).
	Letter(title, text string, x, y int)
	// Changed is called after a hook changes research outside a tick
	// (the view cache key).
	Changed()
}

// Pawn is a researcher's research speed inputs (wiki: Research speed):
// the Intellectual level, the global work speed (permille, the body's
// manipulation left out), manipulation and sight (permille) and the
// trait factor on Intellectual work (percent).
type Pawn struct {
	Intellectual             uint8
	GlobalWork, Manipulation int
	Sight, TraitPct          int
}
