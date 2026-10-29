package craft

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a tiny in-memory simulation behind every craft role.
type fakeHost struct {
	ids      []entity.EntityID
	bldgs    map[entity.EntityID]construction.Building
	ver      uint64
	unpower  map[entity.EntityID]bool
	fuel     map[entity.EntityID]int
	temp     int
	items    []entity.EntityID
	stacks   map[entity.EntityID]item.Stack
	next     entity.EntityID
	pawns    []entity.EntityID
	jobs     map[entity.EntityID]job.Record
	claims   map[entity.EntityID]entity.EntityID
	unreach  bool
	postp    map[[2]entity.EntityID]bool
	locked   map[recipe.ID]bool
	noSkill  bool
	work     int // BenchWork per call (0: pct, below 0: none)
	lastPct  int
	meals    int
	cremated int
	msgs     []string
	drops    []item.Stack
}

func newFake() *fakeHost {
	return &fakeHost{
		bldgs: map[entity.EntityID]construction.Building{}, unpower: map[entity.EntityID]bool{},
		fuel: map[entity.EntityID]int{}, stacks: map[entity.EntityID]item.Stack{}, next: 1000,
		jobs: map[entity.EntityID]job.Record{}, claims: map[entity.EntityID]entity.EntityID{},
		postp: map[[2]entity.EntityID]bool{}, locked: map[recipe.ID]bool{}, temp: 20,
	}
}

func (f *fakeHost) build(id entity.EntityID, k construction.Kind, x, y int) {
	f.ids = append(f.ids, id)
	f.bldgs[id] = construction.Building{Kind: k, X: x, Y: y}
	f.ver++
}

func (f *fakeHost) remove(id entity.EntityID) {
	delete(f.bldgs, id)
	for i, b := range f.ids {
		if b == id {
			f.ids = append(f.ids[:i:i], f.ids[i+1:]...)
			break
		}
	}
	f.ver++
}

func (f *fakeHost) put(id entity.EntityID, st item.Stack) {
	f.items = append(f.items, id)
	f.stacks[id] = st
}

func (f *fakeHost) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) Bench(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	if !ok || !construction.DefOf(b.Kind).Bench {
		return construction.Building{}, false
	}
	return b, true
}
func (f *fakeHost) ObjectAt(x, y int) entity.EntityID {
	for _, id := range f.ids {
		if b := f.bldgs[id]; b.X == x && b.Y == y {
			return id
		}
	}
	return 0
}
func (f *fakeHost) StructVer() uint64               { return f.ver }
func (f *fakeHost) Powered(id entity.EntityID) bool { return !f.unpower[id] }
func (f *fakeHost) Fuel(id entity.EntityID) int     { return f.fuel[id] }
func (f *fakeHost) TemperatureAt(x, y int) int      { return f.temp }

func (f *fakeHost) Stock(k item.Kind) int {
	n := 0
	for _, id := range f.items {
		if st := f.stacks[id]; st.Kind == k {
			n += st.Qty
		}
	}
	return n
}
func (f *fakeHost) DriedStock() int {
	n := 0
	for _, id := range f.items {
		if st := f.stacks[id]; st.Dried {
			n += st.Qty
		}
	}
	return n
}
func (f *fakeHost) FreeWood() int { return f.Stock(item.Wood) }
func (f *fakeHost) Ingredient(id entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack) {
	for _, iid := range f.items {
		st := f.stacks[iid]
		if st.Carrier == 0 && st.Qty >= in.Qty && in.AcceptsStack(st) {
			return iid, st
		}
	}
	return 0, item.Stack{}
}
func (f *fakeHost) Stack(id entity.EntityID) (item.Stack, bool) {
	st, ok := f.stacks[id]
	return st, ok
}
func (f *fakeHost) Carry(carrier, stack entity.EntityID, take int) entity.EntityID {
	st := f.stacks[stack]
	if st.Qty <= take {
		st.Carrier = carrier
		f.stacks[stack] = st
		return stack
	}
	st.Qty -= take
	f.stacks[stack] = st
	f.next++
	nst := st
	nst.Qty, nst.Carrier = take, carrier
	f.put(f.next, nst)
	return f.next
}

func (f *fakeHost) DeleteItem(id entity.EntityID) {
	delete(f.stacks, id)
	for i, iid := range f.items {
		if iid == id {
			f.items = append(f.items[:i:i], f.items[i+1:]...)
			break
		}
	}
}
func (f *fakeHost) DropStack(k item.Kind, qty, x, y int) {
	f.DropStackOf(item.Stack{Kind: k, Qty: qty}, x, y)
}
func (f *fakeHost) DropStackOf(st item.Stack, x, y int) {
	st.X, st.Y = x, y
	f.next++
	f.put(f.next, st)
	f.drops = append(f.drops, st)
}
func (f *fakeHost) DropMeal(cook entity.EntityID, bench construction.Kind, k item.Kind, qty, x, y int) {
	f.meals += qty
	f.DropStack(k, qty, x, y)
}
func (f *fakeHost) Cremate(pyre, body entity.EntityID, d recipe.Def) {
	f.cremated++
	f.DeleteItem(body)
}
func (f *fakeHost) Message(x, y int, text string) {
	f.msgs = append(f.msgs, fmt.Sprintf("%d,%d %s", x, y, text))
}

func (f *fakeHost) PawnIDs() []entity.EntityID { return f.pawns }
func (f *fakeHost) Job(id entity.EntityID) (job.Record, bool) {
	j, ok := f.jobs[id]
	return j, ok
}
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record) { f.jobs[id] = j }
func (f *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	f.jobs[id] = rec
	return true
}
func (f *fakeHost) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	if f.unreach {
		return nil, false
	}
	return []pathing.Point{{X: x, Y: y}}, true
}
func (f *fakeHost) Reachable(id entity.EntityID, x, y int) bool { return !f.unreach }
func (f *fakeHost) Owner(target entity.EntityID) (entity.EntityID, bool) {
	o, ok := f.claims[target]
	return o, ok
}
func (f *fakeHost) Claim(owner, target entity.EntityID) bool {
	if o, ok := f.claims[target]; ok && o != owner {
		return false
	}
	f.claims[target] = owner
	return true
}

func (f *fakeHost) MeetsMinSkill(id entity.EntityID, sk skill.ID, min uint8) bool { return !f.noSkill }
func (f *fakeHost) BenchWork(id entity.EntityID, sk skill.ID, pct int) int {
	f.lastPct = pct
	if f.work < 0 {
		return 0 // not a working tick
	}
	if f.work != 0 {
		return f.work
	}
	return pct
}
func (f *fakeHost) ButcherYield(id entity.EntityID, qty int) int { return qty }
func (f *fakeHost) Postponed(id, bench entity.EntityID) bool {
	return f.postp[[2]entity.EntityID{id, bench}]
}
func (f *fakeHost) Postpone(id, bench entity.EntityID) { f.postp[[2]entity.EntityID{id, bench}] = true }
func (f *fakeHost) RecipeLocked(id recipe.ID) bool     { return f.locked[id] }

var _ Host = (*fakeHost)(nil)
