package prison

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	warden entity.EntityID = 1
	rid    entity.EntityID = 5
	rid2   entity.EntityID = 6
)

// rig is a prison with prisoner rid at (10, 10) and warden at (0, 0).
func rig(t *testing.T) (*Prison, *fakeHost) {
	t.Helper()
	f := newFake()
	f.put(warden, 0, 0)
	f.put(rid, 10, 10)
	f.raiders = []entity.EntityID{rid}
	p := New(f)
	p.Take(rid, warden, 0)
	return p, f
}

func TestTakeSetModeRelease(t *testing.T) {
	p, f := rig(t)
	st := p.State(rid)
	if !p.Is(rid) || st.Mode != prisoner.Recruit || st.Captor != uint64(warden) || st.Resistance <= 0 {
		t.Fatalf("taken prisoner %+v", st)
	}
	if want := uint64(rid) * visitStagger % prisoner.VisitInterval; st.NextVisit != want {
		t.Fatalf("first visit %d, want %d (staggered by id)", st.NextVisit, want)
	}
	p.SetMode(rid, uint8(prisoner.Execute)+1)
	if st.Mode != prisoner.Recruit {
		t.Fatal("an unknown mode is ignored")
	}
	p.SetMode(rid, uint8(prisoner.Reduce))
	if st.Mode != prisoner.Reduce {
		t.Fatal("SetMode sets the mode")
	}
	f.now = 77
	p.Release(rid, warden)
	r := f.Held(rid)
	if p.Is(rid) || !r.Leaving || r.Hungry || r.Idle != 77 || r.Warden != warden || len(f.messages) != 1 {
		t.Fatalf("released: prisoner %v, held %+v, messages %v", p.Is(rid), r, f.messages)
	}
	p.Release(rid, warden) // no prisoner: nothing happens
	if len(f.messages) != 1 {
		t.Fatal("releasing a non-prisoner does nothing")
	}
	if p.Export() == nil {
		t.Fatal("Export is the live map")
	}
	p.Import(nil)
	if p.Is(rid2) || p.Assign(warden) {
		t.Fatal("no prisoners: no warden work")
	}
}

func TestFoodBandDoesNotFlap(t *testing.T) {
	p, f := rig(t)
	flips, was := 0, false
	for i := 0; i < 400; i++ {
		h := FoodLow - 50 + (i%40)*20 // wobbles across FoodLow, never reaches FoodFed
		p.FoodBand(rid, needs.Pair{Hunger: h, Rest: needs.Full})
		if now := f.Held(rid).Hungry; now != was {
			flips, was = flips+1, now
		}
	}
	if flips != 1 || !was {
		t.Fatalf("hungry flag flipped %d times inside the band (want once, on)", flips)
	}
	p.FoodBand(rid, needs.Pair{Hunger: FoodFed})
	if f.Held(rid).Hungry {
		t.Fatal("fed past FoodFed: no more meals")
	}
}

func TestAssignFoodRunAndRetry(t *testing.T) {
	p, f := rig(t)
	f.held[rid] = Held{Hungry: true}
	if p.Assign(warden) {
		t.Fatal("no food to be had: no run")
	}
	if r := f.Held(rid); r.FoodRetry != foodRetry {
		t.Fatalf("food retry %d, want %d", r.FoodRetry, foodRetry)
	}
	f.feedOK, f.now = true, foodRetry
	if !p.Assign(warden) {
		t.Fatal("food found: a run starts")
	}
	f.bedBack = true
	if !p.Assign(warden) {
		t.Fatal("a downed prisoner back to bed comes first")
	}
}

func TestAssignVisitNearest(t *testing.T) {
	p, f := rig(t)
	f.put(rid2, 3, 3)
	f.raiders = append(f.raiders, rid2)
	p.Take(rid2, warden, 0)
	p.State(rid).NextVisit, p.State(rid2).NextVisit = 0, 0
	p.State(rid2).Mode = prisoner.Execute
	if !p.Assign(warden) {
		t.Fatal("a visit is due")
	}
	j := f.started[len(f.started)-1]
	if j.Kind != job.Warden || j.Target != rid2 || j.WorkLeft != executeWork || j.TX != 4 {
		t.Fatalf("visit %+v: the nearest prisoner, beside it, execution's work", j)
	}
	f.owner[rid2] = 99 // another warden holds it
	f.started = nil
	if !p.Assign(warden) || f.started[0].Target != rid || f.started[0].WorkLeft != chatWork {
		t.Fatalf("a held prisoner is skipped: %+v", f.started)
	}
	p.State(rid).NextVisit = 1 << 40
	delete(f.owner, rid2)
	f.vital[rid2] = health.Record{Status: health.Dead}
	if p.Assign(warden) {
		t.Fatal("nobody due: no visit")
	}
}

func TestDoChatsChasesAndRecruits(t *testing.T) {
	p, f := rig(t)
	st := p.State(rid)
	st.Mode, st.Resistance = prisoner.Reduce, 20000
	j := job.Record{Kind: job.Warden, Target: rid, WorkLeft: 100}
	p.Do(warden, j)
	if f.chases != 1 {
		t.Fatal("a warden not beside the prisoner follows it")
	}
	f.pos[warden] = [2]int{9, 10}
	p.Do(warden, job.Record{Kind: job.Warden, Target: rid, WorkLeft: 150})
	if f.jobs[warden].WorkLeft != 50 || f.chats != 0 {
		t.Fatalf("work left %d: a visit takes its time", f.jobs[warden].WorkLeft)
	}
	p.Do(warden, f.jobs[warden])
	want := 20000 - prisoner.Reduction(f.negotiation, prisoner.ResistanceFactor(f.opinion, f.mood))
	if st.Resistance != want || f.chats != 1 || f.Held(rid).Warden != warden || f.jobs[warden].State != job.Complete {
		t.Fatalf("resistance %d want %d, chats %d, warden %d", st.Resistance, want, f.chats, f.Held(rid).Warden)
	}
	st.Mode, st.Resistance = prisoner.Recruit, 0
	p.Chat(warden, rid, 5)
	if p.Is(rid) || len(f.recruited) != 1 || f.chats != 2 {
		t.Fatalf("broken prisoner, roll 0: recruited %v", f.recruited)
	}
}

func TestDoReleaseAndExecute(t *testing.T) {
	p, f := rig(t)
	f.pos[warden] = [2]int{10, 11}
	p.State(rid).Mode = prisoner.Release
	p.Do(warden, job.Record{Kind: job.Warden, Target: rid, WorkLeft: 1})
	if p.Is(rid) || !f.Held(rid).Leaving {
		t.Fatal("a Release visit frees the prisoner")
	}
	p.Take(rid, warden, 0)
	p.State(rid).Mode = prisoner.Execute
	p.Do(warden, job.Record{Kind: job.Warden, Target: rid, WorkLeft: 1})
	if p.Is(rid) || len(f.executed) != 1 {
		t.Fatal("an Execute visit kills the prisoner")
	}
	p.Do(warden, job.Record{Kind: job.Warden, Target: rid, WorkLeft: 1})
	if f.jobs[warden].State != job.Complete {
		t.Fatal("a visit to no prisoner completes")
	}
}

func TestLive(t *testing.T) {
	p, f := rig(t)
	f.needs[rid] = needs.Pair{Hunger: needs.Full, Rest: 0}
	f.bed = 40
	p.Live(rid, 1)
	if j := f.jobs[rid]; j.Kind != job.Sleep || j.Target != 40 {
		t.Fatalf("a tired prisoner goes to its bed: %+v", j)
	}
	f.jobs[rid] = job.Record{Kind: job.Sleep, State: job.Do}
	f.needs[rid] = needs.Pair{Hunger: needs.Full, Rest: needs.Full}
	p.Live(rid, 2)
	if f.jobs[rid].Kind != job.None {
		t.Fatal("rested: it gets up")
	}
	p.Live(rid, 900-7*uint64(rid)%900)
	if f.jobs[rid].Kind != job.Move {
		t.Fatalf("idle on its stroll tick: it potters about, %+v", f.jobs[rid])
	}
	f.jobs[rid] = job.Record{State: job.Complete}
	f.owner[40] = rid
	p.Live(rid, 3)
	if f.jobs[rid].Kind != job.None || f.jobs[rid].State == job.Complete || len(f.owner) != 0 {
		t.Fatal("a finished job is cleared and its reservations go")
	}
	f.held[rid] = Held{Leaving: true}
	p.Live(rid, 4)
	if j := f.jobs[rid]; j.Kind != job.Move || j.TX != 0 || j.TY != 10 {
		t.Fatalf("a released pawn walks out: %+v", j)
	}
	f.needs[rid] = needs.Pair{Hunger: 0, Rest: needs.Full}
	p.Live(rid, 5)
	if f.vital[rid].Status != health.Alive {
		t.Fatal("a starving prisoner stays up: its malnutrition is the game's (FOOD-1)")
	}
	f.vital[rid] = health.Record{Status: health.Dead, Cause: health.CauseNoFood}
	p.Live(rid, 6)
	if p.Is(rid) || f.Held(rid).Leaving {
		t.Fatal("the dead are no prisoners, and walk nowhere")
	}
}
