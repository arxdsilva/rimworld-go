package mind

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/entity"
)

type brk struct {
	k     mentalbreak.Kind
	until uint64
}

type pair struct{ a, b entity.EntityID }

// fakeHost is a small world for mind's unit tests: pawns on a line,
// scripted rolls, and a record of every act.
type fakeHost struct {
	now        uint64
	x          map[entity.EntityID]int
	dead, down map[entity.EntityID]bool
	colonists  []entity.EntityID
	prisoners  []entity.EntityID
	mood       map[entity.EntityID]int
	shift      map[entity.EntityID]int
	breaks     map[entity.EntityID]brk
	rolls      []uint32
	ri         int
	jobs       map[entity.EntityID]job.Record
	outcomes   []pair
	fightNews  int

	peaceful          map[entity.EntityID]bool
	canBinge, canHide bool
	nearest           func() entity.EntityID
	unreachable       map[entity.EntityID]bool
	seized, wandered  int
	binged, hid       int
	downs, ups        int
	approached        []pair
	insults           []pair
	assaults          []pair
	started           []brk
	targets           []entity.EntityID
	recovered         []mentalbreak.Kind
	punches           []pair
	fakeMuses         // inspiration_test.go (PRD-2)
	fakeSmash         // tantrum_test.go (CBT-4)
}

func newFake(ids ...entity.EntityID) *fakeHost {
	f := &fakeHost{
		x: map[entity.EntityID]int{}, dead: map[entity.EntityID]bool{}, down: map[entity.EntityID]bool{},
		mood: map[entity.EntityID]int{}, shift: map[entity.EntityID]int{}, breaks: map[entity.EntityID]brk{},
		peaceful: map[entity.EntityID]bool{}, jobs: map[entity.EntityID]job.Record{}, unreachable: map[entity.EntityID]bool{},
	}
	for i, id := range ids {
		f.colonists = append(f.colonists, id)
		f.x[id] = i * 10
		f.mood[id] = 50
	}
	return f
}

func (f *fakeHost) Now() uint64 { return f.now }
func (f *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	x, ok := f.x[id]
	return x, 0, ok
}
func (f *fakeHost) Alive(id entity.EntityID) bool {
	_, ok := f.x[id]
	return ok && !f.dead[id]
}
func (f *fakeHost) Standing(id entity.EntityID) bool  { return f.Alive(id) && !f.down[id] }
func (f *fakeHost) Colonists() []entity.EntityID      { return f.colonists }
func (f *fakeHost) Prisoners() []entity.EntityID      { return f.prisoners }
func (f *fakeHost) Mood(id entity.EntityID) int       { return f.mood[id] }
func (f *fakeHost) BreakShift(id entity.EntityID) int { return f.shift[id] }

func (f *fakeHost) Break(id entity.EntityID) (mentalbreak.Kind, uint64, bool) {
	b, ok := f.breaks[id]
	return b.k, b.until, ok
}
func (f *fakeHost) SetBreak(id entity.EntityID, k mentalbreak.Kind, until uint64) {
	f.breaks[id] = brk{k, until}
}
func (f *fakeHost) ClearBreak(id entity.EntityID) { delete(f.breaks, id) }
func (f *fakeHost) Roll(n int) int {
	if len(f.rolls) == 0 {
		return 0
	}
	r := f.rolls[f.ri%len(f.rolls)]
	f.ri++
	return int(r % uint32(n))
}

func (f *fakeHost) Seize(entity.EntityID)               { f.seized++ }
func (f *fakeHost) Wander(entity.EntityID, int) bool    { f.wandered++; return true }
func (f *fakeHost) CanBinge(entity.EntityID) bool       { return f.canBinge }
func (f *fakeHost) Binge(entity.EntityID) bool          { f.binged++; return f.canBinge }
func (f *fakeHost) CanHide(entity.EntityID) bool        { return f.canHide }
func (f *fakeHost) Hide(entity.EntityID, int)           { f.hid++ }
func (f *fakeHost) Down(id entity.EntityID)             { f.downs++; f.down[id] = true }
func (f *fakeHost) StandUp(id entity.EntityID)          { f.ups++; f.down[id] = false }
func (f *fakeHost) CanViolence(id entity.EntityID) bool { return !f.peaceful[id] }
func (f *fakeHost) Attackable(id entity.EntityID) bool  { return f.Standing(id) }
func (f *fakeHost) NearestCreature(entity.EntityID, int) entity.EntityID {
	if f.nearest == nil {
		return 0
	}
	return f.nearest()
}
func (f *fakeHost) Approach(id, t entity.EntityID) bool {
	f.approached = append(f.approached, pair{id, t})
	if f.unreachable[t] {
		return false
	}
	f.x[id] = f.x[t] + 1 // arrives next to it at once
	return true
}
func (f *fakeHost) Insult(a, b entity.EntityID) { f.insults = append(f.insults, pair{a, b}) }
func (f *fakeHost) Assault(id, t entity.EntityID) bool {
	f.assaults = append(f.assaults, pair{id, t})
	return !f.unreachable[t]
}
func (f *fakeHost) Started(_ entity.EntityID, k mentalbreak.Kind, t entity.EntityID) {
	f.started = append(f.started, brk{k: k})
	f.targets = append(f.targets, t)
}
func (f *fakeHost) Recovered(_ entity.EntityID, k mentalbreak.Kind) {
	f.recovered = append(f.recovered, k)
}

func (f *fakeHost) Job(id entity.EntityID) job.Record { return f.jobs[id] }
func (f *fakeHost) Brawl(id, foe entity.EntityID) {
	f.jobs[id] = job.Record{Kind: job.Brawl, State: job.Goto, Target: foe}
}
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record) { f.jobs[id] = j }

// PathTo fails to the tile of an unreachable pawn; any other path is the
// one step onto (x, y).
func (f *fakeHost) PathTo(_ entity.EntityID, x, y int) ([]pathing.Point, bool) {
	for id, px := range f.x {
		if px == x && f.unreachable[id] {
			return nil, false
		}
	}
	return []pathing.Point{{X: x, Y: y}}, true
}
func (f *fakeHost) Fighter(id entity.EntityID) bool { return f.Standing(id) }
func (f *fakeHost) Punch(id, foe entity.EntityID)   { f.punches = append(f.punches, pair{id, foe}) }
func (f *fakeHost) PunchTicks(entity.EntityID) int  { return 10 }
func (f *fakeHost) StopBrawl(id entity.EntityID) {
	if f.jobs[id].Kind == job.Brawl {
		delete(f.jobs, id)
	}
}
func (f *fakeHost) FightStarted(_, _ entity.EntityID) { f.fightNews++ }
func (f *fakeHost) Outcome(a, b entity.EntityID)      { f.outcomes = append(f.outcomes, pair{a, b}) }

var _ Host = (*fakeHost)(nil)
