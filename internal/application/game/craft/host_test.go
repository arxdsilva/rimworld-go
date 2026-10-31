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
	ids          []entity.EntityID
	bldgs        map[entity.EntityID]construction.Building
	ver          uint64
	unpower      map[entity.EntityID]bool
	fuel         map[entity.EntityID]int
	temp         int
	items        []entity.EntityID
	stacks       map[entity.EntityID]item.Stack
	next         entity.EntityID
	pawns        []entity.EntityID
	jobs         map[entity.EntityID]job.Record
	claims       map[entity.EntityID]entity.EntityID
	counts       map[entity.EntityID]map[entity.EntityID]int // fake_claims_test.go (RES-1)
	tiles        map[[2]int]entity.EntityID
	sources      map[uint64][4]int // a bill source's rectangle (x1, y1, x2, y2)
	unreach      bool
	postp        map[[2]entity.EntityID]bool
	locked       map[recipe.ID]bool
	noSkill      bool
	skills       map[entity.EntityID]uint8
	names        map[entity.EntityID]string
	now          uint64
	scans        int // ItemIDs calls (Nearest's pass and the multi-stack search)
	stored       []storeCall
	startFail    job.FailReason // StartJob fails for this reason (FailNone: starts)
	work         int            // BenchWork per call (0: pct, below 0: none)
	lastPct      int
	meals        int
	cremated     int
	msgs         []string
	drops        []item.Stack
	butchered    []entity.EntityID // FOOD-3: ButcheredHuman calls
	fakeMakers                     // condition_test.go (PRD-2): the craft dice, inspirations, weather and outfits
	fakeWorkshop                   // fake_workshop_test.go (PRD-3): light, rooms, forge fuel, sculptures
}

func newFake() *fakeHost {
	return &fakeHost{
		bldgs: map[entity.EntityID]construction.Building{}, unpower: map[entity.EntityID]bool{},
		fuel: map[entity.EntityID]int{}, stacks: map[entity.EntityID]item.Stack{}, next: 1000,
		jobs: map[entity.EntityID]job.Record{}, claims: map[entity.EntityID]entity.EntityID{},
		postp: map[[2]entity.EntityID]bool{}, locked: map[recipe.ID]bool{}, temp: 20,
		skills: map[entity.EntityID]uint8{}, names: map[entity.EntityID]string{},
	}
}

func (f *fakeHost) build(id entity.EntityID, k construction.Kind, x, y int) {
	f.ids = append(f.ids, id)
	f.bldgs[id] = construction.Building{Kind: k, X: x, Y: y}
	if k == construction.Stove {
		f.fuel[id] = 1 << 20 // FOOD-4: the stove burns wood; built stocked (TestStoveBurnsWood empties it)
	}
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

func (f *fakeHost) ItemIDs() []entity.EntityID {
	f.scans++
	return f.items
}
func (f *fakeHost) Merge(into, from entity.EntityID) {
	a, b := f.stacks[into], f.stacks[from]
	a.Qty += b.Qty
	f.stacks[into] = a
	f.DeleteItem(from)
}
func (f *fakeHost) PutDown(id entity.EntityID, x, y int) {
	st := f.stacks[id]
	st.Carrier, st.X, st.Y = 0, x, y
	f.stacks[id] = st
}

// storeCall is one Store request.
type storeCall struct {
	carrier entity.EntityID
	kind    item.Kind
	x, y    int
	zone    uint64
}

func (f *fakeHost) Store(carrier entity.EntityID, k item.Kind, x, y int, zone uint64) bool {
	f.stored = append(f.stored, storeCall{carrier, k, x, y, zone})
	return true
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
func (f *fakeHost) DropMeal(cook entity.EntityID, bench construction.Kind, st item.Stack, x, y int) {
	f.meals += st.Qty
	f.DropStackOf(st, x, y)
}
func (f *fakeHost) Cremate(pyre, body entity.EntityID, d recipe.Def) {
	f.cremated++
	f.DeleteItem(body)
}
func (f *fakeHost) ButcheredHuman(b entity.EntityID) { f.butchered = append(f.butchered, b) }
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
	if f.startFail != job.FailNone {
		rec.State, rec.Fail = job.Fail, f.startFail
		f.jobs[id] = rec
		return false
	}
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

func (f *fakeHost) SkillLevel(id entity.EntityID, sk skill.ID) uint8 {
	if f.noSkill {
		return 0
	}
	if l, ok := f.skills[id]; ok {
		return l
	}
	return 10
}
func (f *fakeHost) Name(id entity.EntityID) string { return f.names[id] }
func (f *fakeHost) Now() uint64                    { return f.now }
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
