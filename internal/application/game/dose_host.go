package game

import (
	"rimworld-go/internal/application/game/dose"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// doseOf is game/dose over the game: g.drugs holds each colonist's drug
// state and policy, saved as Drugs (below). Small adapters, pointer-shaped,
// so that building the component where it is needed allocates nothing.
func doseOf(g *Game) dose.Drugs {
	return dose.New(dose.Host{Pawns: dosePawns{g}, Minds: doseMinds{g}, Bodies: doseBodies{g}, Stock: doseStock{g}}, &g.drugs, idleRecheck)
}

// DrugPolicy is colonist id's drug policy, a drug.Preset (the Assign tab's
// read model).
func (g *Game) DrugPolicy(id entity.EntityID) uint8 { return uint8(doseOf(g).Policy(id)) }

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

type dosePawns struct{ g *Game }

func (h dosePawns) Now() uint64                      { return uint64(h.g.clock.Now()) }
func (h dosePawns) Colonist(id entity.EntityID) bool { return h.g.colonist(id) }
func (h dosePawns) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h dosePawns) Job(id entity.EntityID) (job.Record, bool)      { return h.g.jobs.Get(id) }
func (h dosePawns) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h dosePawns) StartJob(id entity.EntityID, r job.Record) bool { return h.g.startJob(id, r) }
func (h dosePawns) Interrupt(id entity.EntityID, why job.FailReason) {
	h.g.interrupt(id, why)
}

type doseMinds struct{ g *Game }

func (h doseMinds) Joy(id entity.EntityID) int         { return h.g.joy[id] }
func (h doseMinds) SetJoy(id entity.EntityID, joy int) { h.g.joy[id] = joy }
func (h doseMinds) Needs(id entity.EntityID) (needs.Pair, bool) {
	return h.g.need.Get(id)
}
func (h doseMinds) SetNeeds(id entity.EntityID, n needs.Pair) { h.g.need.Set(id, n) }
func (h doseMinds) Traits(id entity.EntityID) []trait.ID {
	if fx := h.g.sk.stats[id]; fx != nil {
		return fx.traits
	}
	return nil
}
func (h doseMinds) Remember(id entity.EntityID, k mood.Kind) { h.g.remember(id, k) }
func (h doseMinds) LogIngested(id entity.EntityID, name string) {
	h.g.logEvent(id, logEntry{Kind: logIngested, Name: name})
}
func (h doseMinds) DrugRolls() func(int) int { return h.g.stream(streamDrug).IntN }

type doseBodies struct{ g *Game }

func (h doseBodies) Body(id entity.EntityID) *body.Body { return h.g.bodies[id] }
func (h doseBodies) Vital(id entity.EntityID) (health.Record, bool) {
	return h.g.vital.Get(id)
}
func (h doseBodies) EnsureBody(id entity.EntityID, v health.Record) *body.Body {
	return h.g.ensureBody(id, &v)
}
func (h doseBodies) Settle(id entity.EntityID, b *body.Body, v health.Record) {
	h.g.bodyChanged(id, b, &v) // body.go: stats follow consciousness
	h.g.updateDowned(id, b, &v)
}
func (h doseBodies) DieOfOverdose(id entity.EntityID) {
	g := h.g
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s died of an overdose.", g.name(id))
	g.dieMedical(id, health.CauseOverdose) // caravan_medical.go
	g.clearPatient(id)
}
func (h doseBodies) Vomit(x, y int)        { h.g.addDirt(y*h.g.grid.Width()+x, vomitDirt) } // filth.go: as a disease's vomit
func (h doseBodies) MedicalRoll(n int) int { return h.g.stream(streamMedical).IntN(n) }

type doseStock struct{ g *Game }

func (h doseStock) ItemIDs() []entity.EntityID { return h.g.itemIDs }
func (h doseStock) Stack(id entity.EntityID) (item.Stack, bool) {
	return h.g.stacks.Get(id)
}
func (h doseStock) PutStack(id entity.EntityID, st item.Stack) { h.g.putStack(id, st) }
func (h doseStock) DeleteItem(id entity.EntityID)              { h.g.deleteItem(id) }
func (h doseStock) EntityOwner(id entity.EntityID) (entity.EntityID, bool) {
	return h.g.reserve.EntityOwner(id)
}
