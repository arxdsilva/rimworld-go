package thermo

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
)

// Host is everything the model needs from the simulation, as
// consumer-owned roles (.agents/specs/SOLID_GO.md §7, §10).
type Host interface {
	Map
	Sites
	Devices
	Bodies
}

// Map is the room partition and the sky over it.
type Map interface {
	// RoomGrid is the room id of every cell (room.Wall, room.Outdoors),
	// row-major, w wide: the host's own slice, read only.
	RoomGrid() (grid []int, w, h int)
	// Roof is cell i's roof.
	Roof(i int) roof.Cell
	// RoofVer moves whenever a cell gains or loses its roof.
	RoofVer() uint64
	// Outdoor is the outdoor air now.
	Outdoor() temperature.Temp
}

// Sites is the buildings, in building order. The id slice is the host's
// own, read only.
type Sites interface {
	// StructVer moves whenever a building is added, removed or changed.
	StructVer() uint64
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
}

// Devices is the live state of what moves heat or air.
type Devices interface {
	// Burning reports a fuel burner alight now (fuelled, not doused, not
	// banked; a generator running).
	Burning(id entity.EntityID) bool
	// Powered reports an electric device on power now.
	Powered(id entity.EntityID) bool
	// Open reports a door or vent letting air through now.
	Open(id entity.EntityID) bool
	// Worked reports a bench someone is working at now.
	Worked(id entity.EntityID) bool
	// Thermostat is heater or cooler id's target (tenths of a degree C)
	// and whether its band has it working.
	Thermostat(id entity.EntityID) (target int, active bool)
	SetThermoActive(id entity.EntityID, on bool)
}

// Bodies lists what is warm and moves: creatures and fires.
type Bodies interface {
	// AppendBodies appends every living creature on the map.
	AppendBodies(dst []Body) []Body
	// AppendFires appends every burning fire.
	AppendFires(dst []Fire) []Fire
}

// Body is a creature's heat: where it is, its body size (thousandths:
// 1000 a human) and whether it is humanlike.
type Body struct {
	X, Y  int
	Size  int
	Human bool
}

// Fire is one burning fire (fire.Fire's size scale, 1..100).
type Fire struct{ X, Y, Size int }

// Changes is an incremental room rebuild (region.Rooms): the cells whose
// room changed, the old room a new one is identical to (-1: none), and a
// new room's cells.
type Changes interface {
	EachChanged(fn func(i int))
	Same(r int) int
	EachCell(r int, fn func(i int))
}
