package game

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
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Drugs (ENG-003, M2): g.drugs holds each colonist's drug state and drug
// policy; nil until the first dose, policy change or failed search, so a
// colony that never meets a drug saves and hashes as before. Saved as
// Drugs. The state decays hourly (drugTick, staggered by id); a colonist
// takes a dose when its addiction craves one (the domain's 30%/70% need
// band) or its policy allows one for fun, through a job.Ingest at the
// nearest allowed stack.
//
// Medical drugs (MED-07). Every policy but "no drugs" has a colonist take
// numbroot tincture when its pain reaches drug.PainkillerAt and no dose
// still acts, and a prophyl tablet every five days (drug.AllowsMedical);
// it looks for one before a recreational dose. Numbroot halves the pain
// for 12 h (body.CondPainkiller); prophyl keeps plague, malaria and
// sleeping sickness away while it acts (diseaseCandidates). A hard drug
// (the spark tablet) or prophyl adds overdose (body.CondOverdose), rolled
// on the medical stream; the hourly drug tick wears it and the painkiller
// off, vomits at the major stage and may kill at the extreme one.

const (
	// drugIngestTicks is how long drinking or swallowing a dose takes.
	drugIngestTicks = 120
	// drugRetry: no allowed drug in reach, so no new search until then
	// (as noFood; it keeps a colonist from retrying every idle recheck).
	drugRetry = 2 * clock.TicksPerHour
	// drugRetryBusy: the stack was taken or the path search deferred.
	drugRetryBusy = 4 * idleRecheck
	// drugLookEvery: a colonist looks for a recreational dose on one idle
	// recheck in drugLookEvery (staggered by id); a craving looks every time.
	drugLookEvery = 8
	// drugTickStagger spreads the hourly drug ticks over the hour by id.
	drugTickStagger = 37
)

// prophylBlocks lists the diseases an acting prophyl tablet keeps away.
// wiki: Penoxycyline.
var prophylBlocks = [...]body.CondKind{body.CondPlague, body.CondMalaria, body.CondSleepingSickness}

// drugState is id's drug state, created with the default policy.
func (g *Game) drugState(id entity.EntityID) *drug.State {
	if s := g.drugs[id]; s != nil {
		return s
	}
	if g.drugs == nil {
		g.drugs = make(map[entity.EntityID]*drug.State)
	}
	s := &drug.State{Policy: drug.SocialDrinking}
	g.drugs[id] = s
	return s
}

// drugPolicy is id's drug policy (social drinking until set).
func (g *Game) drugPolicy(id entity.EntityID) drug.Preset {
	if s := g.drugs[id]; s != nil {
		return s.Policy
	}
	return drug.SocialDrinking
}

// DrugPolicy is colonist id's drug policy, a drug.Preset (the Assign tab's
// read model).
func (g *Game) DrugPolicy(id entity.EntityID) uint8 { return uint8(g.drugPolicy(id)) }

// cmdSetDrugPolicy sets id's drug policy to preset p. Setting "no drugs"
// stops a dose on its way.
func (g *Game) cmdSetDrugPolicy(id entity.EntityID, p uint8) {
	pr := drug.Preset(p)
	if pr > drug.Unrestricted || !g.colonist(id) {
		return
	}
	if g.drugPolicy(id) == pr {
		return // no change (and no state for a default)
	}
	g.drugState(id).Policy = pr
	if j, ok := g.jobs.Get(id); ok && pr == drug.NoDrugs && j.Kind == job.Ingest && j.Active() {
		g.interrupt(id, job.Cancelled)
	}
}

// drugTick advances id's drug state once an hour, staggered by id: alcohol
// and highs wear off, tolerance and the addiction need fall, and a
// hangover that began leaves its memory.
func (g *Game) drugTick(id entity.EntityID, now uint64) {
	s := g.drugs[id]
	if s == nil || (now+uint64(id)*drugTickStagger)%clock.TicksPerHour != 0 {
		return
	}
	s.Tick(1, now)
	if s.ConsumeHangover() {
		g.remember(id, mood.Hungover)
	}
	g.stepDrugBody(id, now)
}

// stepDrugBody wears an hour of id's painkiller and overdose off (from
// drugTick): at the extreme stage death strikes with a mean of three days,
// at the major stage the patient vomits now and then (both rolled on the
// medical stream); the stats and the downing rule follow a change.
func (g *Game) stepDrugBody(id entity.EntityID, now uint64) {
	b := g.bodies[id]
	if b == nil || !b.Numbed() && b.Overdose() == 0 {
		return
	}
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return
	}
	changed := b.StepPainkiller(now)
	if c, ok := b.Cond(body.CondOverdose); ok {
		stage, since := c.Stage, c.Since
		pos, _ := g.positions.Get(id)
		if stage >= body.OverdoseStageExtreme && g.stream(streamMedical).IntN(body.OverdoseDeathMTBHours) == 0 {
			g.message(pos.X, pos.Y, "%s died of an overdose.", g.name(id))
			g.dieMedical(id, health.CauseOverdose) // caravan_medical.go
			g.clearPatient(id)
			return
		}
		if stage >= body.OverdoseStageMajor && g.stream(streamMedical).IntN(body.OverdoseVomitMTBTenths) < 10 {
			g.addDirt(pos.Y*g.grid.Width()+pos.X, vomitDirt) // filth.go: as a disease's vomit
		}
		var hour uint64
		if now > since {
			hour = (now - since) / clock.TicksPerHour
		}
		changed = b.StepOverdose(hour) || changed
	}
	if changed {
		g.bodyChanged(id, b, &v) // body.go: stats follow consciousness
		g.updateDowned(id, b, &v)
	}
}

// drugBody puts dose of drug d on id's body at tick now: numbroot numbs
// it; a hard drug or prophyl adds its overdose, rolled on the medical
// stream, and a lethal sum kills. It reports whether id died.
func (g *Game) drugBody(id entity.EntityID, d drug.Def, now uint64) bool {
	if !d.Painkiller && d.OverdoseMax <= 0 {
		return false
	}
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return false
	}
	b := g.ensureBody(id, &v)
	if b == nil {
		return false
	}
	if d.Painkiller {
		b.Numb(now)
	}
	if d.OverdoseMax > 0 {
		sev := d.OverdoseMin + int32(g.stream(streamMedical).IntN(int(d.OverdoseMax-d.OverdoseMin)+1))
		if b.AddOverdose(sev, now) >= body.OverdoseDeath {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s died of an overdose.", g.name(id))
			g.dieMedical(id, health.CauseOverdose) // caravan_medical.go
			g.clearPatient(id)
			return true
		}
	}
	g.bodyChanged(id, b, &v)
	g.updateDowned(id, b, &v)
	return false
}

// overdoseView is id's overdose for the pawn view: severity in percent
// and stage (0, 0: none).
func (g *Game) overdoseView(id entity.EntityID) (int, int) {
	b := g.bodies[id]
	if b == nil {
		return 0, 0
	}
	return int(b.Overdose()) / 10, int(b.OverdoseStage())
}

// prophylaxed reports colonist id kept from disease k by an acting
// prophyl tablet (diseaseCandidates).
func (g *Game) prophylaxed(id entity.EntityID, k body.CondKind) bool {
	s := g.drugs[id]
	if s == nil {
		return false
	}
	for _, b := range prophylBlocks {
		if b == k {
			return s.Protected(uint64(g.clock.Now()))
		}
	}
	return false
}

// medicalDrugs are the medical drugs id's policy has it take now: the
// painkiller while in pain (looked for on the recreational stagger), a
// scheduled drug once due (looked for in one idle recheck an hour,
// staggered by id, so a colony without any never scans more often).
func (g *Game) medicalDrugs(id entity.EntityID, probe *drug.State, now uint64) []item.Kind {
	if probe.Policy == drug.NoDrugs {
		return nil
	}
	var pain int32
	numbed := false
	if b := g.bodies[id]; b != nil {
		pain, numbed = b.Pain(), b.Numbed()
	}
	lookNow := ((now+uint64(id))/idleRecheck+uint64(id))%drugLookEvery == 0
	hourly := (now+uint64(id)*drugTickStagger)%clock.TicksPerHour < idleRecheck
	var out []item.Kind
	for _, d := range drug.Medical() {
		if d.Painkiller && !lookNow || d.ScheduleDays > 0 && !hourly {
			continue
		}
		if probe.Policy.AllowsMedical(d.Kind, pain, numbed, probe, now) {
			out = append(out, d.Kind)
		}
	}
	return out
}

// drugTraits is id's traits (nil for a legacy colonist).
func (g *Game) drugTraits(id entity.EntityID) []trait.ID {
	if fx := g.sk.stats[id]; fx != nil {
		return fx.traits
	}
	return nil
}

// assignDrugUse sends colonist id to take a drug its policy allows or its
// addiction craves; called before idle recreation in phaseAI. A craving
// seeks only the craved drug; a recreational dose needs low recreation
// (or a hedonist), the policy, the traits and the dose gap to agree.
func (g *Game) assignDrugUse(id entity.EntityID) bool {
	if !g.colonist(id) {
		return false
	}
	now := uint64(g.clock.Now())
	s := g.drugs[id]
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
		if med := g.medicalDrugs(id, probe, now); len(med) > 0 {
			if best, bx, by := g.nearestDrug(id, med); best != 0 {
				return g.startDrugJob(id, best, bx, by, now)
			}
		}
	}
	if !craving && ((now+uint64(id))/idleRecheck+uint64(id))%drugLookEvery != 0 {
		return false
	}
	joyLow := g.joy[id] < recreation.Bored
	traits := g.drugTraits(id)
	var allowed []item.Kind
	for _, d := range drug.All() {
		if craving && d.Kind != crave {
			continue
		}
		if probe.Policy.Allows(d.Kind, joyLow, probe, traits, now) {
			allowed = append(allowed, d.Kind)
		}
	}
	if len(allowed) == 0 {
		return false
	}
	best, bx, by := g.nearestDrug(id, allowed)
	if best == 0 {
		if s != nil {
			s.Retry = now + drugRetry
		}
		return false
	}
	return g.startDrugJob(id, best, bx, by, now)
}

// startDrugJob sends id to take a dose from stack best at (bx, by); a
// failed start (taken, or the path search deferred) holds off the next
// search.
func (g *Game) startDrugJob(id, best entity.EntityID, bx, by int, now uint64) bool {
	if !g.startJob(id, job.Record{Kind: job.Ingest, Target: best, TX: bx, TY: by, WorkLeft: drugIngestTicks}) {
		wait := uint64(drugRetry)
		if j, _ := g.jobs.Get(id); j.Fail == job.Reserved {
			wait = drugRetryBusy // taken, or the path search was deferred
		}
		g.drugState(id).Retry = now + wait
		return false
	}
	return true
}

// nearestDrug is the nearest free, unforbidden stack of one of kinds.
func (g *Game) nearestDrug(id entity.EntityID, kinds []item.Kind) (entity.EntityID, int, int) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD, bx, by := 1<<30, 0, 0
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || !st.Kind.IsDrug() || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden || !drugKindIn(kinds, st.Kind) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(iid); taken && owner != id {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if d < bestD || d == bestD && iid < best {
			best, bestD, bx, by = iid, d, st.X, st.Y
		}
	}
	return best, bx, by
}

func drugKindIn(ks []item.Kind, k item.Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// doIngest runs a drug-taking job: at the stack, it takes one dose.
func (g *Game) doIngest(id entity.EntityID, j job.Record) {
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Qty <= 0 || !st.Kind.IsDrug() || st.Carrier != 0 {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	k := st.Kind
	st.Qty--
	if st.Qty <= 0 {
		g.deleteItem(j.Target)
	} else {
		g.putStack(j.Target, st)
	}
	if g.ingestDose(id, k) {
		return // died of an overdose: die cancelled the job
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// ingestDose applies one dose of drug k to id: the domain effects (drawn
// from the drug stream), the recreation it gives, a stimulant's rest, the
// log line and the body's share (drugBody, MED-07). It reports whether
// the dose killed id (an overdose).
func (g *Game) ingestDose(id entity.EntityID, k item.Kind) bool {
	d, ok := drug.DefOf(k)
	if !ok {
		return false
	}
	now := uint64(g.clock.Now())
	g.drugState(id).Ingest(k, g.stream(streamDrug).IntN, now)
	g.joy[id] = recreation.Clamp(g.joy[id] + int(d.Joy)*recreation.Full/1000)
	if d.RestBoost > 0 {
		if n, ok := g.need.Get(id); ok {
			n.Rest = min(n.Rest+int(d.RestBoost)*needs.Full/1000, needs.Full)
			g.need.Set(id, n)
		}
	}
	g.logEvent(id, logEntry{Kind: logIngested, Name: k.String()})
	return g.drugBody(id, d, now)
}

// savedDrug is a drug state in the save (MED-00). Tolerance and LastDose
// are the fixed per-drug arrays saves up to version 5 wrote, read only:
// Tolerances and LastDoses (always written now) replace them, so drugs can
// be appended. gob matches fields by name, so an older save's drug.State
// decodes into it.
type savedDrug struct {
	Alcohol    int32
	Highs      []drug.High
	Addictions []drug.Addiction
	Tolerance  [drug.LegacyDrugs]int32 // legacy read only
	Hangover   uint64
	Doses      int32
	Peaked     bool
	HangPend   bool
	LastDose   [drug.LegacyDrugs]uint64 // legacy read only
	Policy     drug.Preset
	Retry      uint64
	Tolerances []int32
	LastDoses  []uint64
}

// exportDrugs is the Drugs save field in id order (nil: none).
func (g *Game) exportDrugs() []entry[savedDrug] {
	es := exportMap(g.drugs)
	if es == nil {
		return nil
	}
	out := make([]entry[savedDrug], len(es))
	for i, e := range es {
		s := e.V
		out[i] = entry[savedDrug]{e.ID, savedDrug{Alcohol: s.Alcohol, Highs: s.Highs, Addictions: s.Addictions,
			Hangover: s.Hangover, Doses: s.Doses, Peaked: s.Peaked, HangPend: s.HangPend, Policy: s.Policy,
			Retry: s.Retry, Tolerances: s.Tolerance, LastDoses: s.LastDose}}
	}
	return out
}

// importDrugs restores exportDrugs' state; an older save's arrays become
// the per-drug slices (used only when neither slice was saved).
func (g *Game) importDrugs(es []entry[savedDrug]) {
	ss := make([]entry[drug.State], len(es))
	for i, e := range es {
		d := e.V
		tol, last := d.Tolerances, d.LastDoses
		if tol == nil && last == nil {
			tol, last = drug.LegacyLists(d.Tolerance, d.LastDose)
		}
		ss[i] = entry[drug.State]{e.ID, drug.State{Alcohol: d.Alcohol, Highs: d.Highs, Addictions: d.Addictions,
			Tolerance: tol, Hangover: d.Hangover, Doses: d.Doses, Peaked: d.Peaked, HangPend: d.HangPend,
			LastDose: last, Policy: d.Policy, Retry: d.Retry}}
	}
	g.drugs = importMap(ss)
}

// drugLines lists id's drug effects for the Health tab.
func (g *Game) drugLines(id entity.EntityID) []DrugLineView {
	s := g.drugs[id]
	if s == nil {
		return nil
	}
	var out []DrugLineView
	for _, l := range s.Lines() {
		out = append(out, DrugLineView{Label: l.Label, Pct: int(l.Percent)})
	}
	if b := g.bodies[id]; b != nil { // MED-07: an acting numbroot dose
		if c, ok := b.Cond(body.CondPainkiller); ok {
			out = append(out, DrugLineView{Label: "numbroot: pain halved", Pct: int(c.Sev) / 10})
		}
	}
	return out
}

// drugThoughts are id's drug state thoughts (tipsy, high, withdrawal).
func (g *Game) drugThoughts(id entity.EntityID) []mood.Thought {
	s := g.drugs[id]
	if s == nil {
		return nil
	}
	ks := s.Thoughts()
	if len(ks) == 0 {
		return nil
	}
	return mood.Active(nil, ks...)
}
