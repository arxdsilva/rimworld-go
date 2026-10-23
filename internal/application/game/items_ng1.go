package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// Components and the NG-1 buildings in play (NG-1 §4.8, §5.6).

// componentRefund is the components deconstruction returns: half, rounded
// down.
func componentRefund(k construction.Kind) int { return construction.ComponentCost(k) / 2 }

// refundStuff is the material a deconstructed building returns. A placed
// kind with a salvage (the drop pod's shell) is steel whatever stuff it
// was recorded with.
func refundStuff(b construction.Building) item.Kind {
	if d := construction.DefOf(b.Kind); d.Salvage > 0 && d.MetalOnly {
		return item.Steel
	}
	return b.Stuff
}

// supplyKind is what a hand-ordered delivery to frame fr brings: its
// material until that is in, then its components.
func supplyKind(fr construction.Frame) item.Kind {
	if fr.Wood >= fr.Need && fr.Comp < fr.CompNeed {
		return item.Components
	}
	return fr.Stuff
}

// supplyNeed is how much of kind k frame fr still lacks.
func supplyNeed(fr construction.Frame, k item.Kind) int {
	if k == item.Components {
		return fr.CompNeed - fr.Comp
	}
	return fr.Need - fr.Wood
}

// supplyHave is how much of kind k frame fr already holds.
func supplyHave(fr construction.Frame, k item.Kind) int {
	if k == item.Components {
		return fr.Comp
	}
	return fr.Wood
}

// Medicine tiers, the hospital bed and the tool cabinet (NG-1 §4.8).

// severeInjury: an injury this bad, or any infection, is treated with
// industrial medicine when there is some.
const severeInjury = 50

const (
	// bedHealPct is a tend's healing for a patient lying in a hospital bed;
	// bedInfectionPct the share of the infection chance that remains.
	bedHealPct      = 115
	bedInfectionPct = 50
	// cabinetPct is the bench work bonus of one tool cabinet; cabinetMax
	// cabinets count; cabinetReach is how near (Chebyshev tiles) they must be.
	cabinetPct   = 6
	cabinetMax   = 2
	cabinetReach = 6
)

// isMedicine reports a medicine item, either tier.
func isMedicine(k item.Kind) bool { return k == item.HerbalMedicine || k == item.IndustrialMedicine }

// medicineTiers is the medicine kinds a tend for patient may use, best
// first: industrial for a severe injury or an infection, herbal otherwise,
// each falling back to the other.
func (g *Game) medicineTiers(patient entity.EntityID) [2]item.Kind {
	v, _ := g.vital.Get(patient)
	if v.Injury >= severeInjury || g.wounds[patient].Infection > 0 {
		return [2]item.Kind{item.IndustrialMedicine, item.HerbalMedicine}
	}
	return [2]item.Kind{item.HerbalMedicine, item.IndustrialMedicine}
}

// inHospitalBed reports the patient lying in a hospital bed.
func (g *Game) inHospitalBed(patient entity.EntityID) bool {
	bid := g.bedUnder(patient)
	if bid == 0 {
		return false
	}
	b, _ := g.bldgs.Get(bid)
	return b.Kind == construction.HospitalBed
}

// infectionChance is the percent chance a tend with care c leaves a
// wound infected: halved (rounded up) for a patient in a hospital bed.
func (g *Game) infectionChance(patient entity.EntityID, c infection.Care) int {
	pct := infection.Chance(c)
	if g.inHospitalBed(patient) {
		pct = (pct*bedInfectionPct + 50) / 100
	}
	return pct
}

// tendCareOf is the treatment doctor's current tend job gives: what it
// carries, or none yet. The patient is the job's target.
func (g *Game) tendCareOf(doctor entity.EntityID) (patient entity.EntityID, c infection.Care, ok bool) {
	j, found := g.jobs.Get(doctor)
	if !found || j.Kind != job.Tend {
		return 0, infection.Untreated, false
	}
	return j.Target, g.tendCare(doctor, j.Target, j), true
}

// medicineHealPct is the percent of a plain tend's healing (TendHeal)
// that doctor's current tend job mends: the medicine tier it carries
// times the hospital bed bonus. 100 when doctor is not tending.
func (g *Game) medicineHealPct(doctor entity.EntityID) int {
	patient, c, ok := g.tendCareOf(doctor)
	if !ok {
		return 100
	}
	pct := infection.HealPct(c)
	if g.inHospitalBed(patient) {
		pct = pct * bedHealPct / 100
	}
	return pct
}

// medicineInfectionPct is the percent chance that doctor's current tend
// leaves the wound infected (care tier, halved in a hospital bed). 0 when
// doctor is not tending.
func (g *Game) medicineInfectionPct(doctor entity.EntityID) int {
	patient, c, ok := g.tendCareOf(doctor)
	if !ok {
		return 0
	}
	return g.infectionChance(patient, c)
}

// cabinetState caches tool cabinet counts per bench for one structVer.
type cabinetState struct {
	ver uint64
	by  map[entity.EntityID]int
}

// cabinetBonus is the percent bill-work bonus of bench from tool cabinets
// within cabinetReach tiles (cabinetPct each, at most cabinetMax). It is
// cached per bench until a building or terrain change moves structVer.
func (g *Game) cabinetBonus(bench entity.EntityID) int {
	c := &g.cabinets
	if c.by == nil || c.ver != g.structVer {
		c.by, c.ver = map[entity.EntityID]int{}, g.structVer
	}
	if pct, ok := c.by[bench]; ok {
		return pct
	}
	pct := g.countCabinets(bench) * cabinetPct
	c.by[bench] = pct
	return pct
}

func (g *Game) countCabinets(bench entity.EntityID) int {
	b, ok := g.bldgs.Get(bench)
	if !ok {
		return 0
	}
	n := 0
	for _, bid := range g.bldgIDs {
		o, ok := g.bldgs.Get(bid)
		if !ok || o.Kind != construction.ToolCabinet || chebyshev(Position{X: o.X, Y: o.Y}, Position{X: b.X, Y: b.Y}) > cabinetReach {
			continue
		}
		if n++; n == cabinetMax {
			break
		}
	}
	return n
}

// minedOres is every ore a rock cell can hold, in code order: the metal
// veins of geology.Ores() and the buried machinery (WorldView.Ore codes,
// OreNames, SetGround).
func minedOres() []geology.Ore { return append(geology.Ores(), geology.Machinery) }
