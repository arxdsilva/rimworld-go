package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// dryBatch is the food laid out on a drying rack. It doesn't rot while it
// dries; Left counts down full-speed drying ticks.
type dryBatch struct {
	Kind item.Kind
	Qty  int
	Rot  int // spoilage it arrived with, restored if the rack is torn down
	Left int
}

func (b dryBatch) percent() int {
	total := b.Kind.DryTicks()
	if total <= 0 {
		return 100
	}
	return 100 - b.Left*100/total
}

// billStock is the colony's stock of a bill's output: dried food for a
// drying bill, otherwise the output kind.
func (g *Game) billStock(d recipe.Def) int {
	if !d.Dry {
		return g.stockOf(d.Output)
	}
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Dried {
			n += st.Qty
		}
	}
	return n
}

// dryRacks advances every rack's batch by the air temperature at the rack;
// a finished batch drops off the rack as a dried stack on its work spot.
func (g *Game) dryRacks() {
	for _, bid := range append([]entity.EntityID(nil), g.bldgIDs...) {
		rb := g.racks[bid]
		if rb.Qty <= 0 {
			continue
		}
		b, ok := g.bldgs.Get(bid)
		if !ok {
			delete(g.racks, bid)
			continue
		}
		rb.Left -= climateInterval * item.DryRate(int(g.temperatureAt(b.X, b.Y))) / 1000
		if rb.Left > 0 {
			g.racks[bid] = rb
			continue
		}
		delete(g.racks, bid)
		_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		g.dropStackOf(item.Stack{Kind: rb.Kind, Qty: rb.Qty, Dried: true}, spot[0], spot[1])
		g.message(spot[0], spot[1], "%d dried %s ready", rb.Qty, rb.Kind)
	}
}

// emptyRack puts a torn-down rack's undried batch back on the ground.
func (g *Game) emptyRack(id entity.EntityID, x, y int) {
	rb := g.racks[id]
	delete(g.racks, id)
	if rb.Qty > 0 {
		g.dropStackOf(item.Stack{Kind: rb.Kind, Qty: rb.Qty, Rot: rb.Rot}, x, y)
	}
}

// dropStackOf spawns a copy of st (kind, quantity, rot, dried, found and
// marked) at (x, y), merging into a matching stack there.
func (g *Game) dropStackOf(st item.Stack, x, y int) {
	sid, err := g.SpawnStack(st.Kind, st.Qty, x, y)
	if err != nil {
		return
	}
	if s, ok := g.stacks.Get(sid); ok {
		s.Rot, s.Dried = st.Rot, st.Dried
		s.Unclaimed, s.HaulMarked = st.Unclaimed, st.HaulMarked
		g.mergeOrSet(sid, s)
	}
}
