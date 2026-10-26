package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Surgery (ENG-003, M1). The player queues operations on a patient from the
// Health tab (command.AddOperation). A doctor (Doctor work, after tending)
// takes the nearest patient lying down with an operation it can do now,
// fetches the implant (or a dose of medicine for other operations), and
// works SurgeryWork ticks scaled by Medicine. The body domain rolls the
// outcome on the body stream: success installs, removes or amputates;
// failure wounds the patient and calls onSurgeryBotched. Either way the
// patient remembers the surgery and lies under an anaesthetic for an hour.

const (
	// SurgeryWork is the work an operation takes (Medicine scales speed).
	SurgeryWork = 600
	// anaesthTicks is how long a patient stays down after an operation.
	anaesthTicks = clock.TicksPerHour
	// maxOperations caps one patient's queue.
	maxOperations = 8
	// surgeryRestRetry: a patient who waited patientWait in bed for a
	// surgeon gets up and does not lie down for surgery again this long.
	surgeryRestRetry = 6 * clock.TicksPerHour
)

// surgeryState is the queued operations per patient, who lies under an
// anaesthetic until when, who gave up waiting for a surgeon until when
// (ENG-003, M1), and the hospital bed a patient claimed only to wait for
// surgery in, given back once it is up again (SIM-049); nil until the
// first operation.
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

// cmdAddOperation queues operation op on part for patient: only a valid
// operation on the body as it is now (an implant on a missing part that
// fits it, a removal of an implant, an amputation of a part still there),
// once.
func (g *Game) cmdAddOperation(patient entity.EntityID, op int, part, kind uint8) {
	if !g.isPawn(patient) || op < 0 || op >= int(body.NumOps) { // CanOperate refuses the kinds MED-06 adds
		return
	}
	if v, ok := g.vital.Get(patient); !ok || v.Status == health.Dead {
		return
	}
	o := body.Operation{Kind: body.OpKind(op), Part: body.Part(part)}
	if o.Kind == body.InstallImplant {
		o.Item = item.Kind(kind)
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

// opNeedsItem is the item an operation uses up (0: none).
func opNeedsItem(op body.Operation) item.Kind {
	if op.Kind == body.InstallImplant {
		return op.Item
	}
	return 0
}

// readyOp is the first operation in id's queue doctor could start now:
// valid on the body, with its implant (if any) in stock, free and within
// the doctor's reach (opBlocked).
func (g *Game) readyOp(doctor, id entity.EntityID) (int, entity.EntityID, bool) {
	q := g.operationsOf(id)
	if len(q) == 0 {
		return 0, 0, false
	}
	b := g.viewBody(id)
	for i, op := range q {
		if why, sid := g.opBlocked(id, doctor, b, op, false); why == "" {
			return i, sid, true
		}
	}
	return 0, 0, false
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
	if uint64(g.clock.Now()) < g.surg.rest[id] || !g.otherDoctor(id) {
		return false
	}
	b, lying := g.viewBody(id), g.lyingForCare(id)
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
// picker's disabled options; SIM-049).
const (
	waitBody      = "can't be done on the body as it is"
	waitResearch  = "needs research: "
	waitNoDoctor  = "needs another colonist who can doctor"
	waitLieDown   = "waiting for the patient to lie down"
	waitOnTheWay  = "a doctor is on the way"
	waitNoStockFm = "no %s in stock"
	waitReachFm   = "the %s can't be reached"
)

// opBlocked is why doctor can't start op on patient (body b) now, ""
// with the implant stack to fetch (0: none needed) when it can. Doctor 0
// asks about any other colonist who doctors. The reasons come in order:
// the operation no longer fits the body, the implant needs research, none
// in stock (free: unforbidden, uncarried, unreserved), none in reach, no
// one else to operate. Loose counts a stack a hauler holds as in stock
// (a waiting patient stays down; awaitingSurgery).
func (g *Game) opBlocked(patient, doctor entity.EntityID, b *body.Body, op body.Operation, loose bool) (string, entity.EntityID) {
	if !b.CanOperate(op) {
		return waitBody, 0
	}
	var sid entity.EntityID
	if k := opNeedsItem(op); k != 0 {
		var why string
		if sid, why = g.implantFor(patient, doctor, k, loose); sid == 0 {
			return why, 0
		}
	}
	if doctor == 0 && !g.otherDoctor(patient) {
		return waitNoDoctor, 0
	}
	return "", sid
}

// implantFor is the nearest free stack of implant k that doctor (0: any
// other colonist who doctors) can reach, or why there is none: its recipe
// still needs research and there is none to be had, none is in stock, or
// none can be reached (reachable: region labels, no search). Loose also
// takes a stack carried or reserved by someone else.
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
// bed (freeBedFor ranks them first), claimed only for the stay: its own
// bed stays its own, and leaveHospitalBed gives this one back once it is
// up again. False when there is none free (or it can't be reached), and
// nothing changes then.
func (g *Game) lieInHospitalBed(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	bid, ok := g.freeBedFor(id, pos) // medical.go
	if !ok {
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

// assignSurgery sends doctor id to the nearest patient lying down (in bed,
// downed or asleep) with an operation it can do now.
func (g *Game) assignSurgery(id entity.EntityID) bool {
	if g.surg == nil || len(g.surg.ops) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best job.Record
	bestD, found := 0, false
	for _, pid := range g.pawnIDs {
		if pid == id || len(g.surg.ops[pid]) == 0 || g.carriedBy(pid) != 0 {
			continue
		}
		if v, _ := g.vital.Get(pid); v.Status == health.Dead || !g.lyingForCare(pid) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(pid); taken && owner != id {
			continue
		}
		slot, sid, ok := g.readyOp(id, pid)
		if !ok {
			continue
		}
		pp, _ := g.positions.Get(pid)
		sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
		if !ok {
			continue
		}
		d := manhattan(pos.X, pos.Y, pp.X, pp.Y)
		if found && d >= bestD {
			continue
		}
		rec := job.Record{Kind: job.Surgery, Target: pid, TX: sx, TY: sy, WorkLeft: SurgeryWork, Count: opCode(g.surg.ops[pid][slot])}
		if sid != 0 {
			st, _ := g.stacks.Get(sid)
			rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
			rec.TX, rec.TY = st.X, st.Y
		}
		best, bestD, found = rec, d, true
	}
	if !found {
		return false
	}
	if best.Dest == 0 {
		best = g.withMedicine(id, best) // infection.go: a dose for the operation, if there is one
	}
	if !g.startJob(id, best) {
		return false
	}
	if best.Dest != 0 {
		g.reserve.TryEntity(id, best.Dest)
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
	if j.Dest != 0 && j.Carry == 0 {
		need := opNeedsItem(op)
		accept := isMedicine
		if need != 0 {
			accept = func(k item.Kind) bool { return k == need }
		}
		if !g.pickUpItem(id, &j, accept) { // infection.go
			return // walking to the patient (or the job ended)
		}
	}
	if need := opNeedsItem(op); need != 0 && !g.carryingKind(id, j, need) {
		j.State, j.Fail = job.Fail, job.TargetGone // the implant went missing
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if pp, _ := g.positions.Get(patient); abs(pp.X-pos.X) > 1 || abs(pp.Y-pos.Y) > 1 || !g.lyingForCare(patient) {
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
	g.operate(id, patient, slot, &j)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// carryingKind reports the job carrying an item of kind k.
func (g *Game) carryingKind(id entity.EntityID, j job.Record, k item.Kind) bool {
	if j.Carry == 0 {
		return false
	}
	st, ok := g.stacks.Get(j.Carry)
	return ok && st.Kind == k && st.Carrier == id
}

// surgeryChanceOf is surgeon's permille chance on patient in its current
// bed with medicine med (an implant brings its own fittings: neutral).
func (g *Game) surgeryChanceOf(surgeon, patient entity.EntityID, med item.Kind) int32 {
	manip, sight := int32(1000), int32(1000)
	if b := g.bodies[surgeon]; b != nil {
		manip, sight = b.Capacity(body.Manipulation), b.Capacity(body.Sight)
	}
	return body.SurgeryChance(int(g.skillOf(surgeon, skill.Medicine).Level), g.inHospitalBed(patient), med, manip, sight)
}

// operate carries out slot's operation on patient: roll, apply, use up the
// carried item, wound or kill on failure, then anaesthetic and memory.
func (g *Game) operate(surgeon, patient entity.EntityID, slot int, j *job.Record) {
	now := uint64(g.clock.Now())
	op := g.surg.ops[patient][slot]
	g.dropOperation(patient, slot)
	v, _ := g.vital.Get(patient)
	b := g.ensureBody(patient, &v) // body.go
	if b == nil || !b.CanOperate(op) {
		return
	}
	med := item.HerbalMedicine // an implant's own fittings: the neutral factor
	if opNeedsItem(op) == 0 {
		med = 0
		if st, ok := g.stacks.Get(j.Carry); ok && j.Carry != 0 && isMedicine(st.Kind) {
			med = st.Kind
		}
	}
	rng := g.stream(streamBody)
	outcome := body.RollOutcome(g.surgeryChanceOf(surgeon, patient, med), rng.IntN)
	removed := b.Implant(op.Part)
	done := b.Apply(op, outcome, rng.IntN, now)
	if j.Carry != 0 && (outcome != body.MinorFail || opNeedsItem(op) == 0) {
		g.deleteItem(j.Carry) // medicine is spent; an implant is fitted or ruined
		j.Carry = 0
	}
	pp, _ := g.positions.Get(patient)
	if done && op.Kind == body.RemoveImplant && removed.IsImplant() {
		g.dropStack(removed, 1, pp.X, pp.Y) // world.go: the implant comes out whole
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
	case v.Injury >= health.MaxHP || b.VitalLost():
		g.message(pp.X, pp.Y, "%s died on the operating table.", g.name(patient))
		g.dieBy(patient, health.CauseSurgery, 0)
		return
	case v.Status == health.Alive && g.downsNow(patient, v, b): // bleed.go (MED-01)
		g.down(patient, health.CauseInjury)
		return
	}
	g.surg.ensure()
	g.surg.under[patient] = now + anaesthTicks
	if v.Status == health.Alive {
		g.interrupt(patient, job.Interrupted)
		v, _ = g.vital.Get(patient)
		v.Status, v.Cause, v.DownedTick = health.Downed, health.CauseSurgery, now
		g.vital.Set(patient, v)
	}
}

// operationDone is the log wording of an operation's outcome.
func operationDone(op body.Operation, o body.Outcome) string {
	part := op.Part.Label()
	switch o {
	case body.MinorFail:
		return "slipped during an operation on the " + part
	case body.Catastrophic:
		return "badly botched an operation on the " + part
	}
	switch op.Kind {
	case body.InstallImplant:
		return "fitted a " + op.Item.String() + " to the " + part
	case body.RemoveImplant:
		return "removed the implant from the " + part
	}
	return "amputated the " + part
}

// wakeFromAnaesthetic stands a patient back up an hour after surgery (or
// leaves it down from its wounds if they are bad enough).
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

// UnderAnaesthetic reports a patient still out after an operation.
func (g *Game) UnderAnaesthetic(id entity.EntityID) bool {
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
	lying := g.lyingForCare(patient)
	if why, _ := g.opBlocked(patient, 0, g.viewBody(patient), op, lying); why != "" {
		return why, false
	}
	if !lying {
		return waitLieDown, true
	}
	return "", true
}
