package ship

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fake is a map of buildings, a crew and a clock.
type fake struct {
	ver     uint64
	ids     []entity.EntityID
	b       map[entity.EntityID]construction.Building
	now     uint64
	raids   []bool // mech per raid
	letters []string
	msgs    []string
	aboard  map[entity.EntityID]entity.EntityID // pawn -> casket
	jobs    map[entity.EntityID]job.Record
}

func newFake() *fake {
	return &fake{b: map[entity.EntityID]construction.Building{}, aboard: map[entity.EntityID]entity.EntityID{}, jobs: map[entity.EntityID]job.Record{}}
}

func (f *fake) add(k construction.Kind, x, y int) entity.EntityID {
	id := entity.EntityID(len(f.ids) + 1)
	f.ids = append(f.ids, id)
	f.b[id] = construction.Building{Kind: k, X: x, Y: y, Stuff: 0}
	f.ver++
	return id
}

func (f *fake) StructVer() uint64              { return f.ver }
func (f *fake) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fake) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.b[id]
	return b, ok
}
func (f *fake) Lift(id entity.EntityID) {
	delete(f.b, id)
	f.ids = slices.DeleteFunc(f.ids, func(v entity.EntityID) bool { return v == id })
	f.ver++
}
func (f *fake) Ready(id entity.EntityID) bool {
	_, away := f.aboard[id]
	return id >= 100 && !away
}
func (f *fake) Name(id entity.EntityID) string                   { return "P" + string(rune('A'+id-100)) }
func (f *fake) Spot(entity.EntityID, int, int) (int, int, bool)  { return 0, 0, true }
func (f *fake) StartJob(id entity.EntityID, rec job.Record) bool { f.jobs[id] = rec; return true }
func (f *fake) SetJob(id entity.EntityID, j job.Record)          { f.jobs[id] = j }
func (f *fake) Board(id, casket entity.EntityID) bool            { f.aboard[id] = casket; return true }
func (f *fake) Wake(id, _ entity.EntityID) bool                  { delete(f.aboard, id); return true }
func (f *fake) Escaped(entity.EntityID)                          {}
func (f *fake) Now() uint64                                      { return f.now }
func (f *fake) Seed() uint64                                     { return 7 }
func (f *fake) Raid(mech bool)                                   { f.raids = append(f.raids, mech) }
func (f *fake) Letter(title string, _, _ int, _ bool, _ string, _ ...any) {
	f.letters = append(f.letters, title)
}
func (f *fake) Message(_, _ int, format string, args ...any) {
	f.msgs = append(f.msgs, fmt.Sprintf(format, args...))
}
func (f *fake) Changed() {}

// minimal lays the wiki's minimum ship (beam, casket, core, reactor,
// sensor, three engines), all joined, and returns the casket and reactor.
func minimal(f *fake) (casket, reactor entity.EntityID) {
	f.add(construction.ShipBeam, 10, 2) // 2x6: x10-11 y2-7
	reactor = f.add(construction.ShipReactor, 12, 2)
	f.add(construction.ShipCore, 8, 2)
	f.add(construction.ShipSensor, 8, 4)
	casket = f.add(construction.ShipCasket, 9, 6) // touches the beam only
	f.add(construction.ShipEngine, 10, 9)
	f.add(construction.ShipEngine, 13, 9)
	f.add(construction.ShipEngine, 16, 9)
	return casket, reactor
}

// board puts pawn into casket through the command and its job.
func board(t *testing.T, s *State, f *fake, pawn, casket entity.EntityID) {
	t.Helper()
	s.Apply(f, command.Command{Kind: command.BoardShip, Pawn: uint64(pawn), Target: uint64(casket)})
	j, ok := f.jobs[pawn]
	if !ok || j.Kind != job.BoardShip {
		t.Fatalf("no board job: %+v msgs %v", j, f.msgs)
	}
	s.Do(f, pawn, j)
	if s.Sleeper(casket) != pawn {
		t.Fatalf("sleeper %d, want %d", s.Sleeper(casket), pawn)
	}
}

func TestPartsJoinIntoOneShipCasketsOnlyThroughBeams(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	minimal(f)
	if ships := s.Ships(f); len(ships) != 1 || len(ships[0].Parts) != 8 || ships[0].Engines != 3 || ships[0].ID != 1 {
		t.Fatalf("ships: %+v", ships)
	}
	// a casket beside the core but no beam stands alone
	lone := f.add(construction.ShipCasket, 7, 2)
	sh, _ := s.ShipOf(f, lone)
	if len(sh.Parts) != 1 || sh.ID != uint64(lone) {
		t.Fatalf("lone casket ship %+v", sh)
	}
	if p, _ := s.Primary(f); p.ID != 1 {
		t.Fatalf("primary %d", p.ID)
	}
}

func TestLaunchReportListsWhatIsMissing(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	reactor := f.add(construction.ShipReactor, 12, 2)
	sh, _ := s.ShipOf(f, reactor)
	got := strings.Join(s.Issues(sh, 0), "|")
	for _, want := range []string{"Needs 3 more engines", "Needs a ship computer core", "Needs a ship sensor cluster",
		"Needs a ship structural beam", "Needs a ship stasis casket joined to a beam", "The reactor has not been started"} {
		if !strings.Contains(got, want) {
			t.Errorf("report %q lacks %q", got, want)
		}
	}
	f2 := newFake()
	var s2 State
	casket, r2 := minimal(f2)
	s2.Start(f2, r2)
	sh2, _ := s2.ShipOf(f2, r2)
	got2 := s2.Issues(sh2, f2.now)
	if len(got2) != 2 || !strings.HasPrefix(got2[0], "The reactor is still waking: 15 days") || got2[1] != "Nobody sleeps in a ship casket" {
		t.Fatalf("report %q", got2)
	}
	board(t, &s2, f2, 100, casket)
	if got := s2.Issues(sh2, CountdownTicks); len(got) != 0 {
		t.Fatalf("ready ship still reports %q", got)
	}
}

func TestReactorCountdownRaidsAreSpacedAndStop(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	_, reactor := minimal(f)
	if why := s.Start(f, reactor); why != "" {
		t.Fatal(why)
	}
	if why := s.Start(f, reactor); why != "already started" {
		t.Fatalf("second start: %q", why)
	}
	var at []uint64
	for f.now = 0; f.now <= CountdownTicks+5*RaidGap; f.now += 250 {
		n := len(f.raids)
		s.Tick(f)
		if len(f.raids) > n {
			at = append(at, f.now)
		}
	}
	if len(at) < 9 || len(at) > 15 {
		t.Fatalf("%d countdown raids over 15 days: %v", len(at), at)
	}
	// no flap: never two raids closer than the gap, none after the countdown
	for i := 1; i < len(at); i++ {
		if at[i]-at[i-1] < RaidGap {
			t.Fatalf("raids %d and %d only %d ticks apart", i-1, i, at[i]-at[i-1])
		}
	}
	if at[len(at)-1] >= CountdownTicks || at[0] < FirstRaid {
		t.Fatalf("raids outside the countdown: %v", at)
	}
	if !f.raids[1] || f.raids[0] {
		t.Fatalf("people then machines by turns: %v", f.raids)
	}
	if !slices.Contains(f.letters, "Ship reactor ready") || s.NextRaid() != 0 {
		t.Fatalf("letters %v next raid %d", f.letters, s.NextRaid())
	}
}

func TestDestroyedReactorLosesItsWakingAndStopsRaids(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	_, reactor := minimal(f)
	s.Start(f, reactor)
	f.Lift(reactor)
	f.now = FirstRaid + 10
	s.Tick(f)
	if len(f.raids) != 0 || s.Busy() || len(s.reactors) != 0 {
		t.Fatalf("raids %v busy %v", f.raids, s.Busy())
	}
}

func TestLaunchLiftsTheShipWithItsSleepers(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	casket, reactor := minimal(f)
	if issues := s.Launch(f, reactor); len(issues) == 0 || s.Launched() {
		t.Fatal("an unready ship launched")
	}
	s.Start(f, reactor)
	board(t, &s, f, 100, casket)
	f.now = CountdownTicks
	s.Tick(f)
	if issues := s.Launch(f, casket); issues != nil {
		t.Fatalf("launch refused: %v", issues)
	}
	if !s.Launched() || len(f.ids) != 0 || !slices.Equal(s.Escaped(), []string{"PA"}) {
		t.Fatalf("launched %v left %v escaped %v", s.Launched(), f.ids, s.Escaped())
	}
	var w axview.World
	s.FillWorld(f, &w)
	if !w.ShipLaunched || w.Ending != EndingShip || len(w.Escaped) != 1 || w.ShipID != 0 {
		t.Fatalf("world view %+v", w)
	}
	if why := s.Start(f, reactor); why == "" {
		t.Fatal("started a reactor after the launch")
	}
}

func TestSleeperWakesAndFallenCasketFreesIt(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	casket, _ := minimal(f)
	board(t, &s, f, 100, casket)
	s.Apply(f, command.Command{Kind: command.BoardShip, Pawn: 101, Target: uint64(casket)})
	if len(f.msgs) == 0 || !strings.Contains(f.msgs[len(f.msgs)-1], "someone already sleeps in it") {
		t.Fatalf("second boarder: %v", f.msgs)
	}
	s.Apply(f, command.Command{Kind: command.LeaveShip, Target: uint64(casket)})
	if s.Sleeper(casket) != 0 || len(f.aboard) != 0 {
		t.Fatal("the sleeper did not wake")
	}
	board(t, &s, f, 101, casket)
	f.Lift(casket)
	s.Tick(f)
	if len(s.sleepers) != 0 || len(f.aboard) != 0 {
		t.Fatal("a fallen casket kept its sleeper")
	}
}

func TestBuildingViewAndSave(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	casket, reactor := minimal(f)
	var v axview.Building
	s.FillBuilding(f, &v, reactor, f.b[reactor])
	if v.Countdown != -1 || v.ShipID != 1 {
		t.Fatalf("idle reactor view %+v", v)
	}
	s.Start(f, reactor)
	board(t, &s, f, 100, casket)
	f.now = 100
	v = axview.Building{}
	s.FillBuilding(f, &v, reactor, f.b[reactor])
	if v.Countdown != CountdownTicks || v.Countdown%2500 != 0 {
		t.Fatalf("countdown view %d (rounded up to the hour)", v.Countdown)
	}
	v = axview.Building{}
	s.FillBuilding(f, &v, casket, f.b[casket])
	if v.Sleeper != "PA" || v.SleeperID != 100 {
		t.Fatalf("casket view %+v", v)
	}
	var back State
	back.Import(s.Export())
	if back.Sleeper(casket) != 100 || back.reactor(reactor) == nil || back.NextRaid() != s.NextRaid() {
		t.Fatalf("round trip %+v", back.Export())
	}
	var empty State
	if empty.Export() != nil {
		t.Fatal("a fresh state saves something")
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()
	for ticks, want := range map[int]string{CountdownTicks: "15 days", 60000 + 2500: "1 day 1 hour", 100: "1 hour", 2 * 2500: "2 hours"} {
		if got := Duration(ticks); got != want {
			t.Errorf("Duration(%d) = %q, want %q", ticks, got, want)
		}
	}
}

func TestRewindSkipsTheCountdownAndBringsTheRaid(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	_, reactor := minimal(f)
	f.now = 1000
	s.Start(f, reactor)
	s.Rewind(FirstRaid)
	s.Tick(f)
	if len(f.raids) != 1 {
		t.Fatalf("raids %v after skipping to the first", f.raids)
	}
	s.Rewind(CountdownTicks)
	s.Tick(f)
	sh, _ := s.ShipOf(f, reactor)
	if s.left(sh, f.now) != 0 || !slices.Contains(f.letters, "Ship reactor ready") {
		t.Fatalf("left %d letters %v", s.left(sh, f.now), f.letters)
	}
}

func TestLayACarriedColonistInAShipCasket(t *testing.T) {
	t.Parallel()
	f := newFake()
	var s State
	casket, reactor := minimal(f)
	if !s.CasketFree(f, casket) || s.CasketFree(f, reactor) {
		t.Fatal("free casket check")
	}
	if !s.LayIn(f, 100, casket) || s.Sleeper(casket) != 100 || s.CasketFree(f, casket) || s.LayIn(f, 101, casket) {
		t.Fatalf("lay: sleeper %d", s.Sleeper(casket))
	}
}
