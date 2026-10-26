package game

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
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
	// maxSpill bounds how many parts one hit may spread over.
	maxSpill = 8
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
	reconcileInjury(b, v.Injury, uint64(g.clock.Now()))
	return b
}

// reconcileInjury makes b's damage match injury where wounds allow it.
func reconcileInjury(b *body.Body, injury int, now uint64) {
	d := b.Damage()
	switch {
	case injury > d:
		b.Wounds = append(b.Wounds, body.Wound{Part: body.Torso, Dmg: injury - d, Kind: body.Bruise, At: now})
	case injury < d:
		trimOpen(b, d-injury)
	}
}

// trimOpen removes n points of open-wound damage, smallest wound first,
// without scarring (an outside correction, not healing).
func trimOpen(b *body.Body, n int) {
	for n > 0 {
		idx := -1
		for i, w := range b.Wounds {
			if !w.Permanent && (idx < 0 || w.Dmg < b.Wounds[idx].Dmg) {
				idx = i
			}
		}
		if idx < 0 {
			return
		}
		w := &b.Wounds[idx]
		if w.Dmg > n {
			w.Dmg -= n
			return
		}
		n -= w.Dmg
		b.Wounds = slices.Delete(b.Wounds, idx, idx+1)
	}
}

// partDmg is the wound damage on part p.
func partDmg(b *body.Body, p body.Part) int {
	n := 0
	for _, w := range b.Wounds {
		if w.Part == p {
			n += w.Dmg
		}
	}
	return n
}

// woundBody deals dmg of kind to b on parts picked by coverage. A vital
// part is never destroyed by a hit: what it cannot take spills to the next
// pick, so the body's Damage rises by exactly dmg and death still comes
// at health.MaxHP. Other parts may be destroyed (lost).
func (g *Game) woundBody(b *body.Body, dmg int, kind body.WoundKind, now uint64) {
	rng := g.stream(streamBody)
	left := dmg
	for try := 0; left > 0 && try < maxSpill; try++ {
		drawn := -1
		roll := func(n int) int { // one draw, replayed so Hurt picks the same part
			if drawn < 0 {
				drawn = rng.IntN(n)
			}
			return drawn
		}
		p, ok := b.PickPart(roll)
		if !ok {
			break
		}
		hit := left
		if body.DefOf(p).Vital {
			room := body.DefOf(p).MaxHP - partDmg(b, p) - 1
			if room <= 0 {
				continue
			}
			hit = min(hit, room)
		}
		before := b.Damage()
		b.Hurt(roll, hit, kind, now)
		left -= b.Damage() - before
	}
	if left > 0 { // nothing left to take it: it all lands on the torso
		b.Wounds = append(b.Wounds, body.Wound{Part: body.Torso, Dmg: left, Kind: kind, At: now, Size: int32(left)})
	}
}

// woundKindBy is the kind of wound attacker by deals (0: a trap, a burst
// or a fixture, which bruise).
func (g *Game) woundKindBy(by entity.EntityID) body.WoundKind {
	if by == 0 {
		return body.Bruise
	}
	wk := g.weaponOf(by)
	if a, ok := g.animals.Get(by); ok {
		wk = animal.DefOf(a.Species).Attack
	}
	switch wk {
	case weapon.Knife, weapon.Spear, weapon.Rifle, weapon.Claw, weapon.Gore:
		return body.Cut
	case weapon.Bite:
		return body.Bite
	case weapon.Peck:
		return body.Scratch
	}
	return body.Bruise
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

// capacityLabels name body.Capacity values for the Health tab.
var capacityLabels = [body.NumCaps]string{
	body.Consciousness: "Consciousness", body.Moving: "Moving", body.Manipulation: "Manipulation",
	body.Sight: "Sight", body.Hearing: "Hearing", body.Talking: "Talking", body.Breathing: "Breathing",
}

// Part status words of the Health tab (PartView.Status starts with one).
const (
	PartHealthy    = "healthy"
	PartMissing    = "missing"
	PartReplaced   = "replaced"   // under an artificial limb
	PartArtificial = "artificial" // "artificial: peg leg"
	PartWounded    = "wounded"    // "wounded: 2 wounds (tended), 1 scar"
	PartScarred    = "scarred"
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
		reconcileInjury(b, v.Injury, 0)
	}
	return b
}

// coveredByImplant reports a missing part p standing under an implant on
// one of its ancestors (a foot under a prosthetic leg).
func coveredByImplant(b *body.Body, p body.Part) bool {
	for q, i := body.DefOf(p).Parent, 0; i < int(body.NumParts); q, i = body.DefOf(q).Parent, i+1 {
		if b.Implant(q).IsImplant() {
			return true
		}
		if body.DefOf(q).Parent == q {
			return false
		}
	}
	return false
}

// partStatus is the Health tab's status line for one part.
func partStatus(b *body.Body, s body.PartStatus) string {
	switch {
	case s.Missing && s.Fitted:
		return PartArtificial + ": " + s.Implant.String()
	case s.Missing && coveredByImplant(b, s.Part):
		return PartReplaced
	case s.Missing:
		return PartMissing
	case s.Wounds > 0:
		t := fmt.Sprintf("%s: %d wound%s", PartWounded, s.Wounds, plural(s.Wounds))
		if s.Tended {
			t += " (tended)"
		}
		if s.Scars > 0 {
			t += fmt.Sprintf(", %d scar%s", s.Scars, plural(s.Scars))
		}
		return t
	case s.Scars > 0:
		return fmt.Sprintf("%s: %d scar%s", PartScarred, s.Scars, plural(s.Scars))
	}
	return PartHealthy
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// operationLabel is how the Health tab lists op on body b: fitting an
// implant where the part is missing installs it, where it is still there
// replaces it (SIM-049).
func operationLabel(b *body.Body, op body.Operation) string {
	part := op.Part.Label()
	switch op.Kind {
	case body.InstallImplant:
		if op.Part < body.NumParts && b.Missing&(1<<op.Part) == 0 {
			return fmt.Sprintf("Replace %s with %s", part, op.Item)
		}
		return fmt.Sprintf("Install %s (%s)", op.Item, part)
	case body.RemoveImplant:
		return fmt.Sprintf("Remove implant (%s)", part)
	case body.Amputate:
		return fmt.Sprintf("Amputate %s", part)
	case body.HarvestOrgan: // MED-00 placeholders until MED-06
		return fmt.Sprintf("Harvest %s", part)
	case body.InstallOrgan:
		return fmt.Sprintf("Install organ (%s)", part)
	case body.Euthanize:
		return "Euthanize"
	case body.Anesthetize:
		return "Anesthetize"
	case body.ExciseCarcinoma:
		return fmt.Sprintf("Excise carcinoma (%s)", part)
	}
	return "Operation (" + part + ")"
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
	hv := HealthView{Pain: int(b.Pain()) / 10, Drugs: g.drugLines(id), // drugs.go
		State: healthState(v, b), BloodLossPct: int(b.BloodLoss) / 10, BleedPerDay: int(b.BleedPerDay()) / 10,
		DeathInHours: b.BleedDeathHours()} // bleed.go (MED-01)
	g.conditionRows(id, b, &hv) // MED-02: conditions, the infection headline and its death clock
	if v.Status == health.Dead {
		hv.DeathInHours = -1
	}
	hv.Capacities = make([]CapacityView, 0, body.NumCaps)
	for c := body.Capacity(0); c < body.NumCaps; c++ {
		hv.Capacities = append(hv.Capacities, CapacityView{Label: capacityLabels[c], Pct: int(b.Capacity(c)) / 10})
	}
	ps := b.Parts()
	hv.Parts = make([]PartView, 0, len(ps))
	for _, s := range ps {
		pv := PartView{Label: s.Label, Status: partStatus(b, s), HP: s.HP, Part: int(s.Part),
			Bleed: int(b.PartBleedPerDay(s.Part)) / 10, TendTip: partTendTip(b, s.Part)}
		if s.Fitted {
			pv.Implant = s.Implant.String()
		}
		hv.Parts = append(hv.Parts, pv)
	}
	for i, op := range g.operationsOf(id) {
		why, ready := g.surgeryWait(id, op) // surgery.go: Blocked needs the player
		hv.Operations = append(hv.Operations, OperationView{Label: operationLabel(b, op), Slot: i, Status: why, Blocked: !ready})
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
		out = append(out, OperationOptionView{Label: operationLabel(b, op), Kind: uint8(op.Kind), Part: uint8(op.Part),
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
		add(body.Operation{Kind: body.RemoveImplant, Part: p})
	}
	for p := body.Part(0); p < body.NumParts; p++ {
		add(body.Operation{Kind: body.Amputate, Part: p})
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
	if !missingPart(g.bodies[id]) {
		return nil
	}
	d := mood.DefOf(mood.MissingPart)
	return []mood.Thought{{Kind: mood.MissingPart, Label: d.Label, Value: d.Value}}
}

// woundKindLabels name body.WoundKind values for the Health tab.
var woundKindLabels = [...]string{body.Cut: "cut", body.Bruise: "bruise", body.Bite: "bite", body.Scratch: "scratch",
	body.Burn: "burn", body.Surgical: "surgical cut"}

func woundKindLabel(k body.WoundKind) string {
	if int(k) < len(woundKindLabels) {
		return woundKindLabels[k]
	}
	return "wound"
}

// partTendTip lists p's open wounds with their tend quality, one a line
// ("" when it has none).
func partTendTip(b *body.Body, p body.Part) string {
	var lines []string
	for _, w := range b.Wounds {
		if w.Part != p || w.Permanent {
			continue
		}
		t := fmt.Sprintf("%s, %d HP: ", woundKindLabel(w.Kind), w.Dmg)
		if w.Tended {
			t += fmt.Sprintf("tended %d%%", int(w.TendQ)/10)
		} else {
			t += "untended"
		}
		lines = append(lines, t)
	}
	return strings.Join(lines, "\n")
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
		cv := ConditionView{Label: conditionLabel(c, d), SevPct: int(c.Sev) / 10, ImmPct: int(c.Immunity) / 10,
			TendPct: int(c.TendQ) / 10, Losing: infection.Losing(c, d, tended, imm)}
		if d.TendEvery > 0 {
			if tended {
				cv.DueInHours = int((c.TendUntil - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
			} else {
				cv.Due = true
			}
		}
		cv.Tip = conditionTip(c, d, cv, tended)
		losing = losing || cv.Losing
		hv.Conditions = append(hv.Conditions, cv)
	}
	if hv.BleedPerDay == 0 && hv.State != "Dead" {
		hv.State = "Infection: winning"
		if losing {
			hv.State = "Infection: losing"
		}
	}
	if h := g.condDeathHours(id, b); h >= 0 && (hv.DeathInHours < 0 || h < hv.DeathInHours) {
		hv.DeathInHours = h
	}
}

// conditionLabel is a condition row's name: kind, stage and part
// ("Infection (major), left arm").
func conditionLabel(c body.Condition, d body.CondDef) string {
	l := d.Label
	if l != "" {
		l = strings.ToUpper(l[:1]) + l[1:]
	}
	if int(c.Stage) < len(d.Stages) && d.Stages[c.Stage].Label != "" {
		l += " (" + d.Stages[c.Stage].Label + ")"
	}
	if c.Part != body.Torso || c.Kind == body.CondInfection {
		l += ", " + c.Part.Label()
	}
	return l
}

// conditionTip is a condition row's hover card: severity and immunity,
// the tend and its quality, and how the race goes.
func conditionTip(c body.Condition, d body.CondDef, cv ConditionView, tended bool) string {
	t := fmt.Sprintf("%s\nSeverity %d%% · immunity %d%%.", cv.Label, cv.SevPct, cv.ImmPct)
	switch {
	case tended:
		t += fmt.Sprintf("\nTended at %d%% quality: slowed by %d%% a day for %d more hours.", cv.TendPct, int(d.TendMax*c.TendQ/1000)/10, cv.DueInHours)
	case d.TendEvery > 0 && c.TendQ > 0:
		t += fmt.Sprintf("\nLast tended at %d%% quality; it needs tending again now.", cv.TendPct)
	case d.TendEvery > 0:
		t += "\nUntended: it needs a doctor now."
	}
	switch {
	case c.Immunity >= infection.Full:
		t += "\nImmune: it is receding."
	case cv.Losing:
		t += "\nLosing: at this rate it kills before immunity builds. Better tends (skill, medicine, a hospital bed) slow it."
	default:
		t += "\nWinning: immunity should build before it gets worse."
	}
	return t
}
