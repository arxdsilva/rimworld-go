package combat

import (
	"cmp"
	"math/bits"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/door"
	"rimworld-go/internal/domain/sim/entity"
)

// Doors (DEF-3): each door's open state (domain building/door). A
// creature stepping into a door opens it and waits for it (Wait, the
// game's step hook), at the speed its stuff gives (an autodoor four times
// faster while powered); it closes door.StayOpen ticks after the last
// one went through, never on someone standing in it, unless the player
// holds it open. An open door lets shots and sight through (Sight.blocks).
// Only doors that are open are stepped: no door list, no scan.
//
// The game's other hooks: HeldOpen (a door hostiles may walk through:
// its shut-door set) and Barred (a forbidden door, shut to colonists: its
// walkability).
type Doors struct {
	h      Host
	pw     Power
	states map[entity.EntityID]door.State // doors not shut (saved)
	near   []Seen
}

// doorListEvery: the mortar list is trusted this long (a new mortar
// starts working within it). original tuning.
const doorListEvery = 120

// DoorSave is one door's state on disk.
type DoorSave struct {
	ID    entity.EntityID
	State door.State
}

// phase closes the open doors left alone long enough (one standing in
// the doorway keeps it open).
func (d *Doors) phase(now uint64) {
	for id, s := range d.states {
		b, ok := d.h.Building(id)
		switch {
		case !ok:
			delete(d.states, id) // the door is gone
		case s.Step(now):
			if d.near = d.h.Near(b.X, b.Y, 0, Everyone, d.near[:0]); len(d.near) > 0 {
				s = d.states[id]
				s.Use(now)
				d.states[id] = s
				continue
			}
			delete(d.states, id) // closed: an unheld door shut is the zero state
		}
	}
}

func (d *Doors) set(id entity.EntityID, s door.State) {
	if d.states == nil {
		d.states = map[entity.EntityID]door.State{}
	}
	d.states[id] = s
}

// openTicks is door id's (built as b) time to open now.
func (d *Doors) openTicks(id entity.EntityID, b construction.Building) int {
	return door.OpenTicks(door.Speed(b.Kind, b.Stuff, b.Kind == construction.Autodoor && d.pw.Powered(id)))
}

// isOpen reports door id fully open now (Sight.blocks: a shot passes).
func (d *Doors) isOpen(id entity.EntityID) bool {
	if d == nil {
		return false
	}
	s, ok := d.states[id]
	if !ok {
		return false
	}
	b, ok := d.h.Building(id)
	return ok && s.Open(d.h.Now(), d.openTicks(id, b))
}

// SetHold holds door id open (on) or lets it close on its timer.
func (d *Doors) SetHold(id entity.EntityID, on bool) {
	s := d.states[id]
	s.SetHold(on, d.h.Now())
	if s.Shut() {
		delete(d.states, id)
		return
	}
	d.set(id, s)
}

// State is door id's state (zero: shut).
func (d *Doors) State(id entity.EntityID) door.State { return d.states[id] }

// Held reports door id held open.
func (d *Doors) Held(id entity.EntityID) bool { return d.states[id].Hold }

// Shut clears from the door cells set (a bitset, one bit per cell of a
// map w wide) the doors held open, and returns it: the doors shut to
// hostiles (the game's shut-door set).
func (d *Doors) Shut(set []uint64, w int) []uint64 {
	for wi, word := range set {
		for ; word != 0; word &= word - 1 {
			if i := wi<<6 + bits.TrailingZeros64(word); d.HeldOpen(i%w, i/w) {
				set[wi] &^= 1 << (uint(i) & 63)
			}
		}
	}
	return set
}

// HeldOpen reports the door on (x, y) held open: hostiles may walk
// through it (the game's hostile path hook).
func (d *Doors) HeldOpen(x, y int) bool {
	id, k := d.h.Object(x, y)
	return id != 0 && k.IsDoor() && d.states[id].Hold
}

// Wait is how many ticks a colonist stepping into the door on (x, y) at
// now waits for it to open (0: open, or no door); it opens the door (the
// game's step hook).
func (d *Doors) Wait(x, y int, now uint64) int {
	id, k := d.h.Object(x, y)
	if id == 0 || !k.IsDoor() {
		return 0
	}
	b, _ := d.h.Building(id)
	s := d.states[id]
	s.Use(now)
	d.set(id, s)
	return s.Wait(now, d.openTicks(id, b))
}

// Barred reports the door on (x, y) forbidden: shut to colonists (BED-1's
// Forbidden flag; the game's colonist path hook).
func (d *Doors) Barred(x, y int) bool {
	id, k := d.h.Object(x, y)
	if id == 0 || !k.IsDoor() {
		return false
	}
	b, _ := d.h.Building(id)
	return b.Forbidden
}

// export is the doors not shut, ascending (nil when none).
func (d *Doors) export() []DoorSave {
	if len(d.states) == 0 {
		return nil
	}
	out := make([]DoorSave, 0, len(d.states))
	for id, s := range d.states {
		out = append(out, DoorSave{ID: id, State: s})
	}
	slices.SortFunc(out, func(a, b DoorSave) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// importDoors restores saved door states.
func (d *Doors) importDoors(s []DoorSave) {
	d.states = nil
	for _, v := range s {
		d.set(v.ID, v.State)
	}
}
