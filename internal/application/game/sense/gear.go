package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// GearRoles is the game's side of Gear (adapters: game/sense_gear_host.go). Each
// role takes and returns values only; a new need is a new embedded role.
type GearRoles interface {
	GearPawns
	GearStock
}

// Gear is what a colonist carries off the map: the pack, spare meals, and
// what it has been told to put away (CSE-005, 036…039).
type Gear struct {
	s *Sense
	r GearRoles
}

func newGear(s *Sense, r GearRoles) Gear { return Gear{s: s, r: r} }

// BackToPack puts a carried stack back in the pack it came from; false: it
// did not come from one, and the caller drops it as usual.
func (g Gear) BackToPack(id, carry entity.EntityID) bool { return false }

// Mark is command.MarkUnload: mode 0 pack entry, 1 apparel, 2 weapon, 3 spare.
func (g Gear) Mark(pawn entity.EntityID, mode, index int, enabled bool) {}

// DoUnload runs one tick of job.UnloadPack.
func (g Gear) DoUnload(id entity.EntityID, j job.Record) {}

// DoSpare runs one tick of job.TakeSpare.
func (g Gear) DoSpare(id entity.EntityID, j job.Record) {}
