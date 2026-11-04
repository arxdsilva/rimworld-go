package sense

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
)

// CSE-038/039: the player marks something to put away. A pack entry carries the
// mark (Entry.Unload), so it is saved with the pack; a worn garment, the weapon
// or the spare weapon has no place to keep one, so its mark is the job itself:
// pressing Unload starts it now, Cancel ends it. job.Record.Count says which
// (the Mode of command.MarkUnload) and BillIndex holds the garment's slot.

// The modes of command.MarkUnload, and a pack unload whose mark was taken away.
const (
	modePack = iota
	modeApparel
	modeWeapon
	modeSpare
	modeCancelled = 9
)

const (
	// unloadWait is the pause before anything is done; a garment takes longer to
	// get off and a weapon to put down.
	unloadWait  = 10
	apparelWait = 40
	weaponWait  = 30
	unloadRetry = 500 // a colonist that could not begin waits this long (ours)
)

// Mark is command.MarkUnload: mode 0 pack entry, 1 apparel, 2 weapon, 3 spare.
// Index is an entry's ID, the n-th garment worn, or ignored. Anything that does
// not name a real thing of a colonist on this map does nothing.
func (g Gear) Mark(pawn entity.EntityID, mode, index int, enabled bool) {
	if !g.s.Rules.On(gameplay.ManualUnload) || !slices.Contains(g.r.PawnIDs(), pawn) {
		return
	}
	switch mode {
	case modePack:
		g.markEntry(pawn, index, enabled)
	case modeApparel:
		if slot, ok := g.r.Worn(pawn, max(index, 0)); ok && index >= 0 {
			g.markJob(pawn, modeApparel, slot, apparelWait, enabled)
		}
	case modeWeapon:
		if g.r.Armed(pawn) {
			g.markJob(pawn, modeWeapon, 0, weaponWait, enabled)
		}
	case modeSpare:
		if g.r.HasSpare(pawn) {
			g.markJob(pawn, modeSpare, 0, weaponWait, enabled)
		}
	}
}

// markEntry marks or unmarks entry id of pawn's pack. Unmarking stops an unload
// that already has the item in hand: it goes back, or down if the pack is full.
func (g Gear) markEntry(pawn entity.EntityID, id int, on bool) {
	p := g.s.Packs[pawn]
	i := slices.IndexFunc(p.Items, func(e inventory.Entry) bool { return int(e.ID) == id })
	if id < 0 || i < 0 {
		return
	}
	p.Items = slices.Clone(p.Items)
	p.Items[i].Unload = on
	g.s.Packs[pawn] = p
	if j := g.r.Job(pawn); !on && j.Kind == job.UnloadPack && j.Count == modePack && j.Carry != 0 {
		j.Count = modeCancelled
		g.r.SetJob(pawn, j)
		g.r.Interrupt(pawn)
	}
}

// markJob starts (on) or ends (off) the unload of one worn or wielded thing.
func (g Gear) markJob(pawn entity.EntityID, mode, slot, wait int, on bool) {
	j := g.r.Job(pawn)
	mine := j.Kind == job.UnloadPack && j.Count == mode && j.BillIndex == slot
	switch {
	case on && mine:
	case on:
		g.r.Interrupt(pawn)
		x, y := g.r.Pos(pawn)
		g.r.StartJob(pawn, job.Record{Kind: job.UnloadPack, TX: x, TY: y, Count: mode, BillIndex: slot, WorkLeft: unloadWait + wait})
	case mine:
		g.r.Interrupt(pawn)
	}
}

// marked is the first entry of p to put away.
func marked(p inventory.Pack) (uint32, bool) {
	for _, e := range p.Items {
		if e.Unload {
			return e.ID, true
		}
	}
	return 0, false
}

// unloadNext begins putting away the next marked entry of id's pack, at any
// hour. It waits unloadRetry after beginning, until the entry is done, so a
// colonist that is interrupted again and again does not start over each tick.
func (g Gear) unloadNext(id entity.EntityID, p inventory.Pack) bool {
	now := g.r.Now()
	if now < g.s.UnloadAt[id] {
		return false
	}
	if _, ok := marked(p); !ok || !g.r.Rested(id) {
		return false
	}
	g.s.UnloadAt[id] = now + unloadRetry
	x, y := g.r.Pos(id)
	return g.r.StartJob(id, job.Record{Kind: job.UnloadPack, TX: x, TY: y, WorkLeft: unloadWait})
}

// DoUnload runs one tick of job.UnloadPack.
func (g Gear) DoUnload(id entity.EntityID, j job.Record) {
	if j.WorkLeft > 0 {
		j.WorkLeft--
		g.r.SetJob(id, j)
		return
	}
	switch j.Count {
	case modeApparel:
		g.end(id, j, g.r.TakeOff(id, j.BillIndex))
	case modeWeapon:
		g.end(id, j, g.r.DropWeapon(id))
	case modeSpare:
		g.end(id, j, g.r.DropSpare(id))
	case modePack:
		g.unloadPack(id, j)
	default:
		g.end(id, j, false)
	}
}

// unloadPack takes the first marked entry to the colonist's hands, walks it to
// a store cell if the colony has one with room, and sets it down there, or
// where the colonist stands if not.
func (g Gear) unloadPack(id entity.EntityID, j job.Record) {
	if j.Carry != 0 { // arrived
		g.r.Lay(id, j.Carry)
		delete(g.s.FromPack, id)
		delete(g.s.UnloadAt, id) // the next marked entry may begin at once
		j.Carry = 0
		g.end(id, j, true)
		return
	}
	entry, ok := marked(g.s.Packs[id])
	if !ok {
		g.end(id, j, false) // the mark was taken away
		return
	}
	stack, ok := g.TakeOut(id, entry, item.CarryLimit)
	if !ok {
		g.end(id, j, false)
		return
	}
	j.Carry = stack
	g.r.SetJob(id, j)
	st, _ := g.r.Stack(stack)
	if x, y, found := g.r.StoreCell(id, st); found {
		j.TX, j.TY, j.WorkLeft = x, y, 0
		if g.r.StartJob(id, j) {
			return
		}
		j.State = job.Do // out of reach: down where it stands
		g.r.SetJob(id, j)
	}
	g.unloadPack(id, j)
}
