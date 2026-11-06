package game

import (
	"slices"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// senseGear is game/sense's Gear roles over the game (CSE-036).
type senseGear struct{ g *Game }

func (h senseGear) PawnIDs() []entity.EntityID { return h.g.pawnIDs }
func (h senseGear) Temperature(id entity.EntityID) int {
	p, _ := h.g.positions.Get(id)
	return int(h.g.temperatureAt(p.X, p.Y))
}
func (h senseGear) Manipulation(id entity.EntityID) int {
	return int(h.g.capacity(id, body.Manipulation))
}
func (h senseGear) Rotted(id entity.EntityID, k item.Kind, qty int) {
	p, _ := h.g.positions.Get(id)
	h.g.message(p.X, p.Y, "%d %s rotted away in %s's pack", qty, k, h.g.personName(id))
}

func (h senseGear) Stack(stack entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(stack) }
func (h senseGear) RemoveQty(stack entity.EntityID, n int) {
	if st, ok := h.g.stacks.Get(stack); ok && st.Qty <= n {
		h.g.deleteItem(stack)
	} else if ok {
		st.Qty -= n
		h.g.putStack(stack, st)
	}
	h.g.viewGen++ // view cache (indexes.go): a command may stow between ticks
}
func (h senseGear) SpawnCarried(id entity.EntityID, st item.Stack) (entity.EntityID, bool) {
	p, ok := h.g.positions.Get(id)
	if !ok {
		return 0, false
	}
	stack, err := h.g.SpawnStack(st.Kind, st.Qty, p.X, p.Y)
	if err != nil {
		return 0, false
	}
	st.X, st.Y, st.Carrier = p.X, p.Y, id
	h.g.putStack(stack, st)
	return stack, true
}
func (h senseGear) SetDown(id entity.EntityID, st item.Stack) {
	g := h.g
	for _, m := range g.maps {
		if p, ok := m.positions.Get(id); ok {
			layOn(g, m, st, p.X, p.Y)
			return
		}
	}
	if i, ok := slices.BinarySearchFunc(g.worldS().stashed, id, stashCmp); ok {
		r := g.worldSt.stashed[i]
		if fleetOf(g).At(r.Holder) != nil {
			travelOf(g).Carry(uint64(r.Holder), st) // on the road: the caravan takes it home
			return
		}
		if m := mapByID(g, r.Map); m != nil {
			layOn(g, m, st, r.X, r.Y) // a casket: where it boarded
			return
		}
	}
	cur := g.mapState
	g.mapState = g.maps[0] // nowhere left to be: the first map's home
	x, y := g.home()
	g.mapState = cur
	layOn(g, g.maps[0], st, x, y)
}

// The errand roles (CSE-005, 037…039).
func (h senseGear) Pos(id entity.EntityID) (int, int)              { p, _ := h.g.positions.Get(id); return p.X, p.Y }
func (h senseGear) Now() uint64                                    { return uint64(h.g.clock.Now()) }
func (h senseGear) StartJob(id entity.EntityID, r job.Record) bool { return h.g.startJob(id, r) }
func (h senseGear) Job(id entity.EntityID) job.Record              { j, _ := h.g.jobs.Get(id); return j }
func (h senseGear) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h senseGear) Rested(id entity.EntityID) bool                 { n, _ := h.g.need.Get(id); return !n.Tired() }
func (h senseGear) Free(id entity.EntityID) bool {
	act := h.g.activityOf(id)
	return h.Rested(id) && !needs.WorkScheduled(act) && !needs.SleepScheduled(act)
}
func (h senseGear) Hunger(id entity.EntityID) int  { n, _ := h.g.need.Get(id); return n.Hunger }
func (h senseGear) Hungry(id entity.EntityID) bool { n, _ := h.g.need.Get(id); return n.Hungry() }
func (h senseGear) Allows(id entity.EntityID, k item.Kind) bool {
	return h.g.sk.diet.Picker(id, false, cannibal(h.g, id)).Allows(k)
}
func (h senseGear) StoreCell(id entity.EntityID, st item.Stack) (int, int, bool) {
	return h.g.haulDest(id, st, 0) // haul_priority.go
}
func (h senseGear) Lay(id, stack entity.EntityID) {
	st, ok := h.g.stacks.Get(stack)
	if p, _ := h.g.positions.Get(id); ok {
		st.Carrier, st.X, st.Y = 0, p.X, p.Y
		claim(&st) // haul_claim.go: what a colonist put down is the colony's
		h.g.mergeOrSet(stack, st)
	}
}

// NearestMeal looks within mealReach tiles through the food index.
func (h senseGear) NearestMeal(id entity.EntityID) (best entity.EntityID, bx, by int, ok bool) {
	g, pos := h.g, h.g.positions.Ref(id)
	pick, bestD := g.sk.diet.Picker(id, false, cannibal(g, id)), 1<<30
	g.food().items.each(pos.X, pos.Y, mealReach, func(iid entity.EntityID) bool {
		st := g.stacks.Ref(iid)
		if st == nil || !sense.IsMeal(st.Kind) || st.Qty <= 0 || st.Forbidden || !pick.Allows(st.Kind) || g.reserve.FreeFor(id, iid, st.Qty) <= 0 || !g.inArea(id, st.X, st.Y) {
			return true
		}
		if d := pathing.Dist(pos.X, pos.Y, st.X, st.Y); d < bestD || d == bestD && iid < best {
			best, bx, by, bestD, ok = iid, st.X, st.Y, d, true
		}
		return true
	})
	return best, bx, by, ok
}

const mealReach = 40

func (h senseGear) Worn(id entity.EntityID, n int) (int, bool) {
	for _, s := range wearSlots {
		if h.g.apparel[id].Kind(s) != 0 {
			if n--; n < 0 {
				return int(s), true
			}
		}
	}
	return 0, false
}
func (h senseGear) TakeOff(id entity.EntityID, slot int) bool {
	g, s := h.g, item.Slot(slot)
	w, pos := g.apparel[id], g.positions.Ref(id)
	if p := w.On[s]; p.Kind == 0 || !slices.Contains(wearSlots[:], s) {
		return false
	} else {
		w.On[s] = craft.Piece{}
		w.SetForced(s, false)
		g.apparel[id] = w
		g.dropStackOf(p.Stack(pos.X, pos.Y), pos.X, pos.Y)
		g.dressChanged(id, s)
		g.dressNote(id, logTookOff, p.Kind)
	}
	return true
}
func (h senseGear) Armed(id entity.EntityID) bool {
	w := h.g.Equipped(id)
	return w != "" && w != "fists"
}
func (h senseGear) DropWeapon(id entity.EntityID) bool {
	armed := h.Armed(id)
	h.g.combat.Drafts.DropWeapon(id, false)
	return armed
}
func (h senseGear) HasSpare(id entity.EntityID) bool {
	_, _, ok := h.g.combat.Drafts.Spare(id)
	return ok
}
func (h senseGear) DropSpare(id entity.EntityID) bool {
	had := h.HasSpare(id)
	h.g.combat.Drafts.DropSpare(id, false)
	return had
}
func (h senseGear) Interrupt(id entity.EntityID)                 { h.g.interrupt(id, job.Interrupted) }
func (h senseGear) Up(id entity.EntityID) bool                   { return h.g.standingPawn(id) } // GEAR-1 (sense/drop.go)
func (h senseGear) Prisoner(id entity.EntityID) bool             { return h.g.prison.Is(id) }
func (h senseGear) Outfit(id entity.EntityID) craft.Outfit       { return h.g.apparel[id] }
func (h senseGear) SetOutfit(id entity.EntityID, o craft.Outfit) { h.g.apparel[id] = o; h.g.viewGen++ }
func (h senseGear) MayDose(id entity.EntityID, k item.Kind) bool {
	return slices.Contains(doseOf(h.g).Allowed(id), k)
}
func (h senseGear) Breaking(id entity.EntityID) bool { return h.g.breaking(id) } // GEAR-1: no gear orders in a break

// layOn sets st down at (x, y) on map m, merging as anything set down does, with
// everything it was (its make, a book's title, its rot) kept.
func layOn(g *Game, m *mapState, st item.Stack, x, y int) {
	cur := g.mapState
	g.mapState = m
	defer func() { g.mapState = cur }()
	if stack, err := g.SpawnStack(st.Kind, st.Qty, x, y); err == nil {
		st.X, st.Y, st.Carrier = x, y, 0
		g.mergeOrSet(stack, st)
	}
}

// packView is PawnView's pack: its rows, weight and limit.
func packView(g *Game, id entity.EntityID) ([]PackItemView, int, int) {
	gear := senseOf(g).Gear
	p := gear.Pack(id)
	var rows []PackItemView
	for _, e := range p.Items {
		rows = append(rows, PackItemView{ID: e.ID, Name: e.Stack.Kind.String(), Qty: e.Stack.Qty, Unload: e.Unload, Gear: gear.PackGear(id, e)})
	}
	return rows, p.Grams(), gear.Capacity(id)
}
