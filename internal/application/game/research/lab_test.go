package research

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a tiny colony: buildings by id, one indoor room 1 (or
// outdoors), a temperature, one researcher.
type fakeHost struct {
	ids             []entity.EntityID
	bldgs           map[entity.EntityID]construction.Building
	off             map[entity.EntityID]bool // unpowered
	ver             uint64
	outdoors        bool
	clean, beds     int
	reading         int // AX-7: the room's reading bonus
	benches, temp   int
	pawn            Pawn
	tech            proj.TechLevel
	costPct, diff   int
	now             uint64
	letters         []string
	researcher      bool
	started, worked int
}

func newFake() *fakeHost {
	return &fakeHost{bldgs: map[entity.EntityID]construction.Building{}, off: map[entity.EntityID]bool{}, temp: 210,
		pawn: Pawn{Intellectual: 8, GlobalWork: 1000, Manipulation: 1000, Sight: 1000, TraitPct: 100},
		tech: proj.Industrial, costPct: 100, diff: 100, researcher: true}
}

func (f *fakeHost) add(k construction.Kind, x, y int) entity.EntityID {
	id := entity.EntityID(len(f.ids) + 1)
	f.ids = append(f.ids, id)
	f.bldgs[id] = construction.Building{Kind: k, X: x, Y: y}
	f.ver++
	return id
}

func (f *fakeHost) StructVer() uint64              { return f.ver }
func (f *fakeHost) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) Powered(id entity.EntityID) bool                      { return !f.off[id] }
func (f *fakeHost) Room(int, int) (int, bool)                            { return 1, !f.outdoors }
func (f *fakeHost) Cleanliness(int) int                                  { return f.clean }
func (f *fakeHost) ReadingBonus(int) int                                 { return f.reading }
func (f *fakeHost) Contents(int) (int, int)                              { return f.beds, f.benches }
func (f *fakeHost) TemperatureAt(int, int) int                           { return f.temp }
func (f *fakeHost) Pos(entity.EntityID) (int, int)                       { return 0, 0 }
func (f *fakeHost) Free(entity.EntityID, entity.EntityID, int, int) bool { return true }
func (f *fakeHost) Start(entity.EntityID, entity.EntityID, int, int, int) bool {
	f.started++
	return true
}
func (f *fakeHost) Work(entity.EntityID) bool {
	f.worked++
	return true
}
func (f *fakeHost) Speed(entity.EntityID) Pawn { return f.pawn }
func (f *fakeHost) Researcher() bool           { return f.researcher }
func (f *fakeHost) Tech() proj.TechLevel       { return f.tech }
func (f *fakeHost) CostPct() int               { return f.costPct }
func (f *fakeHost) Difficulty() int            { return f.diff }
func (f *fakeHost) Now() uint64                { return f.now }
func (f *fakeHost) Letter(title, _ string, _, _ int) {
	f.letters = append(f.letters, title)
}
func (f *fakeHost) Changed() {}

// TestSpeedWikiNumbers (wiki: Research speed, Research): a researcher at
// Intellectual 8 with healthy hands and eyes has research speed 1.0 and
// makes 20.625 points an hour at a hi-tech bench at difficulty x1, 0.75 of
// that at a simple one; 0 gives 8 %, 20 gives 238 %; the floor is 10 %.
func TestSpeedWikiNumbers(t *testing.T) {
	t.Parallel()
	p := Pawn{Intellectual: 8, GlobalWork: 1000, Manipulation: 1000, Sight: 1000, TraitPct: 100}
	if s := Speed(p); s != 1000 {
		t.Fatalf("speed at 8 = %d permille", s)
	}
	if h := PointsNano(Speed(p), hiTechFactor, 100) * clock.TicksPerHour; h != 20_625_000_000 {
		t.Fatalf("hi-tech: %d billionths an hour, want 20.625 points", h)
	}
	if h := PointsNano(Speed(p), simpleFactor, 100) * clock.TicksPerHour; h != 15_468_750_000 {
		t.Fatalf("simple: %d billionths an hour, want 0.75 x 20.625", h)
	}
	for lvl, want := range map[uint8]int{0: minSpeed, 20: 2380} {
		p.Intellectual = lvl
		if s := Speed(p); s != want {
			t.Errorf("speed at %d = %d, want %d", lvl, s, want)
		}
	}
	p.Intellectual = 1 // 19.5 %
	if s := Speed(p); s != 195 {
		t.Errorf("speed at 1 = %d", s)
	}
	p.Intellectual, p.Manipulation, p.Sight = 8, 500, 1500 // half hands: 75 %; sight capped at 110 %: 105 %
	if s := Speed(p); s != 787 {
		t.Errorf("speed with half hands, keen eyes = %d, want 787", s)
	}
	for d, want := range map[difficulty.Preset]int64{difficulty.Wanderer: 9_900_000, difficulty.Gentle: 9_900_000,
		difficulty.Settled: 8_250_000, difficulty.Seasoned: 8_250_000, difficulty.Grim: 7_837_500, difficulty.Merciless: 7_425_000} {
		if got := PointsNano(1000, 1000, difficulty.Of(d).ResearchSpeed); got != want {
			t.Errorf("%s: %d a tick, want %d (wiki difficulty factor)", d, got, want)
		}
	}
}

// TestBenchFactor (wiki: Research): simple 0.75, hi-tech 1.0, +0.10 with
// a powered linked analyzer (within the facility reach only); not a
// laboratory 0.8; cleanliness; outdoors.
func TestBenchFactor(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	simple := f.add(construction.ResearchBench, 0, 0)
	hi := f.add(construction.ElectricResearchBench, 5, 0)
	if l.Factor(simple) != 750 || l.Factor(hi) != 1000 || l.Analyzer(hi) != 0 {
		t.Fatalf("lab: simple %d hi %d", l.Factor(simple), l.Factor(hi))
	}
	far := f.add(construction.MultiAnalyzer, 5+7, 0)
	if l.Analyzer(hi) != 0 {
		t.Fatal("an analyzer 7 tiles off is out of reach")
	}
	near := f.add(construction.MultiAnalyzer, 5, 3)
	if l.Analyzer(hi) != near || l.Analyzer(simple) != 0 || l.Factor(hi) != 1100 || l.Factor(simple) != 750 {
		t.Fatalf("linked %d (far %d), factor %d", l.Analyzer(hi), far, l.Factor(hi))
	}
	f.off[near] = true
	if l.Factor(hi) != 1000 {
		t.Fatal("an unpowered analyzer adds nothing")
	}
	f.off[near] = false
	f.beds = 1 // a bedroom: not a laboratory
	if l.Factor(hi) != 880 {
		t.Fatalf("bedroom %d", l.Factor(hi))
	}
	f.beds, f.benches = 0, 3 // two research benches against three bill benches: a workshop
	if l.Factor(simple) != 600 {
		t.Fatalf("workshop %d", l.Factor(simple))
	}
	f.benches, f.clean = 0, -500
	if l.Factor(hi) != 825 {
		t.Fatalf("very dirty %d", l.Factor(hi))
	}
	f.clean = 60
	if l.Factor(hi) != 1199 {
		t.Fatalf("sterile %d", l.Factor(hi))
	}
	f.clean, f.outdoors = 0, true
	if l.Factor(simple) != 405 { // 0.75 x 0.75 x 0.9 x 0.8
		t.Fatalf("outdoors %d", l.Factor(simple))
	}
}

// TestBenchGating (wiki: Research): a hi-tech project is never worked at
// a simple bench, an analyzer project only at a hi-tech bench with a
// powered linked analyzer; an unpowered hi-tech bench blocks.
func TestBenchGating(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	l.Prog = proj.Complete()
	l.Prog[proj.RifledBarrels], l.Prog[proj.Fabrication] = 0, 0
	simple := f.add(construction.ResearchBench, 0, 0)
	l.Pick(proj.RifledBarrels)
	if l.Assign(9, 3) || l.Work(9, simple) != Blocked {
		t.Fatal("a hi-tech project at a simple bench")
	}
	views, _ := l.Lists(false)
	if views[proj.RifledBarrels].BenchMissing != MissingHiTech || views[proj.Traps].BenchMissing != "" {
		t.Fatalf("missing: %q", views[proj.RifledBarrels].BenchMissing)
	}
	hi := f.add(construction.ElectricResearchBench, 5, 0)
	if !l.Assign(9, 3) || l.Work(9, hi) != Continue {
		t.Fatal("a hi-tech project at a hi-tech bench")
	}
	f.off[hi] = true
	if l.Work(9, hi) != Blocked {
		t.Fatal("an unpowered hi-tech bench works nothing")
	}
	f.off[hi] = false
	l.Pick(proj.Fabrication)
	if l.CanWork(hi, f.bldgs[hi], proj.Fabrication) || l.Work(9, hi) != Blocked {
		t.Fatal("an analyzer project without an analyzer")
	}
	if views, _ := l.Lists(false); views[proj.Fabrication].BenchMissing != MissingAnalyzer {
		t.Fatalf("missing: %q", views[proj.Fabrication].BenchMissing)
	}
	f.add(construction.MultiAnalyzer, 6, 2)
	if !l.CanWork(hi, f.bldgs[hi], proj.Fabrication) || l.Work(9, hi) != Continue || l.CanWork(simple, f.bldgs[simple], proj.Fabrication) {
		t.Fatal("an analyzer project at a linked hi-tech bench only")
	}
}

// TestTechCostAndLetter (wiki: Research): a tribe earns Industrial
// projects at half the rate (2x cost) and Medieval at two thirds (1.5x);
// finishing posts the letter and drops the project; ETA reads the rate.
func TestTechCostAndLetter(t *testing.T) {
	t.Parallel()
	run := func(tech proj.TechLevel, p proj.Project, ticks int) int64 {
		f := newFake()
		f.tech = tech
		l := New(f)
		bench := f.add(construction.ElectricResearchBench, 0, 0)
		l.Pick(p)
		for i := 0; i < ticks; i++ {
			f.now++
			l.Work(1, bench)
		}
		return int64(l.Prog[p])*nano + int64(l.Carry)
	}
	ind, tribe := run(proj.Industrial, proj.Electricity, 2500), run(proj.Neolithic, proj.Electricity, 2500)
	if ind != 20_625_000_000 || tribe*2 != ind {
		t.Fatalf("an hour on Electricity: industrial %d, tribe %d (want half)", ind, tribe)
	}
	if med := run(proj.Neolithic, proj.Tailoring, 2500); med*3 != ind*2 {
		t.Fatalf("tribe on Tailoring %d, want two thirds of %d", med, ind)
	}
	f := newFake()
	l := New(f)
	bench := f.add(construction.ElectricResearchBench, 0, 0)
	l.Pick(proj.Bowcraft) // 400 points: under 20 hours
	f.now = 1
	l.Work(1, bench)
	f.now = 2
	if v, _ := l.Lists(false); v[proj.Bowcraft].ETAHours != 20 || v[proj.Bowcraft].CostNow != 400 {
		t.Fatalf("ETA %d hours, cost %d", v[proj.Bowcraft].ETAHours, v[proj.Bowcraft].CostNow)
	}
	for i := 0; i < 400*clock.TicksPerHour && l.Set; i++ {
		f.now++
		l.Work(1, bench)
	}
	if !l.Prog.Done(proj.Bowcraft) || l.Set || len(f.letters) != 1 || f.letters[0] != "Research complete: Bowcraft" {
		t.Fatalf("done %v set %v letters %v", l.Prog.Done(proj.Bowcraft), l.Set, f.letters)
	}
	if l.Work(1, bench) != Stop || l.Assign(1, 3) {
		t.Fatal("no queue: research stops when a project finishes")
	}
}

// TestTemperatureNoFlap: the bad-temperature factor turns on below 10 C
// and off only at 12 C, so a bench wobbling around 10 C does not flip.
func TestTemperatureNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	bench := f.add(construction.ElectricResearchBench, 0, 0)
	l.Pick(proj.Traps)
	flips, was := 0, false
	for i, temp := range []int{150, 99, 101, 99, 105, 110, 119, 101, 99, 120, 115, 101, 351, 340, 331, 330} {
		f.temp = temp
		f.now = uint64(i)
		l.Work(1, bench)
		if bad := l.Factor(bench) == 700; bad != was {
			flips, was = flips+1, bad
		}
	}
	// on at 9.9 C, off at 12 C, on at 35.1 C, off at 33 C
	if flips != 4 || was {
		t.Fatalf("%d flips (last bad %v), want 4", flips, was)
	}
	cold := []entity.EntityID{bench}
	l.SetCold(cold)
	if got := l.ColdList(); len(got) != 1 || got[0] != bench {
		t.Fatalf("cold round trip %v", got)
	}
}

// TestAlertAndDevFinish: the alert stands while nothing is picked, a
// bench stands and someone can research; the dev finish ends the project.
func TestAlertAndDevFinish(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	if l.Alert() {
		t.Fatal("no bench, no alert")
	}
	f.add(construction.ResearchBench, 0, 0)
	if !l.Alert() {
		t.Fatal("a bench and a researcher, nothing picked")
	}
	f.researcher = false
	if l.Alert() {
		t.Fatal("nobody can research")
	}
	f.researcher = true
	l.Pick(proj.Traps)
	if l.Alert() {
		t.Fatal("a project is picked")
	}
	l.Carry = 7
	l.DevFinish()
	if !l.Prog.Done(proj.Traps) || l.Set || l.Carry != 0 || !l.Alert() {
		t.Fatal("dev finish ends the project; the alert is back")
	}
	if len(f.letters) != 1 || f.letters[0] != "Research complete: Traps" {
		t.Fatalf("dev finish posts the completion letter: %v", f.letters)
	}
}

// TestMigrate: saved progress keeps its fraction of the cost, finished
// projects stay finished, a picked project no longer open is let go; the
// presets RSCH-1 moved read the preset again.
func TestMigrate(t *testing.T) {
	t.Parallel()
	l := New(newFake())
	l.Prog[proj.Electricity] = 1500 // half of the old 3000
	l.Prog[proj.Traps] = 2500       // done at the old cost
	l.Prog[proj.Gunsmithing] = 1    // a sliver stays a sliver
	l.Cur, l.Set, l.Carry = proj.Gunsmithing, true, 99
	l.Migrate()
	if l.Prog[proj.Electricity] != 800 || !l.Prog.Done(proj.Traps) || l.Prog[proj.Gunsmithing] != 1 || l.Carry != 0 || l.Set {
		t.Fatalf("migrated %v set %v carry %d", l.Prog, l.Set, l.Carry)
	}
	old := difficulty.Of(difficulty.Settled)
	old.ResearchSpeed = 105
	if MigrateDifficulty(old) != difficulty.Of(difficulty.Settled) {
		t.Fatal("old Settled reads Settled")
	}
	custom := old
	custom.ThreatScale++
	if MigrateDifficulty(custom) != custom {
		t.Fatal("custom settings stay")
	}
	if len(legacyCost) != 45 {
		t.Fatalf("%d legacy costs", len(legacyCost))
	}
}

// TestReadingBonusAndStudy (AX-7): the room's reading bonus multiplies
// the bench factor; a schematic's points finish a project with the
// letter and keep another pick.
func TestReadingBonusAndStudy(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	hi := f.add(construction.ElectricResearchBench, 5, 0)
	f.reading = 60 // ten shelved books
	if got := l.Factor(hi); got != 1060 {
		t.Fatalf("reading bonus factor %d, want 1060", got)
	}
	l.Pick(proj.Herbalism)
	p := proj.Tailoring
	cost := proj.DefOf(p).Cost
	l.Study(p, cost-1, 0, 0)
	if l.Prog.Done(p) || len(f.letters) != 0 {
		t.Fatal("one point short is not done")
	}
	l.Study(p, 5, 0, 0)
	if !l.Prog.Done(p) || len(f.letters) != 1 || l.Cur != proj.Herbalism || !l.Set {
		t.Fatalf("done %v letters %v cur %v set %v", l.Prog.Done(p), f.letters, l.Cur, l.Set)
	}
	if l.CostOf(p) < 1 {
		t.Fatal("cost percent")
	}
}
