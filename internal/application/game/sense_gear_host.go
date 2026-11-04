package game

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
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
	st, ok := h.g.stacks.Get(stack)
	switch {
	case !ok:
	case st.Qty <= n:
		h.g.deleteItem(stack)
	default:
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
		rows = append(rows, PackItemView{ID: e.ID, Name: e.Stack.Kind.String(), Qty: e.Stack.Qty, Unload: e.Unload})
	}
	return rows, p.Grams(), gear.Capacity(id)
}
