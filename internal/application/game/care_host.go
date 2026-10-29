package game

import (
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// careHost implements care.Host (care/deps.go) over the game: one small
// adapter type per role, each a thin view of *Game, so no method is added
// to Game itself (.cursor/SOLID_GO.md §10).
type careHost struct {
	careBodies
	careRoster
	careBeds
	careReach
	careJobs
	careHealth
	careTreat
	careEvents
}

var _ care.Host = careHost{}

func newCareHost(g *Game) careHost {
	return careHost{careBodies{g}, careRoster{g}, careBeds{g}, careReach{g}, careJobs{g}, careHealth{g}, careTreat{g}, careEvents{g}}
}

type careBodies struct{ g *Game }

func (h careBodies) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h careBodies) Place(id entity.EntityID, x, y int)             { h.g.positions.Set(id, Position{X: x, Y: y}) }
func (h careBodies) Vital(id entity.EntityID) (health.Record, bool) { return h.g.vital.Get(id) }
func (h careBodies) SetVital(id entity.EntityID, v health.Record)   { h.g.vital.Set(id, v) }
func (h careBodies) Job(id entity.EntityID) (job.Record, bool)      { return h.g.jobs.Get(id) }
func (h careBodies) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h careBodies) Now() uint64                                    { return uint64(h.g.clock.Now()) }
func (h careBodies) BleedPerDay(id entity.EntityID) int32 {
	if b := h.g.bodies[id]; b != nil {
		return b.BleedPerDay() // body/bleed.go
	}
	return 0
}

type careRoster struct{ g *Game }

func (h careRoster) PawnIDs() []entity.EntityID         { return h.g.pawnIDs }
func (h careRoster) RaiderIDs() []entity.EntityID       { return h.g.raiderIDs }
func (h careRoster) IsPrisoner(id entity.EntityID) bool { return h.g.isPrisoner(id) }
func (h careRoster) IsColonist(id entity.EntityID) bool { return h.g.colonist(id) }
func (h careRoster) Drafted(id entity.EntityID) bool    { return h.g.drafted(id) }
func (h careRoster) Name(id entity.EntityID) string     { return h.g.name(id) }
func (h careRoster) CareGroup(id entity.EntityID) medcare.Group {
	switch g := h.g; {
	case g.isAnimal(id):
		return medcare.Animals
	case g.isPrisoner(id):
		return medcare.Prisoners
	case g.visitor(id):
		return medcare.Guests
	}
	return medcare.Colonists
}
func (h careRoster) DoctorBlocked(id entity.EntityID) bool {
	g := h.g
	return !g.alive(id) || g.breaking(id) || g.incapableWork(id).Enabled(work.Doctor)
}

type careBeds struct{ g *Game }

func (h careBeds) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h careBeds) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h careBeds) SetBuilding(id entity.EntityID, b construction.Building) { h.g.bldgs.Set(id, b) }
func (h careBeds) BedUnder(id entity.EntityID) entity.EntityID             { return h.g.bedUnder(id) }
func (h careBeds) SleepCell(id entity.EntityID, b construction.Building) (int, int) {
	return h.g.sleepCellFor(id, b)
}
func (h careBeds) ReleaseBed(id entity.EntityID)      { h.g.releaseBed(id) }
func (h careBeds) SetSleptAt(id, bed entity.EntityID) { h.g.sleptAt[id] = bed }

type careReach struct{ g *Game }

func (h careReach) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h careReach) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h careReach) Owner(target entity.EntityID) (entity.EntityID, bool) {
	return h.g.reserve.EntityOwner(target) // never kept: the table is replaced on load
}
func (h careReach) Claim(owner, target entity.EntityID) bool {
	return h.g.reserve.TryEntity(owner, target)
}

type careJobs struct{ g *Game }

func (h careJobs) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h careJobs) Interrupt(id entity.EntityID, reason job.FailReason) {
	h.g.interrupt(id, reason)
}
func (h careJobs) Breaking(id entity.EntityID) bool { return h.g.breaking(id) }
func (h careJobs) GoEat(id entity.EntityID) bool {
	if n, _ := h.g.need.Get(id); n.Hungry() && h.g.seekFood(id) {
		h.g.assignEat(id)
		return true
	}
	return false
}
func (h careJobs) SleepInBed(id entity.EntityID) bool       { return h.g.sleepInBed(id) }
func (h careJobs) SleepInPlace(id entity.EntityID)          { h.g.assignSleepInPlace(id) }
func (h careJobs) LieInHospitalBed(id entity.EntityID) bool { return h.g.lieInHospitalBed(id) } // surgery.go

type careHealth struct{ g *Game }

func (h careHealth) NeedsTending(id entity.EntityID) bool { return h.g.needsTending(id) } // infection.go
func (h careHealth) Standing(id entity.EntityID) bool     { return h.g.standingPawn(id) }
func (h careHealth) DoesDoctor(id entity.EntityID) bool   { return h.g.prioOf(id)[work.Doctor] > 0 }
func (h careHealth) IsPatient(id entity.EntityID) bool    { return h.g.isPawn(id) || h.g.isAnimal(id) }
func (h careHealth) HealingInjury(id entity.EntityID) int {
	v, _ := h.g.vital.Get(id)
	return healingInjury(h.g.bodies[id], v) // medical.go
}
func (h careHealth) SickRest(id entity.EntityID) bool { // disease.go, food_poison.go
	return h.g.infectionSick(id) || h.g.diseaseRest(id) || h.g.foodPoisonRest(id)
}
func (h careHealth) SickStill(id entity.EntityID) bool {
	return h.g.infectionSick(id) || h.g.diseaseSick(id) || h.g.foodPoisonSick(id)
}
func (h careHealth) AwaitingSurgery(id entity.EntityID) bool { return h.g.awaitingSurgery(id) } // surgery.go

type careTreat struct{ g *Game }

func (h careTreat) MedicineTiers(patient entity.EntityID) [3]item.Kind {
	return h.g.medicineTiers(patient) // items_ng1.go
}
func (h careTreat) NearestStack(id entity.EntityID, k item.Kind) (entity.EntityID, item.Stack) {
	return h.g.ingredientFor(id, recipe.Input{Kind: k, Qty: 1, Max: 1})
}
func (h careTreat) PickUpDose(id entity.EntityID, j job.Record) (job.Record, bool) {
	ok := h.g.pickUpMedicine(id, &j) // infection.go
	return j, ok
}
func (h careTreat) DeleteItem(id entity.EntityID) { h.g.deleteItem(id) }
func (h careTreat) MedicineWork(id entity.EntityID) int {
	return h.g.workProgress(id, skMedicine, true)
}
func (h careTreat) TendQuality(doctor, patient entity.EntityID, j job.Record) int32 {
	return h.g.tendQualityFor(doctor, patient, h.g.tendMedicine(j)) // infection.go (MED-02)
}

// ApplyTend: an older save's Injury becomes wounds first (ensureBody),
// every wound is tended, and a patient down from injury that can stand
// again gets up (bleed.go).
func (h careTreat) ApplyTend(patient entity.EntityID, q int32) {
	g := h.g
	v, _ := g.vital.Get(patient)
	if b := g.ensureBody(patient, &v); b != nil {
		b.TendAll(q, uint64(g.clock.Now()))
		g.bodyChanged(patient, b, &v)
	}
	if v.Status == health.Downed && v.Cause == health.CauseInjury && g.standsUp(patient, v) {
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(patient, v)
}

type careEvents struct{ g *Game }

func (h careEvents) Rescued(rescuer, patient entity.EntityID) {
	h.g.logPair(patient, rescuer, logRescuedBy, logRescued, 0, 0)
	h.g.onRescued(rescuer, patient) // social.go: the patient's gratitude
}
func (h careEvents) TendedBy(patient, doctor entity.EntityID) {
	h.g.logPair(patient, doctor, logTendedBy, logTended, 0, 0)
}
func (h careEvents) Message(x, y int, text string) { h.g.message(x, y, "%s", text) }
