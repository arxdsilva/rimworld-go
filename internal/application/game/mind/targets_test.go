package mind

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/entity"
)

// TestInsultingSpreeInsultsAndMovesOn: the spree walks up to a victim,
// insults it, waits InsultGap, then picks someone (rolls pick the victim).
func TestInsultingSpreeInsultsAndMovesOn(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	f.rolls = []uint32{0}
	m.Force(1, mentalbreak.InsultingSpree)
	m.Tick(1, job.Record{}) // picks 2 (roll 0), walks to it
	if len(f.approached) != 1 || f.approached[0].b != 2 {
		t.Fatalf("approached %v", f.approached)
	}
	m.Tick(1, job.Record{})
	if len(f.insults) != 1 || f.insults[0] != (pair{1, 2}) {
		t.Fatalf("insults %v", f.insults)
	}
	start := f.now
	for f.now = start + 1; f.now < start+InsultGap; f.now += 50 {
		m.Tick(1, job.Record{})
	}
	if len(f.insults) != 1 {
		t.Fatal("insulted again inside the gap")
	}
	f.now = start + InsultGap
	f.rolls = []uint32{1}
	m.Tick(1, job.Record{}) // picks 3 (roll 1), walks
	m.Tick(1, job.Record{})
	if len(f.insults) != 2 || f.insults[1].b != 3 {
		t.Fatalf("second insult %v", f.insults)
	}
}

// TestSpreeDoesNotFlap: with victims dropping out every tick the spree
// picks a new one at most every RetargetTicks.
func TestSpreeDoesNotFlap(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	f.unreachable[2], f.unreachable[3] = true, true
	m.Force(1, mentalbreak.InsultingSpree)
	for f.now = 0; f.now < 10*RetargetTicks; f.now++ {
		m.Tick(1, job.Record{})
	}
	if n := len(f.approached); n > 10 {
		t.Fatalf("%d picks in %d ticks: flapping", n, 10*RetargetTicks)
	}
	if len(f.insults) != 0 {
		t.Fatal("insulted someone out of reach")
	}
}

func TestTargetedInsultEndsWhenTargetDies(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	f.rolls = []uint32{1}
	m.Force(1, mentalbreak.TargetedInsult)
	if m.Target(1) != 3 {
		t.Fatalf("target %d", m.Target(1))
	}
	for range 3 {
		m.Tick(1, job.Record{})
		f.now += InsultGap
	}
	for _, p := range f.insults {
		if p.b != 3 {
			t.Fatalf("insulted %d, not the target", p.b)
		}
	}
	if len(f.insults) < 2 {
		t.Fatalf("insults %v", f.insults)
	}
	f.dead[3] = true
	if m.Tick(1, job.Record{}); len(f.recovered) != 1 {
		t.Fatal("the targeted spree ends with its target")
	}
}

func TestBerserkAttacksAndEndsWhenDowned(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	f.nearest = func() entity.EntityID { return 2 }
	m.Force(1, mentalbreak.Berserk)
	m.Tick(1, job.Record{})
	if len(f.assaults) != 1 || f.assaults[0] != (pair{1, 2}) || m.Target(1) != 2 {
		t.Fatalf("assaults %v target %d", f.assaults, m.Target(1))
	}
	f.down[2] = true // the victim goes down: the next one, after the cooldown
	f.nearest = func() entity.EntityID { return 3 }
	f.now++
	m.Tick(1, job.Record{})
	if m.Target(1) != 0 || f.wandered != 1 {
		t.Fatal("waits out the retarget cooldown wandering")
	}
	f.now = RetargetTicks
	m.Tick(1, job.Record{})
	if m.Target(1) != 3 {
		t.Fatalf("target %d after the cooldown", m.Target(1))
	}
	m.Downed(1) // still standing: nothing
	if _, _, ok := f.Break(1); !ok {
		t.Fatal("a standing berserker keeps its break")
	}
	f.down[1] = true
	m.Downed(1)
	if _, _, ok := f.Break(1); ok || len(f.recovered) != 1 {
		t.Fatal("a downed berserker calms")
	}
}

// TestBerserkDoesNotFlap: victims that drop in and out every tick change
// the target at most once per RetargetTicks.
func TestBerserkDoesNotFlap(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	flip := entity.EntityID(2)
	f.nearest = func() entity.EntityID { return flip }
	m.Force(1, mentalbreak.Berserk)
	changes, last := 0, entity.EntityID(0)
	for f.now = 0; f.now < 20*RetargetTicks; f.now++ {
		f.down[2], f.down[3] = f.now%2 == 0, f.now%2 == 1
		flip = 5 - flip
		m.Tick(1, job.Record{})
		if tg := m.Target(1); tg != 0 && tg != last {
			changes, last = changes+1, tg
		}
	}
	if changes > 21 {
		t.Fatalf("%d target changes in %d ticks: flapping", changes, 20*RetargetTicks)
	}
}

func TestBerserkWithNobodyWandersAndUnreachableDrops(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	m.Force(1, mentalbreak.Berserk)
	m.Tick(1, job.Record{})
	if f.wandered != 1 {
		t.Fatal("nobody near: wanders")
	}
	f.nearest = func() entity.EntityID { return 2 }
	f.unreachable[2] = true
	f.now = RetargetTicks
	m.Tick(1, job.Record{})
	if m.Target(1) != 0 {
		t.Fatal("an unreachable victim is dropped")
	}
	f.unreachable[2] = false
	f.x[2] = 1000 // too far to keep
	m.track[1] = Track{Target: 2}
	f.now = 2 * RetargetTicks
	f.nearest = func() entity.EntityID { return 0 }
	m.Tick(1, job.Record{})
	if m.Target(1) != 0 {
		t.Fatal("a victim far out of range is dropped")
	}
}

func TestMurderousRageHuntsItsTargetUntilDead(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	f.prisoners = []entity.EntityID{9}
	f.x[9] = 50
	f.rolls = []uint32{1} // the prisoner
	m.Force(1, mentalbreak.MurderousRage)
	if m.Target(1) != 9 || len(f.targets) != 1 || f.targets[0] != 9 {
		t.Fatalf("target %d", m.Target(1))
	}
	m.Tick(1, job.Record{})
	f.down[9] = true // downed is not enough
	m.Tick(1, job.Record{})
	if len(f.assaults) != 2 || f.assaults[1].b != 9 {
		t.Fatalf("assaults %v", f.assaults)
	}
	if _, _, ok := f.Break(1); !ok {
		t.Fatal("ended before the target died")
	}
	f.dead[9] = true
	m.Tick(1, job.Record{})
	if _, _, ok := f.Break(1); ok || len(f.recovered) != 1 {
		t.Fatal("a murderous rage ends when its target dies")
	}
}

func TestMurderousRageRetriesUnreachableOnCadence(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	f.unreachable[2] = true
	m.Force(1, mentalbreak.MurderousRage)
	for f.now = 0; f.now < 4*RetargetTicks; f.now++ {
		m.Tick(1, job.Record{})
	}
	if n := len(f.assaults); n > 5 {
		t.Fatalf("%d attempts on an unreachable target", n)
	}
	m.track[1] = Track{}
	m.Tick(1, job.Record{})
	if _, _, ok := f.Break(1); ok {
		t.Fatal("a rage with no target ends")
	}
}

func TestCatatonicLiesDownAndRecovers(t *testing.T) {
	f := newFake(1)
	m := New(f)
	m.Force(1, mentalbreak.Catatonic)
	_, until, _ := f.Break(1)
	if !f.down[1] || until != mentalbreak.DefOf(mentalbreak.Catatonic).Duration {
		t.Fatalf("down %v until %d", f.down[1], until)
	}
	f.now = until - 1
	m.Downed(1)
	if _, _, ok := f.Break(1); !ok || !f.down[1] {
		t.Fatal("recovered early")
	}
	f.down[1] = false // someone stood it up
	m.Tick(1, job.Record{})
	if !f.down[1] {
		t.Fatal("a catatonic pawn collapses again")
	}
	f.now = until
	m.Downed(1)
	if _, _, ok := f.Break(1); ok || f.ups != 1 || f.down[1] || len(f.recovered) != 1 {
		t.Fatal("recovers and gets up at its time")
	}
}

func TestDeadKeepNoBreak(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	m.Force(1, mentalbreak.Berserk)
	f.dead[1] = true
	m.Downed(1)
	if _, _, ok := f.Break(1); ok || len(f.recovered) != 0 {
		t.Fatal("the dead lose the break, without a recovery")
	}
	m.Downed(2) // no break: nothing
}

func TestHostile(t *testing.T) {
	for k, want := range map[mentalbreak.Kind]bool{
		mentalbreak.Berserk: true, mentalbreak.MurderousRage: true,
		mentalbreak.InsultingSpree: false, mentalbreak.Catatonic: false, mentalbreak.None: false,
	} {
		if Hostile(k) != want {
			t.Errorf("Hostile(%v) = %v", k, !want)
		}
	}
}
