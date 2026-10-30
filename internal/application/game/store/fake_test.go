package store

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// world is a small fake map for the store's roles: buildings with their
// footprints, loose stacks, colonists and their jobs, claims.
type world struct {
	zs      *zone.Map
	bldgs   map[entity.EntityID]construction.Building
	ids     []entity.EntityID
	stacks  map[entity.EntityID]item.Stack
	items   []entity.EntityID
	next    entity.EntityID
	dark    map[entity.EntityID]bool
	pos     map[entity.EntityID][2]int
	jobs    map[entity.EntityID]job.Record
	claims  map[entity.EntityID]entity.EntityID
	reserve map[[2]int]bool
	blocked map[[2]int]bool
	walls   bool // nothing reachable
}

func newWorld() *world {
	return &world{zs: zone.NewMap(30, 30), bldgs: map[entity.EntityID]construction.Building{}, stacks: map[entity.EntityID]item.Stack{},
		next: 100, dark: map[entity.EntityID]bool{}, pos: map[entity.EntityID][2]int{}, jobs: map[entity.EntityID]job.Record{},
		claims: map[entity.EntityID]entity.EntityID{}, reserve: map[[2]int]bool{}, blocked: map[[2]int]bool{}}
}

// build places a finished building and lays its storage zone.
func (w *world) build(k construction.Kind, x, y int) entity.EntityID {
	w.next++
	id := w.next
	w.bldgs[id] = construction.Building{Kind: k, X: x, Y: y}
	w.ids = append(w.ids, id)
	cells, _, _ := construction.Footprint(k, construction.South, x, y)
	Buildings{}.Lay(w.zs, w, k, cells)
	return id
}

func (w *world) put(k item.Kind, qty, x, y int) entity.EntityID {
	return w.Spawn(k, qty, x, y)
}

func (w *world) BuildingIDs() []entity.EntityID { return w.ids }
func (w *world) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := w.bldgs[id]
	return b, ok
}
func (w *world) ObjectAt(x, y int) entity.EntityID {
	for _, id := range w.ids {
		b := w.bldgs[id]
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			if c[0] == x && c[1] == y {
				return id
			}
		}
	}
	return 0
}
func (w *world) Powered(id entity.EntityID) bool { return !w.dark[id] }
func (w *world) ItemAt(x, y int) entity.EntityID {
	for _, id := range w.items {
		if st := w.stacks[id]; st.X == x && st.Y == y && st.Carrier == 0 {
			return id
		}
	}
	return 0
}
func (w *world) Stack(id entity.EntityID) (item.Stack, bool) {
	st, ok := w.stacks[id]
	return st, ok
}
func (w *world) SetStack(id entity.EntityID, st item.Stack) { w.stacks[id] = st }
func (w *world) DeleteItem(id entity.EntityID) {
	delete(w.stacks, id)
	w.items = slices.DeleteFunc(w.items, func(i entity.EntityID) bool { return i == id })
}
func (w *world) Spawn(k item.Kind, qty, x, y int) entity.EntityID {
	if x < 0 {
		return 0
	}
	w.next++
	w.stacks[w.next] = item.Stack{Kind: k, Qty: qty, X: x, Y: y}
	w.items = append(w.items, w.next)
	return w.next
}
func (w *world) StacksAt(x, y int) int {
	n := 0
	for _, id := range w.items {
		if st := w.stacks[id]; st.X == x && st.Y == y && st.Carrier == 0 {
			n++
		}
	}
	return n
}
func (w *world) ItemIDs() []entity.EntityID { return w.items }
func (w *world) Position(id entity.EntityID) (int, int) {
	p := w.pos[id]
	return p[0], p[1]
}
func (w *world) SetJob(id entity.EntityID, j job.Record) { w.jobs[id] = j }
func (w *world) Claim(owner, target entity.EntityID) bool {
	if o, ok := w.claims[target]; ok && o != owner {
		return false
	}
	w.claims[target] = owner
	return true
}
func (w *world) Release(owner, target entity.EntityID) {
	if w.claims[target] == owner {
		delete(w.claims, target)
	}
}
func (w *world) Reachable(id entity.EntityID, x, y int) bool { return !w.walls }

// Cells.
func (w *world) Stores(p zone.Priority) []zone.Store {
	var s zone.Stores
	return slices.Clone(s.Of(w.zs, p))
}
func (w *world) Zones() *zone.Map                  { return w.zs }
func (w *world) First(x, y int) (item.Stack, bool) { return w.Stack(w.ItemAt(x, y)) }
func (w *world) Shelf(k item.Kind, x, y int) (int, bool) {
	n, top := 0, false
	for _, id := range w.items {
		if st := w.stacks[id]; st.X == x && st.Y == y && st.Carrier == 0 {
			n++
			top = top || st.Kind == k && st.Qty < k.StackLimit()
		}
	}
	return n, top
}
func (w *world) TileTaken(x, y int) bool           { return w.reserve[[2]int{x, y}] }
func (w *world) Blocked(x, y int, shelf bool) bool { return w.blocked[[2]int{x, y}] }
