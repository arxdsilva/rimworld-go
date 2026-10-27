package game

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/entity"
)

// Diseases (MED-04). From day 15 the nature storyteller (incidents.go)
// may roll a disease among its events, weighted by the difficulty's
// disease frequency (0 turns them off): flu, plague or malaria strikes a
// random handful of the colonists at home, sleeping sickness, gut worms or
// muscle parasites one of them; malaria keeps to warm lands and sleeping
// sickness to the tropics (domain/incident). The victims are chosen on the
// medical stream and a letter names them.
//
// A disease is a condition on the torso (body/cond_disease.go). Once an
// hour, after the wounds and infections (phaseMedical), the diseased
// patients step here, in id order: flu, plague, malaria and sleeping
// sickness race the body's immunity on the shared engine
// (infection.StepRace) and kill at full severity; the parasites never
// change by themselves and are cured once their summed tend quality
// reaches 300 %. Their stages change the capacities and pain (Capacity,
// Pain); gut worms double the hunger and muscle parasites the awake rest
// fall (diseaseNeedRates, from decayNeeds); some stages vomit now and then
// (a splash of dirt where the patient stands). A doctor tends a disease
// once per tend window (body.TendDue), and a colonist rests in bed while
// one is at its major stage or worse, getting up only once immune or
// cured (bedrest.go).

const (
	// vomitDirt is the dirt a vomit leaves on the patient's tile (enough
	// to be cleaned: filth.CleanStart is 120). Original tuning.
	vomitDirt = 150
	// diseaseTargetsMax caps how many colonists one flu, plague or malaria
	// outbreak strikes (original tuning).
	diseaseTargetsMax = 6
)

// diseaseIncidents maps each disease event to the condition it gives.
var diseaseIncidents = [...]struct {
	ev    incident.ID
	kind  body.CondKind
	group bool // several victims (else one)
}{
	{incident.Flu, body.CondFlu, true},
	{incident.Plague, body.CondPlague, true},
	{incident.Malaria, body.CondMalaria, true},
	{incident.SleepingSickness, body.CondSleepingSickness, false},
	{incident.GutWorms, body.CondGutWorms, false},
	{incident.MuscleParasites, body.CondMuscleParasites, false},
}

// diseaseEvent is event id's condition and whether it strikes several
// (false: id is no disease).
func diseaseEvent(id incident.ID) (kind body.CondKind, group, ok bool) {
	for _, d := range diseaseIncidents {
		if d.ev == id {
			return d.kind, d.group, true
		}
	}
	return body.CondNone, false, false
}

// diseasePct is the difficulty's disease frequency (percent; 0: no
// diseases); a game with no setup keeps the full frequency.
func (g *Game) diseasePct() int {
	if !g.hasSetup() {
		return incident.FullFreq
	}
	return max(g.ng.setup.Difficulty.DiseaseFreq, 0)
}

// diseaseCandidates are the colonists disease k may strike, in id order:
// at home or away on a caravan (WS-M), alive, not prisoners, raiders or
// visitors, without k already, not kept from it by prophyl, and for gut
// worms without a synth stomach.
func (g *Game) diseaseCandidates(k body.CondKind) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.colonistsHomeAndAway() { // caravan_medical.go
		if g.isPrisoner(id) || g.isRaider(id) || g.visitor(id) || g.prophylaxed(id, k) { // drugs.go (MED-07): prophyl
			continue
		}
		if b := g.bodies[id]; b != nil {
			if _, has := b.Cond(k); has || k == body.CondGutWorms && b.GutWormImmune() {
				continue
			}
		}
		out = append(out, id)
	}
	return out
}

// diseaseFits is the map's check for disease event id: diseases are on
// and someone can catch it.
func (g *Game) diseaseFits(id incident.ID) bool {
	k, _, ok := diseaseEvent(id)
	return ok && g.diseasePct() > 0 && len(g.diseaseCandidates(k)) > 0
}

// diseaseOutbreak makes disease event id strike: one candidate, or for
// flu, plague and malaria a random 1..n of them (at most
// diseaseTargetsMax), drawn on the medical stream. Each gets the condition
// at its onset severity, and one letter names them all.
func (g *Game) diseaseOutbreak(id incident.ID, now uint64) bool {
	k, group, ok := diseaseEvent(id)
	if !ok || g.diseasePct() <= 0 {
		return false
	}
	cands := g.diseaseCandidates(k)
	if len(cands) == 0 {
		return false
	}
	r := g.stream(streamMedical)
	n := 1
	if group {
		n = 1 + r.IntN(min(len(cands), diseaseTargetsMax))
	}
	for i := 0; i < n; i++ { // partial Fisher–Yates: the first n are the victims
		j := i + r.IntN(len(cands)-i)
		cands[i], cands[j] = cands[j], cands[i]
	}
	victims := cands[:n]
	slices.Sort(victims)
	dz, _ := body.DiseaseOf(k)
	d := body.DefOfCond(k)
	for _, pid := range victims {
		v, _ := g.vital.Get(pid)
		b := g.ensureBody(pid, &v)
		if b == nil {
			continue
		}
		b.AddCond(body.Condition{Kind: k, Part: body.Torso, Sev: dz.Onset, Since: now, Stage: d.StageAt(dz.Onset)})
		g.bodyChanged(pid, b, &v)
	}
	g.diseaseLetter(k, victims)
	return true
}

// diseaseLetter posts the outbreak's letter (caution) at the first
// victim: who caught what, and what to do.
func (g *Game) diseaseLetter(k body.CondKind, victims []entity.EntityID) {
	d := body.DefOfCond(k)
	names := make([]string, len(victims))
	for i, id := range victims {
		names[i] = g.personName(id)
	}
	var who string
	switch len(names) {
	case 1:
		who = names[0]
	case 2:
		who = names[0] + " and " + names[1]
	default:
		who = strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
	var text string
	if d.CureTendSum > 0 {
		text = fmt.Sprintf("%s has %s. They build no immunity and never kill, but stay until tended away: tends adding up to %d%% quality, one every %d hours.",
			who, d.Label, d.CureTendSum/10, d.TendEvery)
	} else {
		verb := "has"
		if len(names) > 1 {
			verb = "have"
		}
		text = fmt.Sprintf("%s %s caught %s. It races their immunity and can kill: keep them in bed and tended every %d hours, with the best medicine and doctor you have.",
			who, verb, d.Label, d.TendEvery)
	}
	text += g.awayNote(victims) // caravan_medical.go (WS-M): who is off on a caravan
	title := strings.ToUpper(d.Label[:1]) + d.Label[1:]
	pos, on := g.positions.Get(victims[0])
	focus := victims[0]
	if !on { // away on a caravan: the letter points home
		pos.X, pos.Y = g.home()
		focus = 0
	}
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: pos.X, Y: pos.Y, Focus: focus,
		Letter: true, Title: title, Kind: letterCaution})
}

// hasDisease reports a disease on b.
func hasDisease(b *body.Body) bool {
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if body.IsDisease(c.Kind) {
			return true
		}
	}
	return false
}

// stepDiseases runs an hour of every disease on the patients, in id order
// (phaseMedical, after stepCare).
func (g *Game) stepDiseases(now uint64) {
	if len(g.med.patients) == 0 {
		return
	}
	for _, id := range slices.Clone(g.med.patients) { // a death touches the index
		b := g.bodies[id]
		poisoned := g.poisoned(id) // food_poison.go (WS-M)
		if !poisoned && !hasDisease(b) {
			continue
		}
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead {
			continue
		}
		if hasDisease(b) {
			g.stepDisease(id, b, &v, now)
			if v, ok = g.vital.Get(id); !ok || v.Status == health.Dead {
				continue
			}
		}
		if poisoned {
			g.stepFoodPoison(id, b, &v, now)
		}
	}
}

// stepDisease advances id's diseases by an hour: the immunity races, the
// parasites' tend cure, deaths, cures and vomiting; the body's stats and
// the downing rule follow a change of stage.
func (g *Game) stepDisease(id entity.EntityID, b *body.Body, v *health.Record, now uint64) {
	pos, _ := g.positions.Get(id)
	changed, vomit := false, false
	for i := range b.Conds {
		c := &b.Conds[i]
		dz, ok := body.DiseaseOf(c.Kind)
		if !ok {
			continue
		}
		d := body.DefOfCond(c.Kind)
		stage := c.Stage
		if d.CureTendSum > 0 {
			c.Sev = body.TendCureSev(c.TendSum, d.CureTendSum)
			if c.TendSum >= d.CureTendSum {
				c.Sev = 0
			}
		} else if dead, cured := infection.StepRace(c, d, now < c.TendUntil, g.immunityGain(id), 1); dead {
			g.message(pos.X, pos.Y, "%s died of %s.", g.name(id), d.Label)
			g.dieMedical(id, health.CauseDisease) // caravan_medical.go: away on a caravan, no corpse
			g.clearPatient(id)
			return
		} else if cured {
			c.Sev = 0
		}
		if c.Sev <= 0 {
			changed = true
			if d.CureTendSum > 0 {
				g.message(pos.X, pos.Y, "%s is rid of the %s.", g.name(id), d.Label)
			} else {
				g.message(pos.X, pos.Y, "%s has recovered from %s.", g.name(id), d.Label)
			}
			continue
		}
		changed = changed || c.Stage != stage
		if every := dz.VomitEveryAt(c.Stage); every > 0 && g.stream(streamMedical).IntN(int(every)) == 0 {
			vomit = true
		}
	}
	if changed {
		b.Conds = slices.DeleteFunc(b.Conds, func(c body.Condition) bool { return body.IsDisease(c.Kind) && c.Sev <= 0 })
		if len(b.Conds) == 0 {
			b.Conds = nil
		}
		g.bodyChanged(id, b, v) // body.go: stats follow the capacities
		g.updateDowned(id, b, v)
	}
	if vomit && g.onMap(id) { // away on a caravan: no splash on the map
		g.addDirt(pos.Y*g.grid.Width()+pos.X, vomitDirt) // filth.go
	}
}

// diseaseNeedRates scales id's hunger and rest rates (permille) by its
// diseases: gut worms double the hunger, muscle parasites the awake rest
// fall (decayNeeds, skills.go).
func (g *Game) diseaseNeedRates(id entity.EntityID, hr, rr int32) (int32, int32) {
	b := g.bodies[id]
	if b == nil || len(b.Conds) == 0 {
		return hr, rr
	}
	for _, c := range b.Conds {
		if dz, ok := body.DiseaseOf(c.Kind); ok {
			hr = hr * dz.HungerMul / 1000
			rr = rr * dz.RestMul / 1000
		}
	}
	return hr, rr
}

// infectionSick reports id fighting a condition other than a disease it
// is not yet immune to (a wound infection: bed rest as before MED-04).
func (g *Game) infectionSick(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if body.IsDisease(c.Kind) {
			continue
		}
		if d := body.DefOfCond(c.Kind); !d.NoImmunity && c.Immunity < infection.Full {
			return true
		}
	}
	return false
}

// diseaseRest reports a disease of id at its major stage or worse and not
// yet beaten by immunity: the patient goes to bed (needsBedRest).
func (g *Game) diseaseRest(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if dz, ok := body.DiseaseOf(c.Kind); ok && dz.Rest > 0 && c.Stage >= dz.Rest && c.Immunity < infection.Full {
			return true
		}
	}
	return false
}

// diseaseSick reports a disease of id still racing its immunity: a
// patient in bed stays there until immune or cured (restOver), so a
// disease falling back below its major stage never gets it up early
// (hysteresis: in at major, out only once immune).
func (g *Game) diseaseSick(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if body.IsDisease(c.Kind) && !body.DefOfCond(c.Kind).NoImmunity && c.Immunity < infection.Full {
			return true
		}
	}
	return false
}

// feelsSick reports id under the "sick" thought (rooms.go): fighting a
// condition (sick, infection.go), host to parasites, or food poisoned.
func (g *Game) feelsSick(id entity.EntityID) bool {
	if g.sick(id) || g.poisoned(id) { // food_poison.go (WS-M)
		return true
	}
	b := g.bodies[id]
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if body.IsDisease(c.Kind) {
			return true
		}
	}
	return false
}

// diseaseLabel is id's diseases for the pawn view and the disease alert
// ("flu", "flu, gut worms"; "": none).
func (g *Game) diseaseLabel(id entity.EntityID) string {
	b := g.bodies[id]
	if b == nil {
		return ""
	}
	var out []string
	for _, c := range b.Conds {
		if body.IsDisease(c.Kind) {
			out = append(out, body.DefOfCond(c.Kind).Label)
		}
	}
	return strings.Join(out, ", ")
}

// GiveDisease gives living colonist id disease k (a body.CondKind) at its
// onset (dev scenarios and tests); false when it can't have it.
func (g *Game) GiveDisease(id entity.EntityID, k uint8) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	kind := body.CondKind(k)
	dz, ok := body.DiseaseOf(kind)
	v, alive := g.vital.Get(id)
	if !ok || !alive || v.Status == health.Dead {
		return false
	}
	b := g.ensureBody(id, &v)
	if b == nil || kind == body.CondGutWorms && b.GutWormImmune() {
		return false
	}
	d := body.DefOfCond(kind)
	if !b.AddCond(body.Condition{Kind: kind, Part: body.Torso, Sev: dz.Onset, Since: uint64(g.clock.Now()), Stage: d.StageAt(dz.Onset)}) {
		return false
	}
	g.bodyChanged(id, b, &v)
	return true
}
