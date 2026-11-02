package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// upkeep is how often flights, sites and cargo are looked at (ticks).
const upkeep = 250

// Apply runs one of travel's commands (EnterCasket, EjectCasket,
// LoadPods, LaunchPods, SetScannerTarget).
func (o Ops) Apply(c command.Command) {
	switch c.Kind {
	case command.EnterCasket:
		caskets(o).cmdEnter(c)
	case command.EjectCasket:
		caskets(o).cmdEject(c)
	case command.LoadPods:
		pods(o).cmdLoad(c)
	case command.LaunchPods:
		pods(o).cmdLaunch(c)
	case command.SetScannerTarget:
		scanner(o).cmdTarget(c)
	}
}

// Do is one step of one of travel's jobs (EnterCasket, CarryToCasket,
// LoadPods, ScanMinerals).
func (o Ops) Do(id entity.EntityID, j job.Record) {
	switch j.Kind {
	case job.EnterCasket:
		caskets(o).doEnter(id, j)
	case job.CarryToCasket:
		caskets(o).doCarry(id, j)
	case job.LoadPods:
		if j.Dest == 0 && j.BillIndex != 0 {
			packing(o).doPack(id, j) // a caravan's load (pack.go)
			return
		}
		pods(o).doLoad(id, j)
	case job.ScanMinerals:
		scanner(o).doScan(id, j)
	}
}

// TickPass is travel's upkeep at tick now on one map's pass (a nil State
// has nothing to do). MAP-0: boarders, led animals and the caskets every
// loadUpkeep ticks are the current map's; the flights, sites, cargo and
// packing every upkeep ticks are the colony's, run only with colony set
// (the first map's pass).
func (o Ops) TickPass(now uint64, colony bool) {
	if o.s == nil {
		return
	}
	if now%loadUpkeep == 0 && (len(o.s.wants) > 0 || len(o.s.roped) > 0) {
		pods(o).loadTick()
	}
	if now%loadUpkeep == 0 {
		caskets(o).fuseTick(now)
	}
	if !colony || now%upkeep != 0 {
		return
	}
	if len(o.s.flights) > 0 {
		pods(o).flightTick(now)
	}
	if len(o.s.sites) > 0 {
		scanner(o).siteTick(now)
	}
	if len(o.s.cargo) > 0 {
		pods(o).cargoTick()
	}
	if len(o.s.packs) > 0 {
		packing(o).packTick()
	}
}

// The concerns' entry points the game calls.

// Sleeper is who lies in casket id (0: empty).
func (o Ops) Sleeper(id entity.EntityID) entity.EntityID { return o.s.caskets[id] }

// AssignLoad starts hauler id on what a pod group waits for (Haul work).
func (o Ops) AssignLoad(id entity.EntityID) bool { return pods(o).AssignLoad(id) }

// AssignScan starts researcher id on a mineral scanner (Research work).
func (o Ops) AssignScan(id entity.EntityID, tries int) bool { return scanner(o).AssignScan(id, tries) }

// Home sets caravan cid's pod cargo down around (x, y).
func (o Ops) Home(cid uint64, x, y int) { pods(o).Home(cid, x, y) }

// LaunchReason is why pod's group would not fly to dest in mode.
func (o Ops) LaunchReason(pod entity.EntityID, dest int, mode uint8) (string, int) {
	return pods(o).LaunchReason(pod, dest, mode)
}

// Cycle is mineral scanner id's progress to its next find, permille.
func (o Ops) Cycle(id entity.EntityID) int { return scanner(o).Cycle(id) }

// Find lays a mineral site now (dev tools and tests).
func (o Ops) Find(target item.Kind) bool { return scanner(o).Find(target) }

// Hitch sends member i of forming caravan cid to a hitching spot.
func (o Ops) Hitch(cid uint64, member entity.EntityID, i int) bool {
	return hitching(o).Hitch(cid, member, i)
}

// Gather is forming caravan cid's hitching check (hitch.go).
func (o Ops) Gather(cid uint64, members []entity.EntityID, formed, now uint64) bool {
	return hitching(o).Gather(cid, members, formed, now)
}

// Gathering reports caravan cid waiting at a hitching spot.
func (o Ops) Gathering(cid uint64) bool { return o.s.hitch[cid] != nil }
