// Package aqua is fish farming in tanks (FISH-*, .cursor/FISH_TANK_PLAN.md
// §3): the state of each tank on a map, stepped hourly over its own list, the
// handler's work (work.Handle on the Animals skill), the player's stock
// choice, and the read model. The rules live in domain/building/aqua.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package; the roles in
// deps.go are its only way in. It draws no dice, reads time only from the
// host's tick, and walks tanks in id order.
package aqua

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	rules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Entry is one tank's saved state. A tank has an entry from the moment the
// player picks a stock for it, and none before: an unused tank costs nothing.
type Entry struct {
	ID   entity.EntityID
	Tank rules.Tank
}

// Tanks is every tank in use on one map, sorted by id.
type Tanks struct {
	h    Host
	list []Entry
}

// New builds an empty set of tanks on host h.
func New(h Host) *Tanks { return &Tanks{h: h} }

// Import is the saved tanks on host h (nil for none).
func Import(h Host, saved []Entry) *Tanks {
	if len(saved) == 0 {
		return nil
	}
	return &Tanks{h: h, list: slices.Clone(saved)}
}

// Export is the state to save (nil for none, so a colony without tanks saves
// and hashes as it did before tanks existed).
func (ts *Tanks) Export() []Entry {
	if ts == nil || len(ts.list) == 0 {
		return nil
	}
	return slices.Clone(ts.list)
}

// entry is tank id's entry (nil: none).
func (ts *Tanks) entry(id entity.EntityID) *Entry {
	i, ok := slices.BinarySearchFunc(ts.list, id, func(e Entry, id entity.EntityID) int {
		switch {
		case e.ID < id:
			return -1
		case e.ID > id:
			return 1
		}
		return 0
	})
	if !ok {
		return nil
	}
	return &ts.list[i]
}

// Stocked reports fish living in tank id: its feed burns (game.burning).
func (ts *Tanks) Stocked(id entity.EntityID) bool {
	e := ts.entry(id)
	return e != nil && e.Tank.Stocked
}

// WantsFeed reports a tank with a stock chosen, so haulers bring it feed.
func (ts *Tanks) WantsFeed(id entity.EntityID) bool { return ts.entry(id) != nil }

// Phase steps every tank in use once an hour.
func (ts *Tanks) Phase() {
	if ts.h.Now()%clock.TicksPerHour != 0 {
		return
	}
	for i := range ts.list {
		e := &ts.list[i]
		if !e.Tank.Stocked {
			continue
		}
		x, y, ok := ts.h.Tank(e.ID)
		if !ok {
			continue
		}
		s := e.Tank.Species
		var out rules.Outcome
		e.Tank, out = e.Tank.Step(clock.TicksPerHour, ts.conditions(e.ID, x, y))
		switch {
		case out.Died:
			ts.h.Message(x, y, "%s", diedText(s, out.Cause))
		case out.Warn:
			ts.h.Message(x, y, "%s", dyingText(s, out.Cause))
		}
	}
}

// conditions is what tank id at (x, y) has right now.
func (ts *Tanks) conditions(id entity.EntityID, x, y int) rules.Conditions {
	return rules.Conditions{Power: ts.h.Powered(id), Air: ts.h.Air(x, y), Fed: ts.h.Feed(id) > 0}
}

// Apply is command.SetTankStock: tank id raises stock s from now on. A new
// choice empties the tank, the fish already in it going as they would with
// the tank itself (Gone); None leaves it empty and asks for nothing.
func (ts *Tanks) Apply(id entity.EntityID, s rules.Species) {
	x, y, ok := ts.h.Tank(id)
	if !ok || s >= rules.NumSpecies {
		return
	}
	e := ts.entry(id)
	switch {
	case e == nil && s == rules.None:
	case e == nil:
		ts.list = append(ts.list, Entry{ID: id, Tank: rules.Tank{Species: s}})
		slices.SortFunc(ts.list, func(a, b Entry) int { return int(a.ID) - int(b.ID) })
	case e.Tank.Species == s:
	case s == rules.None:
		ts.Gone(id, x, y)
	default:
		ts.spill(e, x, y)
		e.Tank = rules.Tank{Species: s}
	}
}

// Gone forgets tank id, removed at (x, y): a ripe stock is left on the ground
// (netted by nobody, so fumbled as by an unskilled hand), a growing one is
// lost. An uninstalled or moved tank goes the same way: its new place starts
// empty, with no stock chosen.
func (ts *Tanks) Gone(id entity.EntityID, x, y int) {
	e := ts.entry(id)
	if e == nil {
		return
	}
	ts.spill(e, x, y)
	ts.list = slices.DeleteFunc(ts.list, func(o Entry) bool { return o.ID == id })
}

// spill leaves a ripe stock's fish at (x, y).
func (ts *Tanks) spill(e *Entry, x, y int) {
	if e.Tank.Ripe() {
		ts.h.DropFish(x, y, rules.NetYield(e.Tank.Species, 0))
	}
}

// due is the work tank e asks of a handler at Animals level (None: nothing).
func (ts *Tanks) due(e *Entry, level int) job.Kind {
	t := e.Tank
	if !t.Species.Valid() || level < rules.DefOf(t.Species).MinSkill {
		return job.None
	}
	switch {
	case t.Ripe():
		return job.NetTank
	case !t.Stocked && ts.h.Powered(e.ID):
		return job.StockTank
	}
	return job.None
}

// Assign is the Handle work giver: id stocks or nets the nearest tank that
// asks for it and no one holds. A tank out of reach is put off (the retry
// cooldown), so a walled-in tank is not tried again every tick.
func (ts *Tanks) Assign(id entity.EntityID) bool {
	if len(ts.list) == 0 {
		return false
	}
	level := ts.h.Animals(id)
	px, py, _ := ts.h.Pos(id)
	for {
		var pick *Entry
		var kind job.Kind
		bestD := 1 << 30
		for i := range ts.list {
			e := &ts.list[i]
			k := ts.due(e, level)
			if k == job.None || !ts.h.Workable(e.ID) {
				continue
			}
			if _, held := ts.h.Owner(e.ID); held || ts.h.Postponed(id, k, e.ID) {
				continue
			}
			x, y, ok := ts.h.Tank(e.ID)
			if !ok {
				continue
			}
			if d := mathx.Abs(x-px) + mathx.Abs(y-py); d < bestD {
				pick, kind, bestD = e, k, d
			}
		}
		if pick == nil {
			return false
		}
		sx, sy, ok := ts.h.BesideTank(id, pick.ID)
		if !ok {
			ts.h.PutOff(id, kind, pick.ID)
			continue
		}
		work := rules.StockWork
		if kind == job.NetTank {
			work = rules.NetWork
		}
		return ts.h.StartJob(id, job.Record{Kind: kind, Target: pick.ID, TX: sx, TY: sy, WorkLeft: work})
	}
}

// Do runs one tick of a tank job. It reads no power and no health: once begun
// a job runs to its end whatever the grid does, so a blinking grid cannot
// start and stop work.
func (ts *Tanks) Do(id entity.EntityID, j job.Record) {
	e := ts.entry(j.Target)
	if _, _, ok := ts.h.Tank(j.Target); !ok || e == nil {
		j.State, j.Fail = job.Fail, job.TargetGone
		ts.h.SetJob(id, j)
		return
	}
	if j.Kind == job.NetTank && !e.Tank.Ripe() || j.Kind == job.StockTank && (e.Tank.Stocked || !e.Tank.Species.Valid()) {
		j.State = job.Complete // the player changed the stock on the way
		ts.h.SetJob(id, j)
		return
	}
	u := ts.h.Work(id)
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		ts.h.SetJob(id, j)
		return
	}
	if j.Kind == job.NetTank {
		ts.h.DropFish(j.TX, j.TY, rules.NetYield(e.Tank.Species, ts.h.Animals(id)))
		e.Tank = e.Tank.Netted()
	} else {
		e.Tank = e.Tank.Fill()
	}
	j.State = job.Complete
	ts.h.SetJob(id, j)
}
