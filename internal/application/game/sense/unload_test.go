package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

// unloadScene is pawn 1 with `qty` steel in its pack, the entry marked.
func unloadScene(qty int) (*Sense, *errand) {
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{10: {Kind: item.Steel, Qty: qty}}}
	s, e := errandScene(f)
	if qty > 100 {
		s.Rules = s.Rules.With(gameplay.RespectCapacity, false) // more than a pack takes
	}
	s.Gear.Stow(1, 10, qty)
	e.hunger = 0 // no spare meal in the way
	s.Gear.Mark(1, 0, int(s.Gear.Pack(1).Items[0].ID), true)
	return s, e
}

// steelInPlay counts steel on the map and in pack 1.
func steelInPlay(e *errand) int {
	n := 0
	for _, st := range e.stacks {
		if st.Kind == item.Steel {
			n += st.Qty
		}
	}
	for _, en := range e.s.Packs[1].Items {
		n += en.Stack.Qty
	}
	return n
}

// until runs ticks until cond, at most max.
func (e *errand) until(max int, cond func() bool) bool {
	for range max {
		if cond() {
			return true
		}
		e.run()
	}
	return cond()
}

func TestMarkUnloadCommand(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	id := int(s.Gear.Pack(1).Items[0].ID)
	if !s.Gear.Pack(1).Items[0].Unload {
		t.Fatal("the mark did not take")
	}
	s.Gear.Mark(1, 0, id, false)
	if s.Gear.Pack(1).Items[0].Unload {
		t.Fatal("the mark did not come off")
	}
	// bad input does nothing
	before := s.Gear.Pack(1)
	for name, f := range map[string]func(){
		"no such pawn":   func() { s.Gear.Mark(7, 0, id, true) },
		"no such entry":  func() { s.Gear.Mark(1, 0, id+5, true) },
		"negative entry": func() { s.Gear.Mark(1, 0, -1, true) },
		"unknown mode":   func() { s.Gear.Mark(1, 4, id, true) },
		"no garment":     func() { s.Gear.Mark(1, 1, 0, true) },
		"negative worn":  func() { s.Gear.Mark(1, 1, -1, true) },
		"unarmed":        func() { s.Gear.Mark(1, 2, 0, true) },
		"no spare":       func() { s.Gear.Mark(1, 3, 0, true) },
	} {
		f()
		if got := s.Gear.Pack(1); len(got.Items) != 1 || got.Items[0].Unload != before.Items[0].Unload || len(e.starts) != 0 {
			t.Fatalf("%s changed something: pack %+v, %d jobs", name, got, len(e.starts))
		}
	}
	s.Rules = s.Rules.With(gameplay.ManualUnload, false)
	s.Gear.Mark(1, 0, id, true)
	if s.Gear.Pack(1).Items[0].Unload {
		t.Fatal("the buttons are off, and the command marked anyway")
	}
}

func TestMarkWornThingsStartTheirUnload(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(1)
	e.worn, e.armed, e.spare = []int{4, 2}, true, true
	s.Gear.Mark(1, 1, 1, true)
	if j := e.jobs[1]; j.Kind != job.UnloadPack || j.Count != modeApparel || j.BillIndex != 2 {
		t.Fatalf("job %+v, want a garment unload of slot 2", j)
	}
	if !e.until(200, func() bool { return len(e.takenOff) == 1 }) || e.takenOff[0] != 2 {
		t.Fatalf("took off %v, want slot 2", e.takenOff)
	}
	s.Gear.Mark(1, 2, 0, true)
	s.Gear.Mark(1, 2, 0, false) // cancelled before the weapon is down
	if e.jobs[1].Kind != job.None || !e.armed {
		t.Fatalf("cancel left %+v, armed %v", e.jobs[1], e.armed)
	}
	s.Gear.Mark(1, 2, 0, true)
	e.until(100, func() bool { return !e.armed })
	s.Gear.Mark(1, 3, 0, true)
	if !e.until(100, func() bool { return !e.spare }) {
		t.Fatal("the spare weapon was not put down")
	}
}

func TestUnloadStoresMarked(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	e.store = true
	if !s.Gear.Assign(1) || e.jobs[1].Kind != job.UnloadPack {
		t.Fatalf("a marked entry started nothing: %+v", e.jobs[1])
	}
	if !e.until(100, func() bool { return len(e.laid) == 1 }) {
		t.Fatal("never set down")
	}
	if last := e.starts[len(e.starts)-1]; last.TX != 9 || last.TY != 9 {
		t.Fatalf("walked to (%d, %d), want the store cell (9, 9)", last.TX, last.TY)
	}
	if steelInPlay(e) != 20 || len(s.Gear.Pack(1).Items) != 0 {
		t.Fatalf("%d steel in play, pack %+v: want all 20 on the ground", steelInPlay(e), s.Gear.Pack(1))
	}
	if _, ok := s.FromPack[1]; ok || len(s.Packs) != 0 {
		t.Fatalf("leftover state: from pack %v, packs %v", s.FromPack, s.Packs)
	}
}

func TestUnloadNoStoreDrops(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	s.Gear.Assign(1)
	if !e.until(100, func() bool { return len(e.laid) == 1 }) {
		t.Fatal("with no store the colonist never set it down")
	}
	for _, r := range e.starts {
		if r.TX != 5 || r.TY != 5 {
			t.Fatalf("walked to (%d, %d) with nowhere to store it", r.TX, r.TY)
		}
	}
	if steelInPlay(e) != 20 {
		t.Fatalf("%d steel in play, want 20", steelInPlay(e))
	}
}

func TestUnloadCancelStops(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(150) // one trip takes 100: the entry stays, marked, with 50
	e.store = true
	s.Gear.Assign(1)
	e.until(100, func() bool { return e.jobs[1].Carry != 0 })
	if s.FromPack[1] == 0 || len(s.Gear.Pack(1).Items) != 1 {
		t.Fatalf("from pack %v, pack %+v: want 100 in hand, 50 still in the pack", s.FromPack, s.Gear.Pack(1))
	}
	s.Gear.Mark(1, 0, int(s.Gear.Pack(1).Items[0].ID), false)
	items := s.Gear.Pack(1).Items
	if e.jobs[1].Kind != job.None || len(items) != 1 || items[0].Stack.Qty != 150 || items[0].Unload || len(e.laid) != 0 {
		t.Fatalf("job %+v, pack %+v, laid %v: want the load back in the pack and the mark gone", e.jobs[1], items, e.laid)
	}
	n := len(e.starts)
	for range 3 * unloadRetry {
		e.run()
	}
	if len(e.starts) != n || steelInPlay(e) != 150 {
		t.Fatalf("it went on unloading: %d new jobs, %d steel", len(e.starts)-n, steelInPlay(e))
	}
}

func TestInterruptReturnsToPack(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	e.store = true
	s.Gear.Assign(1)
	e.until(100, func() bool { return e.jobs[1].Carry != 0 })
	e.Interrupt(1) // hungry, say
	items := s.Gear.Pack(1).Items
	if len(items) != 1 || items[0].Stack.Qty != 20 || !items[0].Unload || len(e.laid) != 0 {
		t.Fatalf("pack %+v, laid %v: want 20 steel back in the pack, still marked", items, e.laid)
	}
	if _, ok := s.FromPack[1]; ok || steelInPlay(e) != 20 {
		t.Fatalf("from pack %v, %d steel in play", s.FromPack, steelInPlay(e))
	}
	// the colonist waits before it tries again
	n := len(e.starts)
	wait := int(s.UnloadAt[1]-e.now) - 2
	for range wait {
		e.run()
	}
	if wait < unloadRetry/2 || len(e.starts) != n {
		t.Fatalf("began again after %d ticks (cooldown %d)", wait, unloadRetry)
	}
}

func TestGroundLoadStillDrops(t *testing.T) {
	t.Parallel()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{10: {Kind: item.Steel, Qty: 5, Carrier: 1}}}
	s, _ := errandScene(f)
	if s.Gear.BackToPack(1, 10) || len(s.Packs) != 0 {
		t.Fatal("a stack picked up off the ground went into the pack")
	}
	s.FromPack[1] = 10 // recorded, but the rule is off
	s.Rules = s.Rules.With(gameplay.PutBack, false)
	if s.Gear.BackToPack(1, 10) {
		t.Fatal("went back with the rule off")
	}
	if _, ok := s.FromPack[1]; ok {
		t.Fatal("kept the record after the stack went down")
	}
}

func TestCancelOverCapacityDrops(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	e.store = true
	s.Gear.Assign(1)
	e.until(100, func() bool { return e.jobs[1].Carry != 0 })
	e.manip = 0 // its hands are hurt now: nothing fits
	e.Interrupt(1)
	if len(s.Gear.Pack(1).Items) != 0 || len(e.laid) != 1 || steelInPlay(e) != 20 {
		t.Fatalf("pack %+v, laid %v, %d steel: want it dropped, none lost", s.Gear.Pack(1), e.laid, steelInPlay(e))
	}
}

func TestUnloadDoesNotFlap(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	e.startOK = false // nowhere it can begin
	const ticks = 5000
	for range ticks {
		e.run()
	}
	if want := ticks/unloadRetry + 1; len(e.starts) > want {
		t.Fatalf("tried %d times in %d ticks, want at most %d", len(e.starts), ticks, want)
	}
	if len(e.starts) < 2 {
		t.Fatalf("tried %d times, it should retry on the cooldown", len(e.starts))
	}
	if steelInPlay(e) != 20 || !s.Gear.Pack(1).Items[0].Unload {
		t.Fatal("a failed start changed the pack or the mark")
	}
}

func TestUnloadTakesEveryMarkedEntry(t *testing.T) {
	t.Parallel()
	s, e := unloadScene(20)
	e.stacks[11] = item.Stack{Kind: item.Wood, Qty: 9}
	s.Gear.Stow(1, 11, 9)
	s.Gear.Mark(1, 0, int(s.Gear.Pack(1).Items[1].ID), true)
	if !e.until(2000, func() bool { return len(e.laid) == 2 }) {
		t.Fatalf("laid %d of 2 marked entries", len(e.laid))
	}
	if len(s.Packs) != 0 {
		t.Fatalf("pack %+v left", s.Packs)
	}
}

func TestAssignAllocatesNothingWhenNothingIsMarked(t *testing.T) {
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{10: {Kind: item.Steel, Qty: 4}}}
	s, e := errandScene(f)
	s.Gear.Stow(1, 10, 4)
	e.hunger = 0
	e.now = 1
	s.SpareAt[1], s.UnloadAt[1] = 1<<60, 1<<60
	if n := testing.AllocsPerRun(100, func() { s.Gear.Assign(1) }); n != 0 {
		t.Fatalf("%v allocations per Assign", n)
	}
	e.s.Rules = e.s.Rules.With(gameplay.SpareMeal, false)
	if n := testing.AllocsPerRun(100, func() { s.Gear.Assign(2) }); n != 0 {
		t.Fatalf("%v allocations per Assign for a pawn with no pack", n)
	}
}

// BenchmarkAssignNothingMarked is a colonist's turn when its pack holds nothing
// to put away and the spare meal is on cooldown.
func BenchmarkAssignNothingMarked(b *testing.B) {
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{10: {Kind: item.Steel, Qty: 4}}}
	s, e := errandScene(f)
	s.Gear.Stow(1, 10, 4)
	e.hunger, e.now = 0, 1
	b.ReportAllocs()
	for b.Loop() {
		s.Gear.Assign(1)
	}
}
