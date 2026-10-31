package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
)

// thermoHost implements thermo.Host (thermo/deps.go) over the game: one
// small adapter type per role, so Game gains no method (TEMP-1,
// .cursor/SOLID_GO.md §10). Heaters' and coolers' thermostats (PLAY-021)
// stay in power.go: their targets and working flags are saved there.
type thermoHost struct {
	thermoMap
	thermoSites
	thermoDevices
	thermoBodies
}

var _ thermo.Host = thermoHost{}

func newThermoHost(g *Game) thermoHost {
	return thermoHost{thermoMap{g}, thermoSites{g}, thermoDevices{g}, thermoBodies{g}}
}

type thermoMap struct{ g *Game }

func (h thermoMap) RoomGrid() ([]int, int, int) {
	return h.g.roomGrid, h.g.grid.Width(), h.g.grid.Height()
}
func (h thermoMap) RoofVer() uint64           { return h.g.roofVer() }
func (h thermoMap) Outdoor() temperature.Temp { return h.g.climate.Outdoor(h.g.clock.Now()) }
func (h thermoMap) Roof(i int) roof.Cell {
	if i < len(h.g.roofs) {
		return h.g.roofs[i]
	}
	return 0
}

type thermoSites struct{ g *Game }

func (h thermoSites) StructVer() uint64              { return h.g.structVer }
func (h thermoSites) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h thermoSites) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}

type thermoDevices struct{ g *Game }

func (h thermoDevices) Powered(id entity.EntityID) bool { return h.g.powered(id) }
func (h thermoDevices) Thermostat(id entity.EntityID) (int, bool) {
	return h.g.thermoTarget(id), h.g.thermoActive(id)
}
func (h thermoDevices) SetThermoActive(id entity.EntityID, on bool) { h.g.setThermoActive(id, on) }
func (h thermoDevices) Burning(id entity.EntityID) bool {
	b := h.g.bldgs.Ref(id)
	return b != nil && h.g.burning(id, *b)
}

// Open: a door open now (combat/door.go), a vent flicked open (a vent is
// a power toggle that draws nothing: power_flick.go, command.ToggleVent).
func (h thermoDevices) Open(id entity.EntityID) bool {
	b := h.g.bldgs.Ref(id)
	if b != nil && b.Kind == construction.Vent {
		return h.g.powerOn(id)
	}
	return b != nil && h.g.combat.Doors.IsOpen(id)
}

// Worked: a colonist at the bench with the ingredients in hand (acting).
func (h thermoDevices) Worked(id entity.EntityID) bool {
	for _, pid := range h.g.pawnIDs {
		if j, ok := h.g.jobs.Get(pid); ok && j.Kind == job.Bill && j.State == job.Do && j.Carry != 0 && j.Dest == id {
			return true
		}
	}
	return false
}

type thermoBodies struct{ g *Game }

func (h thermoBodies) AppendBodies(dst []thermo.Body) []thermo.Body {
	g := h.g
	for _, ids := range [2][]entity.EntityID{g.pawnIDs, g.raiderIDs} {
		for _, id := range ids {
			if p, ok := g.positions.Get(id); ok && g.onMap(id) {
				dst = append(dst, thermo.Body{X: p.X, Y: p.Y, Size: 1000, Human: true})
			}
		}
	}
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if p, at := g.positions.Get(id); ok && at {
			dst = append(dst, thermo.Body{X: p.X, Y: p.Y, Size: combat.BodySize(a)})
		}
	}
	return dst
}

func (h thermoBodies) AppendFires(dst []thermo.Fire) []thermo.Fire {
	for _, f := range h.g.fire.fires {
		dst = append(dst, thermo.Fire{X: f.X, Y: f.Y, Size: int(f.Size)})
	}
	return dst
}

// cmdSetThermostat sets heater or cooler target's target temperature
// (tenths of a degree C, clamped; anything else is ignored).
func (g *Game) cmdSetThermostat(target entity.EntityID, tenths int) {
	g.setThermoTarget(target, tenths)
}
