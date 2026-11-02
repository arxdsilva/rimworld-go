// Package orders is the Architect's remaining vanilla orders (AX-1,
// .cursor/ARCHITECT_GAP_ANALYSIS.md Orders): uninstall a building into a
// crate and reinstall it, strip downed people, open graves and caskets,
// claim abandoned buildings, and the pure rules of chop wood, slaughter
// and remove floor.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package; the roles
// in deps.go are its only way in. It keeps no clock and no dice, and walks
// its marks in ascending id order, so it is deterministic.
package orders

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Orders is the root, built by New: components as fields; each feature's
// entry points go on its component.
type Orders struct {
	h         Host
	Uninstall *Uninstall
	Install   *Install
	Strip     *Strip
	Open      *Open
	Claim     *Claim
}

// Save is the marks a save keeps (gob: field names are the format). The
// uninstall marks live on the buildings and the reinstall blueprints are
// frames, so only these two lists are the orders' own.
type Save struct {
	Strip []entity.EntityID // downed people marked to strip
	Open  []entity.EntityID // graves and sarcophagi marked to open
}

// New builds the orders on host h. The uninstall and install indexes are
// derived: they are rebuilt from the buildings and frames on first use.
func New(h Host) *Orders {
	o := &Orders{h: h}
	o.Uninstall = &Uninstall{h: h}
	o.Install = &Install{h: h}
	o.Strip = &Strip{h: h}
	o.Open = &Open{h: h}
	o.Claim = &Claim{h: h}
	return o
}

// Export is the marks to save (nil when there are none).
func (o *Orders) Export() Save {
	return Save{Strip: slices.Clone(o.Strip.marks), Open: slices.Clone(o.Open.marks)}
}

// Import restores saved marks (an older save has none).
func (o *Orders) Import(s Save) {
	o.Strip.marks = sortedSet(s.Strip)
	o.Open.marks = sortedSet(s.Open)
}

// Cancel takes every orders mark off the rectangle (the Cancel order):
// uninstall, strip and open marks. Blueprints are the game's to cancel.
func (o *Orders) Cancel(minX, minY, maxX, maxY int) {
	o.Uninstall.Mark(minX, minY, maxX, maxY, false)
	o.Strip.Mark(minX, minY, maxX, maxY, false)
	o.Open.Mark(minX, minY, maxX, maxY, false)
}

// Do runs one tick of an orders job and reports whether j was one.
func (o *Orders) Do(id entity.EntityID, j job.Record) bool {
	switch j.Kind {
	case job.Uninstall:
		o.Uninstall.Do(id, j)
	case job.Install:
		o.Install.Do(id, j)
	case job.OpenContainer:
		o.Open.Do(id, j)
	default:
		return false
	}
	return true
}

// AssignConstruct is the Construction work giver after building and
// deconstructing: uninstall, then carry a crate to its blueprint.
func (o *Orders) AssignConstruct(id entity.EntityID) bool {
	return o.Uninstall.Assign(id) || o.Install.Assign(id)
}

// AssignHaul is the hauler's giver for the basic chores: open marked
// containers, then strip marked people (vanilla's basic work, which the
// colony's haulers do here).
func (o *Orders) AssignHaul(id entity.EntityID) bool {
	return o.Open.Assign(id) || o.Strip.Assign(id)
}

// sortedSet is ids sorted, without repeats or zeros.
func sortedSet(ids []entity.EntityID) []entity.EntityID {
	out := slices.Clone(ids)
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > 0 && out[0] == 0 {
		out = out[1:]
	}
	return out
}

// setMark adds (on) or removes id in sorted set s.
func setMark(s []entity.EntityID, id entity.EntityID, on bool) []entity.EntityID {
	i, found := slices.BinarySearch(s, id)
	switch {
	case on && !found:
		return slices.Insert(s, i, id)
	case !on && found:
		return slices.Delete(s, i, i+1)
	}
	return s
}

// marked reports id in sorted set s.
func marked(s []entity.EntityID, id entity.EntityID) bool {
	_, found := slices.BinarySearch(s, id)
	return found
}

// in reports (x, y) inside the rectangle.
func in(x, y, minX, minY, maxX, maxY int) bool {
	return x >= minX && x <= maxX && y >= minY && y <= maxY
}

// dist is the Chebyshev distance between two cells (the nearest-first
// order of every giver here).
func dist(ax, ay, bx, by int) int { return max(abs(ax-bx), abs(ay-by)) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// finish ends job j of id with state s.
func finish(h Hands, id entity.EntityID, j job.Record, s job.State, why job.FailReason) {
	j.State, j.Fail = s, why
	h.SetJob(id, j)
}
