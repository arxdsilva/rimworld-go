package industry

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// Host is everything the industry needs from the simulation, as
// consumer-owned roles (.cursor/SOLID_GO.md §7, §10).
type Host interface {
	Sites
	Ground
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

// Ground is the map's terrain.
type Ground interface {
	// Size is the map's width and height.
	Size() (w, h int)
	Terrain(x, y int) terrain.Type
	// SetTerrain changes a cell's ground (walkability, rooms, zones and
	// grass follow it).
	SetTerrain(x, y int, t terrain.Type)
	// Ocean reports open sea, which a moisture pump never dries.
	Ocean(x, y int) bool
	// StoneAt is the stone under (x, y): a deep drill with no ore brings
	// up its chunks.
	StoneAt(x, y int) geology.Stone
}

// Workers is the operators of a deep drill or a scanner.
type Workers interface {
	Pos(id entity.EntityID) (x, y int)
	// Free reports worker may take bench now: not held by anyone else,
	// not put off (SIM-027), its spot (sx, sy) walkable and not held.
	Free(worker, bench entity.EntityID, k job.Kind, sx, sy int) bool
	// Start starts a sitting of kind k at bench, the worker standing on
	// (sx, sy) for ticks, holding the bench and the spot.
	Start(worker, bench entity.EntityID, k job.Kind, sx, sy, ticks int) bool
	// Work is one working tick of id: the work units done (0 on a tick a
	// low mood idles), trained on Mining (mining) or Intellectual.
	Work(id entity.EntityID, mining bool) int
}

// Colony is the colony-wide inputs and outputs.
type Colony interface {
	// Seed is the colony's base seed (deep ore finds hash it).
	Seed() uint64
	// Drop sets n items of kind k down at (x, y), or beside it.
	Drop(k item.Kind, n, x, y int)
	// Letter posts a letter (and its message) at (x, y).
	Letter(title, text string, x, y int)
	// Changed is called after a change the views show (the view cache key).
	Changed()
}
