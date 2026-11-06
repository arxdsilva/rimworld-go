package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// errand is the game's side of Gear's errands: a pawn that stands still, jobs it
// is given, and switches the test turns.
type errand struct {
	*fakeGear
	s      *Sense
	now    uint64
	jobs   map[entity.EntityID]job.Record
	starts []job.Record // every StartJob, in order
	// switches
	startOK            bool
	hunger             int
	hungry, free, tire bool
	meal               entity.EntityID
	store              bool
	armed, spare       bool
	worn               []int
	searches           int
	laid               []entity.EntityID
	takenOff           []int
	orders             // GEAR-1's side (drop_test.go)
}

// errandScene builds the habits on an errand game side over f.
func errandScene(f *fakeGear) (*Sense, *errand) {
	e := &errand{fakeGear: f, jobs: map[entity.EntityID]job.Record{}, startOK: true, hunger: needs.Full, free: true}
	e.s = New(Roles{Gear: e})
	return e.s, e
}

func (e *errand) Pos(entity.EntityID) (int, int) { return 5, 5 }
func (e *errand) Now() uint64                    { return e.now }
func (e *errand) StartJob(id entity.EntityID, r job.Record) bool {
	e.starts = append(e.starts, r)
	if !e.startOK {
		e.jobs[id] = job.Record{Kind: r.Kind, State: job.Fail}
		return false
	}
	r.State = job.Do
	e.jobs[id] = r
	return true
}
func (e *errand) Job(id entity.EntityID) job.Record       { return e.jobs[id] }
func (e *errand) SetJob(id entity.EntityID, j job.Record) { e.jobs[id] = j }
func (e *errand) Rested(entity.EntityID) bool             { return !e.tire }
func (e *errand) Free(entity.EntityID) bool               { return e.free && !e.tire }
func (e *errand) NearestMeal(entity.EntityID) (entity.EntityID, int, int, bool) {
	e.searches++
	if _, ok := e.stacks[e.meal]; !ok {
		return 0, 0, 0, false
	}
	return e.meal, 6, 6, true
}
func (e *errand) Allows(entity.EntityID, item.Kind) bool { return true }
func (e *errand) StoreCell(entity.EntityID, item.Stack) (int, int, bool) {
	return 9, 9, e.store
}
func (e *errand) Lay(_, stack entity.EntityID) {
	e.laid = append(e.laid, stack)
	st := e.stacks[stack]
	st.Carrier = 0
	e.stacks[stack] = st
}
func (e *errand) Hunger(entity.EntityID) int  { return e.hunger }
func (e *errand) Hungry(entity.EntityID) bool { return e.hungry }
func (e *errand) Worn(_ entity.EntityID, n int) (int, bool) {
	if n < 0 || n >= len(e.worn) {
		return 0, false
	}
	return e.worn[n], true
}
func (e *errand) TakeOff(_ entity.EntityID, slot int) bool {
	e.takenOff = append(e.takenOff, slot)
	return true
}
func (e *errand) Armed(entity.EntityID) bool      { return e.armed }
func (e *errand) DropWeapon(entity.EntityID) bool { a := e.armed; e.armed = false; return a }
func (e *errand) HasSpare(entity.EntityID) bool   { return e.spare }
func (e *errand) DropSpare(entity.EntityID) bool  { a := e.spare; e.spare = false; return a }

// Interrupt ends the job the way the game does: what is in hand goes back to
// the pack, or down.
func (e *errand) Interrupt(id entity.EntityID) {
	if j := e.jobs[id]; j.Carry != 0 && !e.s.Gear.BackToPack(id, j.Carry) {
		e.Lay(id, j.Carry)
	}
	e.jobs[id] = job.Record{}
}

// run advances pawn 1 by one tick: a job in Do is done, a finished one clears,
// and a pawn with none is given the chance to find one.
func (e *errand) run() {
	e.now++
	j := e.jobs[1]
	switch {
	case j.State == job.Do && j.Kind == job.TakeSpare:
		e.s.Gear.DoSpare(1, j)
	case j.State == job.Do && j.Kind == job.UnloadPack:
		e.s.Gear.DoUnload(1, j)
	case j.State == job.Do && j.Kind == job.Eat:
		if j.WorkLeft--; j.WorkLeft > 0 {
			e.jobs[1] = j
			return
		}
		delete(e.stacks, j.Target) // eaten
		e.hunger, e.hungry = needs.Full, false
		j.Carry, j.State = 0, job.Complete
		e.jobs[1] = j
	case j.State == job.Goto:
		j.State = job.Do // arrives at once
		e.jobs[1] = j
	}
	if j = e.jobs[1]; j.State == job.Complete || j.State == job.Fail {
		if j.Carry != 0 && !e.s.Gear.BackToPack(1, j.Carry) {
			e.Lay(1, j.Carry)
		}
		e.jobs[1] = job.Record{}
	}
	if e.jobs[1].Kind == job.None {
		e.s.Gear.Assign(1)
	}
}

// meals sums the meals on the map and in 1's pack.
func (e *errand) meals() int {
	n := 0
	for _, st := range e.stacks {
		if st.Kind == item.Meal {
			n += st.Qty
		}
	}
	for _, en := range e.s.Packs[1].Items {
		if en.Stack.Kind == item.Meal {
			n += en.Stack.Qty
		}
	}
	return n
}

func TestSpareMealTakenOnce(t *testing.T) {
	t.Parallel()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{11: {Kind: item.Meal, Qty: 3}}}
	s, e := errandScene(f)
	e.meal = 11
	if !s.Gear.Assign(1) {
		t.Fatal("a fed colonist in free time takes no spare meal")
	}
	if j := e.jobs[1]; j.Kind != job.TakeSpare || j.Target != 11 || j.TX != 6 {
		t.Fatalf("job %+v, want TakeSpare of stack 11 at (6, 6)", j)
	}
	e.run()
	if got := s.Gear.Pack(1).Items; len(got) != 1 || got[0].Stack.Kind != item.Meal || got[0].Stack.Qty != 1 {
		t.Fatalf("pack %+v, want one meal", got)
	}
	if e.meals() != 3 {
		t.Fatalf("%d meals in play, want 3: nothing duplicated or lost", e.meals())
	}
	searches := e.searches
	for range 10 * spareGap {
		e.run()
	}
	if e.searches != searches || e.meals() != 3 || s.Gear.Pack(1).Items[0].Stack.Qty != 1 {
		t.Fatalf("took another with one in the pack: %d searches, pack %+v", e.searches-searches, s.Gear.Pack(1))
	}
}

func TestSpareMealWaitsOnItsRules(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*errand){
		"toggle off":     func(e *errand) { e.s.Rules = e.s.Rules.With(gameplay.SpareMeal, false) },
		"food too low":   func(e *errand) { e.hunger = spareFoodMin - 1 },
		"at work":        func(e *errand) { e.free = false },
		"no meal around": func(e *errand) { e.meal = 0 },
	} {
		f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{11: {Kind: item.Meal, Qty: 3}}}
		s, e := errandScene(f)
		e.meal = 11
		set(e)
		if s.Gear.Assign(1) {
			t.Errorf("%s: began a job", name)
		}
	}
}

func TestSpareMealDoesNotFlap(t *testing.T) {
	t.Parallel()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{11: {Kind: item.Meal, Qty: 40}}}
	s, e := errandScene(f)
	e.meal = 11
	var takes, eats []uint64
	last := 0
	for range 60_000 {
		if e.now%3 == 0 { // hunger falls a point every three ticks
			e.hunger = max(e.hunger-1, 0)
			e.hungry = e.hungry && e.hunger < needs.HungryAt+needs.BandGap || e.hunger <= needs.HungryAt
		}
		e.run()
		for ; last < len(e.starts); last++ {
			switch e.starts[last].Kind {
			case job.TakeSpare:
				takes = append(takes, e.now)
			case job.Eat:
				eats = append(eats, e.now)
			}
		}
		if p := s.Gear.Pack(1); len(p.Items) > 1 || len(p.Items) == 1 && p.Items[0].Stack.Qty > 1 {
			t.Fatalf("tick %d: more than one spare in the pack: %+v", e.now, p)
		}
		if m := e.meals() + len(eats); m != 40 && m != 41 { // 41: the one being eaten is both
			t.Fatalf("tick %d: %d meals left after %d eaten: some vanished or doubled", e.now, e.meals(), len(eats))
		}
	}
	if len(takes) < 2 || len(eats) < 2 {
		t.Fatalf("the cycle never ran: %d takes, %d meals eaten", len(takes), len(eats))
	}
	for i := 1; i < len(takes); i++ {
		if takes[i]-takes[i-1] < spareGap {
			t.Fatalf("took spares at %d and %d, closer than %d ticks", takes[i-1], takes[i], spareGap)
		}
	}
	if len(takes) > len(eats)+1 {
		t.Fatalf("%d takes for %d meals eaten: it stocks up and lets them rot", len(takes), len(eats))
	}
}

func TestEatsFromPack(t *testing.T) {
	t.Parallel()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{11: {Kind: item.Meal, Qty: 2}}}
	s, e := errandScene(f)
	s.Gear.Stow(1, 11, 2)
	e.hungry, e.hunger = true, needs.HungryAt
	if !s.Gear.Assign(1) {
		t.Fatal("a hungry colonist with a meal in its pack does not eat")
	}
	j := e.jobs[1]
	st := e.stacks[j.Target]
	if j.Kind != job.Eat || j.Carry != j.Target || st.Carrier != 1 || st.Qty != 1 || s.FromPack[1] != j.Target {
		t.Fatalf("job %+v, stack %+v, from pack %v: want one meal taken out and being eaten", j, st, s.FromPack)
	}
	if got := s.Gear.Pack(1).Items; len(got) != 1 || got[0].Stack.Qty != 1 {
		t.Fatalf("pack %+v, want one meal left", got)
	}
	for range needs.EatWorkTicks + 2 {
		e.run()
	}
	if len(e.stacks) != 0 || e.hungry || e.meals() != 1 {
		t.Fatalf("after eating: stacks %+v, %d meals in play, want only the one in the pack", e.stacks, e.meals())
	}
	if _, stale := s.FromPack[1]; stale {
		t.Fatal("the eaten meal is still recorded as in hand")
	}
}

func TestEatFromPackBeatsNothingWhenKeptByHunger(t *testing.T) {
	t.Parallel()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{12: {Kind: item.Steel, Qty: 5}}}
	s, e := errandScene(f)
	s.Gear.Stow(1, 12, 5)
	e.hungry = true
	if s.Gear.Assign(1) {
		t.Fatal("ate steel")
	}
}
