package game

import (
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// Work priorities and per-colonist schedules. A colonist looks for work
// priority by priority (1 first, then 2, ...); among work types with the
// same priority it follows work.Order. Each colonist has its own 24-hour
// schedule (a copy of the colony default when it arrives).

// prioOf is a colonist's priorities (derived from its enable mask for
// colonists made before priorities existed).
func (g *Game) prioOf(id entity.EntityID) work.Priorities {
	p, ok := g.prio[id]
	if !ok {
		m, _ := g.labor.Get(id)
		for t := work.Type(0); t < work.NumTypes; t++ {
			if m.Enabled(t) {
				p[t] = work.DefaultPriority
			}
		}
	}
	if inc := g.incapableWork(id); inc != 0 { // skills.go: never incapable work
		for t := work.Type(0); t < work.NumTypes; t++ {
			if inc.Enabled(t) {
				p[t] = 0
			}
		}
	}
	return p
}

func (g *Game) setPriority(id entity.EntityID, t work.Type, v int) {
	if _, ok := g.labor.Get(id); !ok {
		return
	}
	if v > 0 && g.incapableWork(id).Enabled(t) {
		g.incapableToast(id, t) // skills.go: ignored, "Incapable: <reason>"
		return
	}
	p := g.prioOf(id).Set(t, v)
	g.prio[id] = p
	g.labor.Set(id, p.Mask())
}

// assignWorkByPriority starts the most urgent work the colonist can do. Only a fire
// and a patient come before a harvest sweep, and leave the mark (HRV-001).
func (g *Game) assignWorkByPriority(id entity.EntityID) bool {
	pr := g.prioOf(id)
	if g.haul.sweep.Armed(id) && (pr[work.Firefight] > 0 && g.assignWorkType(id, work.Firefight) ||
		pr[work.Doctor] > 0 && g.care.Doctor.Assign(id) || g.haul.sweep.Next(haulHost{g}, id)) {
		return true
	}
	for p := uint8(1); p <= work.Lowest; p++ {
		for _, t := range work.Order {
			if pr[t] == p && g.assignWorkType(id, t) {
				return true
			}
		}
	}
	return false
}

func (g *Game) assignWorkType(id entity.EntityID, t work.Type) bool {
	if g.incapableWork(id).Enabled(t) {
		return false // skills.go: incapable work is never assigned
	}
	switch t {
	case work.Firefight:
		return g.assignFightFire(id) // firefight.go
	case work.Doctor:
		// MED-03: rescue, then tend the bleeding, then the other tends
		// (medical.go), then operations, then animals, then doctoring
		// bills (refining medicine) at a bench
		return g.care.Doctor.Assign(id) ||
			g.assignSurgery(id) || g.assignTendAnimal(id) || // surgery.go, vet_animals.go
			g.craft.Bills.AnyFor(work.Doctor) && g.craft.Shop.Assign(id, work.Mask(0).Set(work.Doctor, true)) // craft package
	case work.Warden:
		return g.prison.Assign(id) // prison/warden.go
	case work.Haul:
		return g.haulUrgent(id) || g.assignRefuel(id) || ordersOf(g).AssignHaul(id) || travelOf(g).AssignLoad(id) || travelOf(g).AssignPack(id) || g.assignBury(id) || g.fillHoppers(id) || g.fillTroughs(id) ||
			g.craft.Bills.AnyFor(work.Haul) && g.craft.Shop.Assign(id, work.Mask(0).Set(work.Haul, true)) || g.assignHaul(id) // WRK-007: urgent (STO-2), refuel, open, strip (AX-1), travel load (AX-9), corpses, hoppers (FOOD-4), troughs, bills, general
	case work.Construct:
		return g.assignFlick(id) || // power_flick.go (PLAY-021): flicks first; WRK-004 and WRK-003 for the rest
			ordersOf(g).AssignConstruct(id) || g.assignRoof(id) || g.assignConstruct(id) || g.assignDelivery(id) || g.assignRepair(id) || g.assignSmooth(id) // AX-1: uninstall, install
	case work.Cook, work.Craft:
		return g.craft.Shop.Assign(id, work.Mask(0).Set(t, true)) // craft/shop.go
	case work.Hunt:
		return g.combat.Hunting.Assign(id) // combat/hunt.go
	case work.Handle: // SIM-026: taming moved here from Hunt; WRK-005: slaughter and collecting come before it
		return g.assignRope(id) || g.herdOf().AssignRest(id) || g.herdOf().AssignTame(id) || g.assignTrain(id) || (g.tanks != nil && g.tanks.Assign(id)) // rope.go, herd, bonds.go, game/aqua
	case work.Grow:
		return haul.Harvest(haulHost{g}, id, haul.RipeCrops) || g.assignSow(id) // game/haul
	case work.PlantCut: // FOOD-2: cutting split from growing (wiki: Plants); HRV-003: wild harvests first
		return haul.Harvest(haulHost{g}, id, haul.RipeWild) || g.assignCut(id)
	case work.Mine:
		return g.assignMine(id) || g.br.ind.Ops(industryHost{g}).Assign(id, true, workTries) // game/industry: deep drills (AX-4)
	case work.Research:
		return g.lab.Assign(id, workTries) || g.br.ind.Ops(industryHost{g}).Assign(id, false, workTries) || travelOf(g).AssignScan(id, workTries) // game/research, game/industry and game/travel scanners
	case work.Clean:
		return g.snowOf().AssignClear(id) || cleaningOf(g).Assign(id) // WRK-006; game/snow, game/sense
	}
	return false
}

// scheduleOf is a colonist's daily schedule.
func (g *Game) scheduleOf(id entity.EntityID) schedule.Grid {
	if s, ok := g.scheds[id]; ok {
		return s
	}
	return g.sched
}

// activityOf is what the colonist's schedule says for the current hour.
func (g *Game) activityOf(id entity.EntityID) schedule.Activity {
	return g.scheduleOf(id).At(g.Date().Hour)
}

// setSchedule paints one hour of a colonist's schedule (id 0: everyone,
// and the default for colonists still to come).
func (g *Game) setSchedule(id entity.EntityID, hour int, a schedule.Activity) {
	if hour < 0 || hour >= len(g.sched) || a < schedule.Anything || a > schedule.Sleep {
		return
	}
	if id == 0 {
		g.sched[hour] = a
		for _, pid := range g.pawnIDs {
			s := g.scheduleOf(pid)
			s[hour] = a
			g.scheds[pid] = s
		}
		return
	}
	s := g.scheduleOf(id)
	s[hour] = a
	g.scheds[id] = s
}
