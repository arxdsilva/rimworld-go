package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/drug"
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
	if !craving && ((now+uint64(id))/idleRecheck+uint64(id))%drugLookEvery != 0 {
		return false
	}
	probe := s
	if probe == nil {
		probe = &drug.State{Policy: drug.SocialDrinking}
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
		g.stacks.Set(j.Target, st)
	}
	g.ingestDose(id, k)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// ingestDose applies one dose of drug k to id: the domain effects (drawn
// from the drug stream), the recreation it gives, a stimulant's rest and
// the log line.
func (g *Game) ingestDose(id entity.EntityID, k item.Kind) {
	d, ok := drug.DefOf(k)
	if !ok {
		return
	}
	g.drugState(id).Ingest(k, g.stream(streamDrug).IntN, uint64(g.clock.Now()))
	g.joy[id] = recreation.Clamp(g.joy[id] + int(d.Joy)*recreation.Full/1000)
	if d.RestBoost > 0 {
		if n, ok := g.need.Get(id); ok {
			n.Rest = min(n.Rest+int(d.RestBoost)*needs.Full/1000, needs.Full)
			g.need.Set(id, n)
		}
	}
	g.logEvent(id, logEntry{Kind: logIngested, Name: k.String()})
}

// exportDrugs is the Drugs save field in id order (nil: none).
func (g *Game) exportDrugs() []entry[drug.State] { return exportMap(g.drugs) }

// importDrugs restores exportDrugs' state.
func (g *Game) importDrugs(es []entry[drug.State]) { g.drugs = importMap(es) }

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
