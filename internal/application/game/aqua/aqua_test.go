package aqua

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	rules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

const (
	tankA  entity.EntityID = 10
	tankB  entity.EntityID = 11
	hand   entity.EntityID = 1
	nobody entity.EntityID = 2
)

type drop struct{ x, y, qty int }

type fake struct {
	now      uint64
	tanks    map[entity.EntityID][2]int
	blocked  map[entity.EntityID]bool
	power    bool
	air      temperature.Temp
	feed     int
	msgs     []string
	drops    []drop
	skill    int
	best     int
	jobs     map[entity.EntityID]job.Record
	starts   int
	owners   map[entity.EntityID]entity.EntityID
	unreach  bool
	putOff   map[entity.EntityID]bool
	offCalls int
	work     int
}

func newFake() *fake {
	return &fake{tanks: map[entity.EntityID][2]int{tankA: {5, 5}, tankB: {30, 5}}, blocked: map[entity.EntityID]bool{}, power: true,
		air: temperature.C(20), feed: rules.FeedFull, skill: 10, best: 10, jobs: map[entity.EntityID]job.Record{},
		owners: map[entity.EntityID]entity.EntityID{}, putOff: map[entity.EntityID]bool{}, work: 1}
}

func (f *fake) Now() uint64                { return f.now }
func (f *fake) Message(_, _ int, s string) { f.msgs = append(f.msgs, s) }
func (f *fake) Tank(id entity.EntityID) (int, int, bool) {
	p, ok := f.tanks[id]
	return p[0], p[1], ok
}
func (f *fake) Workable(id entity.EntityID) bool        { return !f.blocked[id] }
func (f *fake) Powered(entity.EntityID) bool            { return f.power }
func (f *fake) Air(_, _ int) temperature.Temp           { return f.air }
func (f *fake) Feed(entity.EntityID) int                { return f.feed }
func (f *fake) DropFish(x, y, qty int)                  { f.drops = append(f.drops, drop{x, y, qty}) }
func (f *fake) Animals(entity.EntityID) int             { return f.skill }
func (f *fake) BestAnimals() int                        { return f.best }
func (f *fake) Pos(entity.EntityID) (int, int, bool)    { return 0, 0, true }
func (f *fake) Work(entity.EntityID) int                { return f.work }
func (f *fake) SetJob(id entity.EntityID, j job.Record) { f.jobs[id] = j }
func (f *fake) StartJob(id entity.EntityID, rec job.Record) bool {
	f.starts++
	rec.State = job.Do
	f.jobs[id] = rec
	return true
}
func (f *fake) Owner(t entity.EntityID) (entity.EntityID, bool) {
	o, ok := f.owners[t]
	return o, ok
}
func (f *fake) BesideTank(_, tank entity.EntityID) (int, int, bool) {
	p := f.tanks[tank]
	return p[0] + 1, p[1], !f.unreach
}
func (f *fake) Postponed(_ entity.EntityID, _ job.Kind, t entity.EntityID) bool { return f.putOff[t] }
func (f *fake) PutOff(_ entity.EntityID, _ job.Kind, t entity.EntityID) {
	f.offCalls++
	f.putOff[t] = true
}

// hours runs n hours of Phase.
func hours(f *fake, ts *Tanks, n int) {
	for i := 0; i < n; i++ {
		f.now += clock.TicksPerHour
		ts.Phase()
	}
}

// finish works id's job to its end.
func finish(t *testing.T, f *fake, ts *Tanks, id entity.EntityID) job.Record {
	t.Helper()
	for i := 0; i < 2000; i++ {
		j := f.jobs[id]
		if j.State != job.Do {
			return j
		}
		ts.Do(id, j)
	}
	t.Fatal("job never ended")
	return job.Record{}
}

func TestApplyChoosesAndClears(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Apply(99, rules.Carp) // no such tank
	ts.Apply(tankA, rules.None)
	ts.Apply(tankA, rules.NumSpecies)
	if ts.Export() != nil {
		t.Fatalf("no state expected: %+v", ts.Export())
	}
	ts.Apply(tankB, rules.Carp)
	ts.Apply(tankA, rules.Trout)
	if got := ts.Export(); len(got) != 2 || got[0].ID != tankA || got[1].ID != tankB || got[0].Tank.Species != rules.Trout {
		t.Fatalf("sorted by id: %+v", got)
	}
	if !ts.WantsFeed(tankA) || ts.Stocked(tankA) {
		t.Fatal("a chosen tank wants feed, and has no fish yet")
	}
	ts.Apply(tankA, rules.None)
	ts.Apply(tankB, rules.None)
	if ts.Export() != nil || ts.WantsFeed(tankA) {
		t.Fatalf("cleared: %+v", ts.Export())
	}
}

// TestApplyNewStockEmptiesTank: a new choice replaces what lives there; a
// ripe stock's fish are left on the ground.
func TestApplyNewStockEmptiesTank(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	e := ts.entry(tankA)
	e.Tank = e.Tank.Fill()
	ts.Apply(tankA, rules.Tilapia) // the same: nothing moves
	if !ts.Stocked(tankA) {
		t.Fatal("the same choice emptied the tank")
	}
	ts.Apply(tankA, rules.Carp)
	if ts.Stocked(tankA) || len(f.drops) != 0 {
		t.Fatalf("growing stock: %+v %+v", ts.entry(tankA), f.drops)
	}
	e = ts.entry(tankA)
	e.Tank = e.Tank.Fill()
	e.Tank.Growth = rules.DefOf(rules.Carp).GrowTicks
	ts.Apply(tankA, rules.Trout)
	if len(f.drops) != 1 || f.drops[0].qty != rules.NetYield(rules.Carp, 0) || ts.Stocked(tankA) {
		t.Fatalf("ripe stock: %+v %+v", ts.entry(tankA), f.drops)
	}
}

// TestPhaseIsHourlyAndGrows: only on the hour, only stocked tanks.
func TestPhaseIsHourlyAndGrows(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	ts.Apply(tankB, rules.Tilapia)
	ts.entry(tankA).Tank = ts.entry(tankA).Tank.Fill()
	f.now = 7
	ts.Phase()
	if ts.entry(tankA).Tank.Growth != 0 {
		t.Fatal("stepped off the hour")
	}
	f.now = 0
	hours(f, ts, 3)
	if got := ts.entry(tankA).Tank.Growth; got != 3*clock.TicksPerHour {
		t.Fatalf("growth %d", got)
	}
	if ts.entry(tankB).Tank != (rules.Tank{Species: rules.Tilapia}) {
		t.Fatalf("an empty tank stepped: %+v", ts.entry(tankB).Tank)
	}
}

// TestDeathsAreToldWithTheirReason, for each reason, with one warning first.
func TestDeathsAreToldWithTheirReason(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name        string
		set         func(*fake)
		dying, died string
	}{
		{"power", func(f *fake) { f.power = false }, "no power", "died without power"},
		{"cold", func(f *fake) { f.air = temperature.C(-20) }, "too cold", "died of cold"},
		{"hot", func(f *fake) { f.air = temperature.C(60) }, "too hot", "died of heat"},
		{"feed", func(f *fake) { f.feed = 0 }, "no feed", "starved"},
	} {
		f := newFake()
		ts := New(f)
		ts.Apply(tankA, rules.Tilapia)
		ts.entry(tankA).Tank = ts.entry(tankA).Tank.Fill()
		c.set(f)
		hours(f, ts, 300)
		if ts.Stocked(tankA) {
			t.Fatalf("%s: still alive", c.name)
		}
		if len(f.msgs) != 2 || !strings.HasSuffix(f.msgs[0], c.dying) || !strings.HasSuffix(f.msgs[1], c.died) ||
			!strings.Contains(f.msgs[1], "tilapia") {
			t.Fatalf("%s: %q", c.name, f.msgs)
		}
	}
}

// TestAssignOffersOnlyDueWork: stock needs a chosen species, power and skill;
// netting needs ripe fish and skill and no power.
func TestAssignOffersOnlyDueWork(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	if ts.Assign(hand) {
		t.Fatal("no tanks in use: nothing to do")
	}
	ts.Apply(tankA, rules.Trout)
	f.skill = 7
	if ts.Assign(hand) {
		t.Fatal("trout need Animals 8")
	}
	f.skill = 8
	f.power = false
	if ts.Assign(hand) {
		t.Fatal("no stocking without power")
	}
	f.power = true
	f.blocked[tankA] = true
	if ts.Assign(hand) {
		t.Fatal("a tank marked for removal or forbidden")
	}
	delete(f.blocked, tankA)
	f.owners[tankA] = nobody
	if ts.Assign(hand) {
		t.Fatal("a tank someone holds")
	}
	delete(f.owners, tankA)
	if !ts.Assign(hand) {
		t.Fatal("stock job expected")
	}
	if j := f.jobs[hand]; j.Kind != job.StockTank || j.Target != tankA || j.WorkLeft != rules.StockWork || j.TX != 6 {
		t.Fatalf("%+v", j)
	}
	finish(t, f, ts, hand)
	if !ts.Stocked(tankA) || ts.entry(tankA).Tank.Health != rules.HealthFull {
		t.Fatalf("%+v", ts.entry(tankA))
	}
	delete(f.jobs, hand)
	if ts.Assign(hand) {
		t.Fatal("a growing stock asks for nothing")
	}
	ts.entry(tankA).Tank.Growth = rules.DefOf(rules.Trout).GrowTicks
	f.power = false
	if !ts.Assign(hand) || f.jobs[hand].Kind != job.NetTank {
		t.Fatalf("netting needs no power: %+v", f.jobs[hand])
	}
}

// TestAssignNearestFirst: the nearest tank first, whichever its id.
func TestAssignNearestFirst(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.tanks[tankA], f.tanks[tankB] = [2]int{40, 40}, [2]int{3, 3}
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	ts.Apply(tankB, rules.Tilapia)
	if !ts.Assign(hand) || f.jobs[hand].Target != tankB {
		t.Fatalf("%+v", f.jobs[hand])
	}
}

// TestNetDropsScaledFishAndRestocks.
func TestNetDropsScaledFishAndRestocks(t *testing.T) {
	t.Parallel()
	for _, lvl := range []int{0, 5, 20} {
		f := newFake()
		ts := New(f)
		f.skill = lvl
		ts.Apply(tankA, rules.Tilapia)
		e := ts.entry(tankA)
		e.Tank = e.Tank.Fill()
		e.Tank.Growth = rules.DefOf(rules.Tilapia).GrowTicks
		if !ts.Assign(hand) {
			t.Fatal("net job expected")
		}
		if j := finish(t, f, ts, hand); j.State != job.Complete {
			t.Fatalf("%+v", j)
		}
		if want := rules.NetYield(rules.Tilapia, lvl); len(f.drops) != 1 || f.drops[0] != (drop{6, 5, want}) {
			t.Fatalf("level %d: %+v want %d", lvl, f.drops, want)
		}
		if tk := ts.entry(tankA).Tank; !tk.Stocked || tk.Growth != 0 || tk.Species != rules.Tilapia {
			t.Fatalf("the tank restocks by itself: %+v", tk)
		}
	}
}

// TestDoEnds: gone tank fails the job; a stock changed on the way completes
// it without doing anything; a no-work tick (low mood) waits.
func TestDoEnds(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	j := job.Record{Kind: job.StockTank, State: job.Do, Target: tankA, WorkLeft: 1}
	f.work = 0
	ts.Do(hand, j)
	if ts.Stocked(tankA) {
		t.Fatal("no work done, yet stocked")
	}
	f.work = 1
	f.jobs[hand] = j
	ts.Apply(tankA, rules.None)
	f.tanks[tankA] = [2]int{5, 5}
	ts.Do(hand, j)
	if got := f.jobs[hand]; got.State != job.Fail || got.Fail != job.TargetGone {
		t.Fatalf("no entry: %+v", got)
	}
	ts.Apply(tankA, rules.Tilapia)
	j.Kind = job.NetTank
	ts.Do(hand, j)
	if got := f.jobs[hand]; got.State != job.Complete || len(f.drops) != 0 {
		t.Fatalf("nothing ripe to net: %+v %+v", got, f.drops)
	}
	delete(f.tanks, tankA)
	ts.Do(hand, j)
	if got := f.jobs[hand]; got.State != job.Fail || got.Fail != job.TargetGone {
		t.Fatalf("no tank: %+v", got)
	}
}

// TestGone: the state goes; ripe fish stay.
func TestGone(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Gone(tankA, 1, 1) // no state: nothing happens
	ts.Apply(tankA, rules.Tilapia)
	ts.Apply(tankB, rules.Carp)
	e := ts.entry(tankA)
	e.Tank = e.Tank.Fill()
	e.Tank.Growth = rules.DefOf(rules.Tilapia).GrowTicks
	ts.Gone(tankB, 2, 2)
	if len(f.drops) != 0 || ts.entry(tankB) != nil {
		t.Fatalf("an empty tank has no fish: %+v", f.drops)
	}
	ts.Gone(tankA, 7, 8)
	if len(f.drops) != 1 || f.drops[0] != (drop{7, 8, rules.NetYield(rules.Tilapia, 0)}) || ts.Export() != nil {
		t.Fatalf("%+v %+v", f.drops, ts.Export())
	}
}

// TestUnreachableTankIsPutOff: a tank out of reach costs one attempt, then
// waits out the cooldown; another tank still gets its handler.
func TestUnreachableTankIsPutOff(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.unreach = true
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	for i := 0; i < 50; i++ {
		if ts.Assign(hand) {
			t.Fatal("no way there")
		}
	}
	if f.offCalls != 1 || f.starts != 0 {
		t.Fatalf("put off %d times, started %d", f.offCalls, f.starts)
	}
}

// TestExportImportRoundTrip.
func TestExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	f := newFake()
	if Import(f, nil) != nil {
		t.Fatal("nil while no tank")
	}
	var none *Tanks
	if none.Export() != nil {
		t.Fatal("a nil set exports nothing")
	}
	ts := New(f)
	ts.Apply(tankA, rules.Carp)
	ts.entry(tankA).Tank = ts.entry(tankA).Tank.Fill()
	hours(f, ts, 5)
	got := Import(f, ts.Export())
	if !reflect.DeepEqual(got.Export(), ts.Export()) {
		t.Fatalf("%+v != %+v", got.Export(), ts.Export())
	}
	got.entry(tankA).Tank.Growth = 1
	if ts.entry(tankA).Tank.Growth == 1 {
		t.Fatal("the import shares its list")
	}
}

// TestPowerBlinkNoFlap: a grid blinking every hour while a handler works
// never ends the job, and after it no new job starts over and over.
func TestPowerBlinkNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	ts.Apply(tankA, rules.Tilapia)
	if !ts.Assign(hand) {
		t.Fatal("stock job expected")
	}
	for i := 0; f.jobs[hand].State == job.Do; i++ {
		f.power = i%2 == 0
		if i%5 == 0 {
			hours(f, ts, 1)
		}
		ts.Do(hand, f.jobs[hand])
	}
	if f.jobs[hand].State != job.Complete || !ts.Stocked(tankA) {
		t.Fatalf("%+v", f.jobs[hand])
	}
	delete(f.jobs, hand)
	// a stock alive and growing asks for nothing however the grid blinks; one
	// that died of it asks once for a new stock, once per death
	f.starts = 0
	deaths := 0
	for i := 0; i < 500; i++ {
		f.power = i%2 == 0
		was := ts.Stocked(tankA)
		hours(f, ts, 1)
		if was && !ts.Stocked(tankA) {
			deaths++
		}
		if ts.Stocked(tankA) && ts.Assign(hand) {
			t.Fatal("a job for a stock that needs none")
		}
		if !ts.Stocked(tankA) && ts.Assign(hand) {
			finish(t, f, ts, hand)
			delete(f.jobs, hand)
		}
	}
	if f.starts > deaths {
		t.Fatalf("%d jobs started for %d deaths", f.starts, deaths)
	}
}

// TestView: the read model, each trouble in the player's words.
func TestView(t *testing.T) {
	t.Parallel()
	f := newFake()
	ts := New(f)
	if ts.View(99) != nil {
		t.Fatal("no tank, no view")
	}
	v := ts.View(tankA)
	if v.Stock != "" || v.StockID != 0 || v.Stocked || v.Ripe || v.Trouble != "" || v.FeedPct != 100 || len(v.Options) != 3 {
		t.Fatalf("no stock: %+v", v)
	}
	if v.Options[0] != (Option{1, "tilapia", ""}) || v.Options[2].Label != "trout" || v.Options[2].ID != 3 {
		t.Fatalf("options: %+v", v.Options)
	}
	f.best = 4
	if o := ts.View(tankA).Options; o[1].Reason != "" || o[2].Reason != "Needs Animals 8" {
		t.Fatalf("reasons: %+v", o)
	}
	f.power = false
	if got := ts.View(tankA).Trouble; got != "No power" {
		t.Fatalf("no stock, no power: %q", got)
	}
	f.power = true
	ts.Apply(tankA, rules.Tilapia)
	f.feed = 0
	if v := ts.View(tankA); v.Trouble != "" || v.Stock != "tilapia" || v.StockID != 1 || v.FeedPct != 0 {
		t.Fatalf("an empty tank needs no feed yet: %+v", v)
	}
	ts.entry(tankA).Tank = ts.entry(tankA).Tank.Fill()
	if got := ts.View(tankA).Trouble; got != "No feed" {
		t.Fatalf("%q", got)
	}
	f.feed = rules.FeedFull / 2
	f.air = temperature.C(-5)
	if v := ts.View(tankA); v.Trouble != "Too cold" || v.FeedPct != 50 {
		t.Fatalf("%+v", v)
	}
	f.air = temperature.C(50)
	if got := ts.View(tankA).Trouble; got != "Too hot" {
		t.Fatalf("%q", got)
	}
	f.air = temperature.C(25)
	ts.entry(tankA).Tank.Growth = rules.DefOf(rules.Tilapia).GrowTicks
	if v := ts.View(tankA); !v.Ripe || v.GrowthPct != 100 || v.HealthPct != 100 || !v.Stocked || v.Trouble != "" {
		t.Fatalf("%+v", v)
	}
}

// TestViewEqual: every field reads.
func TestViewEqual(t *testing.T) {
	t.Parallel()
	base := func() *View {
		return &View{Stock: "carp", StockID: 2, Options: []Option{{1, "a", ""}}, GrowthPct: 1, HealthPct: 2, FeedPct: 3}
	}
	if !base().Equal(base()) {
		t.Fatal("a copy differs")
	}
	var none *View
	if !none.Equal(nil) || none.Equal(base()) || base().Equal(nil) {
		t.Fatal("nil handling")
	}
	rt := reflect.TypeOf(View{})
	for i := range rt.NumField() {
		b := base()
		f := reflect.ValueOf(b).Elem().Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(f.String() + "x")
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.Int:
			f.SetInt(f.Int() + 1)
		case reflect.Slice:
			b.Options[0].Reason = "x"
		}
		if base().Equal(b) {
			t.Errorf("field %s is not compared", rt.Field(i).Name)
		}
	}
}
