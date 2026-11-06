package outfit

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Index is every apparel stack on a map (free, carried or forbidden), ids
// ascending; never saved. Note adds spawned stacks; a read adds stacks
// split off without a spawn (item ids are handed out ascending and
// appended in order, so new ones are those above mark), drops gone ones
// when asked, and rebuilds the lot once an in-game hour. gen bumps
// whenever stacks join; kinds, the distinct kinds, is taken per gen for
// the optimistic plan (gone stacks may linger in it until the next
// validating read: it only ever over-counts).
type Index struct {
	ids      []entity.EntityID
	mark     entity.EntityID
	hour     uint64 // 1 + the hour of the last rebuild (0: never built)
	gen      uint64
	kindsGen uint64 // 1 + the gen kinds was taken at
	kinds    []item.Kind
}

// Note adds new apparel stack id to the index (PLAY-017).
func (x *Index) Note(id entity.EntityID) {
	if x.add(id) {
		x.gen++
	}
}

// add puts id in the index; false when it was there.
func (x *Index) add(id entity.EntityID) bool {
	i, found := slices.BinarySearch(x.ids, id)
	if found {
		return false
	}
	x.ids = slices.Insert(x.ids, i, id)
	return true
}

// Stock is the apparel index brought up to date (validate: gone stacks
// dropped too). The slice is the index's own: read only.
func (d Dresser) Stock(validate bool) []entity.EntityID {
	x, items := d.idx, d.r.Stock.ItemIDs()
	if hour := d.r.Weather.Now()/clock.TicksPerHour + 1; x.hour != hour {
		x.hour, x.ids, x.mark = hour, x.ids[:0], 0
		x.gen++
	}
	n := len(items)
	i := n
	for i > 0 && items[i-1] > x.mark {
		i--
	}
	for _, sid := range items[i:] {
		if sp := d.r.Stock.Stack(sid); sp != nil && sp.Kind.IsApparel() && x.add(sid) {
			x.gen++
		}
	}
	if n > 0 {
		x.mark = max(x.mark, items[n-1])
	}
	if validate {
		x.ids = slices.DeleteFunc(x.ids, func(sid entity.EntityID) bool { return d.r.Stock.Stack(sid) == nil })
	}
	return x.ids
}

// Kinds is the distinct kinds in the apparel index (a superset of those
// lying on the map), taken again whenever its gen moves.
func (d Dresser) Kinds() []item.Kind {
	ids := d.Stock(false)
	x := d.idx
	if x.kindsGen == x.gen+1 {
		return x.kinds
	}
	x.kinds, x.kindsGen = x.kinds[:0], x.gen+1
	for _, sid := range ids {
		if sp := d.r.Stock.Stack(sid); sp != nil && !slices.Contains(x.kinds, sp.Kind) {
			x.kinds = append(x.kinds, sp.Kind)
		}
	}
	return x.kinds
}
