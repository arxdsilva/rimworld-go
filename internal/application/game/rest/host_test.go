package rest

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// fakeHost is a small in-memory simulation for rest's unit tests.
type fakeHost struct {
	pos      map[entity.EntityID][2]int
	vital    map[entity.EntityID]health.Record
	need     map[entity.EntityID]needs.Pair
	jobs     map[entity.EntityID]job.Record
	noStart  map[entity.EntityID]bool // StartJob fails (out of reach)
	floor    [2]int                   // FloorSpot answer when set
	hasFloor bool

	bldgIDs []entity.EntityID
	bldgs   map[entity.EntityID]construction.Building
	slept   map[entity.EntityID]entity.EntityID

	outside   map[[2]int]bool // AreaAllows false
	colonists map[entity.EntityID]bool
	partner   map[entity.EntityID]entity.EntityID

	act        schedule.Activity
	food       bool
	bedRest    map[entity.EntityID]bool
	medicalOK  map[entity.EntityID]bool
	surgeryRan []int

	room    int
	sky     bool
	role    room.Role // RoomQuality answer
	tenths  int
	thought []mood.Kind
	values  []int    // RememberValue values, by thought
	labels  []string // RememberValue labels
	forgot  []mood.Kind
	logged  []bool

	tame  map[entity.EntityID]bool
	rooms map[[2]int]int // RoomAt overrides by cell

	mate    map[entity.EntityID]entity.EntityID
	opinion map[[2]entity.EntityID]int32
	age     map[entity.EntityID]int
	lovinAt map[entity.EntityID]uint64
	roll    int // SocialRoll answer
	rolls   int // SocialRoll calls
	withOf  map[entity.EntityID]entity.EntityID
	down    map[entity.EntityID]bool  // not Standing
	pain    map[entity.EntityID]int32 // Hurt pain (with a body)
	noBody  bool
}

func newFake() *fakeHost {
	return &fakeHost{
		pos: map[entity.EntityID][2]int{}, vital: map[entity.EntityID]health.Record{},
		need: map[entity.EntityID]needs.Pair{}, jobs: map[entity.EntityID]job.Record{},
		noStart: map[entity.EntityID]bool{}, bldgs: map[entity.EntityID]construction.Building{},
		slept: map[entity.EntityID]entity.EntityID{}, outside: map[[2]int]bool{},
		colonists: map[entity.EntityID]bool{}, partner: map[entity.EntityID]entity.EntityID{},
		bedRest: map[entity.EntityID]bool{}, medicalOK: map[entity.EntityID]bool{},
		act: schedule.Anything, room: -1, tame: map[entity.EntityID]bool{}, rooms: map[[2]int]int{},
		mate: map[entity.EntityID]entity.EntityID{}, opinion: map[[2]entity.EntityID]int32{}, age: map[entity.EntityID]int{},
		lovinAt: map[entity.EntityID]uint64{}, withOf: map[entity.EntityID]entity.EntityID{}, down: map[entity.EntityID]bool{},
		pain: map[entity.EntityID]int32{},
	}
}

// pawn adds a living colonist at (x, y).
func (f *fakeHost) pawn(id entity.EntityID, x, y int) {
	f.pos[id] = [2]int{x, y}
	f.vital[id] = health.Record{Status: health.Alive}
	f.need[id] = needs.Pair{Hunger: needs.Full, Rest: needs.Full}
	f.colonists[id] = true
}

// bed adds a bed of kind k at (x, y).
func (f *fakeHost) bed(id entity.EntityID, k construction.Kind, x, y int) {
	f.bldgIDs = append(f.bldgIDs, id)
	slices.Sort(f.bldgIDs)
	f.bldgs[id] = construction.Building{Kind: k, X: x, Y: y, Facing: construction.South}
}

func (f *fakeHost) Pos(id entity.EntityID) (int, int) { p := f.pos[id]; return p[0], p[1] }
func (f *fakeHost) Vital(id entity.EntityID) (health.Record, bool) {
	v, ok := f.vital[id]
	return v, ok
}
func (f *fakeHost) SetVital(id entity.EntityID, v health.Record) { f.vital[id] = v }
func (f *fakeHost) Needs(id entity.EntityID) needs.Pair          { return f.need[id] }
func (f *fakeHost) SetNeeds(id entity.EntityID, n needs.Pair)    { f.need[id] = n }
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record)      { f.jobs[id] = j }
func (f *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	if f.noStart[id] {
		return false
	}
	f.jobs[id] = rec
	return true
}
func (f *fakeHost) FloorSpot(_ entity.EntityID, x, y int) (int, int) {
	if f.hasFloor {
		return f.floor[0], f.floor[1]
	}
	return x, y
}

func (f *fakeHost) BuildingIDs() []entity.EntityID { return f.bldgIDs }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) SetBuilding(id entity.EntityID, b construction.Building) { f.bldgs[id] = b }
func (f *fakeHost) SleptAt(id entity.EntityID) (entity.EntityID, bool) {
	b, ok := f.slept[id]
	return b, ok
}
func (f *fakeHost) SetSleptAt(id, bed entity.EntityID) { f.slept[id] = bed }
func (f *fakeHost) ForgetSleptAt(id entity.EntityID)   { delete(f.slept, id) }
func (f *fakeHost) ReleaseBed(id entity.EntityID) {
	for _, bid := range f.bldgIDs {
		f.bldgs[bid] = f.bldgs[bid].Release(uint64(id))
	}
}
func (f *fakeHost) SleepCell(id entity.EntityID, b construction.Building) (int, int) {
	slot := b.SlotOf(uint64(id))
	if slot < 0 {
		slot = 0
	}
	return construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
}

func (f *fakeHost) AreaAllows(_ entity.EntityID, x, y int) bool { return !f.outside[[2]int{x, y}] }
func (f *fakeHost) MayOwnBed(id entity.EntityID) bool {
	v, ok := f.vital[id]
	return f.colonists[id] && ok && v.Status != health.Dead
}
func (f *fakeHost) Partner(id entity.EntityID) entity.EntityID { return f.partner[id] }

func (f *fakeHost) Activity(entity.EntityID) schedule.Activity { return f.act }
func (f *fakeHost) SeekFood(entity.EntityID) bool              { return f.food }

func (f *fakeHost) OnBedRest(id entity.EntityID) bool        { return f.bedRest[id] }
func (f *fakeHost) MayUseMedicalBed(id entity.EntityID) bool { return f.medicalOK[id] }
func (f *fakeHost) SurgeryWait(_ entity.EntityID, workLeft int) {
	f.surgeryRan = append(f.surgeryRan, workLeft)
}

func (f *fakeHost) RoomAt(x, y int) int {
	if r, ok := f.rooms[[2]int{x, y}]; ok {
		return r
	}
	return f.room
}
func (f *fakeHost) UnderSky(int, int) bool           { return f.sky }
func (f *fakeHost) RoomQuality(int) (room.Role, int) { return f.role, f.tenths }
func (f *fakeHost) RememberValue(_ entity.EntityID, k mood.Kind, v int, label string) {
	f.thought, f.values, f.labels = append(f.thought, k), append(f.values, v), append(f.labels, label)
}
func (f *fakeHost) Remember(_ entity.EntityID, k mood.Kind)    { f.thought = append(f.thought, k) }
func (f *fakeHost) ForgetGroup(_ entity.EntityID, k mood.Kind) { f.forgot = append(f.forgot, k) }
func (f *fakeHost) Slept(_ entity.EntityID, inBed bool)        { f.logged = append(f.logged, inBed) }

func (f *fakeHost) ObjectAt(x, y int) entity.EntityID {
	for _, id := range f.bldgIDs {
		b := f.bldgs[id]
		cells, n := construction.SmallFootprint(b.Kind, b.Facing, b.X, b.Y) // no allocation (TestStatsOfAllocatesNothing)
		if slices.Contains(cells[:n], [2]int{x, y}) {
			return id
		}
	}
	return 0
}
func (f *fakeHost) TameAnimal(id entity.EntityID) bool { return f.tame[id] }

func (f *fakeHost) Mate(id entity.EntityID) entity.EntityID  { return f.mate[id] }
func (f *fakeHost) Opinion(a, b entity.EntityID) int32       { return f.opinion[[2]entity.EntityID{a, b}] }
func (f *fakeHost) Age(id entity.EntityID) int               { return f.age[id] }
func (f *fakeHost) LovinAt(id entity.EntityID) uint64        { return f.lovinAt[id] }
func (f *fakeHost) SetLovinAt(id entity.EntityID, at uint64) { f.lovinAt[id] = at }
func (f *fakeHost) SocialRoll(int) int                       { f.rolls++; return f.roll }
func (f *fakeHost) RememberWith(id entity.EntityID, k mood.Kind, o entity.EntityID) {
	f.thought = append(f.thought, k)
	f.withOf[id] = o
}
func (f *fakeHost) Job(id entity.EntityID) job.Record { return f.jobs[id] }
func (f *fakeHost) Standing(id entity.EntityID) bool  { return !f.down[id] }
func (f *fakeHost) Hurt(id entity.EntityID) (int32, int32, bool) {
	if f.noBody {
		return 0, 0, false
	}
	return f.pain[id], 1000, true
}

var _ Host = (*fakeHost)(nil)
