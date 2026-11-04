package craft

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
)

// fakePrep is the fake's habits and packs. Every rule is off until a test turns
// it on, so the older tests run today's path; the calls are counted, so a test
// can show a path never asked. room is the grams a pack holds (0: unlimited).
type fakePrep struct {
	rules map[gameplay.Toggle]bool
	packs map[entity.EntityID]*inventory.Pack
	room  int

	tidyCalls   int
	tidyUntil   uint64 // TidyBench declines before this tick (the host's cooldown)
	tidyStarts  bool   // TidyBench starts a clean (else it declines)
	resume      job.Record
	stackCalls  int
	stackStarts bool
	stowCalls   int
}

func (f *fakeHost) on(ts ...gameplay.Toggle) {
	if f.rules == nil {
		f.rules = map[gameplay.Toggle]bool{}
	}
	for _, t := range ts {
		f.rules[t] = true
	}
}

func (f *fakeHost) Rule(t gameplay.Toggle) bool { return f.rules[t] }

func (f *fakeHost) TidyBench(id, bench entity.EntityID, j job.Record) bool {
	f.tidyCalls++
	if !f.tidyStarts || f.now < f.tidyUntil {
		return false
	}
	f.tidyUntil = f.now + 1500
	f.resume = j
	f.jobs[id] = job.Record{Kind: job.CleanRoom, State: job.Do}
	return true
}

// releaseOwner drops every claim id holds.
func (f *fakeHost) releaseOwner(id entity.EntityID) {
	for t, o := range f.claims {
		if o == id {
			delete(f.claims, t)
		}
	}
	for _, m := range f.counts {
		delete(m, id)
	}
	for k, o := range f.tiles {
		if o == id {
			delete(f.tiles, k)
		}
	}
}

func (f *fakeHost) StackFirst(id entity.EntityID, x, y int, stacks []entity.EntityID) bool {
	f.stackCalls++
	if !f.stackStarts {
		return false
	}
	f.jobs[id] = job.Record{Kind: job.Haul, State: job.Goto, Target: stacks[0]}
	return true
}

func (f *fakeHost) pack(id entity.EntityID) *inventory.Pack {
	if f.packs == nil {
		f.packs = map[entity.EntityID]*inventory.Pack{}
	}
	p, ok := f.packs[id]
	if !ok {
		p = &inventory.Pack{}
		f.packs[id] = p
	}
	return p
}

func (f *fakeHost) PackRoom(id entity.EntityID) int {
	if f.room == 0 {
		return 1 << 30
	}
	return max(f.room-f.pack(id).Grams(), 0)
}

func (f *fakeHost) Stow(id, stack entity.EntityID, n int) (uint32, bool) {
	f.stowCalls++
	st, ok := f.stacks[stack]
	if !ok || n <= 0 || st.Qty < n || st.Kind.IsCorpse() || st.Kind.MassGrams()*n > f.PackRoom(id) {
		return 0, false
	}
	if st.Qty == n {
		f.DeleteItem(stack)
	} else {
		left := st
		left.Qty -= n
		f.stacks[stack] = left
	}
	st.Qty = n
	p := f.pack(id)
	entry := p.Add(st)
	for i := range p.Items {
		if p.Items[i].ID == entry {
			p.Items[i].Unload = true
		}
	}
	return entry, true
}

func (f *fakeHost) Unstow(id entity.EntityID, entry uint32, n int) (entity.EntityID, bool) {
	st, ok := f.pack(id).Take(entry, n)
	if !ok {
		return 0, false
	}
	f.next++
	st.Carrier = id
	f.put(f.next, st)
	return f.next, true
}

func (f *fakeHost) Settled(entity.EntityID) {}

// inPlay is every item of kind k on the map, in hands and in packs.
func (f *fakeHost) inPlay(k item.Kind) int {
	n := 0
	for _, st := range f.stacks {
		if st.Kind == k {
			n += st.Qty
		}
	}
	for _, p := range f.packs {
		for _, e := range p.Items {
			if e.Stack.Kind == k {
				n += e.Stack.Qty
			}
		}
	}
	return n
}
