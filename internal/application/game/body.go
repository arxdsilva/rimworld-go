// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/saveutil"
	"slices"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearfacts"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/application/game/hit"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
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
// part, split by its form (body.StrikeBlow, CBT 3.4: cut, blunt, scratch,
// stab and bullet rules; overkill may destroy a part, vital too);
// any other is spread on parts by coverage (body.Spread: death at MaxHP).
func (g *Game) woundBody(b *body.Body, bl hit.Blow, now uint64) {
	rng := g.stream(streamBody)
	if bl.Placed {
		body.StrikeBlow(b, bl.Part, bl.Dmg, bl.Kind, bl.Split, bl.Lo, bl.Hi, rng.IntN, now)
		return
	}
	body.Spread(b, bl.Dmg, bl.Kind, rng.IntN, now)
}

// bodyKey contains all body capacities that affect shooting or stat sheets:
// Moving, Manipulation and Sight (permille).
type bodyKey [3]int32

// bodyStatKey excludes Sight because the shooting capacity offset is cached separately.
type bodyStatKey [2]int32

var healthyKey = bodyKey{1000, 1000, 1000}
var healthyStatKey = bodyStatKey{1000, 1000}

func keyOf(b *body.Body) bodyKey {
	if b == nil {
		return healthyKey
	}
	return bodyKey{b.Capacity(body.Moving), b.Capacity(body.Manipulation), b.Capacity(body.Sight)}
}

func statKeyOf(k bodyKey) bodyStatKey { return bodyStatKey{k[0], k[1]} }

func bodyAimOf(k bodyKey) int32 {
	return int32(attack.ManipPoints(int(k[1])) + attack.SightPoints(int(k[2])))
}

// bodyMods changes movement and work speeds. Shooting capacity offsets
// are cached separately without rebuilding the stat sheet.
func bodyMods(k bodyStatKey) []stat.Mod { return stat.BodyMods(k) }

// bodyChanged follows a change to id's body: Injury is re-derived, the
// stat sheet is rebuilt only when a capacity it uses moved, and a pawn
// with no consciousness left goes down.
func (g *Game) bodyChanged(id entity.EntityID, b *body.Body, v *health.Record) {
	damage := b.Damage()
	v.Injury = damage
	g.vital.Set(id, *v)
	cur := healthyStatKey
	if fx := g.sk.stats[id]; fx != nil {
		cur = fx.body
	}
	k := keyOf(b)
	if statKeyOf(k) != cur || g.sk.stats[id] == nil && bodyAimOf(k) != 0 {
		g.sk.ensure()
		g.rebuildStats(id)
	} else if fx := g.sk.stats[id]; fx != nil {
		fx.bodyAim = bodyAimOf(k)
	}
	g.syncPatient(id, b)
	if g.soc != nil {
		g.refreshSig(id)
	}
}

// consciousEnough: id is awake enough to be spoken to (no body, or
// consciousness at consciousRecover or more). Downing and standing up
// follow the capacity rule instead (downsNow / standsUp, bleed.go).
func (g *Game) consciousEnough(id entity.EntityID) bool {
	b := g.bodies[id]
	return b == nil || b.Capacity(body.Consciousness) >= consciousRecover
}

// legacyParts is how many parts the fixed implant array of saves before
// MED-00 held (body.NumParts may grow; this never does).
const legacyParts = 20

// savedBody reads pre-v18 Fitted and the older fixed Implants array.
// New saves write Replacements and Added only.
type savedBody struct {
	Wounds       []body.Wound
	Missing      uint32
	Implants     [legacyParts]item.Kind // pre-v6, read only
	Fitted       []item.Kind            // v6-v17, read only
	Replacements []body.Replacement
	Added        []body.Implant
	BloodLoss    int32
	Conds        []body.Condition
	// BloodStage is the blood loss stage's hysteresis memory and Fresh
	// the untended stumps (MED-01).
	BloodStage uint8
	Fresh      uint32
}

// exportBodies is the Bodies save field in id order (nil: none).
func (g *Game) exportBodies(owned ...bool) []entry[savedBody] {
	es := saveutil.ExportMap[entry[body.Body]](g.bodies)
	if es == nil {
		return nil
	}
	out := make([]entry[savedBody], len(es))
	for i, e := range es {
		b := saveutil.OwnBody(len(owned) > 0 && owned[0], e.V)
		out[i] = entry[savedBody]{e.ID, savedBody{Wounds: savefmt.CanonicalSlice(b.Wounds), Missing: b.Missing,
			Replacements: savefmt.CanonicalSlice(b.Replacements), Added: savefmt.CanonicalSlice(b.Implants),
			BloodLoss: b.BloodLoss, Conds: savefmt.CanonicalSlice(b.Conds),
			BloodStage: b.BloodStage, Fresh: b.Fresh}}
	}
	return out
}

// importBodies converts old prosthetics at the save boundary and rebuilds
// derived stats and patient indexes after vital records load.
func (g *Game) importBodies(es []entry[savedBody]) {
	bs := make([]entry[body.Body], len(es))
	for i, e := range es {
		sb := e.V
		replacements := sb.Replacements
		if replacements == nil {
			legacy := sb.Fitted
			if legacy == nil {
				legacy = sb.Implants[:]
			}
			replacements = body.ReplacementsFrom(legacy)
		}
		bs[i] = entry[body.Body]{e.ID, body.Body{Wounds: sb.Wounds, Missing: sb.Missing, Replacements: replacements,
			Implants: sb.Added, BloodLoss: sb.BloodLoss, Conds: sb.Conds, BloodStage: sb.BloodStage, Fresh: sb.Fresh}}
	}
	g.bodies = saveutil.ImportMap(bs)
	for _, e := range es {
		if k := keyOf(g.bodies[e.ID]); statKeyOf(k) != healthyStatKey || bodyAimOf(k) != 0 {
			g.sk.ensure()
			g.rebuildStats(e.ID)
		}
	}
	g.rebuildPatients() // patients.go (MED-00)
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
		for _, imp := range b.Implants {
			if imp.Part == s.Part {
				pv.Implants = append(pv.Implants, imp.Kind.String())
			}
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
	queued := g.operationsOf(id)
	var out []OperationOptionView
	for _, op := range body.AvailableOperations(b) {
		if slices.ContainsFunc(queued, func(q body.Operation) bool {
			return body.OperationsConflict(q, op)
		}) {
			continue
		}
		why, _ := g.opBlocked(id, 0, b, op, false)
		out = append(out, OperationOptionView{Label: body.OperationLabel(b, op),
			Kind: uint8(op.Kind), Part: uint8(op.Part), Item: uint8(op.Item),
			Disabled: why != "", Reason: why})
	}
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
	if b := g.bodies[id]; b != nil && body.HasImplant(b, body.Brain, item.BlissCircuit) {
		d := mood.DefOf(mood.BlissCircuit)
		ts = append(ts, mood.Thought{Kind: mood.BlissCircuit, Label: d.Label, Value: d.Value})
	}
	if bio, ok := g.bioFor(id); ok {
		count := 0
		if b := g.bodies[id]; b != nil {
			count = b.ArtificialParts()
		}
		ts = trait.AppendImplantThoughts(ts, bio.Traits, count)
	}
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

// gearFacts is PawnView.Gear's mass (weapons, apparel, pack), capacity, armour and speed (game/gearfacts, GEAR-1).
func gearFacts(g *Game, id entity.EntityID) gearview.Pawn {
	sk, sq, _ := g.combat.Drafts.Spare(id)
	return gearfacts.Of(g.sk.content.Items, g.apparel[id], gearfacts.Grams(g.sk.content.Items, g.equip[id], 1)+gearfacts.Grams(g.sk.content.Items, sk, sq)+senseOf(g).Gear.Pack(id).Grams(g.sk.content.Items), int(keyOf(g.bodies[id])[1]), hitBodies{g}.Size(id))
}
