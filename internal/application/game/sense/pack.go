package sense

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
)

// The pack (CSE-036). A stack in one is a value off the map: no store, index,
// stockpile, reservation or carrier filter sees it, so hauling, search and the
// colony's stock counts leave it out; only Wealth counts it. It leaves through
// the pawn's hands (TakeOut, Unstow) or falls where the pawn is (Spill). Stow
// refuses only: nothing is dropped when a pawn's capacity falls, so no
// threshold here drives a behaviour.

const (
	// packLimitGrams is a pawn's capacity at full Manipulation (our tuning,
	// MOD-000 lead decision).
	packLimitGrams = 40_000
	// roomUnlimited is Room with gameplay.RespectCapacity off.
	roomUnlimited = 1 << 30
)

// GearPawns is the pawns Gear asks about.
type GearPawns interface {
	// PawnIDs are the colonists on the current map, in id order.
	PawnIDs() []entity.EntityID
	// Temperature is the air at id, tenths of a degree (id stands on the
	// current map).
	Temperature(id entity.EntityID) int
	// Manipulation is id's capacity, permille.
	Manipulation(id entity.EntityID) int
	// Rotted reports that qty of k rotted away in id's pack.
	Rotted(id entity.EntityID, k item.Kind, qty int)
}

// GearStock is the stacks on the map, and the way back to them.
type GearStock interface {
	Stack(stack entity.EntityID) (item.Stack, bool)
	// RemoveQty takes n from stack, deleting it at none.
	RemoveQty(stack entity.EntityID, n int)
	// SpawnCarried makes st a stack in id's hands; false: id stands on no map.
	SpawnCarried(id entity.EntityID, st item.Stack) (entity.EntityID, bool)
	// SetDown lays st down where id is: on its tile; away, in its caravan's
	// goods, else where it left the map. It never fails.
	SetDown(id entity.EntityID, st item.Stack)
}

// Pack is id's pack, to read: its slice stays Gear's.
func (g Gear) Pack(id entity.EntityID) inventory.Pack { return g.s.Packs[id] }

// Has reports id's pack holds something.
func (g Gear) Has(id entity.EntityID) bool { return len(g.s.Packs[id].Items) > 0 }

// Capacity is what id can carry in its pack, in grams: 40 kg times Manipulation.
func (g Gear) Capacity(id entity.EntityID) int {
	return packLimitGrams * g.r.Manipulation(id) / 1000
}

// Room is the grams id's pack still takes: unlimited with
// gameplay.RespectCapacity off.
func (g Gear) Room(id entity.EntityID) int {
	if !g.s.Rules.On(gameplay.RespectCapacity) {
		return roomUnlimited
	}
	return max(g.Capacity(id)-g.s.Packs[id].Grams(), 0)
}

// Stow moves n items of stack from the map into id's pack. False, with nothing
// changed: no such stack or not that many, in another pawn's hands, a corpse
// (no pack takes one), or more than Room. The caller clears its job's Carry when
// it stows the stack in hand.
func (g Gear) Stow(id, stack entity.EntityID, n int) bool {
	st, ok := g.r.Stack(stack)
	if !ok || n <= 0 || st.Qty < n || st.Carrier != 0 && st.Carrier != id || st.Kind.IsCorpse() {
		return false
	}
	if st.Kind.MassGrams()*n > g.Room(id) {
		return false
	}
	g.r.RemoveQty(stack, n)
	st.Qty = n
	p := g.s.Packs[id]
	p.Add(st)
	g.s.Packs[id] = p
	return true
}

// TakeOut makes up to n items of entry an ordinary stack in id's hands and
// records it in FromPack. The caller sets its job's Carry. False, with nothing
// changed: no such entry, or id stands on no map.
func (g Gear) TakeOut(id entity.EntityID, entry uint32, n int) (entity.EntityID, bool) {
	p, ok := g.s.Packs[id]
	if !ok {
		return 0, false
	}
	p.Items = slices.Clone(p.Items)
	st, ok := p.Take(entry, n)
	if !ok {
		return 0, false
	}
	stack, ok := g.r.SpawnCarried(id, st)
	if !ok {
		return 0, false
	}
	g.setPack(id, p)
	g.s.FromPack[id] = stack
	return stack, true
}

// Unstow takes the first entry of kind k out into id's hands, one hand load at
// most (TakeOut).
func (g Gear) Unstow(id entity.EntityID, k item.Kind) (entity.EntityID, bool) {
	entry, ok := g.s.Packs[id].Find(k)
	if !ok {
		return 0, false
	}
	return g.TakeOut(id, entry, item.CarryLimit)
}

// Spill empties id's pack where it is (GearStock.SetDown), on every path that
// takes a pawn out of play: nothing in a pack may vanish with its pawn.
func (g Gear) Spill(id entity.EntityID) {
	delete(g.s.FromPack, id) // what it holds is an ordinary stack from here on
	p, ok := g.s.Packs[id]
	if !ok {
		return
	}
	delete(g.s.Packs, id)
	for _, e := range p.Items {
		g.r.SetDown(id, e.Stack)
	}
}

// Rot spoils the packs of the current map's colonists by the air around them,
// over interval ticks, as craft.Condition does what lies on the ground. Packs on
// the road keep, like a caravan's goods. Spoiled food is gone. It walks the map's
// colonists, not the packs, so its order is theirs.
func (g Gear) Rot(interval int) {
	if len(g.s.Packs) == 0 {
		return
	}
	for _, id := range g.r.PawnIDs() {
		p, ok := g.s.Packs[id]
		if !ok {
			continue
		}
		rate := item.RotRate(g.r.Temperature(id))
		kept := p.Items[:0]
		for _, e := range p.Items {
			if e.Stack.Shelf() > 0 {
				e.Stack.Rot += interval * rate / 1000
			}
			if e.Stack.Rotten() {
				g.r.Rotted(id, e.Stack.Kind, e.Stack.Qty)
				continue
			}
			kept = append(kept, e)
		}
		p.Items = kept
		g.setPack(id, p)
	}
}

// Wealth is what the packs of the current map's colonists are worth, as
// Game.Wealth counts what lies on that map.
func (g Gear) Wealth() int {
	if len(g.s.Packs) == 0 {
		return 0
	}
	w := 0
	for _, id := range g.r.PawnIDs() {
		for _, e := range g.s.Packs[id].Items {
			w += e.Stack.Kind.Value() * e.Stack.Qty
		}
	}
	return w
}

// setPack stores id's pack; an empty one is no pack, so a colony that used a
// pack and emptied it saves as one that never did.
func (g Gear) setPack(id entity.EntityID, p inventory.Pack) {
	if len(p.Items) == 0 {
		delete(g.s.Packs, id)
		return
	}
	g.s.Packs[id] = p
}
