// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/domain/pawn/trait"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/mind"
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
	"rimworld-go/pkg/mathx"
)

// MED-06: queued operations reserve materials and use the medical
// surgery chance formula; the body domain resolves outcomes and sedation.

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

// surgeryState stores operation queues, legacy sedation, cooldowns and
// temporarily claimed surgery beds.
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

// exportSurgery is the Surgery save field (nil: nothing queued or pending).
func (g *Game) exportSurgery(owned ...bool) *surgerySave {
	if g.surg == nil || len(g.surg.ops) == 0 && len(g.surg.under) == 0 && len(g.surg.rest) == 0 && len(g.surg.hosp) == 0 {
		return nil
	}
	return &surgerySave{Ops: saveutil.DetachEntries(len(owned) > 0 && owned[0], saveutil.ExportValues[entry[[]body.Operation]](g.surg.ops), slices.Clone[[]body.Operation]), Under: saveutil.ExportValues[entry[uint64]](g.surg.under), Rest: saveutil.ExportValues[entry[uint64]](g.surg.rest), Hosp: saveutil.ExportValues[entry[entity.EntityID]](g.surg.hosp)}
}

// importSurgery restores exportSurgery's state (nil: an older save).
func (g *Game) importSurgery(s *surgerySave) {
	g.surg = nil
	if s == nil {
		return
	}
	g.surg = &surgeryState{ops: saveutil.ImportValues(s.Ops), under: saveutil.ImportValues(s.Under), rest: saveutil.ImportValues(s.Rest), hosp: saveutil.ImportValues(s.Hosp)}
}

// operationsOf is patient id's queue (read only).
func (g *Game) operationsOf(id entity.EntityID) []body.Operation {
	if g.surg == nil {
		return nil
	}
	return g.surg.ops[id]
}

// cmdAddOperation queues a body-valid procedure for a living patient,
// avoiding duplicate or mutually exclusive requests.
func (g *Game) cmdAddOperation(patient entity.EntityID, op int, part, kind uint8) {
	if !g.isPawn(patient) || op < 0 || op >= int(body.NumOps) {
		return
	}
	if v, ok := g.vital.Get(patient); !ok || v.Status == health.Dead {
		return
	}
	o := body.Operation{Kind: body.OpKind(op), Part: body.Part(part)}
	switch {
	case o.Kind == body.InstallImplant || o.Kind == body.InstallOrgan || o.Kind == body.RemoveImplant && item.Kind(kind).IsAddedImplant():
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
		if body.OperationsConflict(x, o) {
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

// opFetch is what a doctor fetches for an operation: the implant or
// organ stack (0: none needed) and the medicine stack.
type opFetch struct {
	item, med entity.EntityID
}

// readyOp chooses the first bill with feasible stock, doctor and body.
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

// awaitingSurgery holds the patient in bed while a feasible operation
// awaits a doctor; temporary reservations cannot make the patient flap.
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

// opBlocked checks surgery preconditions, preferring raw stock but
// accepting legacy prefabricated parts from older saves.
func (g *Game) opBlocked(patient, doctor entity.EntityID, b *body.Body, op body.Operation, loose bool) (string, opFetch) {
	if !b.CanOperate(op) {
		return waitBody, opFetch{}
	}
	if op.Kind == body.Anesthetize && b.Sedated() {
		return waitSedated, opFetch{}
	}
	var f opFetch
	if k := body.OperationMaterial(op); k != 0 {
		var why string
		f.item, why = g.implantFor(patient, doctor, k, loose)
		if f.item == 0 {
			if legacy := body.LegacyMaterial(op); legacy != 0 {
				f.item, _ = g.implantFor(patient, doctor, legacy, loose)
			}
			if f.item == 0 {
				return why, opFetch{}
			}
		}
	} else if legacy := body.LegacyMaterial(op); legacy != 0 {
		f.item, _ = g.implantFor(patient, doctor, legacy, loose)
	}
	if doctor == 0 && !g.care.Doctor.OtherDoctor(patient) {
		return waitNoDoctor, opFetch{}
	}
	if lvl := body.OperationMinSkill(op); lvl > 0 && !g.skilledSurgeon(patient, doctor, lvl) {
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

// surgeryMedicine finds the nearest allowed, reachable and unreserved
// medicine stack meeting MedicineNeed, preferring cheaper tiers.
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
		if g.reserve.FreeFor(doctor, sid, st.Qty) < qty { // RES-1: doses other doctors claimed
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

// implantFor locates reachable stock with stable ties and reports
// stock, reach or research blockers.
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

// canFetch checks reachability for a designated or eligible doctor.
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

// implantResearch names the project locking every recipe for k.
func (g *Game) implantResearch(k item.Kind) string {
	locked := ""
	for _, r := range g.craft.Bills.Book().All() {
		if r.Output != k {
			continue
		}
		p := g.lab.LockedBy("recipe:" + r.Name) // game/research
		if p == "" {
			return ""
		}
		if locked == "" {
			locked = p
		}
	}
	return locked
}

// lieInHospitalBed temporarily claims a reachable free hospital bed.
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
	owned := construction.BedSlot(b, uint64(id)) >= 0
	if !owned {
		b = construction.ClaimBed(b, uint64(id))
		g.bldgs.Set(bid, b)
	}
	sx, sy := g.sleepCellFor(id, b)
	if !g.startJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		if !owned {
			g.bldgs.Set(bid, construction.ReleaseBed(b, uint64(id)))
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
	if b, ok := g.bldgs.Get(bid); ok && construction.BedSlot(b, uint64(id)) >= 0 {
		g.bldgs.Set(bid, construction.ReleaseBed(b, uint64(id)))
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

// surgeryPatients visits operable colonists, prisoners and guests in order.
func (g *Game) surgeryPatients(fn func(entity.EntityID) bool) {
	for _, pid := range g.pawnIDs {
		if !fn(pid) {
			return
		}
	}
	for _, pid := range g.raiderIDs {
		if g.prison.Is(pid) && !fn(pid) {
			return
		}
	}
	for _, pid := range g.visitorIDs() {
		if !fn(pid) {
			return
		}
	}
}

// assignSurgery chooses the nearest feasible patient for a doctor.
func (g *Game) assignSurgery(id entity.EntityID) bool {
	if g.surg == nil || len(g.surg.ops) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best job.Record
	var bestMed entity.EntityID
	bestD, bestDoses, found := 0, 0, false
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
		work := body.OperationWork(g.surg.ops[pid][slot])
		if work == 0 {
			work = SurgeryWork
		}
		rec := job.Record{Kind: job.Surgery, Target: pid, TX: sx, TY: sy, WorkLeft: work,
			Count: body.OpCode(g.surg.ops[pid][slot]), Route: []entity.EntityID{f.med}}
		fetch := f.med // the medicine, carried, unless there is an implant to carry
		if f.item != 0 {
			fetch = f.item
		}
		st, _ := g.stacks.Get(fetch)
		rec.Dest, rec.DestX, rec.DestY = fetch, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		best, bestMed, bestD, found = rec, f.med, d, true
		bestDoses, _ = body.MedicineNeed(g.surg.ops[pid][slot].Kind)
		return true
	})
	if !found || !g.startJob(id, best) {
		return false
	}
	if bestMed != best.Dest {
		g.reserve.TryEntity(id, best.Dest) // the implant
	}
	if st, ok := g.stacks.Get(bestMed); ok {
		g.reserve.TryCount(id, bestMed, max(1, bestDoses), st.Qty) // RES-1: only the doses it uses
	}
	return true
}

// surgerySlot finds the queued operation a job is for.
func (g *Game) surgerySlot(patient entity.EntityID, code int) (int, bool) {
	for i, op := range g.operationsOf(patient) {
		if body.OpCode(op) == code {
			return i, true
		}
	}
	return 0, false
}

// jobMedicine resolves the carried or reserved medicine stack.
func jobMedicine(j job.Record) entity.EntityID {
	if len(j.Route) == 0 {
		return 0
	}
	return j.Route[0]
}

// doSurgery fetches supplies, approaches the patient and performs the bill.
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
	requiresItem := g.surgeryHasMaterial(j, op)
	if j.Dest != 0 && j.Carry == 0 {
		accept := isMedicine
		if requiresItem {
			accept = func(k item.Kind) bool { return body.AcceptsMaterial(op, k) }
		}
		src := j.Dest
		walking := !g.pickUpItem(id, &j, accept) // infection.go
		if !requiresItem && j.Carry != 0 {
			qty, _ := body.MedicineNeed(op.Kind)
			g.topUpCarry(src, j.Carry, qty-1) // the rest of the doses
		}
		if walking {
			return // walking to the patient (or the job ended)
		}
	}
	if requiresItem && !g.carryingMaterial(id, j, op) {
		j.State, j.Fail = job.Fail, job.TargetGone // the implant went missing
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if pp, _ := g.positions.Get(patient); mathx.Abs(pp.X-pos.X) > 1 || mathx.Abs(pp.Y-pos.Y) > 1 || !g.care.Doctor.Lying(patient) {
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
	g.putStack(carry, c)
	if n == s.Qty {
		g.deleteItem(src)
		return
	}
	s.Qty -= n
	g.putStack(src, s)
}

// surgeryDoses validates either carried medicine or the reserved
// stack when the doctor carries a separate operation item.
func (g *Game) surgeryDoses(id entity.EntityID, j job.Record, op body.Operation) (item.Stack, bool) {
	qty, _ := body.MedicineNeed(op.Kind)
	if !g.surgeryHasMaterial(j, op) {
		st, ok := g.stacks.Get(j.Carry)
		return st, ok && j.Carry != 0 && st.Carrier == id && isMedicine(st.Kind) && st.Qty >= qty
	}
	sid := jobMedicine(j)
	st, ok := g.stacks.Get(sid)
	return st, ok && sid != 0 && st.Carrier == 0 && !st.Forbidden && isMedicine(st.Kind) && st.Qty >= qty
}

// useDoses consumes the validated medicine for an operation.
func (g *Game) useDoses(id entity.EntityID, j *job.Record, med item.Stack, op body.Operation) {
	qty, _ := body.MedicineNeed(op.Kind)
	if !g.surgeryHasMaterial(*j, op) {
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
	g.putStack(sid, med)
}

// carryingKind reports the job carrying an item of kind k.
// surgeryHasMaterial handles materials even after pickup clears Dest.
func (g *Game) surgeryHasMaterial(j job.Record, op body.Operation) bool {
	st, ok := g.stacks.Get(j.Carry)
	return body.OperationMaterial(op) != 0 ||
		ok && body.AcceptsMaterial(op, st.Kind) ||
		j.Carry == 0 && j.Dest != 0 && j.Dest != jobMedicine(j)
}

// carryingMaterial checks a raw resource or legacy prefab held by the doctor.
func (g *Game) carryingMaterial(id entity.EntityID, j job.Record, op body.Operation) bool {
	if j.Carry == 0 {
		return false
	}
	st, ok := g.stacks.Get(j.Carry)
	return ok && st.Carrier == id && body.AcceptsMaterial(op, st.Kind)
}

// surgeryInput captures the surgeon, patient and room factors.
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

// surgeryCleanliness reads the room cleanliness or identifies an outdoor operation.
func (g *Game) surgeryCleanliness(x, y int) (int32, bool) {
	r := g.roomAt(x, y)
	if r < 0 || r >= len(g.rooms) || g.thermo.OutdoorsForWork(r) {
		return 0, true
	}
	sum, _ := g.roomFilth(r)
	return room.SurgeryCleanliness(g.rooms[r].contents, g.rooms[r].tally, sum), false
}

// surgeryChanceOf is surgeon's permille chance of op on patient where it
// lies with medicine med, with its breakdown.
func (g *Game) surgeryChanceOf(surgeon, patient entity.EntityID, op body.OpKind, med item.Kind) (int32, []body.Factor) {
	chance, factors := body.SurgeryChance(g.surgeryInput(surgeon, patient, op, med))
	if g.mind.Inspire.Has(surgeon, mind.Surgery) {
		chance = min(chance*2, body.MaxSurgeryChance)
		factors = append(factors, body.Factor{Label: "Focused surgeon", Pct: 200})
	}
	return chance, factors
}

// bestSurgeon selects highest Medical skill, lowest ID on a tie.
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

// operationChance supplies the Health tab's percentage and factor breakdown.
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

// operate resolves a queued bill and its costs, outcome, consequences and sedation.
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
	g.mind.Inspire.Use(surgeon, mind.Surgery)
	lvl := int(g.skillOf(surgeon, skill.Medicine).Level)
	outcome := body.RollOutcome(chance, lvl, rng.IntN)
	drops := body.SurgeryYields(b, op)
	done := b.Apply(op, outcome, rng.IntN, now)
	if j.Carry != 0 && (outcome != body.MinorFail || op.Kind == body.InstallImplant) {
		g.deleteItem(j.Carry)
		j.Carry = 0
	}
	if done {
		for _, drop := range drops {
			g.dropStack(drop.Kind, drop.Qty, pp.X, pp.Y)
		}
	}
	if done && op.Kind == body.HarvestOrgan {
		g.dropStack(body.OrganOf(op.Part), 1, pp.X, pp.Y)
		g.organHarvested(surgeon, patient, op.Part)
		if b.VitalLost() {
			g.fatalOrganHarvest(patient)
		}
	}
	if done && op.Kind == body.InstallImplant && op.Item == item.ValveHeart &&
		len(drops) > 0 && drops[0].Kind == item.Heart && !slices.Contains(g.pawnIDs, patient) && !g.prison.Is(patient) {
		if p := g.personOf(patient); p != nil {
			if f := g.factionOf(p.Faction); f != nil {
				faction.Adjust(f, -70, fmt.Sprintf("Day %d: unnecessary surgery on %s", g.relDay(now)+1, g.name(patient))) // wiki: Prosthetic heart
			}
		}
	}
	failDeath := outcome != body.Success && body.FailDeathChance(op) > 0 &&
		int32(rng.IntN(1000)) < body.FailDeathChance(op)
	if v.Status != health.Dead && !failDeath {
		body.AnesthetizeFor(b, now, 45000+rng.IntN(30001)) // wiki: Anesthetic
	}
	g.bodyChanged(patient, b, &v)
	g.remember(patient, mood.Surgery) // rooms.go
	line := g.name(surgeon) + " " + op.DoneText(outcome) + " on " + g.name(patient)
	g.logEvent(patient, logEntry{Kind: evlog.Surgery, Name: line})
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
	adaptationLoss(g, patient, health.CauseAnesthetic, false)
	v.Status, v.Cause, v.DownedTick = health.Downed, health.CauseAnesthetic, now
	g.vital.Set(patient, v)
}

// euthanize: the surgeon puts patient down (MED-06). It cannot fail; the
// patient dies of euthanasia, its relatives holding it against the
// surgeon as for an execution (socialOnDeath).
func (g *Game) euthanize(surgeon, patient entity.EntityID, b *body.Body, v *health.Record, pp Position) {
	g.bodyChanged(patient, b, v)
	g.logEvent(patient, logEntry{Kind: evlog.Surgery, Name: g.name(surgeon) + " euthanized " + g.name(patient)})
	g.message(pp.X, pp.Y, "%s euthanized %s.", g.name(surgeon), g.name(patient))
	g.dieBy(patient, health.CauseEuthanasia, surgeon) // combat.go
	if g.prison.Is(patient) {
		g.prison.Drop(patient)
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
		if pid == donor || pid == surgeon {
			continue
		}
		if bio, ok := g.personBio(pid); ok && (slices.Contains(bio.Traits, trait.ColdHearted) || slices.Contains(bio.Traits, trait.Bloodthirsty) || (!colonistDonor && cannibal(g, pid))) {
			continue
		}
		if v, ok := g.vital.Get(pid); !ok || v.Status == health.Dead {
			continue
		}
		g.memories[pid] = thoughtsOf(g).AddFor(g.memories[pid], know, 0, now)
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

// stepAnesthetics wears every patient's anaesthetic off by an hour's
// worth (hourly, from the medical phase, in patients-index order; no
// scan) and stands up a patient it no longer holds down: up when its body
// allows (bleed.go standsUp), else left down from its wounds.
func (g *Game) stepAnesthetics(now uint64) {
	for _, id := range ownPatients(g) { // patients.go: this map's, a copy (a change touches the index)
		b := g.bodies[id]
		v, ok := g.vital.Get(id)
		if b == nil || !ok || v.Status == health.Dead {
			continue
		}
		if b.StepAnesthetic(now) {
			g.bodyChanged(id, b, &v)
		}
		if c, ok := b.Cond(body.CondAnesthetic); ok {
			r := g.stream(streamMedical)
			if !g.poisoned(id) {
				g.foodVomit(id, b)
			}
			if c.Stage < 2 {
				// wiki: Anesthetic; woozy confusion MTB 5 days,
				// wearing-off confusion MTB 50 days.
				wanderDays := 50
				if c.Stage == 1 {
					wanderDays = 5
				}
				if r.IntN(wanderDays*24) == 0 {
					confusedWander(g, id, now, "anesthetic")
				}
				if c.Stage == 1 && r.IntN(5*24) == 0 {
					g.memories[id] = mood.ForgetOne(g.memories[id], now, r.IntN)
				}
			}
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
	if g.surgeonComing(patient, body.OpCode(op)) {
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
// the tool-cabinet bonus in craft/cabinet.go), and power is checked live.
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

// fatalOrganHarvest records the distinct death consequence of a harvest.
func (g *Game) fatalOrganHarvest(donor entity.EntityID) {
	resident := slices.Contains(g.pawnIDs, donor)
	for _, id := range g.pawnIDs {
		if id == donor || !g.alive(id) {
			continue
		}
		if bio, ok := g.personBio(id); ok && (slices.Contains(bio.Traits, trait.ColdHearted) || (!resident && (slices.Contains(bio.Traits, trait.Bloodthirsty) || cannibal(g, id)))) {
			continue
		}
		g.remember(id, mood.FatalOrganHarvest)
	}
}
