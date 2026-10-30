package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a small world for prison's unit tests: pawns on a grid,
// scripted rolls, and a record of what the prison asked for.
type fakeHost struct {
	now     uint64
	pos     map[entity.EntityID][2]int
	vital   map[entity.EntityID]health.Record
	jobs    map[entity.EntityID]job.Record
	needs   map[entity.EntityID]needs.Pair
	held    map[entity.EntityID]Held
	raiders []entity.EntityID
	owner   map[entity.EntityID]entity.EntityID
	rolls   []int
	ri      int

	feedOK, bedBack bool
	opinion         int32
	mood            int
	negotiation     int32
	social          int
	bed             entity.EntityID

	started             []job.Record
	recruited, executed []entity.EntityID
	chats, chases       int
	downs, dies         int
	messages            []string
	fakeEscapes         // escape_test.go (CBT-4)
	failStart           bool
}

func newFake() *fakeHost {
	return &fakeHost{
		pos: map[entity.EntityID][2]int{}, vital: map[entity.EntityID]health.Record{},
		jobs: map[entity.EntityID]job.Record{}, needs: map[entity.EntityID]needs.Pair{},
		held: map[entity.EntityID]Held{}, owner: map[entity.EntityID]entity.EntityID{},
		mood: 50, negotiation: 1000, social: 5,
	}
}

// put places a standing, fed pawn id at (x, y).
func (f *fakeHost) put(id entity.EntityID, x, y int) {
	f.pos[id] = [2]int{x, y}
	f.vital[id] = health.Record{Status: health.Alive}
	f.needs[id] = needs.NewFull()
}

func (f *fakeHost) Now() uint64 { return f.now }
func (f *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := f.pos[id]
	return p[0], p[1], ok
}
func (f *fakeHost) Vital(id entity.EntityID) (health.Record, bool) {
	v, ok := f.vital[id]
	return v, ok
}
func (f *fakeHost) SetVital(id entity.EntityID, v health.Record) { f.vital[id] = v }
func (f *fakeHost) Job(id entity.EntityID) (job.Record, bool)    { j, ok := f.jobs[id]; return j, ok }
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record)      { f.jobs[id] = j }

func (f *fakeHost) RaiderIDs() []entity.EntityID       { return f.raiders }
func (f *fakeHost) Held(id entity.EntityID) Held       { return f.held[id] }
func (f *fakeHost) SetHeld(id entity.EntityID, r Held) { f.held[id] = r }
func (f *fakeHost) Captured(entity.EntityID) bool      { return false }
func (f *fakeHost) Owner(t entity.EntityID) (entity.EntityID, bool) {
	o, ok := f.owner[t]
	return o, ok
}
func (f *fakeHost) ReleaseOwner(o entity.EntityID) {
	for t, w := range f.owner {
		if w == o {
			delete(f.owner, t)
		}
	}
}
func (f *fakeHost) Roll(n int) int {
	if f.ri >= len(f.rolls) {
		return 0
	}
	f.ri++
	return f.rolls[f.ri-1] % n
}

func (f *fakeHost) Needs(id entity.EntityID) needs.Pair       { return f.needs[id] }
func (f *fakeHost) SetNeeds(id entity.EntityID, n needs.Pair) { f.needs[id] = n }
func (f *fakeHost) Heal(_ entity.EntityID, v health.Record, _ uint64) health.Record {
	return v
}
func (f *fakeHost) Decay(_ entity.EntityID, n needs.Pair, _ uint64) needs.Pair {
	n.Hunger--
	return n
}
func (f *fakeHost) DownedHunger(id entity.EntityID, v health.Record, _ uint64) (needs.Pair, health.Record) {
	return f.needs[id], v
}
func (f *fakeHost) Down(id entity.EntityID, c health.Cause) {
	f.downs++
	f.vital[id] = health.Record{Status: health.Downed, Cause: c}
}
func (f *fakeHost) Die(id entity.EntityID, c health.Cause) {
	f.dies++
	f.vital[id] = health.Record{Status: health.Dead, Cause: c}
}

func (f *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	if f.failStart {
		return false
	}
	f.started = append(f.started, rec)
	f.jobs[id] = rec
	return true
}
func (f *fakeHost) Interrupt(id entity.EntityID, _ job.FailReason) { delete(f.jobs, id) }
func (f *fakeHost) BesideSpot(_ entity.EntityID, x, y int) (int, int, bool) {
	return x + 1, y, true
}
func (f *fakeHost) ReleaseBed(entity.EntityID) {}
func (f *fakeHost) SleepSpot(entity.EntityID) (entity.EntityID, int, int, bool) {
	return f.bed, 7, 7, f.bed != 0
}
func (f *fakeHost) BedAt(entity.EntityID) (int, int, int, bool) { return 7, 7, 1, f.bed != 0 }
func (f *fakeHost) FreeInRoom(_, _, room int) bool              { return room == 1 }

func (f *fakeHost) BedBack(entity.EntityID) bool                    { return f.bedBack }
func (f *fakeHost) FeedRun(_, _ entity.EntityID) bool               { return f.feedOK }
func (f *fakeHost) Chase(_, _ entity.EntityID, _ job.Record, _ int) { f.chases++ }
func (f *fakeHost) SocialWork(entity.EntityID) int                  { return 100 }
func (f *fakeHost) ConsciousEnough(entity.EntityID) bool            { return true }

func (f *fakeHost) Meet(_, _ entity.EntityID)           {}
func (f *fakeHost) Opinion(_, _ entity.EntityID) int32  { return f.opinion }
func (f *fakeHost) Mood(entity.EntityID) int            { return f.mood }
func (f *fakeHost) Negotiation(entity.EntityID) int32   { return f.negotiation }
func (f *fakeHost) SocialLevel(entity.EntityID) int     { return f.social }
func (f *fakeHost) Chat(_, _ entity.EntityID, _ uint64) { f.chats++ }
func (f *fakeHost) Recruit(p, _ entity.EntityID)        { f.recruited = append(f.recruited, p) }
func (f *fakeHost) Execute(p, _ entity.EntityID)        { f.executed = append(f.executed, p) }
func (f *fakeHost) PersonName(entity.EntityID) string   { return "Rid" }
func (f *fakeHost) Message(_, _ int, text string)       { f.messages = append(f.messages, text) }

var _ Host = (*fakeHost)(nil)
