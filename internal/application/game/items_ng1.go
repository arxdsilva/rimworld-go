package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
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

// Medicine tiers and the hospital bed (NG-1 §4.8).

// severeInjury: an injury this bad, or any infection, is treated with
// industrial medicine when there is some.
const severeInjury = 50

const (
	// hospitalTendOffset is added to the quality of a tend on a patient
	// in a hospital bed (permille; MED-02). wiki: Hospital bed (+10 %
	// medical tend quality). Lying in it also heals faster
	// (body.LieHospital, lyingHeal in infection.go).
	hospitalTendOffset = 100
)

// isMedicine reports a medicine item, either tier.
func isMedicine(k item.Kind) bool { return k.IsMedicine() } // herbal, industrial, nanomed kit (MED-05)

// medicineTiers is the medicine kinds a tend for patient may use, in the
// order tried: the best first (nanomed kit, then industrial) for a severe
// injury or an infection, the cheapest first otherwise; the care level
// filters them (withMedicine).
func (g *Game) medicineTiers(patient entity.EntityID) [3]item.Kind {
	v, _ := g.vital.Get(patient)
	if _, infected := g.worstInfection(patient); v.Injury >= severeInjury || infected {
		return [3]item.Kind{item.NanomedKit, item.IndustrialMedicine, item.HerbalMedicine}
	}
	return [3]item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit}
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

// monitorCache caches each hospital bed's vitals monitor (MED-06) for one
// structVer.
type monitorCache struct {
	ver uint64
	mon map[entity.EntityID]entity.EntityID
}

// fresh resets the cache when a building or terrain change moved
// structVer.
func (c *monitorCache) fresh(ver uint64) {
	if c.mon == nil || c.ver != ver {
		c.mon, c.ver = map[entity.EntityID]entity.EntityID{}, ver
	}
}

// bedMonitor is the vitals monitor serving hospital bed bed: the lowest
// id one touching its footprint (0: none, or not a hospital bed; MED-06).
// It is cached per bed until a building or terrain change moves
// structVer, so the scan runs once per bed after a build or deconstruct.
func (g *Game) bedMonitor(bed entity.EntityID) entity.EntityID {
	c := &g.monitors
	c.fresh(g.structVer)
	if m, ok := c.mon[bed]; ok {
		return m
	}
	m := g.findMonitor(bed)
	c.mon[bed] = m
	return m
}

func (g *Game) findMonitor(bed entity.EntityID) entity.EntityID {
	b, ok := g.bldgs.Get(bed)
	if !ok || b.Kind != construction.HospitalBed {
		return 0
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	var best entity.EntityID
	for _, bid := range g.bldgIDs {
		o, ok := g.bldgs.Get(bid)
		if !ok || o.Kind != construction.VitalsMonitor || best != 0 && bid > best {
			continue
		}
		for _, c := range cells {
			if abs(o.X-c[0]) <= 1 && abs(o.Y-c[1]) <= 1 {
				best = bid
				break
			}
		}
	}
	return best
}

// minedOres is every ore a rock cell can hold, in code order: the metal
// veins of geology.Ores() and the buried machinery (WorldView.Ore codes,
// OreNames, SetGround).
func minedOres() []geology.Ore { return append(geology.Ores(), geology.Machinery) }
