package snow

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// Host is everything the layer needs from the simulation, as
// consumer-owned roles (.cursor/SOLID_GO.md §7, §10).
type Host interface {
	Sky
	Ground
	Shovellers
	Claims
}

// Sky is the clock and the weather over the map.
type Sky interface {
	Now() clock.Tick
	Weather() weather.State
	// SetMapDepth sets the weather's map figure (weather.State.SnowDepth).
	SetMapDepth(d float64)
	// WeatherHeld reports forced weather: it starts no new block.
	WeatherHeld() bool
	// Outdoor is the outdoor air at tick t.
	Outdoor(t clock.Tick) temperature.Temp
}

// Ground is the map the layer lies on.
type Ground interface {
	// Size is the map's width and height.
	Size() (w, h int)
	// EnsureRooms brings the room partition up to date.
	EnsureRooms()
	// Cover is the room id, roof and terrain of every cell, row-major: the
	// host's own slices, read only. The first two may be shorter than the
	// map.
	Cover() (rooms []int, roofs []roof.Cell, cells []terrain.Type)
	// UnderSky reports a tile open to the sky.
	UnderSky(x, y int) bool
	// Terrain is the ground type on (x, y).
	Terrain(x, y int) terrain.Type
}

// Shovellers is what a colonist clearing snow can do and reach.
type Shovellers interface {
	// Pos is a colonist's tile (0, 0 when it has none).
	Pos(id entity.EntityID) (x, y int)
	// Allowed reports (x, y) inside id's allowed area.
	Allowed(id entity.EntityID, x, y int) bool
	// PostponedFor reports job kind on (x, y) recently failed for id.
	PostponedFor(id entity.EntityID, kind job.Kind, x, y int) bool
	// PostponeTile holds job kind on (x, y) back from id for a while.
	PostponeTile(id entity.EntityID, kind job.Kind, x, y int)
	// ReachableSpot is a tile from which id can work on (x, y).
	ReachableSpot(id entity.EntityID, x, y int) (tx, ty int, ok bool)
	// Work is the labour id does this tick (0: none).
	Work(id entity.EntityID) int
}

// Claims is the job and tile a colonist holds.
type Claims interface {
	// TileTaken reports a claim on (x, y).
	TileTaken(x, y int) bool
	// ClaimTile holds (x, y) for id.
	ClaimTile(id entity.EntityID, x, y int)
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
}

// Steps is the simulation's cadence, in ticks.
type Steps struct {
	Climate      uint64 // the climate interval
	WeatherBlock uint64 // the length of a weather block
}
