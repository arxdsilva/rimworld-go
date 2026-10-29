package rest

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

func TestAssignFallsBackToFloor(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 4, 4)
	f.floor, f.hasFloor = [2]int{2, 3}, true
	f.slept[1] = 9
	s.Assign(1) // no bed at all
	if j := f.jobs[1]; j.Kind != job.Sleep || j.TX != 2 || j.TY != 3 || j.Target != 0 || f.slept[1] != 0 {
		t.Fatalf("floor sleep %+v slept %v", j, f.slept[1])
	}
	f.bed(10, construction.Bed, 1, 1)
	f.vital[1] = health.Record{Status: health.Downed}
	s.Assign(1) // not alive and well: the floor, never a bed
	if f.bldgs[10].Claimant != 0 {
		t.Fatal("a downed pawn claimed a bed")
	}
	f.vital[1] = health.Record{Status: health.Alive}
	s.Assign(1)
	if f.jobs[1].Target != 10 {
		t.Fatalf("bed sleep %+v", f.jobs[1])
	}
}

func TestDoWakesAndThinks(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.need[1] = needs.Pair{Hunger: needs.Full, Rest: needs.WakeRest - needs.SleepGain}
	j := job.Record{Kind: job.Sleep, State: job.Do, WorkLeft: 1}
	s.Do(1, j, needs.SleepPeriod) // rest reaches WakeRest outside a Sleep block
	if got := f.jobs[1]; got.State != job.Complete || got.WorkLeft != 0 {
		t.Fatalf("did not wake: %+v", got)
	}
	if len(f.logged) != 1 || f.logged[0] || len(f.thought) != 1 {
		t.Fatalf("log %v thoughts %v", f.logged, f.thought)
	}
	f.need[1] = needs.Pair{Hunger: needs.Full, Rest: 0}
	s.Do(1, j, 1)
	if f.jobs[1].State != job.Do {
		t.Fatal("woke while exhausted")
	}
}

// TestWakeDoesNotFlap: a sleeper goes to bed at CriticalRest and gets up
// only at WakeRest (Full in Sleep hours), so no rest level both starts
// and ends sleep.
func TestWakeDoesNotFlap(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	j := job.Record{Kind: job.Sleep, State: job.Do}
	for _, act := range []schedule.Activity{schedule.Anything, schedule.Sleep, schedule.Work} {
		f.act = act
		for rest := 0; rest <= needs.Full; rest += 50 {
			n := needs.Pair{Hunger: needs.Full, Rest: rest}
			if n.Tired() && s.done(1, n, j) {
				t.Fatalf("%v: rest %d both tired and done", act, rest)
			}
		}
	}
	f.act = schedule.Anything
	if s.done(1, needs.Pair{Hunger: needs.Full, Rest: needs.WakeRest - 1}, j) || !s.done(1, needs.Pair{Hunger: needs.Full, Rest: needs.WakeRest}, j) {
		t.Fatal("wake line is not WakeRest")
	}
}

func TestDoneRules(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	full := needs.Pair{Hunger: needs.Full, Rest: needs.Full}
	j := job.Record{Kind: job.Sleep, State: job.Do}

	// collapsed: sleeps to WakeRest whatever happens, then stands up
	f.vital[1] = health.Record{Status: health.Downed, Cause: health.CauseNoBed}
	if s.done(1, needs.Pair{Rest: needs.WakeRest - 1}, j) {
		t.Fatal("collapsed pawn woke early")
	}
	f.act = schedule.Anything
	if !s.done(1, full, j) || f.vital[1].Status != health.Alive {
		t.Fatalf("collapsed pawn stayed down: %+v", f.vital[1])
	}
	f.vital[1] = health.Record{Status: health.Downed, Cause: health.CauseNone}
	if s.done(1, full, j) {
		t.Fatal("a downed (not collapsed) pawn got up")
	}
	f.vital[1] = health.Record{Status: health.Alive}

	// a healthy pawn in a medical bed gets up at once
	f.bed(10, construction.HospitalBed, 1, 1)
	b := f.bldgs[10]
	b.Medical = true
	f.bldgs[10] = b
	jb := j
	jb.Target = 10
	tired := needs.Pair{Hunger: needs.Full, Rest: 0}
	if !s.done(1, tired, jb) {
		t.Fatal("stayed in a medical bed")
	}
	f.medicalOK[1] = true
	if s.done(1, tired, jb) {
		t.Fatal("patient left its medical bed")
	}

	// hungry with food about: up to eat
	f.food = true
	if !s.done(1, needs.Pair{Hunger: 0, Rest: 0}, j) {
		t.Fatal("did not wake to eat")
	}
	f.food = false

	// on bed rest: stays, and the surgeon wait is checked
	f.bedRest[1] = true
	jw := j
	jw.WorkLeft = -7
	if s.done(1, full, jw) || len(f.surgeryRan) != 1 || f.surgeryRan[0] != -7 {
		t.Fatalf("bed rest: %v", f.surgeryRan)
	}
	f.bedRest[1] = false

	// Sleep hours: full rest, and it is rested for the block
	f.act = schedule.Sleep
	if s.done(1, needs.Pair{Hunger: needs.Full, Rest: needs.Full - 1}, j) || s.RestedFree(1) {
		t.Fatal("woke short of full in Sleep hours")
	}
	if !s.done(1, full, j) || !s.RestedFree(1) {
		t.Fatal("not rested at full in Sleep hours")
	}

	// Work hours: over the tired line once the minimum sleep is done
	f.act = schedule.Work
	over := needs.Pair{Hunger: needs.Full, Rest: needs.CriticalRest + 1}
	jm := j
	jm.WorkLeft = 1
	if s.done(1, over, jm) || !s.done(1, over, j) {
		t.Fatal("work hours wake rule")
	}
}

func TestRestedMarksAndSave(t *testing.T) {
	s := New(newFake())
	s.MarkRested(3)
	if !s.RestedFree(3) || !s.Export()[3] {
		t.Fatal("mark")
	}
	s.Unmark(3)
	if s.RestedFree(3) {
		t.Fatal("unmark")
	}
	s.Import(nil) // an older save keeps the live map
	if s.Export() == nil {
		t.Fatal("nil import dropped the map")
	}
	s.Import(map[entity.EntityID]bool{})
}
