package care

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a tiny in-memory simulation behind care.Host: pawns on an
// open plane (every tile reachable, paths are empty), buildings, a
// reservation map and a record of what care asked the host to do.
type fakeHost struct {
	now       uint64
	breaking  map[entity.EntityID]bool // Breaking (SIM-062)
	pos       map[entity.EntityID][2]int
	vital     map[entity.EntityID]health.Record
	jobs      map[entity.EntityID]job.Record
	bleed     map[entity.EntityID]int32
	pawns     []entity.EntityID
	raiders   []entity.EntityID
	prisoners map[entity.EntityID]bool
	animals   map[entity.EntityID]bool
	drafted   map[entity.EntityID]bool
	blocked   map[entity.EntityID]bool
	doctors   map[entity.EntityID]bool
	healing   map[entity.EntityID]int
	sickRest  map[entity.EntityID]bool
	sickStill map[entity.EntityID]bool
	surgery   map[entity.EntityID]bool
	hungry    map[entity.EntityID]bool
	noBeds    bool                     // SleepInBed fails
	tend      map[entity.EntityID]bool // needs tending
	bldgIDs   []entity.EntityID
	bldgs     map[entity.EntityID]construction.Building
	owners    map[entity.EntityID]entity.EntityID
	stacks    map[entity.EntityID]item.Stack
	sleptAt   map[entity.EntityID]entity.EntityID
	noPath    bool
	work      int
	log       []string
	tended    map[entity.EntityID]int32
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		pos: map[entity.EntityID][2]int{}, vital: map[entity.EntityID]health.Record{},
		jobs: map[entity.EntityID]job.Record{}, bleed: map[entity.EntityID]int32{},
		prisoners: map[entity.EntityID]bool{}, animals: map[entity.EntityID]bool{},
		drafted: map[entity.EntityID]bool{}, blocked: map[entity.EntityID]bool{},
		doctors: map[entity.EntityID]bool{}, healing: map[entity.EntityID]int{},
		sickRest: map[entity.EntityID]bool{}, sickStill: map[entity.EntityID]bool{}, surgery: map[entity.EntityID]bool{},
		hungry: map[entity.EntityID]bool{},
		tend:   map[entity.EntityID]bool{}, bldgs: map[entity.EntityID]construction.Building{},
		owners: map[entity.EntityID]entity.EntityID{}, stacks: map[entity.EntityID]item.Stack{},
		sleptAt: map[entity.EntityID]entity.EntityID{},
		tended:  map[entity.EntityID]int32{}, work: TendWork,
	}
}

// pawn adds a standing colonist at (x, y).
func (h *fakeHost) pawn(id entity.EntityID, x, y int) {
	h.pawns = append(h.pawns, id)
	h.pos[id] = [2]int{x, y}
	h.vital[id] = health.Record{Status: health.Alive}
}

// downed lays pawn id down from injury.
func (h *fakeHost) downed(id entity.EntityID, injury int) {
	h.vital[id] = health.Record{Status: health.Downed, Cause: health.CauseInjury, Injury: injury}
	h.tend[id] = true
}

func (h *fakeHost) bed(id entity.EntityID, b construction.Building) {
	h.bldgIDs = append(h.bldgIDs, id)
	h.bldgs[id] = b
}

func (h *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.pos[id]
	return p[0], p[1], ok
}
func (h *fakeHost) Place(id entity.EntityID, x, y int) { h.pos[id] = [2]int{x, y} }
func (h *fakeHost) Vital(id entity.EntityID) (health.Record, bool) {
	v, ok := h.vital[id]
	return v, ok
}
func (h *fakeHost) SetVital(id entity.EntityID, v health.Record) { h.vital[id] = v }
func (h *fakeHost) Job(id entity.EntityID) (job.Record, bool) {
	j, ok := h.jobs[id]
	return j, ok
}
func (h *fakeHost) SetJob(id entity.EntityID, j job.Record)     { h.jobs[id] = j }
func (h *fakeHost) Now() uint64                                 { return h.now }
func (h *fakeHost) BleedPerDay(id entity.EntityID) int32        { return h.bleed[id] }
func (h *fakeHost) PawnIDs() []entity.EntityID                  { return h.pawns }
func (h *fakeHost) RaiderIDs() []entity.EntityID                { return h.raiders }
func (h *fakeHost) IsPrisoner(id entity.EntityID) bool          { return h.prisoners[id] }
func (h *fakeHost) IsColonist(id entity.EntityID) bool          { return slices.Contains(h.pawns, id) }
func (h *fakeHost) Drafted(id entity.EntityID) bool             { return h.drafted[id] }
func (h *fakeHost) Name(id entity.EntityID) string              { return "P" + string(rune('0'+id%10)) }
func (h *fakeHost) DoctorBlocked(id entity.EntityID) bool       { return h.blocked[id] }
func (h *fakeHost) BuildingIDs() []entity.EntityID              { return h.bldgIDs }
func (h *fakeHost) BedUnder(id entity.EntityID) entity.EntityID { return h.bedAt(id) }
func (h *fakeHost) ReleaseBed(id entity.EntityID)               { h.log = append(h.log, "release") }
func (h *fakeHost) SetSleptAt(id, bed entity.EntityID)          { h.sleptAt[id] = bed }
func (h *fakeHost) CareGroup(id entity.EntityID) medcare.Group {
	switch {
	case h.animals[id]:
		return medcare.Animals
	case h.prisoners[id]:
		return medcare.Prisoners
	}
	return medcare.Colonists
}
func (h *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := h.bldgs[id]
	return b, ok
}
func (h *fakeHost) SetBuilding(id entity.EntityID, b construction.Building) { h.bldgs[id] = b }
func (h *fakeHost) SleepCell(id entity.EntityID, b construction.Building) (int, int) {
	return b.X, b.Y
}

// bedAt: the claimed bed whose tile id lies on.
func (h *fakeHost) bedAt(id entity.EntityID) entity.EntityID {
	p := h.pos[id]
	for _, bid := range h.bldgIDs {
		if b := h.bldgs[bid]; b.X == p[0] && b.Y == p[1] && b.SlotOf(uint64(id)) >= 0 {
			return bid
		}
	}
	return 0
}

func (h *fakeHost) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return x + 1, y, !h.noPath
}
func (h *fakeHost) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	if h.noPath {
		return nil, false
	}
	if p := h.pos[id]; p == [2]int{x, y} {
		return nil, true
	}
	return []pathing.Point{{X: x, Y: y}}, true
}
func (h *fakeHost) Owner(target entity.EntityID) (entity.EntityID, bool) {
	o, ok := h.owners[target]
	return o, ok
}
func (h *fakeHost) Claim(owner, target entity.EntityID) bool {
	if o, ok := h.owners[target]; ok && o != owner {
		return false
	}
	h.owners[target] = owner
	return true
}
func (h *fakeHost) StartJob(id entity.EntityID, rec job.Record) bool {
	if h.blocked[id] {
		return false
	}
	rec.State = job.Goto
	h.jobs[id] = rec
	return true
}
func (h *fakeHost) Interrupt(id entity.EntityID, reason job.FailReason) {
	h.log = append(h.log, "interrupt")
	delete(h.jobs, id)
}
func (h *fakeHost) NeedsTending(id entity.EntityID) bool { return h.tend[id] }
func (h *fakeHost) Standing(id entity.EntityID) bool {
	return h.vital[id].Status == health.Alive && slices.Contains(h.pawns, id)
}
func (h *fakeHost) DoesDoctor(id entity.EntityID) bool { return h.doctors[id] }
func (h *fakeHost) IsPatient(id entity.EntityID) bool {
	return slices.Contains(h.pawns, id) || h.animals[id] || slices.Contains(h.raiders, id)
}
func (h *fakeHost) HealingInjury(id entity.EntityID) int    { return h.healing[id] }
func (h *fakeHost) SickRest(id entity.EntityID) bool        { return h.sickRest[id] }
func (h *fakeHost) SickStill(id entity.EntityID) bool       { return h.sickStill[id] }
func (h *fakeHost) AwaitingSurgery(id entity.EntityID) bool { return h.surgery[id] }
func (h *fakeHost) GoEat(id entity.EntityID) bool {
	if !h.hungry[id] {
		return false
	}
	h.jobs[id] = job.Record{Kind: job.Eat, State: job.Goto}
	return true
}
func (h *fakeHost) SleepInBed(id entity.EntityID) bool {
	if h.noBeds {
		return false
	}
	h.jobs[id] = job.Record{Kind: job.Sleep, Target: 99, State: job.Goto}
	return true
}
func (h *fakeHost) SleepInPlace(id entity.EntityID) {
	h.jobs[id] = job.Record{Kind: job.Sleep, State: job.Goto}
}
func (h *fakeHost) LieInHospitalBed(id entity.EntityID) bool {
	h.log = append(h.log, "hospital")
	return true
}
func (h *fakeHost) MedicineTiers(patient entity.EntityID) [3]item.Kind {
	return [3]item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit}
}
func (h *fakeHost) NearestStack(id entity.EntityID, k item.Kind) (entity.EntityID, item.Stack) {
	for _, sid := range slices.Sorted(maps.Keys(h.stacks)) {
		if st := h.stacks[sid]; st.Kind == k && st.Carrier == 0 {
			return sid, st
		}
	}
	return 0, item.Stack{}
}
func (h *fakeHost) PickUpDose(id entity.EntityID, j job.Record) (job.Record, bool) {
	st := h.stacks[j.Dest]
	st.Carrier = id
	h.stacks[j.Dest] = st
	j.Carry, j.Dest = j.Dest, 0
	j.TX, j.TY = j.DestX, j.DestY
	return j, true
}
func (h *fakeHost) DeleteItem(id entity.EntityID)       { delete(h.stacks, id) }
func (h *fakeHost) MedicineWork(id entity.EntityID) int { return h.work }
func (h *fakeHost) TendQuality(doctor, patient entity.EntityID, j job.Record) int32 {
	if j.Carry != 0 {
		return 800
	}
	return 400
}
func (h *fakeHost) ApplyTend(patient entity.EntityID, q int32) {
	h.tended[patient] = q
	h.tend[patient] = false
}
func (h *fakeHost) Rescued(rescuer, patient entity.EntityID) { h.log = append(h.log, "rescued") }
func (h *fakeHost) TendedBy(patient, doctor entity.EntityID) { h.log = append(h.log, "tended") }
func (h *fakeHost) Message(x, y int, text string)            { h.log = append(h.log, "message: "+text) }

// Breaking: SIM-062 (the capture/arrest handler skips a colonist in a break).
func (h *fakeHost) Breaking(id entity.EntityID) bool { return h.breaking[id] }
