package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
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
// anaesthetic until when, and who gave up waiting for a surgeon until when
// (ENG-003, M1); nil until the first operation.
type surgeryState struct {
	ops   map[entity.EntityID][]body.Operation
	under map[entity.EntityID]uint64
	rest  map[entity.EntityID]uint64
}

// surgerySave is surgeryState in the save, in id order.
type surgerySave struct {
	Ops   []entry[[]body.Operation]
	Under []entry[uint64]
	Rest  []entry[uint64]
}

func (s *surgeryState) ensure() {
	if s.ops == nil {
		s.ops = make(map[entity.EntityID][]body.Operation)
		s.under = make(map[entity.EntityID]uint64)
		s.rest = make(map[entity.EntityID]uint64)
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
	if g.surg == nil || len(g.surg.ops) == 0 && len(g.surg.under) == 0 && len(g.surg.rest) == 0 {
		return nil
	}
	return &surgerySave{Ops: exportValues(g.surg.ops), Under: exportValues(g.surg.under), Rest: exportValues(g.surg.rest)}
}

// importSurgery restores exportSurgery's state (nil: an older save).
func (g *Game) importSurgery(s *surgerySave) {
	g.surg = nil
	if s == nil {
		return
	}
	g.surg = &surgeryState{ops: importValues(s.Ops), under: importValues(s.Under), rest: importValues(s.Rest)}
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
	if !g.isPawn(patient) || op < 0 || op > int(body.Amputate) {
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

// readyOp is the first operation in id's queue a doctor could start now:
// valid on the body, with its implant (if any) in stock and free.
func (g *Game) readyOp(doctor, id entity.EntityID) (int, entity.EntityID, bool) {
	q := g.operationsOf(id)
	if len(q) == 0 {
		return 0, 0, false
	}
	b := g.viewBody(id)
	for i, op := range q {
		if !b.CanOperate(op) {
			continue
		}
		k := opNeedsItem(op)
		if k == 0 {
			return i, 0, true
		}
		if sid, _ := g.ingredientFor(doctor, recipe.Input{Kind: k, Qty: 1, Max: 1}); sid != 0 {
			return i, sid, true
		}
	}
	return 0, 0, false
}

// awaitingSurgery: id has an operation a doctor could do now, someone else
// to do it, and has not just given up waiting (patientRest, infection.go).
func (g *Game) awaitingSurgery(id entity.EntityID) bool {
	if g.surg == nil || len(g.surg.ops[id]) == 0 {
		return false
	}
	if owner, taken := g.reserve.EntityOwner(id); taken {
		if j, _ := g.jobs.Get(owner); j.Kind == job.Surgery && j.Target == id {
			return true // the surgeon is on the way (its implant is reserved)
		}
	}
	if uint64(g.clock.Now()) < g.surg.rest[id] || !g.otherDoctor(id) {
		return false
	}
	_, _, ok := g.readyOp(id, id)
	return ok
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
	removed := b.Implants[op.Part]
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
	case v.Status == health.Alive && (v.Injury >= g.downedAt(patient) || unconscious(b)):
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
	if g.surg == nil || len(g.surg.under) == 0 {
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
	if v.Injury > health.RecoverInjury || !g.consciousEnough(id) {
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
