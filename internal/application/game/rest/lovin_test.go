package rest

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// lovers puts colonists 1 and 2, partners fond of each other, asleep in
// double bed 10 of kind k.
func lovers(k construction.Kind) (*fakeHost, *Sleep) {
	f := newFake()
	s := New(f)
	f.pawn(1, 3, 3)
	f.pawn(2, 4, 3)
	f.mate[1], f.mate[2] = 2, 1
	f.age[1], f.age[2] = 20, 20
	f.bed(10, k, 3, 3)
	b := f.bldgs[10]
	b.Claimant, b.Claimant2 = 1, 2
	f.bldgs[10] = b
	for _, id := range []entity.EntityID{1, 2} {
		f.slept[id] = 10
		f.jobs[id] = job.Record{Kind: job.Sleep, State: job.Do, Target: 10}
	}
	return f, s
}

// TestLovinCheck: the lower id decides for both; a lucky roll starts it
// and sets both cooldowns; downed, hurt, cold-hearted, on cooldown, not
// in the same double bed or an unlucky roll: nothing.
func TestLovinCheck(t *testing.T) {
	for _, k := range []construction.Kind{construction.DoubleBed, construction.DoubleBedroll} {
		f, s := lovers(k)
		s.Lovin.Check(2, 100) // the higher id never decides
		if f.rolls != 0 {
			t.Fatal("higher id rolled")
		}
		f.roll = 999 // unlucky
		s.Lovin.Check(1, 100)
		if f.jobs[1].Kind != job.Sleep || f.rolls != 1 {
			t.Fatalf("unlucky roll started it: %+v", f.jobs[1])
		}
		f.roll = 0
		s.Lovin.Check(1, 100)
		j1, j2 := f.jobs[1], f.jobs[2]
		if j1.Kind != job.Lovin || j2.Kind != job.Lovin || j1.Target != 2 || j2.Target != 1 || j1.Dest != 10 || j1.WorkLeft != LovinTicks {
			t.Fatalf("%v: no lovin': %+v / %+v", k, j1, j2)
		}
		if f.lovinAt[1] != 100+3750 || f.lovinAt[2] != f.lovinAt[1] {
			t.Fatalf("cooldowns %v", f.lovinAt)
		}
	}
	block := []func(f *fakeHost){
		func(f *fakeHost) { f.down[2] = true },
		func(f *fakeHost) { f.pain[1] = lovinMaxPain + 1 },
		func(f *fakeHost) { f.opinion[[2]entity.EntityID{2, 1}] = -1 },
		func(f *fakeHost) { f.lovinAt[2] = 500 },
		func(f *fakeHost) { f.slept[2] = 0 },
		func(f *fakeHost) { f.jobs[2] = job.Record{Kind: job.Eat} },
		func(f *fakeHost) { f.mate[2] = 3 },
		func(f *fakeHost) { b := f.bldgs[10]; b.Claimant2 = 0; f.bldgs[10] = b },
	}
	for i, fn := range block {
		f, s := lovers(construction.DoubleBed)
		fn(f)
		s.Lovin.Check(1, 100)
		if f.jobs[1].Kind == job.Lovin {
			t.Errorf("case %d: lovin' anyway", i)
		}
	}
	f, s := lovers(construction.Bed) // a single bed is never shared
	f.noBody = true
	if _, ok := s.Lovin.Bed(1, 2); ok {
		t.Fatal("single bed")
	}
}

// TestLovinDo: the job counts down and both remember it; a partner who
// left ends it with no thought.
func TestLovinDo(t *testing.T) {
	f, s := lovers(construction.DoubleBed)
	f.jobs[1] = job.Record{Kind: job.Lovin, State: job.Do, Target: 2, WorkLeft: 2}
	f.jobs[2] = job.Record{Kind: job.Lovin, State: job.Do, Target: 1, WorkLeft: 2}
	s.Lovin.Do(1, f.jobs[1])
	if j := f.jobs[1]; j.State != job.Do || j.WorkLeft != 1 || len(f.thought) != 0 {
		t.Fatalf("first tick %+v", j)
	}
	s.Lovin.Do(1, f.jobs[1])
	if j := f.jobs[1]; j.State != job.Complete || len(f.thought) != 1 || f.thought[0] != mood.Lovin || f.withOf[1] != 2 {
		t.Fatalf("done %+v %v", j, f.thought)
	}
	f.jobs[2] = job.Record{Kind: job.Eat}
	s.Lovin.Do(1, job.Record{Kind: job.Lovin, State: job.Do, Target: 2, WorkLeft: 5})
	if j := f.jobs[1]; j.State != job.Complete || len(f.thought) != 1 {
		t.Fatalf("partner left: %+v %v", j, f.thought)
	}
}
