package game

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// Surgery (ENG-003 M1; surgery v2, MED-06). The player queues operations
// on a colonist, prisoner or guest from the Health tab
// (command.AddOperation). A doctor (Doctor work, after tending) takes the
// nearest patient lying down with an operation it can do now: it fetches
// the implant or organ (if any) and the medicine the operation uses up
// (body.MedicineNeed, capped by the patient's care level), and works
// SurgeryWork ticks scaled by Medicine. The success chance follows the
// wiki (body.SurgeryChance: skill, manipulation, sight, bed, cleanliness,
// light, vitals monitor, medicine, operation); the body domain rolls the
// outcome on the body stream: success installs, removes, amputates,
// harvests or transplants; failure wounds the patient (and may kill on a
// heart or carcinoma operation) and calls onSurgeryBotched. Every
// operation but euthanasia leaves the patient under an anaesthetic
// (body.CondAnesthetic), which wears off over 20 hours (stepAnesthetics).

const (
	// SurgeryWork is the work an operation takes (Medicine scales speed).
	SurgeryWork = 600
	// anaesthTicks was how long an older save's patient stayed down after
	// an operation (the legacy Under entries; MED-06 uses the condition).
	anaesthTicks = clock.TicksPerHour
	// maxOperations caps one patient's queue.
	maxOperations = 8
	// surgeryRestRetry: a patient who waited patientWait in bed for a
	// surgeon gets up and does not lie down for surgery again this long.
	surgeryRestRetry = 6 * clock.TicksPerHour
)

// Organ harvest goodwill (MED-06). wiki: Human resources (a kidney, lung
// or liver −80, a heart −160; one harvest costs at most 100).
const (
	harvestGoodwill      = -80
	harvestGoodwillHeart = -160
	harvestGoodwillCap   = -100
)

// surgeryState is the queued operations per patient, an older save's
// patients still under the hour-long anaesthetic (legacy, never added
// to), who gave up waiting for a surgeon until when (ENG-003, M1), and
// the hospital bed a patient claimed only to wait for surgery in, given
// back once it is up again (SIM-049); nil until the first operation.
type surgeryState struct {
	ops   map[entity.EntityID][]body.Operation
	under map[entity.EntityID]uint64
	rest  map[entity.EntityID]uint64
	hosp  map[entity.EntityID]entity.EntityID
}

// surgerySave is surgeryState in the save, in id order.
type surgerySave struct {
	Ops   []entry[[]body.Operation]
	Under []entry[uint64]
	Rest  []entry[uint64]
	Hosp  []entry[entity.EntityID]
}

func (s *surgeryState) ensure() {
	if s.ops == nil {
		s.ops = make(map[entity.EntityID][]body.Operation)
		s.under = make(map[entity.EntityID]uint64)
		s.rest = make(map[entity.EntityID]uint64)
	}
	if s.hosp == nil {
		s.hosp = make(map[entity.EntityID]entity.EntityID)
	}
}

// exportValues lists a value map's entries in id order (nil when empty).
func exportValues[T any](m map[entity.EntityID]T) []entry[T] {
	if len(m) == 0 {
		return nil
	}
	ptrs := make(map[entity.EntityID]*T, len(m))
	for id, v := range m {
		ptrs[id] = &v
	}
	return exportMap(ptrs)
}

// importValues rebuilds exportValues' map (always non-nil).
func importValues[T any](es []entry[T]) map[entity.EntityID]T {
	m := make(map[entity.EntityID]T, len(es))
	for _, e := range es {
		m[e.ID] = e.V
	}
	return m
}

// exportSurgery is the Surgery save field (nil: nothing queued or pending).
func (g *Game) exportSurgery() *surgerySave {
	if g.surg == nil || len(g.surg.ops) == 0 && len(g.surg.under) == 0 && len(g.surg.rest) == 0 && len(g.surg.hosp) == 0 {
		return nil
	}
	return &surgerySave{Ops: exportValues(g.surg.ops), Under: exportValues(g.surg.under), Rest: exportValues(g.surg.rest), Hosp: exportValues(g.surg.hosp)}
}

// importSurgery restores exportSurgery's state (nil: an older save).
func (g *Game) importSurgery(s *surgerySave) {
	g.surg = nil
	if s == nil {
		return
	}
	g.surg = &surgeryState{ops: importValues(s.Ops), under: importValues(s.Under), rest: importValues(s.Rest), hosp: importValues(s.Hosp)}
}

// operationsOf is patient id's queue (read only).
func (g *Game) operationsOf(id entity.EntityID) []body.Operation {
	if g.surg == nil {
		return nil
	}
	return g.surg.ops[id]
}

// opCode packs an operation into a job's Count, so a running job finds its
// operation again even when the queue shifts under it.
func opCode(op body.Operation) int {
	return int(op.Kind)<<16 | int(op.Part)<<8 | int(op.Item)
}

// cmdAddOperation queues operation op on part for patient (a colonist,
// prisoner or guest): only a valid operation on the body as it is now (an
// implant or organ on a part it fits, a removal of an implant, an
// amputation or harvest of a part still there, euthanasia or anaesthesia
// on the whole body), once.
func (g *Game) cmdAddOperation(patient entity.EntityID, op int, part, kind uint8) {
	if !g.isPawn(patient) || op < 0 || op >= int(body.NumOps) {
		return
	}
	if v, ok := g.vital.Get(patient); !ok || v.Status == health.Dead {
		return
	}
	o := body.Operation{Kind: body.OpKind(op), Part: body.Part(part)}
	switch {
	case o.Kind == body.InstallImplant || o.Kind == body.InstallOrgan:
		o.Item = item.Kind(kind)
	case o.Kind.WholeBody():
		o.Part = body.Torso
	}
	if !g.viewBody(patient).CanOperate(o) { // body.go
		return
	}
	if g.surg == nil {
		g.surg = &surgeryState{}
	}
	g.surg.ensure()
	q := g.surg.ops[patient]
	if len(q) >= maxOperations {
		return
	}
	for _, x := range q {
		if x.Kind == o.Kind && x.Part == o.Part {
			return
		}
	}
	g.surg.ops[patient] = append(q, o)
}

// cmdRemoveOperation drops patient's queued operation in slot i.
func (g *Game) cmdRemoveOperation(patient entity.EntityID, i int) {
	q := g.operationsOf(patient)
	if i < 0 || i >= len(q) {
		return
	}
	g.dropOperation(patient, i)
}

// cmdReorderOperation moves patient's queued operation in slot by one
// place: up (by -1) or down (by +1), swapping it with its neighbour (MED-00).
// Any other step, or a slot or neighbour out of range, is a no-op. A
// running operation finds its own again by opCode, so moving it is safe.
func (g *Game) cmdReorderOperation(patient entity.EntityID, slot, by int) {
	q := g.operationsOf(patient)
	to := slot + by
	if by != 1 && by != -1 || slot < 0 || slot >= len(q) || to < 0 || to >= len(q) {
		return
	}
	q[slot], q[to] = q[to], q[slot]
}

// dropOperation removes slot i of patient's queue.
func (g *Game) dropOperation(patient entity.EntityID, i int) {
	q := g.surg.ops[patient]
	q = append(q[:i:i], q[i+1:]...)
	if len(q) == 0 {
		delete(g.surg.ops, patient)
		return
	}
	g.surg.ops[patient] = q
}

// opNeedsItem is the implant or organ an operation fits (0: none).
func opNeedsItem(op body.Operation) item.Kind {
	if op.Kind == body.InstallImplant || op.Kind == body.InstallOrgan {
		return op.Item
	}
	return 0
}

// opFetch is what a doctor fetches for an operation: the implant or
// organ stack (0: none needed) and the medicine stack.
type opFetch struct {
	item, med entity.EntityID
}

// readyOp is the first operation in id's queue doctor could start now:
// valid on the body, with its implant or organ and its medicine free and
// within the doctor's reach (opBlocked).
func (g *Game) readyOp(doctor, id entity.EntityID) (int, opFetch, bool) {
	q := g.operationsOf(id)
	if len(q) == 0 {
		return 0, opFetch{}, false
	}
	b := g.viewBody(id)
	for i, op := range q {
		if why, f := g.opBlocked(id, doctor, b, op, false); why == "" {
			return i, f, true
		}
	}
	return 0, opFetch{}, false
}

// awaitingSurgery: id has an operation a doctor could start now (the same
// test surgeryWait reports on), someone else to do it, and has not just
// given up waiting (patientRest, infection.go). A patient already lying
// down keeps waiting while its implant is only briefly taken (a hauler
// moving it): it lies down on a strict test and gets up on a looser one,
// so a passing reservation does not flap it (SIM-049).
func (g *Game) awaitingSurgery(id entity.EntityID) bool {
	if g.surg == nil || len(g.surg.ops[id]) == 0 {
		return false
	}
	if g.surgeonComing(id, -1) {
		return true // the surgeon is on the way (its implant is reserved)
	}
	if uint64(g.clock.Now()) < g.surg.rest[id] || !g.care.Doctor.OtherDoctor(id) {
		return false
	}
	b, lying := g.viewBody(id), g.care.Doctor.Lying(id)
	for _, op := range g.surg.ops[id] {
		if why, _ := g.opBlocked(id, 0, b, op, lying); why == "" {
			return true
		}
	}
	return false
}

// surgeonComing: a surgeon holds patient for an operation of code (any
// operation when code < 0).
func (g *Game) surgeonComing(patient entity.EntityID, code int) bool {
	owner, taken := g.reserve.EntityOwner(patient)
	if !taken {
		return false
	}
	j, _ := g.jobs.Get(owner)
	return j.Kind == job.Surgery && j.Target == patient && (code < 0 || j.Count == code)
}

// Waiting reasons of a queued operation (OperationView.Status and the
// picker's disabled options; SIM-049, MED-06).
const (
	waitBody       = "can't be done on the body as it is"
	waitResearch   = "needs research: "
	waitNoDoctor   = "needs another colonist who can doctor"
	waitLieDown    = "waiting for the patient to lie down"
	waitOnTheWay   = "a doctor is on the way"
	waitNoStockFm  = "no %s in stock"
	waitReachFm    = "the %s can't be reached"
	waitSedated    = "already under an anesthetic"
	waitSkillFm    = "needs a doctor with Medicine %d"
	waitNoMedsCare = "the care level allows no medicine"
	waitMedsFm     = "needs %d %s or better"
)

// opBlocked is why doctor can't start op on patient (body b) now, ""
// with what to fetch when it can. Doctor 0 asks about any other colonist
// who doctors. The reasons come in order: the operation no longer fits
// the body, the patient is already sedated (anaesthesia), the implant or
// organ needs research or is not in stock or not in reach, no one else
// to operate, no one skilled enough, no medicine the care level allows.
// Loose counts a stack a hauler holds as in stock (a waiting patient
// stays down; awaitingSurgery).
func (g *Game) opBlocked(patient, doctor entity.EntityID, b *body.Body, op body.Operation, loose bool) (string, opFetch) {
	if !b.CanOperate(op) {
		return waitBody, opFetch{}
	}
	if op.Kind == body.Anesthetize && b.Sedated() {
		return waitSedated, opFetch{}
	}
	var f opFetch
	if k := opNeedsItem(op); k != 0 {
		var why string
		if f.item, why = g.implantFor(patient, doctor, k, loose); f.item == 0 {
			return why, opFetch{}
		}
	}
	if doctor == 0 && !g.care.Doctor.OtherDoctor(patient) {
		return waitNoDoctor, opFetch{}
	}
	if lvl := body.MinSkill(op.Kind); lvl > 0 && !g.skilledSurgeon(patient, doctor, lvl) {
		return fmt.Sprintf(waitSkillFm, lvl), opFetch{}
	}
	var why string
	if f.med, why = g.surgeryMedicine(patient, doctor, op); f.med == 0 {
		return why, opFetch{}
	}
	return "", f
}

// skilledSurgeon: doctor (0: any other standing colonist who doctors)
// has Medicine lvl.
func (g *Game) skilledSurgeon(patient, doctor entity.EntityID, lvl int) bool {
	if doctor != 0 {
		return int(g.skillOf(doctor, skill.Medicine).Level) >= lvl
	}
	for _, pid := range g.pawnIDs {
		if pid != patient && g.standingPawn(pid) && g.prioOf(pid)[work.Doctor] > 0 && int(g.skillOf(pid, skill.Medicine).Level) >= lvl {
			return true
		}
	}
	return false
}

// surgeryMedTiers are the medicine kinds an operation may use, cheapest
// first.
var surgeryMedTiers = [3]item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit}

// medTier ranks a medicine kind (herbal 0, industrial 1, nanomed 2).
func medTier(k item.Kind) int { return slices.Index(surgeryMedTiers[:], k) }

// surgeryMedicine is the nearest free stack of medicine op uses up
// (body.MedicineNeed: enough of one kind, at least the kind it needs)
// that patient's care level allows and doctor (0: any other colonist who
// doctors) can reach, cheapest kind first; or why there is none.
func (g *Game) surgeryMedicine(patient, doctor entity.EntityID, op body.Operation) (entity.EntityID, string) {
	qty, least := body.MedicineNeed(op.Kind)
	pol := g.care.Level(patient) // care package (MED-03)
	from := doctor
	if from == 0 {
		from = patient
	}
	allowed := false
	for _, k := range surgeryMedTiers {
		if medTier(k) < medTier(least) || !medcare.Allows(pol, k) {
			continue
		}
		allowed = true
		sid, st := g.ingredientFor(from, recipe.Input{Kind: k, Qty: qty, Max: qty}) // production.go
		if sid == 0 {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(sid); taken && owner != doctor {
			continue
		}
		if !g.canFetch(patient, doctor, st.X, st.Y) {
			continue
		}
		return sid, ""
	}
	if !allowed {
		return 0, waitNoMedsCare
	}
	return 0, fmt.Sprintf(waitMedsFm, qty, least)
}

// implantFor is the nearest free stack of implant (or organ) k that
// doctor (0: any other colonist who doctors) can reach, or why there is
// none: its recipe still needs research and there is none to be had, none
// is in stock, or none can be reached (reachable: region labels, no
// search). Loose also takes a stack carried or reserved by someone else.
func (g *Game) implantFor(patient, doctor entity.EntityID, k item.Kind, loose bool) (entity.EntityID, string) {
	from := doctor
	if from == 0 {
		from = patient
	}
	pos, _ := g.positions.Get(from)
	var best entity.EntityID
	bestD, stocked := 1<<30, false
	for _, iid := range g.itemIDs {
		sp := g.stacks.Ref(iid)
		if sp == nil || sp.Kind != k || sp.Qty < 1 || sp.Forbidden {
			continue
		}
		x, y, carried := sp.X, sp.Y, sp.Carrier != 0 // copied: reachable may search
		if !loose {
			if carried {
				continue
			}
			if owner, taken := g.reserve.EntityOwner(iid); taken && (doctor == 0 || owner != doctor) {
				continue
			}
		}
		stocked = true
		d := pathing.Dist(pos.X, pos.Y, x, y)
		if best != 0 && (d > bestD || d == bestD && iid > best) {
			continue
		}
		if !carried && !g.canFetch(patient, doctor, x, y) {
			continue
		}
		best, bestD = iid, d
	}
	switch {
	case best != 0:
		return best, ""
	case stocked:
		return 0, fmt.Sprintf(waitReachFm, k)
	}
	if p := g.implantResearch(k); p != "" {
		return 0, waitResearch + p
	}
	return 0, fmt.Sprintf(waitNoStockFm, k)
}

// canFetch: doctor (0: any standing colonist besides patient who does
// Doctor work; with none, the patient itself, so a lone colonist hears
// that it needs a doctor rather than about reach) could walk to (x, y).
func (g *Game) canFetch(patient, doctor entity.EntityID, x, y int) bool {
	if doctor != 0 {
		return g.reachable(doctor, x, y)
	}
	found := false
	for _, pid := range g.pawnIDs {
		if pid != patient && g.standingPawn(pid) && g.prioOf(pid)[work.Doctor] > 0 {
			if g.reachable(pid, x, y) {
				return true
			}
			found = true
		}
	}
	return !found && g.reachable(patient, x, y)
}

// implantResearch names the project still locking every recipe that
// makes implant k ("" when one is open, or none makes it).
func (g *Game) implantResearch(k item.Kind) string {
	locked := ""
	for _, r := range recipe.All() {
		if r.Output != k {
			continue
		}
		p := g.lockedBy("recipe:" + r.Name) // research.go
		if p == "" {
			return ""
		}
		if locked == "" {
			locked = p
		}
	}
	return locked
}

// lieInHospitalBed puts a patient waiting for surgery in a free hospital
// bed (care.SafeBedFor ranks them first), claimed only for the stay: its own
// bed stays its own, and leaveHospitalBed gives this one back once it is
// up again. False when there is none free (or it can't be reached), and
// nothing changes then.
func (g *Game) lieInHospitalBed(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	bid := g.care.SafeBedFor(id, pos.X, pos.Y, care.RescueBed)
	if bid == 0 {
		return false
	}
	b, _ := g.bldgs.Get(bid)
	if b.Kind != construction.HospitalBed {
		return false
	}
	owned := b.SlotOf(uint64(id)) >= 0
	if !owned {
		b = b.Claim(uint64(id))
		g.bldgs.Set(bid, b)
	}
	sx, sy := g.sleepCellFor(id, b)
	if !g.startJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		if !owned {
			g.bldgs.Set(bid, b.Release(uint64(id)))
		}
		return false
	}
	g.sleptAt[id] = bid
	if !owned {
		g.surg.ensure()
		g.surg.hosp[id] = bid
	}
	return true
}

// leaveHospitalBed gives back the hospital bed a patient claimed for
// surgery once it stands and is not lying in it (still down: it stays).
func (g *Game) leaveHospitalBed(id entity.EntityID, v *health.Record) {
	bid, ok := g.surg.hosp[id]
	if !ok || v.Status == health.Downed {
		return
	}
	if j, _ := g.jobs.Get(id); v.Status == health.Alive && j.Kind == job.Sleep && j.Target == bid {
		return
	}
	delete(g.surg.hosp, id)
	if b, ok := g.bldgs.Get(bid); ok && b.SlotOf(uint64(id)) >= 0 {
		g.bldgs.Set(bid, b.Release(uint64(id)))
	}
}

// surgeryGaveUp: the patient lay patientWait in bed and no surgeon came;
// it gets up and won't lie down for surgery again for a while (no flap).
func (g *Game) surgeryGaveUp(id entity.EntityID) {
	if g.surg == nil || len(g.surg.ops[id]) == 0 {
		return
	}
	g.surg.ensure()
	g.surg.rest[id] = uint64(g.clock.Now()) + surgeryRestRetry
}

// surgeryPatients calls fn for every pawn that can be operated on, in a
// fixed order: colonists, then prisoners, then guests (MED-06), each in
// id order; fn returning false stops.
func (g *Game) surgeryPatients(fn func(entity.EntityID) bool) {
	for _, pid := range g.pawnIDs {
		if !fn(pid) {
			return
		}
	}
	for _, pid := range g.raiderIDs {
		if g.isPrisoner(pid) && !fn(pid) {
			return
		}
	}
	for _, pid := range g.visitorIDs() {
		if !fn(pid) {
			return
		}
	}
}

// assignSurgery sends doctor id to the nearest patient lying down (in bed,
// downed or asleep) with an operation it can do now: colonists, then
// prisoners, then guests, nearest first (ties keep that order).
func (g *Game) assignSurgery(id entity.EntityID) bool {
	if g.surg == nil || len(g.surg.ops) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best job.Record
	var bestMed entity.EntityID
	bestD, found := 0, false
	g.surgeryPatients(func(pid entity.EntityID) bool {
		if pid == id || len(g.surg.ops[pid]) == 0 || g.care.Carry.Carrier(pid) != 0 {
			return true
		}
		if v, _ := g.vital.Get(pid); v.Status == health.Dead || !g.care.Doctor.Lying(pid) {
			return true
		}
		if owner, taken := g.reserve.EntityOwner(pid); taken && owner != id {
			return true
		}
		pp, _ := g.positions.Get(pid)
		d := manhattan(pos.X, pos.Y, pp.X, pp.Y)
		if found && d >= bestD {
			return true
		}
		slot, f, ok := g.readyOp(id, pid)
		if !ok {
			return true
		}
		sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
		if !ok {
			return true
		}
		rec := job.Record{Kind: job.Surgery, Target: pid, TX: sx, TY: sy, WorkLeft: SurgeryWork,
			Count: opCode(g.surg.ops[pid][slot]), Route: []entity.EntityID{f.med}}
		fetch := f.med // the medicine, carried, unless there is an implant to carry
		if f.item != 0 {
			fetch = f.item
		}
		st, _ := g.stacks.Get(fetch)
		rec.Dest, rec.DestX, rec.DestY = fetch, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		best, bestMed, bestD, found = rec, f.med, d, true
		return true
	})
	if !found || !g.startJob(id, best) {
		return false
	}
	g.reserve.TryEntity(id, best.Dest)
	if bestMed != best.Dest {
		g.reserve.TryEntity(id, bestMed)
	}
	return true
}

// surgerySlot finds the queued operation a job is for.
func (g *Game) surgerySlot(patient entity.EntityID, code int) (int, bool) {
	for i, op := range g.operationsOf(patient) {
		if opCode(op) == code {
			return i, true
		}
	}
	return 0, false
}

// jobMedicine is the stack a surgery job takes its medicine from: the
// one it carries when it carries medicine, else the reserved stack in
// Route (an implant or organ is carried instead).
func jobMedicine(j job.Record) entity.EntityID {
	if len(j.Route) == 0 {
		return 0
	}
	return j.Route[0]
}

// doSurgery runs an operation: fetch, stand beside the patient, work, then
// let the body domain decide.
func (g *Game) doSurgery(id entity.EntityID, j job.Record) {
	patient := j.Target
	v, ok := g.vital.Get(patient)
	slot, queued := g.surgerySlot(patient, j.Count)
	if !ok || v.Status == health.Dead || !queued {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	op := g.surg.ops[patient][slot]
	need := opNeedsItem(op)
	if j.Dest != 0 && j.Carry == 0 {
		accept := isMedicine
		if need != 0 {
			accept = func(k item.Kind) bool { return k == need }
		}
		src := j.Dest
		walking := !g.pickUpItem(id, &j, accept) // infection.go
		if need == 0 && j.Carry != 0 {
			qty, _ := body.MedicineNeed(op.Kind)
			g.topUpCarry(src, j.Carry, qty-1) // the rest of the doses
		}
		if walking {
			return // walking to the patient (or the job ended)
		}
	}
	if need != 0 && !g.carryingKind(id, j, need) {
		j.State, j.Fail = job.Fail, job.TargetGone // the implant went missing
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if pp, _ := g.positions.Get(patient); abs(pp.X-pos.X) > 1 || abs(pp.Y-pos.Y) > 1 || !g.care.Doctor.Lying(patient) {
		j.State, j.Fail = job.Fail, job.TargetGone // the patient got up
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skMedicine, true) // skills.go: Medicine speeds it up
	j.WorkLeft -= u
	if u == 0 || j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	med, ok := g.surgeryDoses(id, j, op)
	if !ok {
		j.State, j.Fail = job.Fail, job.TargetGone // the medicine went missing
		g.jobs.Set(id, j)
		return
	}
	g.useDoses(id, &j, med, op)
	g.operate(id, patient, slot, &j, med.Kind)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// topUpCarry moves up to extra more of stack src into the carried stack
// carry (same kind, src lying free): a dose fetch takes several.
func (g *Game) topUpCarry(src, carry entity.EntityID, extra int) {
	if extra <= 0 || src == carry {
		return
	}
	s, ok := g.stacks.Get(src)
	c, ok2 := g.stacks.Get(carry)
	if !ok || !ok2 || s.Kind != c.Kind || s.Carrier != 0 || s.Qty < 1 {
		return
	}
	n := min(extra, s.Qty)
	c.Qty += n
	g.stacks.Set(carry, c)
	if n == s.Qty {
		g.deleteItem(src)
		return
	}
	s.Qty -= n
	g.stacks.Set(src, s)
}

// surgeryDoses is the medicine an operation about to happen uses: the
// carried doses, or for an implant or organ operation the reserved stack
// (still lying free with enough of a medicine); false when it went
// missing.
func (g *Game) surgeryDoses(id entity.EntityID, j job.Record, op body.Operation) (item.Stack, bool) {
	qty, _ := body.MedicineNeed(op.Kind)
	if opNeedsItem(op) == 0 {
		st, ok := g.stacks.Get(j.Carry)
		return st, ok && j.Carry != 0 && st.Carrier == id && isMedicine(st.Kind) && st.Qty >= qty
	}
	sid := jobMedicine(j)
	st, ok := g.stacks.Get(sid)
	return st, ok && sid != 0 && st.Carrier == 0 && !st.Forbidden && isMedicine(st.Kind) && st.Qty >= qty
}

// useDoses uses up op's medicine (surgeryDoses found it).
func (g *Game) useDoses(id entity.EntityID, j *job.Record, med item.Stack, op body.Operation) {
	qty, _ := body.MedicineNeed(op.Kind)
	if opNeedsItem(op) == 0 {
		g.deleteItem(j.Carry)
		j.Carry = 0
		return
	}
	sid := jobMedicine(*j)
	if med.Qty <= qty {
		g.deleteItem(sid)
		return
	}
	med.Qty -= qty
	g.stacks.Set(sid, med)
}

// carryingKind reports the job carrying an item of kind k.
func (g *Game) carryingKind(id entity.EntityID, j job.Record, k item.Kind) bool {
	if j.Carry == 0 {
		return false
	}
	st, ok := g.stacks.Get(j.Carry)
	return ok && st.Kind == k && st.Carrier == id
}

// surgeryInput is the success chance input of surgeon operating op on
// patient where it lies now with medicine med (surgeon 0: no surgeon
// yet, at Medicine 0 with whole hands and eyes).
func (g *Game) surgeryInput(surgeon, patient entity.EntityID, op body.OpKind, med item.Kind) body.SurgeryInput {
	in := body.SurgeryInput{Manip: 1000, Sight: 1000, HospitalBed: g.inHospitalBed(patient), Monitor: g.monitorBy(patient),
		Op: op, Medicine: med}
	if surgeon != 0 {
		in.Skill = int(g.skillOf(surgeon, skill.Medicine).Level)
		if b := g.bodies[surgeon]; b != nil {
			in.Manip, in.Sight = b.Capacity(body.Manipulation), b.Capacity(body.Sight)
		}
	}
	pos, _ := g.positions.Get(patient)
	in.CleanPct, in.Outdoors = g.surgeryCleanliness(pos.X, pos.Y)
	in.LightPct = int32(g.cellLightPermil(pos.X, pos.Y) / 10) // light.go: the light model
	return in
}

// surgeryCleanliness is the cleanliness (RimWorld cleanliness ×100) of
// the room at (x, y) for surgery, from the rooms index and the room's
// filth total (no scan): −2 per filth permille a cell on average, and
// −100 for a room left on bare ground, never below −500 (original
// tuning of how our filth maps onto the wiki's cleanliness). Outdoors
// reports true.
func (g *Game) surgeryCleanliness(x, y int) (int32, bool) {
	r := g.roomAt(x, y)
	if r == room.Outdoors || r < 0 || r >= len(g.rooms) {
		return 0, true
	}
	c := g.rooms[r].contents
	cells := max(c.Cells, 1)
	sum, _ := g.roomFilth(r)
	clean := -2*sum/cells - 100*(cells-min(c.Floored, cells))/cells
	return int32(max(clean, -500)), false
}

// surgeryChanceOf is surgeon's permille chance of op on patient where it
// lies with medicine med, with its breakdown.
func (g *Game) surgeryChanceOf(surgeon, patient entity.EntityID, op body.OpKind, med item.Kind) (int32, []body.Factor) {
	return body.SurgeryChance(g.surgeryInput(surgeon, patient, op, med))
}

// bestSurgeon is the standing colonist besides patient who doctors with
// the highest Medicine (lowest id on a tie; 0: none), the surgeon the
// Health tab's chance assumes.
func (g *Game) bestSurgeon(patient entity.EntityID) entity.EntityID {
	var best entity.EntityID
	bestLvl := -1
	for _, pid := range g.pawnIDs {
		if pid == patient || !g.standingPawn(pid) || g.prioOf(pid)[work.Doctor] == 0 {
			continue
		}
		if l := int(g.skillOf(pid, skill.Medicine).Level); l > bestLvl {
			best, bestLvl = pid, l
		}
	}
	return best
}

// operationChance is a queued operation's chance for the Health tab:
// percent, and a tooltip breaking it down by factor, as the best doctor
// would do it with the medicine it would use.
func (g *Game) operationChance(patient entity.EntityID, op body.Operation) (int, string) {
	if op.Kind == body.Euthanize {
		return 100, "Euthanasia\nIt cannot fail: the patient dies either way."
	}
	_, least := body.MedicineNeed(op.Kind)
	med := least
	if sid, _ := g.surgeryMedicine(patient, 0, op); sid != 0 {
		if st, ok := g.stacks.Get(sid); ok {
			med = st.Kind
		}
	}
	surgeon := g.bestSurgeon(patient)
	c, fs := g.surgeryChanceOf(surgeon, patient, op.Kind, med)
	who := "no doctor yet"
	if surgeon != 0 {
		who = g.name(surgeon)
	}
	lines := []string{fmt.Sprintf("Success chance %d%% (%s)", c/10, who)}
	for i, f := range fs {
		if i == 0 {
			lines = append(lines, fmt.Sprintf("%s: %d%%", f.Label, f.Pct))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: ×%d%%", f.Label, f.Pct))
	}
	if d := body.FailDeathChance(op); d > 0 {
		lines = append(lines, fmt.Sprintf("A failure kills %d%% of the time.", d/10))
	}
	return int(c / 10), strings.Join(lines, "\n")
}

// moreOperationOptions adds the surgery v2 options to the picker
// (operationOptions, body.go), after the amputations: harvesting each
// natural organ, transplanting an organ into each missing or damaged one,
// euthanasia, anaesthesia, then excising carcinomas, in operation kind
// order (add skips what can't be done on the body
// and what is already queued; a carcinoma excision appears with MED-08's
// condition).
func (g *Game) moreOperationOptions(b *body.Body, add func(body.Operation)) {
	for p := body.Part(0); p < body.NumParts; p++ {
		if body.OrganOf(p) != 0 {
			add(body.Operation{Kind: body.HarvestOrgan, Part: p})
		}
	}
	for p := body.Part(0); p < body.NumParts; p++ {
		if k := body.OrganOf(p); k != 0 {
			add(body.Operation{Kind: body.InstallOrgan, Part: p, Item: k})
		}
	}
	add(body.Operation{Kind: body.Euthanize, Part: body.Torso})
	add(body.Operation{Kind: body.Anesthetize, Part: body.Torso})
	for p := body.Part(0); p < body.NumParts; p++ {
		add(body.Operation{Kind: body.ExciseCarcinoma, Part: p})
	}
}

// operate carries out slot's operation on patient with medicine med (its
// doses already used up): roll, apply, use up the carried implant or
// organ, wound or kill on failure, then the anaesthetic and memories.
func (g *Game) operate(surgeon, patient entity.EntityID, slot int, j *job.Record, med item.Kind) {
	now := uint64(g.clock.Now())
	op := g.surg.ops[patient][slot]
	g.dropOperation(patient, slot)
	v, _ := g.vital.Get(patient)
	b := g.ensureBody(patient, &v) // body.go
	if b == nil || !b.CanOperate(op) {
		return
	}
	pp, _ := g.positions.Get(patient)
	if op.Kind == body.Euthanize {
		g.euthanize(surgeon, patient, b, &v, pp)
		return
	}
	rng := g.stream(streamBody)
	chance, _ := g.surgeryChanceOf(surgeon, patient, op.Kind, med)
	lvl := int(g.skillOf(surgeon, skill.Medicine).Level)
	outcome := body.RollOutcome(chance, lvl, rng.IntN)
	removed := b.Implant(op.Part)
	done := b.Apply(op, outcome, rng.IntN, now)
	if j.Carry != 0 && outcome != body.MinorFail {
		g.deleteItem(j.Carry) // an implant or organ is fitted or ruined
		j.Carry = 0
	}
	if done && (op.Kind == body.RemoveImplant || op.Kind == body.InstallOrgan) && removed.IsImplant() {
		g.dropStack(removed, 1, pp.X, pp.Y) // world.go: the implant comes out whole
	}
	if done && op.Kind == body.HarvestOrgan {
		g.dropStack(body.OrganOf(op.Part), 1, pp.X, pp.Y)
		g.organHarvested(surgeon, patient, op.Part)
	}
	failDeath := outcome != body.Success && body.FailDeathChance(op) > 0 &&
		int32(rng.IntN(1000)) < body.FailDeathChance(op)
	if v.Status != health.Dead && !failDeath {
		b.Anesthetize(now) // body/cond_anesthetic.go: every operation sedates
	}
	g.bodyChanged(patient, b, &v)
	g.remember(patient, mood.Surgery) // rooms.go
	line := g.name(surgeon) + " " + operationDone(op, outcome) + " on " + g.name(patient)
	g.logEvent(patient, logEntry{Kind: logSurgery, Name: line})
	if outcome != body.Success {
		g.woundOpened(patient) // infection.go: a surgical wound wants tending
		g.message(pp.X, pp.Y, "%s botched an operation on %s.", g.name(surgeon), g.name(patient))
		g.onSurgeryBotched(surgeon, patient)
	}
	switch {
	case done && op.Kind == body.HarvestOrgan && b.VitalLost():
		g.message(pp.X, pp.Y, "%s died when the %s was harvested.", g.name(patient), op.Part.Label())
		g.dieBy(patient, health.CauseOrganFailure, surgeon)
		return
	case failDeath || v.Injury >= health.MaxHP || b.VitalLost():
		g.message(pp.X, pp.Y, "%s died on the operating table.", g.name(patient))
		g.dieBy(patient, health.CauseSurgery, 0)
		return
	}
	g.sedate(patient, b, now)
}

// sedate lays a patient down under its anaesthetic (a pawn already down
// keeps its cause: its wounds or the anaesthetic stand it up later).
func (g *Game) sedate(patient entity.EntityID, b *body.Body, now uint64) {
	v, _ := g.vital.Get(patient)
	if v.Status != health.Alive || !g.downsNow(patient, v, b) { // bleed.go
		return
	}
	g.interrupt(patient, job.Interrupted)
	v, _ = g.vital.Get(patient)
	v.Status, v.Cause, v.DownedTick = health.Downed, health.CauseAnesthetic, now
	g.vital.Set(patient, v)
}

// euthanize: the surgeon puts patient down (MED-06). It cannot fail; the
// patient dies of euthanasia, its relatives holding it against the
// surgeon as for an execution (socialOnDeath).
func (g *Game) euthanize(surgeon, patient entity.EntityID, b *body.Body, v *health.Record, pp Position) {
	g.bodyChanged(patient, b, v)
	g.logEvent(patient, logEntry{Kind: logSurgery, Name: g.name(surgeon) + " euthanized " + g.name(patient)})
	g.message(pp.X, pp.Y, "%s euthanized %s.", g.name(surgeon), g.name(patient))
	g.dieBy(patient, health.CauseEuthanasia, surgeon) // combat.go
	if g.isPrisoner(patient) {
		delete(g.prisoners, patient)
	}
}

// organHarvested is the cost of harvesting part from donor: the donor's
// own memory, every colonist's (stacking; a colonist's organ weighs
// more), and goodwill with the donor's faction for a prisoner or guest.
func (g *Game) organHarvested(surgeon, donor entity.EntityID, part body.Part) {
	now := uint64(g.clock.Now())
	g.remember(donor, mood.OrganHarvested)
	know := mood.KnowOrganHarvested
	colonistDonor := slices.Contains(g.pawnIDs, donor)
	if colonistDonor {
		know = mood.KnowColonistOrganHarvested
	}
	for _, pid := range g.pawnIDs {
		if pid == donor {
			continue
		}
		if v, ok := g.vital.Get(pid); !ok || v.Status == health.Dead {
			continue
		}
		g.memories[pid] = g.memories[pid].AddFor(know, 0, now)
	}
	if colonistDonor {
		return
	}
	p := g.personOf(donor)
	if p == nil {
		return
	}
	f := g.factionOf(p.Faction)
	if f == nil {
		return
	}
	gw := harvestGoodwill
	if part == body.Heart {
		gw = harvestGoodwillHeart
	}
	gw = max(gw, harvestGoodwillCap)
	faction.Adjust(f, gw, fmt.Sprintf("Day %d: %s's %s harvested (%d goodwill)", g.relDay(now)+1, g.name(donor), part.Label(), gw))
	_ = surgeon
}

// operationDone is the log wording of an operation's outcome.
func operationDone(op body.Operation, o body.Outcome) string {
	part := op.Part.Label()
	switch o {
	case body.MinorFail:
		return "slipped during an operation on the " + part
	case body.Catastrophic:
		return "badly botched an operation on the " + part
	case body.Ridiculous:
		return "made a ridiculous mess of an operation on the " + part
	}
	switch op.Kind {
	case body.InstallImplant:
		return "fitted a " + op.Item.String() + " to the " + part
	case body.RemoveImplant:
		return "removed the implant from the " + part
	case body.HarvestOrgan:
		return "harvested the " + part
	case body.InstallOrgan:
		return "transplanted a " + op.Item.String() + " into the " + part
	case body.Anesthetize:
		return "anesthetized the patient"
	case body.ExciseCarcinoma:
		return "excised the carcinoma from the " + part
	}
	return "amputated the " + part
}

// stepAnesthetics wears every patient's anaesthetic off by an hour's
// worth (hourly, from the medical phase, in patients-index order; no
// scan) and stands up a patient it no longer holds down: up when its body
// allows (bleed.go standsUp), else left down from its wounds.
func (g *Game) stepAnesthetics(now uint64) {
	for _, id := range slices.Clone(g.med.patients) { // a change touches the index
		b := g.bodies[id]
		v, ok := g.vital.Get(id)
		if b == nil || !ok || v.Status == health.Dead {
			continue
		}
		if b.StepAnesthetic(now) {
			g.bodyChanged(id, b, &v)
		}
		g.wakeIfClear(id, b, &v)
	}
}

// wakeIfClear stands a pawn downed by an anaesthetic once it is no longer
// sedated: up if its body allows, else its wounds keep it down (heal and
// the downing rule stand it up later).
func (g *Game) wakeIfClear(id entity.EntityID, b *body.Body, v *health.Record) {
	if v.Status != health.Downed || v.Cause != health.CauseAnesthetic || b.Sedated() {
		return
	}
	if g.standsUp(id, *v) { // bleed.go
		v.Status, v.Cause = health.Alive, health.CauseNone
	} else {
		v.Cause = health.CauseInjury
	}
	g.vital.Set(id, *v)
}

// wakeFromAnaesthetic gives back a hospital bed claimed for surgery once
// the patient is up, and wakes an older save's patient from its
// hour-long anaesthetic (the legacy Under entries; MED-06 patients wake
// in stepAnesthetics).
func (g *Game) wakeFromAnaesthetic(id entity.EntityID, v *health.Record, now uint64) {
	if g.surg == nil {
		return
	}
	if len(g.surg.hosp) > 0 {
		g.leaveHospitalBed(id, v)
	}
	if len(g.surg.under) == 0 {
		return
	}
	until, ok := g.surg.under[id]
	if !ok || now < until {
		return
	}
	delete(g.surg.under, id)
	if v.Status != health.Downed || v.Cause != health.CauseSurgery {
		return
	}
	if !g.standsUp(id, *v) { // bleed.go (MED-01)
		v.Cause = health.CauseInjury // its wounds keep it down; heal stands it up
	} else {
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(id, *v)
}

// UnderAnaesthetic reports a patient still under an operation's
// anaesthetic (any stage), or an older save's patient still out.
func (g *Game) UnderAnaesthetic(id entity.EntityID) bool {
	if b := g.bodies[id]; b != nil && b.Anesthetized() {
		return true
	}
	if g.surg == nil {
		return false
	}
	_, ok := g.surg.under[id]
	return ok
}

// onSurgeryBotched is the social hook of a failed operation (M3 fills it:
// the patient's opinion of the surgeon).
func (g *Game) onSurgeryBotched(surgeon, patient entity.EntityID) {
	g.socialBotchedSurgery(surgeon, patient) // social.go: the patient's resentment
}

// surgeryWait says what queued operation op on patient waits for: why it
// can't be started (opBlocked's reasons, ready false), or, once it can,
// the patient to lie down or the doctor on the way ("" and true when a
// doctor could start it this moment; SIM-049).
func (g *Game) surgeryWait(patient entity.EntityID, op body.Operation) (string, bool) {
	if g.surgeonComing(patient, opCode(op)) {
		return waitOnTheWay, true
	}
	lying := g.care.Doctor.Lying(patient)
	if why, _ := g.opBlocked(patient, 0, g.viewBody(patient), op, lying); why != "" {
		return why, false
	}
	if !lying {
		return waitLieDown, true
	}
	return "", true
}

// Vitals monitor (MED-06). A powered monitor next to a hospital bed
// helps the patient lying in it: tend quality +7 %, surgery ×105 %,
// immunity gain +2 %. wiki: Vitals monitor. One monitor counts per bed;
// the bed's monitor is cached until a building changes (structVer, as
// cabinetBonus in items_ng1.go), and power is checked live.
const (
	monitorTendOffset = 70 // wiki: Vitals monitor (+7 % medical tend quality)
	monitorImmunity   = 20 // wiki: Vitals monitor (+2 % immunity gain speed)
)

// monitorBy reports patient lying in a hospital bed with a powered vitals
// monitor beside it.
func (g *Game) monitorBy(patient entity.EntityID) bool {
	bid := g.bedUnder(patient)
	if bid == 0 {
		return false
	}
	m := g.bedMonitor(bid)
	return m != 0 && g.powered(m) // power.go
}

// monitorTend is the vitals monitor's tend quality offset for patient
// (permille; 0 without one).
func (g *Game) monitorTend(patient entity.EntityID) int32 {
	if g.monitorBy(patient) {
		return monitorTendOffset
	}
	return 0
}

// monitorImmunityGain is the vitals monitor's immunity gain offset for
// patient (permille; 0 without one).
func (g *Game) monitorImmunityGain(patient entity.EntityID) int32 {
	if g.monitorBy(patient) {
		return monitorImmunity
	}
	return 0
}
