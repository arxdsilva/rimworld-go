// Package allow is the colony's allow system (STK-AL,
// .agents/gaps/ALLOW_GAP_ANALYSIS.md): forbidding and allowing item stacks and
// buildings, the auto-forbid rule for what the dead leave behind, and the
// player's orders that clear the flag (the Haul things mark, an ordered
// pick-up). Allowed areas stay with the game (areas.go); this package
// holds no state, so the game builds a Forbids on demand around its host.
//
// The vanilla rules (wiki: Door, Orders, Zone/Area; game behaviour where
// the wiki's Forbidden page is missing, see the gap analysis):
//   - a thing that falls when a pawn or animal dies (its body, weapon, a
//     thief's haul) starts forbidden when it lands outside the home area;
//     a hunter's kill does not, its killer takes it (a colonist hunting
//     that prey, a tame animal on its hunt); a wild predator's kill is a
//     death like any other, which it holds by reservation only while it
//     feeds (AL-1, AL-2);
//   - loot found on the map (ruins, vaults, caskets) starts forbidden;
//   - the Haul things mark and an order on the stack allow it.
package allow

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything Forbids needs from the simulation, as consumer-owned
// roles (.agents/specs/SOLID_GO.md §7, §10). Values only.
type Host interface {
	Stacks
	Buildings
	Ground
}

// Stacks reads and writes item stacks.
type Stacks interface {
	// ItemIDs is every stack id, the host's own slice: read only.
	ItemIDs() []entity.EntityID
	Stack(id entity.EntityID) (item.Stack, bool)
	// PutStack sets stack id down where it lies (merging with a like
	// stack there).
	PutStack(id entity.EntityID, st item.Stack)
}

// Buildings reads and writes buildings.
type Buildings interface {
	Building(id entity.EntityID) (construction.Building, bool)
	SetBuilding(id entity.EntityID, b construction.Building)
	// DoorChanged tells the host door b's forbidden flag flipped (a
	// forbidden door is shut to colonists: walkability).
	DoorChanged(b construction.Building)
}

// Ground answers questions about tiles.
type Ground interface {
	// InHome: (x, y) lies in the home area.
	InHome(x, y int) bool
	// Seen: (x, y) is explored (not under the fog of war).
	Seen(x, y int) bool
}

// Forbids applies the allow rules over a host.
type Forbids struct{ h Host }

// New returns the rules over h.
func New(h Host) Forbids { return Forbids{h: h} }

// Forbidden reports whether stack id is forbidden.
func (f Forbids) Forbidden(id entity.EntityID) bool {
	st, ok := f.h.Stack(id)
	return ok && st.Forbidden
}

// Set forbids (on) or allows thing id: a ground stack or a building
// (BED-1 beds, DEF-3 doors). A carried stack keeps its flag.
func (f Forbids) Set(id entity.EntityID, on bool) {
	if b, ok := f.h.Building(id); ok {
		if b.Forbidden == on {
			return
		}
		b.Forbidden = on
		f.h.SetBuilding(id, b)
		if b.Kind.IsDoor() {
			f.h.DoorChanged(b)
		}
		return
	}
	st, ok := f.h.Stack(id)
	if !ok || st.Carrier != 0 || st.Forbidden == on {
		return
	}
	st.Forbidden = on
	f.h.PutStack(id, st)
}

// Rect forbids or allows every explored ground stack in the rectangle
// (the Architect's Forbid and Allow tools).
func (f Forbids) Rect(x1, y1, x2, y2 int, on bool) {
	f.each(x1, y1, x2, y2, func(id entity.EntityID, st item.Stack) {
		if st.Forbidden != on {
			f.Set(id, on)
		}
	})
}

// Mark marks (on) or unmarks ground stack id for hauling (the item's
// Mark for hauling button). Marking allows it too (vanilla Haul things);
// only a stack found on the map takes the mark itself, the colony's own
// is hauled anyway.
func (f Forbids) Mark(id entity.EntityID, on bool) {
	st, ok := f.h.Stack(id)
	if !ok || st.Carrier != 0 {
		return
	}
	forbidden, marked := st.Forbidden && !on, st.HaulMarked
	if st.Unclaimed {
		marked = on
	}
	if forbidden != st.Forbidden || marked != st.HaulMarked {
		st.Forbidden, st.HaulMarked = forbidden, marked
		f.h.PutStack(id, st)
	}
}

// HaulRect is the Haul things tool over the rectangle: every explored
// ground stack in it is allowed and, when found on the map, marked; off
// (Cancel) only unmarks, fog or not.
func (f Forbids) HaulRect(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range append([]entity.EntityID(nil), f.h.ItemIDs()...) {
		st, ok := f.h.Stack(id)
		if !ok || st.Carrier != 0 || st.X < minX || st.X > maxX || st.Y < minY || st.Y > maxY || on && !f.h.Seen(st.X, st.Y) {
			continue
		}
		if on && (st.Forbidden || st.Unclaimed && !st.HaulMarked) || !on && st.HaulMarked {
			f.Mark(id, on)
		}
	}
}

// Ordered clears the flag of stack id the player just ordered a colonist
// to pick up, eat, wear or haul (vanilla: an order allows its target).
func (f Forbids) Ordered(id entity.EntityID) {
	if f.Forbidden(id) {
		f.Set(id, false)
	}
}

// DropForbidden reports whether a thing a death leaves at (x, y) starts
// forbidden: outside the home area (vanilla SetForbiddenIfOutsideHomeArea).
func (f Forbids) DropForbidden(x, y int) bool { return !f.h.InHome(x, y) }

// CorpseForbidden is DropForbidden for a body, except a kill its hunter
// takes (taken is true; the game's killTaken says which).
func (f Forbids) CorpseForbidden(x, y int, taken bool) bool {
	return !taken && f.DropForbidden(x, y)
}

// each calls fn for every explored ground stack in the rectangle, over a
// copy of the id list (fn may merge stacks away).
func (f Forbids) each(x1, y1, x2, y2 int, fn func(entity.EntityID, item.Stack)) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range append([]entity.EntityID(nil), f.h.ItemIDs()...) {
		st, ok := f.h.Stack(id)
		if ok && st.Carrier == 0 && st.X >= minX && st.X <= maxX && st.Y >= minY && st.Y <= maxY && f.h.Seen(st.X, st.Y) {
			fn(id, st)
		}
	}
}
