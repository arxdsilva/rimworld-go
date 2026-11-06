package game

import (
	"slices"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/application/game/hit"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Bodies (ENG-003, M1): g.bodies holds each humanlike's body (colonists
// and raiders), made on the first wound, heal or tend. Animals keep the
// plain Injury model. health.Record.Injury stays the derived cache
// body.Damage(): death at health.MaxHP and everything else that reads
// Injury keep working; downing follows the capacities (MED-01, bleed.go).
// Hit locations and scar rolls come from the body stream (rng_streams.go),
// never g.rng. Saved as Bodies.

const (
	// consciousRecover is the consciousness (permille) a pawn needs to be
	// awake enough to talk to (a warden's visit).
	consciousRecover = 300
	// sightAccPoints is the shooting accuracy (points) total blindness
	// costs; one eye costs half.
	sightAccPoints = 30
)

// ensureBody is humanlike id's body, made on first use, with v's injury
// reconciled into it: an older save (or a fixture) that only knows Injury
// gets that much as a torso bruise; a lower Injury set from outside trims
// open wounds. Nil for animals and unknown ids.
func (g *Game) ensureBody(id entity.EntityID, v *health.Record) *body.Body {
	if !g.isPawn(id) {
		return nil
	}
	b := g.bodies[id]
	if b == nil {
		if g.bodies == nil {
			g.bodies = make(map[entity.EntityID]*body.Body)
		}
		b = &body.Body{}
		g.bodies[id] = b
	}
	body.Reconcile(b, v.Injury, uint64(g.clock.Now()))
	return b
}

// woundBody deals bl to b on the body stream. A placed blow strikes its
// part (body.Strike: overkill may destroy it, the brain and heart too,
// and what the part cannot take is lost); any other is spread on parts
// picked by coverage (body.Spread: death still comes at health.MaxHP).
func (g *Game) woundBody(b *body.Body, bl hit.Blow, now uint64) {
	rng := g.stream(streamBody)
	if bl.Placed {
		body.Strike(b, bl.Part, bl.Dmg, bl.Kind, bl.Lo, bl.Hi, rng.IntN, now)
		return
	}
	body.Spread(b, bl.Dmg, bl.Kind, rng.IntN, now)
}

// bodyKey is the capacities the stat sheet was built from: Moving,
// Manipulation, Sight (permille).
type bodyKey [3]int32

var healthyKey = bodyKey{1000, 1000, 1000}

func keyOf(b *body.Body) bodyKey {
	if b == nil {
		return healthyKey
	}
	return bodyKey{b.Capacity(body.Moving), b.Capacity(body.Manipulation), b.Capacity(body.Sight)}
}

// bodyMods is the body's step of id's stat sheet: Moving scales MoveSpeed,
// Manipulation WorkSpeed, and Sight costs shooting accuracy (nil when
// healthy).
func bodyMods(k bodyKey) []stat.Mod {
	if k == healthyKey {
		return nil
	}
	var ms []stat.Mod
	if k[0] != 1000 {
		ms = append(ms, stat.Mod{Stat: stat.MoveSpeed, Mul: max(k[0], 1)})
	}
	if k[1] != 1000 {
		ms = append(ms, stat.Mod{Stat: stat.WorkSpeed, Mul: max(k[1], 1)})
	}
	if k[2] != 1000 {
		ms = append(ms, stat.Mod{Stat: stat.ShootAcc, Add: (k[2] - 1000) * sightAccPoints / 1000})
	}
	return ms
}

// bodyChanged follows a change to id's body: Injury is re-derived, the
// stat sheet is rebuilt only when a capacity it uses moved, and a pawn
// with no consciousness left goes down.
func (g *Game) bodyChanged(id entity.EntityID, b *body.Body, v *health.Record) {
	v.Injury = b.Damage()
	g.vital.Set(id, *v)
	cur := healthyKey
	if fx := g.sk.stats[id]; fx != nil {
		cur = fx.body
	}
	if keyOf(b) != cur {
		g.sk.ensure()
		g.rebuildStats(id) // skills.go: the body layer
	}
	g.syncPatient(id, b) // patients.go (MED-00)
}

// consciousEnough: id is awake enough to be spoken to (no body, or
// consciousness at consciousRecover or more). Downing and standing up
// follow the capacity rule instead (downsNow / standsUp, bleed.go).
func (g *Game) consciousEnough(id entity.EntityID) bool {
	b := g.bodies[id]
	return b == nil || b.Capacity(body.Consciousness) >= consciousRecover
}

// hasOpenWound reports a wound on b that can still heal, or a fresh
// stump a tend must close (MED-01).
func hasOpenWound(b *body.Body) bool {
	if b.Fresh != 0 {
		return true
	}
	for _, w := range b.Wounds {
		if !w.Permanent {
			return true
		}
	}
	return false
}

// legacyParts is how many parts the fixed implant array of saves before
// MED-00 held (body.NumParts may grow; this never does).
const legacyParts = 20

// savedBody is a body in the save (MED-00). Implants is the fixed array
// saves up to version 5 wrote, read only: a body saved with Fitted
// (always written now, nil when nothing is fitted) ignores it. gob
// matches fields by name, so an older save's body.Body decodes into it.
type savedBody struct {
	Wounds    []body.Wound
	Missing   uint32
	Implants  [legacyParts]item.Kind // legacy read only
	Fitted    []item.Kind
	BloodLoss int32
	Conds     []body.Condition
	// BloodStage is the blood loss stage's hysteresis memory and Fresh
	// the untended stumps (MED-01).
	BloodStage uint8
	Fresh      uint32
}

// exportBodies is the Bodies save field in id order (nil: none).
func (g *Game) exportBodies() []entry[savedBody] {
	es := exportMap(g.bodies)
	if es == nil {
		return nil
	}
	out := make([]entry[savedBody], len(es))
	for i, e := range es {
		b := e.V
		out[i] = entry[savedBody]{e.ID, savedBody{Wounds: b.Wounds, Missing: b.Missing, Fitted: b.Fitted, BloodLoss: b.BloodLoss, Conds: b.Conds,
			BloodStage: b.BloodStage, Fresh: b.Fresh}}
	}
	return out
}

// importBodies restores exportBodies' state (an older save's implant
// array becomes Fitted), rebuilds the stat sheets the bodies change and
// the patients index (both derived, not saved). The vital records must be
// loaded first.
func (g *Game) importBodies(es []entry[savedBody]) {
	bs := make([]entry[body.Body], len(es))
	for i, e := range es {
		sb := e.V
		fitted := sb.Fitted
		if fitted == nil {
			fitted = body.FittedFrom(sb.Implants[:])
		}
		bs[i] = entry[body.Body]{e.ID, body.Body{Wounds: sb.Wounds, Missing: sb.Missing, Fitted: fitted, BloodLoss: sb.BloodLoss, Conds: sb.Conds,
			BloodStage: sb.BloodStage, Fresh: sb.Fresh}}
	}
	g.bodies = importMap(bs)
	for _, e := range es {
		if keyOf(g.bodies[e.ID]) != healthyKey {
			g.sk.ensure()
			g.rebuildStats(e.ID)
		}
	}
	g.rebuildPatients() // patients.go (MED-00)
}

// exportMap lists a pointer map's entries in id order (nil when empty).
func exportMap[T any](m map[entity.EntityID]*T) []entry[T] {
	if len(m) == 0 {
		return nil
	}
	out := make([]entry[T], 0, len(m))
	for id, v := range m {
		out = append(out, entry[T]{id, *v})
	}
	slices.SortFunc(out, func(a, b entry[T]) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out
}

// importMap rebuilds exportMap's map (nil when there are no entries).
func importMap[T any](es []entry[T]) map[entity.EntityID]*T {
	if len(es) == 0 {
		return nil
	}
	m := make(map[entity.EntityID]*T, len(es))
	for _, e := range es {
		v := e.V
		m[e.ID] = &v
	}
	return m
}

// Part status words of the Health tab (PartView.Status starts with one).
const (
	PartHealthy    = roster.PartHealthy
	PartMissing    = roster.PartMissing
	PartReplaced   = roster.PartReplaced
	PartArtificial = roster.PartArtificial
	PartWounded    = roster.PartWounded
	PartScarred    = roster.PartScarred
)

// viewBody is id's body for a read model: its own, or for a pawn never hurt
// since an older save, the body its Injury would migrate to (built aside,
// so a view never changes the game).
func (g *Game) viewBody(id entity.EntityID) *body.Body {
	if b := g.bodies[id]; b != nil {
		return b
	}
	b := &body.Body{}
	if v, _ := g.vital.Get(id); v.Injury > 0 {
		body.Reconcile(b, v.Injury, 0)
	}
	return b
}

// implantKinds are the implant item kinds in kind order.
var implantKinds = func() []item.Kind {
	var out []item.Kind
	for k := item.Kind(1); k != 0; k++ {
		if k.IsImplant() {
			out = append(out, k)
		}
	}
	return out
}()

// healthView builds the Health tab: capacities, pain, every part in
// body.Part order (Parts[i] is body.Part(i)), queued operations with what
// each waits for, the operations the picker offers, and drug lines.
// Humanlike pawns only.
func (g *Game) healthView(id entity.EntityID) (HealthView, bool) {
	if !g.isPawn(id) {
		return HealthView{}, false
	}
	if _, ok := g.vital.Get(id); !ok {
		return HealthView{}, false
	}
	b := g.viewBody(id)
	v, _ := g.vital.Get(id)
	hv := HealthView{Pain: int(b.Pain()) / 10, Drugs: doseOf(g).Lines(id), // game/dose
		State: healthState(v, b), BloodLossPct: int(b.BloodLoss) / 10, BleedPerDay: int(b.BleedPerDay()) / 10,
		DeathInHours: b.BleedDeathHours()} // bleed.go (MED-01)
	g.conditionRows(id, b, &hv) // MED-02: conditions, the infection headline and its death clock
	if v.Status == health.Dead {
		hv.DeathInHours = -1
	}
	hv.Capacities = make([]CapacityView, 0, body.NumCaps)
	for c := body.Capacity(0); c < body.NumCaps; c++ {
		hv.Capacities = append(hv.Capacities, CapacityView{Label: roster.CapacityLabels[c], Pct: int(b.Capacity(c)) / 10, Tip: roster.CapacityTip(b, c)})
	}
	ps := b.Parts()
	hv.Parts = make([]PartView, 0, len(ps))
	for _, s := range ps {
		pv := PartView{Label: s.Label, Status: roster.PartStatus(b, s), HP: s.HP, Part: int(s.Part),
			Bleed: int(b.PartBleedPerDay(s.Part)) / 10, TendTip: body.PartTendTip(b, s.Part)}
		if s.Fitted {
			pv.Implant = s.Implant.String()
		}
		hv.Parts = append(hv.Parts, pv)
	}
	for i, op := range g.operationsOf(id) {
		why, ready := g.surgeryWait(id, op)   // surgery.go: Blocked needs the player
		pct, tip := g.operationChance(id, op) // surgery.go (MED-06): the chance and its factors
		hv.Operations = append(hv.Operations, OperationView{Label: body.OperationLabel(b, op), Slot: i, Status: why, Blocked: !ready,
			ChancePct: pct, ChanceTip: tip})
	}
	hv.Options = g.operationOptions(id, b)
	return hv, true
}

// operationOptions lists every operation the picker can queue on id now,
// in a fixed order: each implant on each part it fits and could take
// (missing: install; still there: replace), by part then kind, then
// removing each implant, then amputating each part still there that is
// not vital. Ones already queued on that part are left out. An option
// that can't be started yet (research, stock, reach, no other doctor:
// opBlocked) is Disabled with its reason, and can still be queued.
func (g *Game) operationOptions(id entity.EntityID, b *body.Body) []OperationOptionView {
	q := g.operationsOf(id)
	var out []OperationOptionView
	add := func(op body.Operation) {
		if !b.CanOperate(op) {
			return
		}
		for _, x := range q {
			if x.Kind == op.Kind && x.Part == op.Part {
				return
			}
		}
		why, _ := g.opBlocked(id, 0, b, op, false) // surgery.go
		out = append(out, OperationOptionView{Label: body.OperationLabel(b, op), Kind: uint8(op.Kind), Part: uint8(op.Part),
			Item: uint8(op.Item), Disabled: why != "", Reason: why})
	}
	for p := body.Part(0); p < body.NumParts; p++ {
		for _, k := range implantKinds {
			if body.Fits(k, p) {
				add(body.Operation{Kind: body.InstallImplant, Part: p, Item: k})
			}
		}
	}
	for p := body.Part(0); p < body.NumParts; p++ {
		if !body.DefOf(p).Vital { // taking out an artificial heart kills (organs.go)
			add(body.Operation{Kind: body.RemoveImplant, Part: p})
		}
	}
	for p := body.Part(0); p < body.NumParts; p++ {
		if !body.Inner(p) { // organs are harvested (MED-06), not amputated
			add(body.Operation{Kind: body.Amputate, Part: p})
		}
	}
	g.moreOperationOptions(b, add) // surgery.go (MED-06): harvest, transplant, anaesthesia, euthanasia
	return out
}

// missingPart reports a part gone with nothing standing in for it (a
// glass eye or a wooden nose covers its part too; SIM-049).
func missingPart(b *body.Body) bool {
	return b != nil && b.Missing != 0 && b.MissingUncovered() > 0
}

// bodyThoughts are id's body state thoughts: missing a part (no implant in
// its place). Being disfigured has no thought of its own (others see it).
func (g *Game) bodyThoughts(id entity.EntityID) []mood.Thought {
	ts := g.apparel[id].AppendThoughts(nil) // craft/garment.go (PRD-2): ratty, tattered and tainted apparel
	if !missingPart(g.bodies[id]) {
		return ts
	}
	d := mood.DefOf(mood.MissingPart)
	return append(ts, mood.Thought{Kind: mood.MissingPart, Label: d.Label, Value: d.Value})
}

// conditionRows fills the Health tab's condition rows (MED-02) and lets
// a condition set the headline when nothing bleeds ("Infection: losing"
// or "winning") and shorten the death clock.
func (g *Game) conditionRows(id entity.EntityID, b *body.Body, hv *HealthView) {
	if len(b.Conds) == 0 {
		return
	}
	now := uint64(g.clock.Now())
	imm := g.immunityGain(id)
	losing := false
	for _, c := range b.Conds {
		d := body.DefOfCond(c.Kind)
		tended := now < c.TendUntil
		cv := ConditionView{Label: body.ConditionLabel(c, d), SevPct: int(c.Sev) / 10, ImmPct: int(c.Immunity) / 10,
			TendPct: int(c.TendQ) / 10, Losing: infection.Losing(c, d, tended, imm)}
		aging := body.IsAging(c.Kind) // aging.go (MED-08): its own label and card
		if aging {
			cv.Label, cv.Losing = body.AgingLabel(c, d, g.agingSalt(id)), c.Kind == body.CondHeartAttack
		}
		if d.TendEvery > 0 {
			if tended {
				cv.DueInHours = int((c.TendUntil - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
			} else {
				cv.Due = true
			}
		}
		cv.Tip = roster.ConditionTip(c, d, cv, tended)
		if aging {
			cv.Tip = body.AgingTip(c, cv.Label, g.agingSalt(id))
		}
		losing = losing || cv.Losing
		hv.Conditions = append(hv.Conditions, cv)
	}
	switch {
	case hv.BleedPerDay != 0 || hv.State == "Dead":
	case body.AgingHeads(b): // body/aging_text.go (MED-08): a heart attack, or only age conditions
		hv.State = body.AgingHeadline(b, hv.State, now)
	default:
		hv.State = body.ConditionHeadline(b, losing)
	}
	if h := g.condDeathHours(id, b); h >= 0 && (hv.DeathInHours < 0 || h < hv.DeathInHours) {
		hv.DeathInHours = h
	}
}

// hits lands combat hits (game/hit, CBT-1) on g: a stateless value over
// the adapters below (the stagger timer is the motion record's).
func hits(g *Game) hit.Wounds {
	return hit.Wounds{B: hitBodies{g}, W: hitWardrobe{g}, H: hitHarm{g}, S: g.combat.Gear}
}

// staggered reports id slowed by a stagger now (the read model).
func staggered(g *Game, id entity.EntityID) bool {
	m, _ := g.motion.Get(id)
	return hit.Staggered(m.StaggerUntil, uint64(g.clock.Now()))
}

type hitBodies struct{ g *Game }

func (h hitBodies) Body(id entity.EntityID) *body.Body {
	v, ok := h.g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return nil
	}
	return h.g.ensureBody(id, &v)
}
func (h hitBodies) Plating(id entity.EntityID) (armor.Set, bool) {
	if a, ok := h.g.animals.Get(id); ok {
		if m, mech := animal.MechOf(a.Species); mech {
			return m.Armor, true // DEF-4: a mechanoid's plating
		}
	}
	return armor.Set{}, false
}
func (h hitBodies) Size(id entity.EntityID) int {
	if a, ok := h.g.animals.Get(id); ok {
		return combat.BodySize(a)
	}
	return 1000
}
func (h hitBodies) Roll(n int) int { return h.g.stream(streamBody).IntN(n) }

type hitWardrobe struct{ g *Game }

func (h hitWardrobe) Outfit(id entity.EntityID) craft.Outfit       { return h.g.apparel[id] }
func (h hitWardrobe) SetOutfit(id entity.EntityID, o craft.Outfit) { h.g.apparel[id] = o }
func (h hitWardrobe) Destroyed(id entity.EntityID, k item.Kind) {
	pos, _ := h.g.positions.Get(id)
	h.g.message(pos.X, pos.Y, "%s's %s was destroyed.", h.g.name(id), k)
}

type hitHarm struct{ g *Game }

func (h hitHarm) Now() uint64 { return uint64(h.g.clock.Now()) }
func (h hitHarm) Wound(id entity.EntityID, b hit.Blow, by entity.EntityID) {
	h.g.harm(id, b, by, health.CauseInjury) // combat.go
}
func (h hitHarm) StaggerUntil(id entity.EntityID) uint64 {
	m, _ := h.g.motion.Get(id)
	return m.StaggerUntil
}
func (h hitHarm) SetStaggerUntil(id entity.EntityID, until uint64) {
	m := h.g.motionOf(id)
	m.StaggerUntil = until
	h.g.motion.Set(id, m)
}
func (h hitHarm) Hold(id entity.EntityID, until uint64) {
	m := h.g.motionOf(id)
	m.NextMoveAt = max(m.NextMoveAt, until)
	h.g.motion.Set(id, m)
	h.g.sk.noteStun(id, until, h.Now()) // skills.go: for the card only, NextMoveAt also holds walkers
	if j, ok := h.g.jobs.Get(id); ok && (j.Kind == job.Attack || j.Kind == job.Hunt) {
		j.WorkLeft = max(j.WorkLeft, int(until-h.Now()))
		h.g.jobs.Set(id, j)
	}
}

// gearFacts is PawnView.Gear's mass, armour and speed (GEAR-1 contract: zero until game/gearfacts).
func gearFacts(*Game, entity.EntityID) gearview.Pawn { return gearview.Pawn{} }
