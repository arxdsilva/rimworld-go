package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Rearms (DEF-3) are the hauls that keep the defences shooting: an idle
// colonist (not drafted) fetches steel for a turret's or a mortar's new
// barrel, or shells for a mortar's rack, carries it there and works it
// in (job RearmTurret). Every RearmScanEvery ticks each turret whose
// barrel is down to a quarter and each mortar that is spent or under half
// a rack gets one hauler, so a defence asks once and is topped up to full
// (no flapping between asking and not). A hauler is looked for through
// the creature index around the building, the stock through the host.
type Rearms struct {
	h       Host
	sup     Supply
	br      Breaches
	turrets *Turrets
	mortars *Mortars
	jobs    []Rearm // under way, ascending by building (saved)
	near    []Seen
}

// Rearm is one haul under way: Pawn brings Kind to Building.
type Rearm struct {
	Building, Pawn entity.EntityID
	Kind           item.Kind
}

const (
	// RearmScanEvery is how often the defences are checked for a rearm.
	RearmScanEvery = 250
	// RearmWork is the work of fitting a barrel or loading shells.
	// original tuning: the wiki gives no rearm time.
	RearmWork = 120
	// rearmReach is how far a hauler is looked for.
	rearmReach = 60
)

// BarrelCost is what a new barrel costs for building kind k: the wiki's
// 80 steel (mini-turret), 180 steel (autocannon) and 60 uranium (slug
// turret), scaled by 9 as construction costs are; the mortar takes one
// reinforced barrel (original count). Neither uranium nor reinforced
// barrels have a source in the colony yet (no ore, recipe or trader), so
// a rearm falls back on the mini-turret's steel when none is in stock
// (barrel: original stand-in). wiki: Mini-turret, Autocannon turret,
// Uranium slug turret
func BarrelCost(k construction.Kind) (item.Kind, int) {
	switch k {
	case construction.Autocannon:
		return item.Steel, 20
	case construction.UraniumSlugTurret:
		return item.Uranium, 7
	case construction.Mortar:
		return item.ReinforcedBarrel, 1
	}
	return item.Steel, 9
}

// shellOrder is the order shells are fetched in.
var shellOrder = [...]item.Kind{item.MortarShells, item.IncendiaryShells, item.EMPShells, item.SmokeShells}

// phase drops the hauls that are over and, on scan ticks, starts new
// ones (the game's job phase advances them: Do).
func (r *Rearms) phase(now uint64) {
	if len(r.jobs) > 0 {
		r.jobs = slices.DeleteFunc(r.jobs, r.over)
	}
	if now%RearmScanEvery == 0 && r.sup != nil {
		r.scan(now)
	}
}

// busy: a haul to building id is under way.
func (r *Rearms) busy(id entity.EntityID) bool {
	_, ok := slices.BinarySearchFunc(r.jobs, id, func(a Rearm, id entity.EntityID) int { return cmp.Compare(a.Building, id) })
	return ok
}

// scan starts a haul for every defence that needs one and has none.
func (r *Rearms) scan(now uint64) {
	for _, id := range r.turrets.list() {
		b, ok := r.h.Building(id)
		if ok && r.turrets.Barrel(id, b.Kind) <= 250 && !r.busy(id) {
			r.barrel(id, b)
		}
	}
	for _, id := range r.mortars.list(now) {
		b, ok := r.h.Building(id)
		if !ok || r.busy(id) {
			continue
		}
		switch barrel, room := r.mortars.wants(id); {
		case barrel:
			r.barrel(id, b)
		case room > 0:
			for _, k := range shellOrder {
				if r.start(id, b, k, room) {
					break
				}
			}
		}
	}
}

// barrel starts a haul of building id's barrel material, else of steel
// (BarrelCost: no source of the wiki's material yet).
func (r *Rearms) barrel(id entity.EntityID, b construction.Building) {
	k, n := BarrelCost(b.Kind)
	if k != item.Steel && r.start(id, b, k, n) {
		return
	}
	if k != item.Steel {
		_, n = BarrelCost(construction.SentryTurret)
	}
	r.start(id, b, item.Steel, n)
}

// start sends the nearest free colonist to fetch qty of k (a barrel's
// steel all at once; shells as many as one stack holds) for building id.
func (r *Rearms) start(id entity.EntityID, b construction.Building, k item.Kind, qty int) bool {
	h := r.h
	r.near = h.Near(b.X, b.Y, rearmReach, Pawns, r.near[:0])
	p, ok := nearest(r.near, b.X, b.Y, func(s Seen) bool { return r.free(s.ID) })
	if !ok {
		return false
	}
	need := qty
	if k.IsShell() {
		need = 1
	}
	stack, sx, sy, ok := r.sup.Stock(p.ID, k, need)
	if !ok {
		return false
	}
	dx, dy, ok := r.workSpot(p.ID, b)
	if !ok {
		return false
	}
	rec := job.Record{Kind: job.RearmTurret, Target: stack, TX: sx, TY: sy, Dest: id, DestX: dx, DestY: dy, Count: qty, WorkLeft: RearmWork}
	if !h.StartJob(p.ID, rec) {
		return false
	}
	i, _ := slices.BinarySearchFunc(r.jobs, id, func(a Rearm, id entity.EntityID) int { return cmp.Compare(a.Building, id) })
	r.jobs = slices.Insert(r.jobs, i, Rearm{Building: id, Pawn: p.ID, Kind: k})
	return true
}

// free: c is a colonist on its feet, not drafted, with nothing to do.
func (r *Rearms) free(c entity.EntityID) bool {
	h := r.h
	if h.Drafted(c) || !h.StandingPawn(c) || h.StandingRaider(c) {
		return false
	}
	j, _ := h.Job(c)
	return j.Kind == job.None || j.State == job.Idle || j.Kind.IsIdle()
}

// workSpot is a tile beside building b that colonist c can stand on (a
// mortar's crew spot first).
func (r *Rearms) workSpot(c entity.EntityID, b construction.Building) (int, int, bool) {
	q := r.br.Query(c)
	if b.Kind == construction.Mortar {
		if x, y := spot(b); q.Walkable(x, y) {
			return x, y, true
		}
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, cell := range cells {
		for _, d := range [...][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
			x, y := cell[0]+d[0], cell[1]+d[1]
			if x >= 0 && y >= 0 && x < q.Width() && y < q.Height() && q.Walkable(x, y) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// over: haul a is done, failed or replaced (its building gone: the job
// fails).
func (r *Rearms) over(a Rearm) bool {
	h := r.h
	j, ok := h.Job(a.Pawn)
	if !ok || j.Kind != job.RearmTurret || j.Dest != a.Building || j.State == job.Complete || j.State == job.Fail || j.State == job.Idle {
		return true
	}
	if _, ok := h.Building(a.Building); !ok {
		j.State, j.Fail = job.Fail, job.TargetGone
		h.SetJob(a.Pawn, j)
		return true
	}
	return false
}

// Do advances colonist id's RearmTurret job j, standing at its tile (the
// game's job phase): at the stack it takes the load and heads for the
// building; there it works the load in.
func (r *Rearms) Do(id entity.EntityID, j job.Record) {
	i := slices.IndexFunc(r.jobs, func(a Rearm) bool { return a.Pawn == id && a.Building == j.Dest })
	if i < 0 || r.over(r.jobs[i]) {
		if j, _ = r.h.Job(id); j.State == job.Do { // a haul nobody tracks (or its building gone): over
			j.State, j.Fail = job.Fail, job.TargetGone
			r.h.SetJob(id, j)
		}
		return
	}
	if r.step(r.jobs[i], j) {
		r.jobs = slices.Delete(r.jobs, i, i+1)
	}
}

// step is haul a's job j at its tile; true once it is over.
func (r *Rearms) step(a Rearm, j job.Record) bool {
	h := r.h
	if j.Carry == 0 { // at the stack: take it and head for the building
		carry, _, n, ok := r.sup.PickUp(a.Pawn, j.Target, j.Count)
		if !ok {
			j.State, j.Fail = job.Fail, job.TargetGone
			h.SetJob(a.Pawn, j)
			return true
		}
		j.Carry, j.Count = carry, n
		path, ok := h.PathTo(a.Pawn, j.DestX, j.DestY)
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			h.SetJob(a.Pawn, j)
			return true
		}
		j.TX, j.TY, j.Path, j.PathI = j.DestX, j.DestY, path, 0
		if len(path) > 0 {
			j.State = job.Goto
		}
		h.SetJob(a.Pawn, j)
		return false
	}
	if j.WorkLeft--; j.WorkLeft > 0 {
		h.SetJob(a.Pawn, j)
		return false
	}
	r.apply(a, j.Count)
	r.sup.Spend(j.Carry)
	j.Carry, j.State = 0, job.Complete
	h.SetJob(a.Pawn, j)
	return true
}

// apply works haul a's n units into its building.
func (r *Rearms) apply(a Rearm, n int) {
	b, _ := r.h.Building(a.Building)
	switch {
	case isTurret(b.Kind):
		r.turrets.Rearm(a.Building)
	case !a.Kind.IsShell(): // its barrel
		r.mortars.rearm(a.Building)
	default:
		r.mortars.load(a.Building, a.Kind, n)
	}
}

// Hauler is the colonist bringing building id its load (0: none).
func (r *Rearms) Hauler(id entity.EntityID) entity.EntityID {
	i, ok := slices.BinarySearchFunc(r.jobs, id, func(a Rearm, id entity.EntityID) int { return cmp.Compare(a.Building, id) })
	if !ok {
		return 0
	}
	return r.jobs[i].Pawn
}

// export is the hauls under way (nil when none).
func (r *Rearms) export() []Rearm {
	if len(r.jobs) == 0 {
		return nil
	}
	return slices.Clone(r.jobs)
}

// importRearms restores saved hauls.
func (r *Rearms) importRearms(s []Rearm) { r.jobs = append(r.jobs[:0], s...) }
