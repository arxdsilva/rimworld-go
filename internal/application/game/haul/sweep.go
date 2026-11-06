package haul

import (
	"slices"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// SweepHost is what the harvest sweep needs from the simulation (a
// consumer-owned role, .agents/specs/SOLID_GO.md §7). NearestRipe goes through
// the game's marked-plant index, so a sweep never walks the plants.
type SweepHost interface {
	Harvester
	// HarvestWork is the plants pawn id's work types take (priority above 0).
	HarvestWork(id entity.EntityID) Ripe
	// Position is pawn id's cell.
	Position(id entity.EntityID) (x, y int)
}

// Sweep is the colonists who just finished a harvest (HAUL-003, HRV-001):
// before any other work but their firefighting and doctoring they pick every
// ripe marked plant within Radius of where they stand, so a field is cleared
// before its piles are carried (and haul.Plan gathers them in one load). The
// marks are a sorted id list: deterministic to walk and to save.
type Sweep struct {
	armed []entity.EntityID
}

// Arm marks pawn id, which has just finished a harvest.
func (s *Sweep) Arm(id entity.EntityID) {
	if i, ok := slices.BinarySearch(s.armed, id); !ok {
		s.armed = slices.Insert(s.armed, i, id)
	}
}

// Armed reports whether pawn id is marked.
func (s *Sweep) Armed(id entity.EntityID) bool {
	_, ok := slices.BinarySearch(s.armed, id)
	return ok
}

// Disarm removes pawn id's mark, if any.
func (s *Sweep) Disarm(id entity.EntityID) {
	if i, ok := slices.BinarySearch(s.armed, id); ok {
		s.armed = slices.Delete(s.armed, i, i+1)
	}
}

// Next starts marked pawn id's next harvest: the nearest ripe marked plant
// its work types take (HRV-003), when it lies within Radius (Chebyshev, the
// plant search's own measure) of the pawn. It reports whether one started;
// otherwise the mark goes, so the sweep ends once and the pawn works on
// without a plant that ripens later pulling it back (hysteresis: only a
// finished harvest marks again). An unmarked pawn returns false at once.
func (s *Sweep) Next(h SweepHost, id entity.EntityID) bool {
	if !s.Armed(id) {
		return false
	}
	if takes := h.HarvestWork(id); takes != 0 {
		t, x, y, ok := h.NearestRipe(id, takes)
		px, py := h.Position(id)
		if ok && pathing.Dist(px, py, x, y) <= Radius && h.StartHarvest(id, t) {
			return true
		}
	}
	s.Disarm(id)
	return false
}

// Export is the marks for a save, ascending (nil when none).
func (s *Sweep) Export() []entity.EntityID {
	if len(s.armed) == 0 {
		return nil
	}
	return slices.Clone(s.armed)
}

// Import restores saved marks (none in an older save).
func (s *Sweep) Import(ids []entity.EntityID) {
	s.armed = nil
	if len(ids) > 0 {
		s.armed = slices.Compact(slices.Sorted(slices.Values(ids)))
	}
}
