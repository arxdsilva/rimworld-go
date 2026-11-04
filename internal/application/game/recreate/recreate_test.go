package recreate

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is an open 40x40 field: every cell free and reachable, no
// roofs unless set, full sight, power where set.
type fakeHost struct {
	now     uint64
	ids     []entity.EntityID
	bldgs   map[entity.EntityID]construction.Building
	objects map[[2]int]entity.EntityID
	pos     map[entity.EntityID][2]int
	jobs    map[entity.EntityID]job.Record
	joy     map[entity.EntityID]int
	xp      map[skill.ID]int
	powered map[entity.EntityID]bool
	roofed  map[[2]int]bool
	sky     int
	pawns   []entity.EntityID
	enjoyed int
	held    map[[2]int]bool
	starts  int
}

func newFake() *fakeHost {
	return &fakeHost{now: 1, bldgs: map[entity.EntityID]construction.Building{}, objects: map[[2]int]entity.EntityID{},
		pos: map[entity.EntityID][2]int{}, jobs: map[entity.EntityID]job.Record{}, joy: map[entity.EntityID]int{},
		xp: map[skill.ID]int{}, powered: map[entity.EntityID]bool{}, roofed: map[[2]int]bool{}, sky: 1000}
}

func (f *fakeHost) add(id entity.EntityID, b construction.Building) {
	f.ids = append(f.ids, id)
	f.bldgs[id] = b
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		f.objects[c] = id
	}
}

func (f *fakeHost) pawn(id entity.EntityID, x, y, joy int) {
	f.pawns = append(f.pawns, id)
	f.pos[id], f.joy[id] = [2]int{x, y}, joy
}

func (f *fakeHost) Pos(id entity.EntityID) (int, int)           { return f.pos[id][0], f.pos[id][1] }
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record)     { f.jobs[id] = j }
func (f *fakeHost) BuildingIDs() []entity.EntityID              { return f.ids }
func (f *fakeHost) AreaAllows(entity.EntityID, int, int) bool   { return true }
func (f *fakeHost) Reachable(entity.EntityID, int, int) bool    { return true }
func (f *fakeHost) ReserveTile(entity.EntityID, int, int)       {}
func (f *fakeHost) Job(id entity.EntityID) job.Record           { return f.jobs[id] }
func (f *fakeHost) ObjectAt(x, y int) entity.EntityID           { return f.objects[[2]int{x, y}] }
func (f *fakeHost) Roof(x, y int) (bool, bool)                  { return f.roofed[[2]int{x, y}], false }
func (f *fakeHost) Now() uint64                                 { return f.now }
func (f *fakeHost) Powered(id entity.EntityID) bool             { return f.powered[id] }
func (f *fakeHost) Sky() int                                    { return f.sky }
func (f *fakeHost) Sight(entity.EntityID) int                   { return 1000 }
func (f *fakeHost) PawnIDs() []entity.EntityID                  { return f.pawns }
func (f *fakeHost) Holds(x, y int) bool                         { return f.held[[2]int{x, y}] }
func (f *fakeHost) Wealth() int                                 { return 0 }
func (f *fakeHost) Joy(id entity.EntityID) int                  { return f.joy[id] }
func (f *fakeHost) AddJoy(id entity.EntityID, n int)            { f.joy[id] = recreation.Clamp(f.joy[id] + n) }
func (f *fakeHost) Learn(_ entity.EntityID, sk skill.ID, n int) { f.xp[sk] += n }
func (f *fakeHost) Enjoyed(entity.EntityID)                     { f.enjoyed++ }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	rec.State = job.Do // the fake walks no path: it is there
	f.jobs[id] = rec
	f.starts++
	return true
}
func (f *fakeHost) TileFree(id entity.EntityID, x, y int) bool {
	for pid, j := range f.jobs {
		if pid != id && j.Active() && j.TX == x && j.TY == y {
			return false
		}
	}
	return true
}

// run steps colonist id's session tick by tick until it ends (at most n
// ticks) and returns the ticks it ran.
func run(s *State, f *fakeHost, id entity.EntityID, n int) int {
	t := 0
	for ; t < n && f.jobs[id].Active(); t++ {
		f.now++
		s.Do(id, f.jobs[id])
	}
	return t
}

// TestHourGainsAtWikiRates (AX-6, wiki: Recreation 36 %/h x power): an
// hour at each piece, from empty with no tolerance, gives 36 % times its
// recreation power; a legendary chess table 1.8 times that.
func TestHourGainsAtWikiRates(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k    construction.Kind
		q    uint8
		want int // points of 10000
	}{
		{construction.HorseshoePin, 0, 3600}, {construction.UrBoard, 0, 2880}, {construction.ChessTable, 0, 3600},
		{construction.ChessTable, quality.Stored(quality.Legendary), 6480}, {construction.BilliardsTable, 0, 4680},
		{construction.PokerTable, 0, 4680}, {construction.TubeTV, 0, 4320}, {construction.FlatscreenTV, 0, 5040},
		{construction.BigScreenTV, 0, 5760}, {construction.Telescope, 0, 4320},
	} {
		f := newFake()
		f.sky = 0
		f.add(10, construction.Building{Kind: c.k, X: 20, Y: 20, Quality: c.q})
		f.powered[10] = true
		f.pawn(1, 18, 18, 0)
		s := New(f)
		if !s.Seek(1, clock.TicksPerHour) {
			t.Fatalf("%s: no session", c.k)
		}
		before := f.joy[1]
		run(s, f, 1, clock.TicksPerHour)
		got := f.joy[1] - before
		// tolerance cuts the gain as it builds (2/3 of it): J' = g(1-2J/3),
		// so an hour gives 1.5(1-e^(-2g/3)) of full for g the hour's gain.
		g := float64(c.want) / recreation.Full
		want := int(1.5 * (1 - math.Exp(-2*g/3)) * recreation.Full)
		if got < want-want/100-15 || got > want+want/100+15 {
			t.Fatalf("%s q%d: an hour gave %d, want %d (%d before tolerance)", c.k, c.q, got, want, c.want)
		}
		if sp := mustSpec(t, c.k); sp.Learns && f.xp[sp.Trains] != clock.TicksPerHour/XPEvery {
			t.Fatalf("%s: %d xp in an hour, want %d (0.004 a tick)", c.k, f.xp[sp.Trains], clock.TicksPerHour/XPEvery)
		}
		if p := s.Tolerance(1).Pct(mustSpec(t, c.k).Joy); p < got*2/3/100-1 || p > got*2/3/100 {
			t.Fatalf("%s: tolerance %d%% after %d recreation, want 2/3 of it", c.k, p, got)
		}
	}
}

func mustSpec(t *testing.T, k construction.Kind) Spec {
	t.Helper()
	sp, ok := SpecOf(k)
	if !ok {
		t.Fatalf("%s is no recreation piece", k)
	}
	return sp
}

// TestVarietyPicksTheFreshKind (AX-6, wiki: Recreation): of two pieces
// the same distance off, a colonist picks the joy kind it has the less
// tolerance of, and never one it is bored of.
func TestVarietyPicksTheFreshKind(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.ChessTable, X: 10, Y: 10})
	f.add(11, construction.Building{Kind: construction.HorseshoePin, X: 30, Y: 10})
	f.pawn(1, 20, 10, 0)
	s := New(f)
	p := s.s.Pawns[1]
	p.Tol.Milli[recreation.JoyCerebral] = 20 * recreation.Full * 10 // 20 %
	p.Tol.At = f.now
	s.s.Pawns[1] = p
	if !s.Seek(1, 300) || f.jobs[1].Dest != 11 {
		t.Fatalf("with cerebral at 20%% it went to %d, want the pin (dexterity)", f.jobs[1].Dest)
	}
	f.jobs[1] = job.Record{}
	p.Tol.Milli[recreation.JoyDexterity] = 60 * recreation.Full * 10 // bored
	p.Tol.Bored = 1 << recreation.JoyDexterity
	p.Tol.Milli[recreation.JoyCerebral] = 55 * recreation.Full * 10 // above dexterity, but not marked bored
	s.s.Pawns[1] = p
	if !s.Seek(1, 300) || f.jobs[1].Dest != 10 {
		t.Fatalf("bored of dexterity it went to %d, want the chess table", f.jobs[1].Dest)
	}
}

// TestBoredHysteresis (AX-6, wiki: bored above 50 %, not below 30 %): a
// kind marked bored past 50 % stays bored while its tolerance wears off
// to 30 %, and only below that is sought again; it never flaps.
func TestBoredHysteresis(t *testing.T) {
	t.Parallel()
	var tol recreation.Tolerance
	flips, was := 0, false
	for i := 0; i < 5000; i++ {
		tol.Take(recreation.JoyCerebral, 10*1000) // 10 points a step, until bored
		if b := tol.IsBored(recreation.JoyCerebral); b != was {
			flips, was = flips+1, b
		}
		if was {
			break
		}
	}
	if !was || tol.Pct(recreation.JoyCerebral) < 50 {
		t.Fatalf("not bored at %d%%", tol.Pct(recreation.JoyCerebral))
	}
	// wear off an hour at a time (18 %/day): still bored at 40 %, free below 30 %
	for h := uint64(1); h < 24*5; h++ {
		tol.Decay(tol.At+clock.TicksPerHour, 18, clock.TicksPerDay)
		pct, b := tol.Pct(recreation.JoyCerebral), tol.IsBored(recreation.JoyCerebral)
		if b != was {
			flips, was = flips+1, b
		}
		if pct >= 30 && !b {
			t.Fatalf("no longer bored at %d%%", pct)
		}
	}
	if was || flips != 2 {
		t.Fatalf("bored %v after wearing off, %d flips (want 2)", was, flips)
	}
}

// TestDecayByWealth (AX-6, wiki: Recreation expectations): 18 % a day in
// a poor colony, 7 % in a sky-high one.
func TestDecayByWealth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ w, pct int }{{0, 18}, {20000, 13}, {50000, 11}, {100000, 10}, {200000, 8}, {400000, 7}} {
		if got := recreation.DecayPct(c.w); got != c.pct {
			t.Fatalf("wealth %d: %d%%/day, want %d", c.w, got, c.pct)
		}
		tol := recreation.Tolerance{At: 1}
		tol.Milli[recreation.JoyTelevision] = recreation.Full * 1000 / 2
		tol.Decay(1+clock.TicksPerDay, c.pct, clock.TicksPerDay)
		if got := tol.Pct(recreation.JoyTelevision); got != 50-c.pct {
			t.Fatalf("wealth %d: 50%% wore to %d%% in a day, want %d", c.w, got, 50-c.pct)
		}
	}
}

// TestTelevisionNoFlap (AX-6): a screen whose power flickers off every
// other tick cuts a watcher's session, and the watcher seeks no piece
// for RetryTicks however often the power comes back.
func TestTelevisionNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.TubeTV, X: 20, Y: 20})
	f.pawn(1, 20, 24, 0)
	s := New(f)
	if s.Seek(1, 600) {
		t.Fatal("an unpowered screen is watched")
	}
	f.powered[10] = true
	if !s.Seek(1, 600) || f.jobs[1].Kind != job.WatchTelevision {
		t.Fatalf("job %v, want watching television", f.jobs[1].Kind)
	}
	starts := f.starts
	for i := 0; i < RetryTicks*2; i++ {
		f.now++
		f.powered[10] = i%2 == 0
		if j := f.jobs[1]; j.Active() {
			s.Do(1, j)
		} else {
			s.Seek(1, 600)
		}
	}
	if got := f.starts - starts; got > 2 {
		t.Fatalf("%d sessions started in %d flickering ticks, want at most 2 (one per retry wait)", got, RetryTicks*2)
	}
}

// TestTelescopeAtNightOutdoors (AX-6, wiki: Telescope): only under an
// open sky and only at night; a stargazer holds on into dusk until
// NightEnd and stops past it.
func TestTelescopeAtNightOutdoors(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.Telescope, X: 20, Y: 20})
	f.pawn(1, 20, 25, 0)
	s := New(f)
	f.sky = 600
	if s.Seek(1, 2000) {
		t.Fatal("stargazing by day")
	}
	f.sky, f.roofed[[2]int{20, 20}] = 100, true
	if s.Seek(1, 2000) {
		t.Fatal("stargazing under a roof")
	}
	f.roofed[[2]int{20, 20}] = false
	if !s.Seek(1, 2000) || f.jobs[1].Kind != job.Stargaze || f.jobs[1].TX != 20 || f.jobs[1].TY != 21 {
		t.Fatalf("job %+v, want stargazing from the spot (20,21)", f.jobs[1])
	}
	f.sky = NightStart + 50 // dusk: past the start line, short of the end
	f.now++
	s.Do(1, f.jobs[1])
	if !f.jobs[1].Active() {
		t.Fatal("dusk cut the session short")
	}
	f.sky = NightEnd + 1
	f.now++
	s.Do(1, f.jobs[1])
	if f.jobs[1].Active() {
		t.Fatal("still stargazing in daylight")
	}
}

// TestGamesSeatPlayersTogether (AX-6): a poker table takes 4 players
// around it, a chair beside it is taken first, and a fifth colonist finds
// no place; two players at a chess table fill it.
func TestGamesSeatPlayersTogether(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.PokerTable, X: 20, Y: 20})
	f.add(20, construction.Building{Kind: construction.Chair, X: 22, Y: 21})
	s := New(f)
	for i := entity.EntityID(1); i <= 5; i++ {
		f.pawn(i, 25, 21, 0)
		ok := s.Seek(i, 600)
		if i <= 4 && (!ok || f.jobs[i].Dest != 10) {
			t.Fatalf("player %d: no seat at the poker table", i)
		}
		if i == 5 && ok {
			t.Fatal("a fifth player at a table for 4")
		}
	}
	if j := f.jobs[1]; j.TX != 22 || j.TY != 21 {
		t.Fatalf("first player at %d,%d, want the chair at 22,21", j.TX, j.TY)
	}
	if v := NewView(f, s); v.Joy(1) != "cerebral" {
		t.Fatalf("enjoying %q, want cerebral", v.Joy(1))
	}
	if joy, users := NewView(f, s).PieceView(10); joy != "cerebral" || users != 4 {
		t.Fatalf("pane %q %d, want cerebral 4", joy, users)
	}
}

// TestWatchCellsFaceTheScreen (AX-6, wiki: watch areas 15, 30, 42): the
// cells lie in front of the screen for every facing.
func TestWatchCellsFaceTheScreen(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k construction.Kind
		n int
	}{{construction.TubeTV, 15}, {construction.FlatscreenTV, 30}, {construction.BigScreenTV, 42}} {
		for f := construction.South; f <= construction.West; f++ {
			cells := construction.WatchCells(c.k, f, 20, 20)
			if len(cells) != c.n {
				t.Fatalf("%s %s: %d watch cells, want %d", c.k, f, len(cells), c.n)
			}
			fp, _, _ := construction.Footprint(c.k, f, 20, 20)
			for _, w := range cells {
				for _, b := range fp {
					if w == b {
						t.Fatalf("%s %s: watch cell %v on the screen", c.k, f, w)
					}
				}
				ok := false
				switch f {
				case construction.South:
					ok = w[1] > fp[0][1]
				case construction.North:
					ok = w[1] < fp[0][1]
				case construction.East:
					ok = w[0] > fp[0][0]
				case construction.West:
					ok = w[0] < fp[0][0]
				}
				if !ok {
					t.Fatalf("%s %s: watch cell %v behind the screen at %v", c.k, f, w, fp[0])
				}
			}
		}
	}
}

// TestAdmire (NST-G): a display case is admired only while it holds a
// stack; a sculpture always; the session is a plain Recreate job.
func TestAdmire(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.DisplayCase, X: 20, Y: 20})
	f.pawn(1, 20, 24, 0)
	s := New(f)
	if s.Seek(1, 600) {
		t.Fatal("an empty display case is admired")
	}
	f.held = map[[2]int]bool{{20, 20}: true}
	if !s.Seek(1, 600) || f.jobs[1].Dest != 10 || f.jobs[1].Kind != job.Recreate {
		t.Fatalf("stocked case: job %+v, want Recreate at 10", f.jobs[1])
	}
	if sp := mustSpec(t, construction.DisplayCase); sp.Mode != Admire || sp.Joy != recreation.JoySolitary || sp.Power != 700 || sp.Users != 1 || sp.Verb != "admire" {
		t.Fatalf("spec %+v", sp)
	}
	g := newFake()
	g.add(11, construction.Building{Kind: construction.SmallSculpture, X: 20, Y: 20})
	g.pawn(2, 20, 24, 0)
	if !New(g).Seek(2, 600) || g.jobs[2].Dest != 11 {
		t.Fatal("a sculpture is not admired")
	}
}

// TestAdmireNoFlap: a case whose stack comes and goes every tick cuts the
// session short, and the colonist then waits RetryTicks before another.
func TestAdmireNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.RoundPedestal, X: 20, Y: 20})
	f.pawn(1, 20, 24, 0)
	f.held = map[[2]int]bool{{20, 20}: true}
	s := New(f)
	if !s.Seek(1, 600) {
		t.Fatal("stocked pedestal not admired")
	}
	starts := f.starts
	for i := 0; i < RetryTicks*2; i++ {
		f.now++
		f.held[[2]int{20, 20}] = i%2 == 0
		if j := f.jobs[1]; j.Active() {
			s.Do(1, j)
		} else {
			s.Seek(1, 600)
		}
	}
	if got := f.starts - starts; got > 2 {
		t.Fatalf("%d sessions in %d flickering ticks, want at most 2", got, RetryTicks*2)
	}
}

// TestAdmireDeterministic: the same setup picks the same pieces twice.
func TestAdmireDeterministic(t *testing.T) {
	t.Parallel()
	pick := func() []entity.EntityID {
		f := newFake()
		f.held = map[[2]int]bool{{10, 10}: true, {30, 10}: true}
		f.add(10, construction.Building{Kind: construction.DisplayCase, X: 10, Y: 10})
		f.add(11, construction.Building{Kind: construction.LargeSculpture, X: 30, Y: 10})
		f.add(12, construction.Building{Kind: construction.RoundPedestal, X: 30, Y: 10 + 8})
		var out []entity.EntityID
		s := New(f)
		for pid := entity.EntityID(1); pid <= 3; pid++ {
			f.pawn(pid, 20, 10+int(pid), 0)
			if s.Seek(pid, 300) {
				out = append(out, f.jobs[pid].Dest)
			}
		}
		return out
	}
	a, b := pick(), pick()
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("picks %v vs %v", a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("picks %v vs %v", a, b)
		}
	}
}
