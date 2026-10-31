package herd

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a tiny world: one handler (id 1) at (0, 0), animals and
// stacks by id, every tile reachable, claims by target.
type fakeHost struct {
	now     uint64
	roll    int // Roll's answer
	skill   int
	animals map[entity.EntityID]animal.State
	pos     map[entity.EntityID][2]int
	stacks  map[entity.EntityID]item.Stack
	jobs    map[entity.EntityID]job.Record
	claims  map[entity.EntityID]entity.EntityID
	next    entity.EntityID
	msgs    []string
	killed  []entity.EntityID
	tamed   []entity.EntityID
	failed  []entity.EntityID
	fed     []entity.EntityID
	hauled  []entity.EntityID
}

const handler entity.EntityID = 1

func newFake() *fakeHost {
	return &fakeHost{now: 100 * clock.TicksPerDay, skill: 10, animals: map[entity.EntityID]animal.State{},
		pos: map[entity.EntityID][2]int{handler: {0, 0}}, stacks: map[entity.EntityID]item.Stack{},
		jobs: map[entity.EntityID]job.Record{}, claims: map[entity.EntityID]entity.EntityID{}, next: 1000}
}

func (f *fakeHost) add(id entity.EntityID, a animal.State, x, y int) {
	f.animals[id], f.pos[id] = a, [2]int{x, y}
}

func (f *fakeHost) Now() uint64                   { return f.now }
func (f *fakeHost) Roll(n int) int                { return min(f.roll, n-1) }
func (f *fakeHost) Message(x, y int, text string) { f.msgs = append(f.msgs, text) }
func (f *fakeHost) Name(entity.EntityID) string   { return "Ada" }
func (f *fakeHost) Animals(entity.EntityID) int   { return f.skill }
func (f *fakeHost) Animal(id entity.EntityID) (animal.State, bool) {
	a, ok := f.animals[id]
	return a, ok
}
func (f *fakeHost) SetAnimal(id entity.EntityID, a animal.State) { f.animals[id] = a }
func (f *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := f.pos[id]
	return p[0], p[1], ok
}
func (f *fakeHost) AnimalIDs() []entity.EntityID {
	var ids []entity.EntityID
	for id := range f.animals {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
func (f *fakeHost) AnyAnimal(keep func(entity.EntityID, *animal.State) bool) bool {
	_, _, _, ok := f.Nearest(handler, keep)
	return ok
}
func (f *fakeHost) Nearest(id entity.EntityID, keep func(entity.EntityID, *animal.State) bool) (entity.EntityID, int, int, bool) {
	for _, aid := range f.AnimalIDs() {
		a := f.animals[aid]
		if keep(aid, &a) {
			p := f.pos[aid]
			return aid, p[0], p[1], true
		}
	}
	return 0, 0, 0, false
}
func (f *fakeHost) Kill(aid, by entity.EntityID) {
	f.killed = append(f.killed, aid)
	delete(f.animals, aid)
}
func (f *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	if rec.Target != 0 {
		f.claims[rec.Target] = id
	}
	rec.State = job.Do
	f.jobs[id] = rec
	f.pos[id] = [2]int{rec.TX, rec.TY}
	return true
}
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record) {
	f.jobs[id] = j
	if j.State == job.Goto || j.State == job.Do {
		f.pos[id] = [2]int{j.TX, j.TY} // walks there at once
	}
}
func (f *fakeHost) Job(id entity.EntityID) (job.Record, bool) {
	j, ok := f.jobs[id]
	return j, ok
}
func (f *fakeHost) Owner(t entity.EntityID) (entity.EntityID, bool) {
	o, ok := f.claims[t]
	return o, ok
}
func (f *fakeHost) Claim(id, t entity.EntityID) bool { f.claims[t] = id; return true }
func (f *fakeHost) BesideSpot(_ entity.EntityID, x, y int) (int, int, bool) {
	return x + 1, y, true
}
func (f *fakeHost) PathTo(entity.EntityID, int, int) ([]pathing.Point, bool) { return nil, true }
func (f *fakeHost) Work(entity.EntityID) int                                 { return 1000 }
func (f *fakeHost) NearestFood(_ entity.EntityID, accept func(item.Kind) bool) (entity.EntityID, int, int) {
	ids := make([]entity.EntityID, 0, len(f.stacks))
	for id := range f.stacks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if st := f.stacks[id]; st.Carrier == 0 && accept(st.Kind) {
			return id, st.X, st.Y
		}
	}
	return 0, 0, 0
}
func (f *fakeHost) Take(id, sid entity.EntityID, qty int) (entity.EntityID, bool) {
	st, ok := f.stacks[sid]
	if !ok || st.Carrier != 0 {
		return 0, false
	}
	if st.Qty <= qty {
		st.Carrier = id
		f.stacks[sid] = st
		return sid, true
	}
	st.Qty -= qty
	f.stacks[sid] = st
	f.next++
	st.Qty, st.Carrier = qty, id
	f.stacks[f.next] = st
	return f.next, true
}
func (f *fakeHost) Stack(sid entity.EntityID) (item.Stack, bool) {
	st, ok := f.stacks[sid]
	return st, ok
}
func (f *fakeHost) Delete(sid entity.EntityID) { delete(f.stacks, sid) }
func (f *fakeHost) Drop(st item.Stack) {
	f.next++
	f.stacks[f.next] = st
}
func (f *fakeHost) HaulToStorage(id, sid entity.EntityID) bool {
	f.hauled = append(f.hauled, sid)
	return true
}
func (f *fakeHost) Tamed(id, aid entity.EntityID) {
	f.tamed = append(f.tamed, aid)
	a := f.animals[aid]
	a.Tame, a.TameMark = true, false
	f.animals[aid] = a
}
func (f *fakeHost) TameFailed(id, aid entity.EntityID) {
	f.failed = append(f.failed, aid)
	a := f.animals[aid]
	a.TameRetry = f.now + animal.TameRetryTicks
	f.animals[aid] = a
}
func (f *fakeHost) FedAnimal(id, aid entity.EntityID) { f.fed = append(f.fed, aid) }

// run ticks id's job until it ends (at most n ticks).
func run(f *fakeHost, hd *Herd, id entity.EntityID, n int) job.Record {
	for range n {
		j := f.jobs[id]
		if j.State == job.Complete || j.State == job.Fail {
			return j
		}
		hd.Do(id, j)
	}
	return f.jobs[id]
}

func TestTamingFetchesFoodAndRolls(t *testing.T) {
	f := newFake()
	hd := New(f, &Settings{})
	f.add(10, animal.State{Species: animal.Goat, TameMark: true, Food: 1000, HasSex: true}, 9, 0)
	if hd.AssignTame(handler) {
		t.Fatal("no food, no taming")
	}
	f.stacks[50] = item.Stack{Kind: item.Hay, Qty: 50, X: 2, Y: 2}
	if hd.Taming.Assign(handler) {
		t.Fatal("hay is no taming food")
	}
	f.stacks[51] = item.Stack{Kind: item.Potatoes, Qty: 50, X: 3, Y: 3}
	f.skill = 0
	f.add(11, animal.State{Species: animal.Ostrich, TameMark: true}, 1, 1)
	if !hd.AssignTame(handler) {
		t.Fatal("a goat is tamed with potatoes")
	}
	if j := f.jobs[handler]; j.Target != 10 || j.Dest != 51 {
		t.Fatalf("the goat, not the ostrich beyond a skill-0 handler: %+v", j)
	}
	f.roll = 999 // fail
	j := run(f, hd, handler, 50)
	want := 50 - foodQty(animal.Goat, item.Potatoes)
	if j.State != job.Complete || len(f.failed) != 1 || f.stacks[51].Qty != want || f.animals[10].Food <= 1000 {
		t.Fatalf("job %v failed %v potatoes %d (want %d) food %d", j.State, f.failed, f.stacks[51].Qty, want, f.animals[10].Food)
	}
	a := f.animals[10]
	a.TameRetry = 0
	f.animals[10] = a
	f.roll = 0
	f.skill = 10
	delete(f.claims, 10)
	if !hd.Taming.Assign(handler) {
		t.Fatal("a second try")
	}
	if run(f, hd, handler, 50); len(f.tamed) != 1 {
		t.Fatal("tamed on a good roll")
	}
	if Chance(10, animal.State{Species: animal.Cow, Downed: true}) != 1000 || Chance(7, animal.State{Species: animal.Cow}) != 500 {
		t.Fatal("chance")
	}
}

func TestCollectWastesOnAFailedGather(t *testing.T) {
	f := newFake()
	hd := New(f, &Settings{})
	f.add(10, animal.State{Species: animal.Cow, Tame: true, Female: true, Product: animal.ProductFull}, 4, 0)
	f.skill = 0
	f.roll = 700 // above a skill-0 handler's 60 %
	if !hd.AssignRest(handler) {
		t.Fatal("milk the cow")
	}
	run(f, hd, handler, 50)
	if len(f.msgs) != 1 || len(f.stacks) != 0 || f.animals[10].Product != 0 {
		t.Fatalf("wasted: msgs %v stacks %v", f.msgs, f.stacks)
	}
	a := f.animals[10]
	a.Product = animal.ProductFull
	f.animals[10] = a
	f.roll = 0
	delete(f.claims, 10)
	hd.Collect.Assign(handler)
	run(f, hd, handler, 50)
	if len(f.stacks) != 1 || f.stacks[f.next].Qty != 14 || f.stacks[f.next].Kind != item.Milk {
		t.Fatalf("a cow's 14 milk: %+v", f.stacks)
	}
	if ProductJob(animal.Wool) != job.Shear || ProductJob(animal.Milk) != job.Milk {
		t.Fatal("product jobs")
	}
}

func TestSlaughterIsCareful(t *testing.T) {
	f := newFake()
	hd := New(f, &Settings{})
	f.add(10, animal.State{Species: animal.Cow, Tame: true, Designated: true}, 4, 0)
	f.add(11, animal.State{Species: animal.Deer, Designated: true}, 2, 0) // wild: a hunter's
	careful := false
	f2 := &killSpy{fakeHost: f, hd: hd, saw: &careful}
	hd.h, hd.Slaughter.hd.h = f2, f2
	if !hd.Slaughter.Assign(handler) || f.jobs[handler].Target != 10 || !Holds(f.jobs[handler], 10) {
		t.Fatalf("slaughter the tame cow: %+v", f.jobs[handler])
	}
	if run(f, hd, handler, 50); len(f.killed) != 1 || !careful || hd.Careful(10) {
		t.Fatalf("killed %v careful %v", f.killed, careful)
	}
}

// killSpy notes whether the herd called the kill careful.
type killSpy struct {
	*fakeHost
	hd  *Herd
	saw *bool
}

func (k *killSpy) Kill(aid, by entity.EntityID) {
	*k.saw = k.hd.Careful(aid)
	k.fakeHost.Kill(aid, by)
}

func TestAutoSlaughterOldestFirstNoFlap(t *testing.T) {
	f := newFake()
	s := &Settings{}
	hd := New(f, s)
	day := uint64(clock.TicksPerDay)
	for i := range 5 {
		a := animal.State{Species: animal.Goat, Tame: true, Female: true, Born: f.now - uint64(50+10*i)*day}
		if i == 4 {
			a.Bond = 7 // the oldest is bonded: never culled
		}
		f.add(entity.EntityID(10+i), a, i, 0)
	}
	f.add(20, animal.State{Species: animal.Goat, Tame: true, Born: f.now - day}, 9, 9) // a kid
	hd.Auto.Apply(command.Command{Name: "goat", X: -1, Y: 2, X2: 0})
	hd.Auto.Apply(command.Command{Name: "nobody", X: 1})
	var keys []string
	hd.Auto.Each(func(k string, c Caps) { keys = append(keys, k) })
	if len(keys) != 1 || s.Caps["goat"] != (Caps{-1, 2, 0}) {
		t.Fatalf("caps %v", s.Caps)
	}
	if n := hd.Auto.Check(); n != 4 {
		t.Fatalf("marked %d, want 3 females and the kid", n)
	}
	for id, want := range map[entity.EntityID]bool{10: false, 11: true, 12: true, 13: true, 14: false, 20: true} {
		if f.animals[id].Designated != want {
			t.Fatalf("goat %d marked %v", id, f.animals[id].Designated)
		}
	}
	if hd.Auto.Check() != 0 {
		t.Fatal("no flap: a marked animal never counts again")
	}
	hd.Auto.Apply(command.Command{Name: "goat", X: -1, Y: -1, X2: -1})
	if len(s.Caps) != 0 || hd.Auto.Check() != 0 {
		t.Fatal("all caps off drops the row")
	}
	if !CheckTick(CheckHour*clock.TicksPerHour) || CheckTick(1) {
		t.Fatal("check tick")
	}
}

func TestFeedDownedAnimal(t *testing.T) {
	f := newFake()
	hd := New(f, &Settings{})
	f.add(10, animal.State{Species: animal.Labrador, Tame: true, Downed: true, Food: 1000, HasSex: true}, 4, 0)
	if hd.Feed.Assign(handler) {
		t.Fatal("no food: no run")
	}
	f.now += FeedRetry
	f.stacks[50] = item.Stack{Kind: item.Kibble, Qty: 80, X: 1, Y: 1}
	if !hd.AssignTame(handler) || f.jobs[handler].Kind != job.FeedPatient {
		t.Fatalf("feed the downed dog first: %+v", f.jobs[handler])
	}
	if !hd.Do(handler, f.jobs[handler]) {
		t.Fatal("a herd feeding")
	}
	run(f, hd, handler, 50)
	if a := f.animals[10]; a.Food < animal.FeedStop-500 || len(f.fed) != 1 || Due(&a) {
		t.Fatalf("fed to about FeedStop: %+v fed %v", a, f.fed)
	}
	if hd.Do(handler, job.Record{Kind: job.FeedPatient, Target: 999}) {
		t.Fatal("a care feeding (its target no animal) is not the herd's")
	}
}

func TestHunterCarriesItsKill(t *testing.T) {
	f := newFake()
	hd := New(f, &Settings{})
	f.jobs[handler] = job.Record{Kind: job.Hunt, Target: 10}
	f.stacks[60] = item.Stack{Kind: item.CorpseAnimal, Qty: 1}
	hd.Killed(handler, 11, 60) // another animal: not its prey
	if hd.CarryKill(handler) {
		t.Fatal("only its own prey")
	}
	hd.Killed(handler, 10, 60)
	if !hd.CarryKill(handler) || len(f.hauled) != 1 || hd.CarryKill(handler) {
		t.Fatalf("carried once: %v", f.hauled)
	}
}
