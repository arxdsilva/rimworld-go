package game

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Work priorities and per-colonist schedules. A colonist looks for work
// priority by priority (1 first, then 2, ...); among work types with the
// same priority it follows work.Order. Each colonist has its own 24-hour
// schedule (a copy of the colony default when it arrives).

func workTypeOf(c command.WorkType) (work.Type, bool) {
	switch c {
	case command.WorkHaul:
		return work.Haul, true
	case command.WorkConstruct:
		return work.Construct, true
	case command.WorkGrow:
		return work.Grow, true
	case command.WorkMine:
		return work.Mine, true
	case command.WorkCook:
		return work.Cook, true
	case command.WorkHunt:
		return work.Hunt, true
	case command.WorkCraft:
		return work.Craft, true
	case command.WorkDoctor:
		return work.Doctor, true
	case command.WorkResearch:
		return work.Research, true
	case command.WorkClean:
		return work.Clean, true
	case command.WorkHandle:
		return work.Handle, true
	case command.WorkWarden:
		return work.Warden, true
	case command.WorkFirefight:
		return work.Firefight, true // firefight.go
	}
	return 0, false
}

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

// assignWorkByPriority starts the most urgent work the colonist can do.
func (g *Game) assignWorkByPriority(id entity.EntityID) bool {
	pr := g.prioOf(id)
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
		return g.haulUrgent(id) || g.assignRefuel(id) || g.fillTroughs(id) || g.assignBury(id) || g.assignHaul(id) // haul_priority.go, trough_feed.go
	case work.Construct:
		return g.assignFlick(id) || // power_flick.go (PLAY-021): flicks first
			g.assignConstruct(id) || g.assignRepair(id) || g.assignRoof(id) || g.assignSmooth(id) // smooth.go
	case work.Cook, work.Craft:
		return t == work.Cook && g.fillHoppers(id) || g.craft.Shop.Assign(id, work.Mask(0).Set(t, true)) // haul_priority.go, craft/shop.go
	case work.Hunt:
		return g.combat.Hunting.Assign(id) // combat/hunt.go
	case work.Handle: // SIM-026: taming moved here from Hunt
		return g.assignRope(id) || g.herdOf().AssignTame(id) || g.assignTrain(id) || g.herdOf().AssignRest(id) // rope.go, herd, bonds.go
	case work.Grow:
		return g.assignHarvest(id) || g.assignSow(id)
	case work.PlantCut: // FOOD-2: cutting split from growing (wiki: Plants)
		return g.assignCut(id)
	case work.Mine:
		return g.assignMine(id)
	case work.Research:
		return g.assignResearch(id)
	case work.Clean:
		return g.assignClean(id) || g.assignClearSnow(id) // clean.go, snow_clear.go
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
