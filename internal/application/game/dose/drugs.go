package dose

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Drugs: the states hold each colonist's drug state and drug policy, nil
// until the first dose, policy change or failed search, so a colony that
// never meets a drug saves and hashes as before. The state decays hourly
// (Tick, staggered by id); a colonist takes a dose when its addiction craves
// one (the domain's 30%/70% need band) or its policy allows one for fun,
// through a job.Ingest at the nearest allowed stack.
//
// Medical drugs (MED-07). Every policy but "no drugs" has a colonist take
// numbroot tincture when its pain reaches drug.PainkillerAt and no dose
// still acts, and a prophyl tablet every five days (drug.AllowsMedical);
// it looks for one before a recreational dose. Numbroot halves the pain
// for 12 h (body.CondPainkiller); prophyl keeps plague, malaria and
// sleeping sickness away while it acts (Prophylaxed). A hard drug (the
// spark tablet) or prophyl adds overdose (body.CondOverdose), rolled on the
// medical stream; the hourly tick wears it and the painkiller off, vomits
// at the major stage and may kill at the extreme one.
type Drugs struct {
	h       Host
	states  *map[entity.EntityID]*drug.State
	recheck uint64 // how often an idling colonist looks for work (ticks)
}

// New is the component over the game's states; recheck is the idle recheck
// its searches stagger on.
func New(h Host, states *map[entity.EntityID]*drug.State, recheck uint64) Drugs {
	return Drugs{h, states, recheck}
}

const (
	// ingestTicks is how long drinking or swallowing a dose takes.
	ingestTicks = 120
	// retryNone: no allowed drug in reach, so no new search until then
	// (as noFood; it keeps a colonist from retrying every idle recheck).
	retryNone = 2 * clock.TicksPerHour
	// retryBusyRechecks: the stack was taken or the path search deferred,
	// in idle rechecks.
	retryBusyRechecks = 4
	// lookEvery: a colonist looks for a recreational dose on one idle
	// recheck in lookEvery (staggered by id); a craving looks every time.
	lookEvery = 8
	// TickStagger spreads the hourly drug ticks over the hour by id.
	TickStagger = 37
)

// prophylBlocks lists the diseases an acting prophyl tablet keeps away.
// wiki: Penoxycyline.
var prophylBlocks = [...]body.CondKind{body.CondPlague, body.CondMalaria, body.CondSleepingSickness}

// State is id's drug state, created with the default policy.
func (d Drugs) State(id entity.EntityID) *drug.State {
	if s := (*d.states)[id]; s != nil {
		return s
	}
	if *d.states == nil {
		*d.states = make(map[entity.EntityID]*drug.State)
	}
	s := &drug.State{Policy: drug.SocialDrinking}
	(*d.states)[id] = s
	return s
}

// Policy is id's drug policy (social drinking until set).
func (d Drugs) Policy(id entity.EntityID) drug.Preset {
	if s := (*d.states)[id]; s != nil {
		return s.Policy
	}
	return drug.SocialDrinking
}

// SetPolicy sets id's drug policy to preset p. Setting "no drugs" stops a
// dose on its way.
func (d Drugs) SetPolicy(id entity.EntityID, p uint8) {
	pr := drug.Preset(p)
	if pr > drug.MoodLift || !d.h.Pawns.Colonist(id) {
		return
	}
	if d.Policy(id) == pr {
		return // no change (and no state for a default)
	}
	d.State(id).Policy = pr
	if j, ok := d.h.Pawns.Job(id); ok && pr == drug.NoDrugs && j.Kind == job.Ingest && j.Active() {
		d.h.Pawns.Interrupt(id, job.Cancelled)
	}
}

// Tick advances id's drug state once an hour, staggered by id: alcohol and
// highs wear off, tolerance and the addiction need fall, and a hangover
// that began leaves its memory.
func (d Drugs) Tick(id entity.EntityID, now uint64) {
	s := (*d.states)[id]
	if s == nil || (now+uint64(id)*TickStagger)%clock.TicksPerHour != 0 {
		return
	}
	s.Tick(1, now)
	if s.ConsumeHangover() {
		d.h.Minds.Remember(id, mood.Hungover)
	}
	d.stepBody(id, now)
}

// stepBody wears an hour of id's painkiller and overdose off (from Tick): at
// the extreme stage death strikes with a mean of three days, at the major
// stage the patient vomits now and then (both rolled on the medical stream);
// the stats and the downing rule follow a change.
func (d Drugs) stepBody(id entity.EntityID, now uint64) {
	bs := d.h.Bodies
	b := bs.Body(id)
	if b == nil || !b.Numbed() && b.Overdose() == 0 {
		return
	}
	v, ok := bs.Vital(id)
	if !ok || v.Status == health.Dead {
		return
	}
	changed := b.StepPainkiller(now)
	if c, ok := b.Cond(body.CondOverdose); ok {
		stage, since := c.Stage, c.Since
		if stage >= body.OverdoseStageExtreme && bs.MedicalRoll(body.OverdoseDeathMTBHours) == 0 {
			bs.DieOfOverdose(id)
			return
		}
		if stage >= body.OverdoseStageMajor && bs.MedicalRoll(body.OverdoseVomitMTBTenths) < 10 {
			bs.Vomit(d.h.Pawns.Pos(id))
		}
		var hour uint64
		if now > since {
			hour = (now - since) / clock.TicksPerHour
		}
		changed = b.StepOverdose(hour) || changed
	}
	if changed {
		bs.Settle(id, b, v)
	}
}

// onBody puts a dose of drug def on id's body at tick now: numbroot numbs
// it; a hard drug or prophyl adds its overdose, rolled on the medical
// stream, and a lethal sum kills. It reports whether id died.
func (d Drugs) onBody(id entity.EntityID, def drug.Def, now uint64) bool {
	if !def.Painkiller && def.OverdoseMax <= 0 {
		return false
	}
	bs := d.h.Bodies
	v, ok := bs.Vital(id)
	if !ok || v.Status == health.Dead {
		return false
	}
	b := bs.EnsureBody(id, v)
	if b == nil {
		return false
	}
	if def.Painkiller {
		b.Numb(now)
	}
	if def.OverdoseMax > 0 {
		sev := def.OverdoseMin + int32(bs.MedicalRoll(int(def.OverdoseMax-def.OverdoseMin)+1))
		if b.AddOverdose(sev, now) >= body.OverdoseDeath {
			bs.DieOfOverdose(id)
			return true
		}
	}
	bs.Settle(id, b, v)
	return false
}

// Overdose is id's overdose for the pawn view: severity in percent and stage
// (0, 0: none).
func (d Drugs) Overdose(id entity.EntityID) (int, int) {
	b := d.h.Bodies.Body(id)
	if b == nil {
		return 0, 0
	}
	return int(b.Overdose()) / 10, int(b.OverdoseStage())
}

// Prophylaxed reports colonist id kept from disease k by an acting prophyl
// tablet.
func (d Drugs) Prophylaxed(id entity.EntityID, k body.CondKind) bool {
	s := (*d.states)[id]
	if s == nil {
		return false
	}
	for _, b := range prophylBlocks {
		if b == k {
			return s.Protected(d.h.Pawns.Now())
		}
	}
	return false
}

// medical are the medical drugs id's policy has it take now: the painkiller
// while in pain (looked for on the recreational stagger), a scheduled drug
// once due (looked for in one idle recheck an hour, staggered by id, so a
// colony without any never scans more often).
func (d Drugs) medical(id entity.EntityID, probe *drug.State, now uint64) []item.Kind {
	if probe.Policy == drug.NoDrugs {
		return nil
	}
	var pain int32
	numbed := false
	if b := d.h.Bodies.Body(id); b != nil {
		pain, numbed = b.Pain(), b.Numbed()
	}
	lookNow := d.lookNow(id, now)
	hourly := (now+uint64(id)*TickStagger)%clock.TicksPerHour < d.recheck
	var out []item.Kind
	for _, m := range drug.Medical() {
		if m.Painkiller && !lookNow || m.ScheduleDays > 0 && !hourly {
			continue
		}
		if probe.Policy.AllowsMedical(m.Kind, pain, numbed, probe, now) {
			out = append(out, m.Kind)
		}
	}
	return out
}

// lookNow: this idle recheck is id's turn to look for a dose.
func (d Drugs) lookNow(id entity.EntityID, now uint64) bool {
	return ((now+uint64(id))/d.recheck+uint64(id))%lookEvery == 0
}

// Assign sends colonist id to take a drug its policy allows or its addiction
// craves; called before idle recreation in phaseAI. A craving seeks only the
// craved drug; a recreational dose needs low recreation (or a hedonist), the
// policy, the traits and the dose gap to agree.
func (d Drugs) Assign(id entity.EntityID) bool {
	if !d.h.Pawns.Colonist(id) {
		return false
	}
	now := d.h.Pawns.Now()
	s := (*d.states)[id]
	if s != nil && now < s.Retry {
		return false
	}
	var crave item.Kind
	craving := false
	if s != nil {
		crave, craving = s.Craving()
	}
	probe := s
	if probe == nil {
		probe = &drug.State{Policy: drug.SocialDrinking}
	}
	// MED-07: a medical dose first; none in reach sets no retry, so the
	// recreational look below is as before.
	if !craving {
		if med := d.medical(id, probe, now); len(med) > 0 {
			if d.fromPack(id, med, now) { // CSE-011: what it carries first
				return true
			}
			if best, bx, by := d.nearest(id, med); best != 0 {
				return d.start(id, best, bx, by, now)
			}
		}
	}
	if !craving && !d.lookNow(id, now) {
		return false
	}
	allowed := d.kinds(id, probe, d.h.Minds.Joy(id) < recreation.Bored, crave, craving, now)
	if len(allowed) == 0 {
		return false
	}
	if d.fromPack(id, allowed, now) { // CSE-011: what it carries first
		return true
	}
	best, bx, by := d.nearest(id, allowed)
	if best == 0 {
		if s != nil {
			s.Retry = now + retryNone
		}
		return false
	}
	return d.start(id, best, bx, by, now)
}

// kinds lists the recreational drugs id's policy lets it take now: a craving
// only the craved one, and past the policy's mood gate only that (CSE-010).
func (d Drugs) kinds(id entity.EntityID, probe *drug.State, joyLow bool, crave item.Kind, craving bool, now uint64) []item.Kind {
	if !craving && !d.moodAllows(id, probe.Policy) {
		return nil
	}
	traits := d.h.Minds.Traits(id)
	var allowed []item.Kind
	for _, def := range drug.All() {
		if craving && def.Kind != crave {
			continue
		}
		if probe.Policy.Allows(def.Kind, joyLow, probe, traits, now) {
			allowed = append(allowed, def.Kind)
		}
	}
	return allowed
}

// start sends id to take a dose from stack best at (bx, by); a failed start
// (taken, or the path search deferred) holds off the next search.
func (d Drugs) start(id, best entity.EntityID, bx, by int, now uint64) bool {
	if !d.h.Pawns.StartJob(id, job.Record{Kind: job.Ingest, Target: best, TX: bx, TY: by, WorkLeft: ingestTicks}) {
		wait := uint64(retryNone)
		if j, _ := d.h.Pawns.Job(id); j.Fail == job.Reserved {
			wait = retryBusyRechecks * d.recheck // taken, or the path search was deferred
		}
		d.State(id).Retry = now + wait
		return false
	}
	return true
}

// startHeld is start for the dose id already holds as stack.
func (d Drugs) startHeld(id, stack entity.EntityID) bool {
	x, y := d.h.Pawns.Pos(id)
	return d.h.Pawns.StartJob(id, job.Record{Kind: job.Ingest, Target: stack, Carry: stack, TX: x, TY: y, WorkLeft: ingestTicks})
}

// nearest is the nearest free, unforbidden stack of one of kinds.
func (d Drugs) nearest(id entity.EntityID, kinds []item.Kind) (entity.EntityID, int, int) {
	px, py := d.h.Pawns.Pos(id)
	var best entity.EntityID
	bestD, bx, by := 1<<30, 0, 0
	for _, iid := range d.h.Stock.ItemIDs() {
		st, ok := d.h.Stock.Stack(iid)
		if !ok || !st.Kind.IsDrug() || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden || !kindIn(kinds, st.Kind) {
			continue
		}
		if owner, taken := d.h.Stock.EntityOwner(iid); taken && owner != id {
			continue
		}
		dist := pathing.Dist(px, py, st.X, st.Y)
		if dist < bestD || dist == bestD && iid < best {
			best, bestD, bx, by = iid, dist, st.X, st.Y
		}
	}
	return best, bx, by
}

func kindIn(ks []item.Kind, k item.Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// Do runs a drug-taking job: at the stack, it takes one dose.
func (d Drugs) Do(id entity.EntityID, j job.Record) {
	j.WorkLeft--
	if j.WorkLeft > 0 {
		d.h.Pawns.SetJob(id, j)
		return
	}
	st, ok := d.h.Stock.Stack(j.Target)
	if !ok || st.Qty <= 0 || !st.Kind.IsDrug() || st.Carrier != 0 && st.Carrier != id {
		j.State, j.Fail = job.Fail, job.TargetGone
		d.h.Pawns.SetJob(id, j)
		return
	}
	k := st.Kind
	st.Qty--
	if st.Qty <= 0 {
		d.h.Stock.DeleteItem(j.Target)
	} else {
		d.h.Stock.PutStack(j.Target, st)
	}
	if j.Carry != 0 { // CSE-011: the dose in hand, out of the pack
		j.Carry = 0
		d.h.Pawns.SetJob(id, j)
		d.h.Habits.Used(id)
	}
	if d.Ingest(id, k) {
		return // died of an overdose: die cancelled the job
	}
	j.State = job.Complete
	d.h.Pawns.SetJob(id, j)
}

// Ingest applies one dose of drug k to id: the domain effects (drawn from the
// drug stream), the recreation it gives, a stimulant's rest, the log line and
// the body's share (onBody, MED-07). It reports whether the dose killed id
// (an overdose).
func (d Drugs) Ingest(id entity.EntityID, k item.Kind) bool {
	def, ok := drug.DefOf(k)
	if !ok {
		return false
	}
	now := d.h.Pawns.Now()
	d.State(id).Ingest(k, d.h.Minds.DrugRolls(), now)
	d.h.Minds.SetJoy(id, recreation.Clamp(d.h.Minds.Joy(id)+int(def.Joy)*recreation.Full/1000))
	if def.RestBoost > 0 {
		if n, ok := d.h.Minds.Needs(id); ok {
			n.Rest = min(n.Rest+int(def.RestBoost)*needs.Full/1000, needs.Full)
			d.h.Minds.SetNeeds(id, n)
		}
	}
	d.h.Minds.LogIngested(id, k.String())
	return d.onBody(id, def, now)
}

// Line is one drug effect, tolerance or addiction line.
type Line struct {
	Label string
	Pct   int
}

// Lines lists id's drug effects for the Health tab.
func (d Drugs) Lines(id entity.EntityID) []Line {
	s := (*d.states)[id]
	if s == nil {
		return nil
	}
	var out []Line
	for _, l := range s.Lines() {
		out = append(out, Line{Label: l.Label, Pct: int(l.Percent)})
	}
	if b := d.h.Bodies.Body(id); b != nil { // MED-07: an acting numbroot dose
		if c, ok := b.Cond(body.CondPainkiller); ok {
			out = append(out, Line{Label: "numbroot: pain halved", Pct: int(c.Sev) / 10})
		}
	}
	return out
}

// Thoughts are id's drug state thoughts (tipsy, high, withdrawal).
func (d Drugs) Thoughts(id entity.EntityID) []mood.Thought {
	s := (*d.states)[id]
	if s == nil {
		return nil
	}
	ks := s.Thoughts()
	if len(ks) == 0 {
		return nil
	}
	return mood.Active(nil, ks...)
}
