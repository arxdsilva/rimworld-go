package sense

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// TidyRoles is the game's side of Tidy: the cleaning roles plus what only a
// tidy asks.
type TidyRoles interface {
	Filth
	Ground
	Walks
	Labor
	TidyCrew
	TidyRooms
	TidyHands
}

// TidyCrew is who may tidy now, and the clock.
type TidyCrew interface {
	Now() uint64
	// Colonist: one of the colony's own, not a raider, prisoner or guest.
	Colonist(id entity.EntityID) bool
	// CanClean: id is capable of cleaning, with its priority on.
	CanClean(id entity.EntityID) bool
	// Fit: on its feet, not drafted, not in a break, not hurt or on bed
	// rest, not standing in fire.
	Fit(id entity.EntityID) bool
	// Ravenous: id's hunger is at or below needs.RavenousAt.
	Ravenous(id entity.EntityID) bool
	// OrdersWaiting: a queued player order is due.
	OrdersWaiting(id entity.EntityID) bool
	// HoldHunger keeps the hunger check from cutting a tidy before a meal
	// until tick until; ReleaseHunger ends the hold.
	HoldHunger(id entity.EntityID, until uint64)
	ReleaseHunger(id entity.EntityID)
}

// TidyRooms is the rooms and the ways through them.
type TidyRooms interface {
	// RoomAt is the room of (x, y), negative outdoors and on walls and doors.
	RoomAt(x, y int) int
	// RoomDirt is room's dirt plus stain and its size, read in O(1).
	RoomDirt(room int) (sum, cells int)
	// RoomGrid is the current room per tile, read only.
	RoomGrid() []int
	// MealSeat is where id would eat food lying at (x, y).
	MealSeat(id entity.EntityID, x, y int) (sx, sy int, ok bool)
	// PathLen is id's walk from (sx, sy) to (tx, ty) in steps, under the
	// tick's search budget: ok false when there is none or the budget is
	// spent.
	PathLen(id entity.EntityID, sx, sy, tx, ty int) (n int, ok bool)
	// PathFor is a route for id from where it stands, with its reach.
	PathFor(id entity.EntityID, tx, ty int) ([]pathing.Point, bool)
	// TendUrgent: doctor id's next job is a patient in patient's room, or
	// one bleeding out within hours.
	TendUrgent(id, patient entity.EntityID) bool
	// BenchThere: the building stands, on any map.
	BenchThere(id entity.EntityID) bool
}

// TidyHands is what a pawn holds and claims.
type TidyHands interface {
	// JobKind is id's job kind, on any map.
	JobKind(id entity.EntityID) job.Kind
	// DropHeld puts a stack down that id carries for the job a tidy
	// interrupted.
	DropHeld(id, stack entity.EntityID)
	// TryTile claims (x, y) for id; false when another holds it.
	TryTile(id entity.EntityID, x, y int) bool
	// LeaveTile drops id's own claim on (x, y).
	LeaveTile(id entity.EntityID, x, y int)
	ClaimEntity(id, target entity.EntityID)
}

// Tidy cleans a room around a job, with no job queue: the job waits in
// Resume, saved, and comes back at the job-end seam (CSE-017…027). TidyAt is
// the cooldown that keeps it from ever looping: set before any search, it
// holds on the way back into startJob, and a pawn with only dirt it can't
// reach or isn't allowed to touch tries again after it, not next tick.
type Tidy struct {
	s *Sense
	r TidyRoles
}

func newTidy(s *Sense, r TidyRoles) Tidy { return Tidy{s: s, r: r} }

// tidyCooldown is the ticks between two tidies of one pawn (ours).
const tidyCooldown = 1500

// tidyMealHold is the longest a hungry colonist tidies before its meal: the
// hunger check is held off this long, then cuts the tidy (ours).
const tidyMealHold = 600

// tidyRule is the toggle and the limit job kind k tidies under, if any
// (CSE-017, 020, 025). Sleep isn't one: a pawn only goes to bed tired or at
// bedtime, and phaseAI cuts anything else then, so a tidy would die on the
// next tick. Construct, frames and forced orders never tidy.
func tidyRule(k job.Kind) (gameplay.Toggle, gameplay.Limit, bool) {
	switch k {
	case job.Research, job.Bill, job.Recreate, job.WatchTelevision, job.Stargaze, job.Horseshoes, job.Read,
		job.Warden, job.Gather, job.OperateDrill, job.ScanGround, job.ScanMinerals, job.UseComms:
		return gameplay.CleanBeforeWork, gameplay.TidyStation, true
	case job.Eat, job.SocialRelax:
		return gameplay.CleanBeforeMeal, gameplay.TidyBench, true
	}
	return 0, 0, false
}

// Before may swap the job about to start for a tidy first; ok false keeps it.
// It is on startJob's path: everything up to the room's dirt is a few field
// and map reads, allocating nothing and looking at no tile.
func (t Tidy) Before(id entity.EntityID, rec job.Record) (job.Record, bool) {
	s := t.s
	if len(s.Resume) != 0 && rec.Kind != job.CleanRoom {
		t.Forget(id) // another job took over without an interrupt: a forced order, a duty
	}
	toggle, limit, ok := tidyRule(rec.Kind)
	if !ok || rec.Forced || !s.Rules.On(toggle) {
		return rec, false
	}
	now := t.r.Now()
	if now < s.TidyAt[id] || len(t.r.Queue()) == 0 || s.Rules.On(gameplay.BenchCleanToggle) && (s.NoClean[rec.Dest] || s.NoClean[rec.Target]) {
		return rec, false
	}
	ax, ay := t.anchor(id, rec)
	room, cells, dirty := t.dirtyRoom(ax, ay)
	if !dirty || !t.capable(id) || rec.Kind == job.Eat && t.r.Ravenous(id) {
		return rec, false
	}
	s.TidyAt[id] = now + tidyCooldown // from here on it costs: not again soon, whatever comes
	if t.detour(id, rec, ax, ay) {
		return rec, false
	}
	r, ok := t.route(id, room, cells, ax, ay, s.Rules.LimitOf(limit))
	if !ok {
		return rec, false
	}
	t.prune(id, now)
	if rec.Kind == job.Eat {
		t.r.HoldHunger(id, now+tidyMealHold)
	}
	rec.Path, rec.PathI, rec.State = nil, 0, job.Idle
	s.Resume[id] = rec
	return r, true
}

// anchor is where job rec's work is: its room gets the tidy. rec.TX, rec.TY
// is a fetch for a bill (the ingredient) and a meal (the food).
func (t Tidy) anchor(id entity.EntityID, rec job.Record) (x, y int) {
	switch rec.Kind {
	case job.Bill:
		return rec.DestX, rec.DestY
	case job.Eat:
		if sx, sy, ok := t.r.MealSeat(id, rec.TX, rec.TY); ok {
			return sx, sy
		}
	}
	return rec.TX, rec.TY
}

// dirtyRoom is the room at (x, y), if there is enough dirt in it for a tile
// to be queued: below filth.CleanStart in all, none is (half-swept tiles
// aside, which wait). Both reads are O(1).
func (t Tidy) dirtyRoom(x, y int) (room, cells int, dirty bool) {
	if room = t.r.RoomAt(x, y); room < 0 {
		return room, 0, false
	}
	sum, cells := t.r.RoomDirt(room)
	return room, cells, sum >= filth.CleanStart
}

// capable is CSE-026: who tidies at all. Non-tool-users have no labor, so no
// cleaning priority.
func (t Tidy) capable(id entity.EntityID) bool {
	return t.r.Colonist(id) && t.r.CanClean(id) && t.r.Fit(id)
}

// After runs when a job has ended and been cleared. True: it started a job,
// the one a tidy interrupted or a tidy of the room just worked in, and the
// pawn has a job again.
func (t Tidy) After(id entity.EntityID, j job.Record) bool {
	if j.Kind == job.CleanRoom {
		return t.resume(id)
	}
	patient, toggle, limit, ok := tidyAfter(j)
	if !ok || !t.s.Rules.On(toggle) {
		return false
	}
	now := t.r.Now()
	if now < t.s.TidyAt[id] || t.r.OrdersWaiting(id) {
		return false
	}
	return t.afterWork(id, patient, j.Kind, t.s.Rules.LimitOf(limit), now)
}

// tidyAfter is the patient whose room gets a tidy after job j, if j did what
// it set out to (CSE-021, 022). A tend that found nothing left to do ends
// with its work unspent; a feeding that delivered holds nothing any more.
func tidyAfter(j job.Record) (patient entity.EntityID, toggle gameplay.Toggle, limit gameplay.Limit, ok bool) {
	if j.State != job.Complete || j.Forced {
		return 0, 0, 0, false
	}
	switch j.Kind {
	case job.Tend:
		return j.Target, gameplay.CleanAfterTend, gameplay.TidyTend, j.WorkLeft <= 0
	case job.FeedPatient, job.DeliverFood:
		return j.Dest, gameplay.CleanBeforeWork, gameplay.TidyStation, j.Carry == 0 && j.Dest != 0
	}
	return 0, 0, 0, false
}

// afterWork starts the tidy of patient's room, unless a patient needs the
// doctor more.
func (t Tidy) afterWork(id, patient entity.EntityID, kind job.Kind, limit int, now uint64) bool {
	px, py := t.r.Pos(id)
	x, y := t.r.Pos(patient)
	if kind != job.Tend && pathing.Dist(px, py, x, y) > 1 {
		return false // the patient no longer wanted feeding before the feeder got there
	}
	room, cells, dirty := t.dirtyRoom(x, y)
	if !dirty || !t.capable(id) || kind == job.Tend && t.r.TendUrgent(id, patient) {
		return false
	}
	t.s.TidyAt[id] = now + tidyCooldown
	rec, ok := t.route(id, room, cells, x, y, limit)
	if !ok {
		return false
	}
	t.prune(id, now)
	return t.r.StartJob(id, rec)
}

// resume starts the job a finished tidy interrupted. The cooldown is what
// keeps it from being tidied again on the way in. A stale target fails
// through startJob, like any job, and the pawn looks for something else.
func (t Tidy) resume(id entity.EntityID) bool {
	s := t.s
	rec, ok := s.Resume[id]
	if !ok {
		return false
	}
	delete(s.Resume, id)
	if rec.Kind.Eating() {
		t.r.ReleaseHunger(id)
	}
	s.TidyAt[id] = t.r.Now() + tidyCooldown
	if !t.r.StartJob(id, rec) {
		return false
	}
	t.reclaim(id, rec)
	return true
}

// reclaim takes back what job rec's giver claimed after startJob, which a
// tidy can't know: the job-end seam released it all. While the tidy runs,
// those claims are simply still there, so nobody takes the bench meanwhile.
func (t Tidy) reclaim(id entity.EntityID, rec job.Record) {
	switch rec.Kind {
	case job.Bill:
		t.r.ClaimEntity(id, rec.Dest)
		t.r.ClaimTile(id, rec.DestX, rec.DestY)
		for _, load := range rec.Route { // what is already laid on the bench
			t.r.ClaimEntity(id, load)
		}
	case job.Research, job.Recreate, job.WatchTelevision, job.Stargaze, job.Horseshoes, job.Gather,
		job.OperateDrill, job.ScanGround, job.ScanMinerals:
		t.r.ClaimTile(id, rec.TX, rec.TY)
	}
}

// Forget drops the tidy state of an interrupted pawn: a job that a threat, a
// break or an order cut never comes back. A load held for that job goes down.
func (t Tidy) Forget(id entity.EntityID) {
	if len(t.s.Resume) == 0 {
		return
	}
	rec, ok := t.s.Resume[id]
	if !ok {
		return
	}
	delete(t.s.Resume, id)
	if rec.Carry != 0 {
		t.r.DropHeld(id, rec.Carry)
	}
	if rec.Kind.Eating() {
		t.r.ReleaseHunger(id)
	}
}

// prune sweeps the saved state when a tidy starts, which is rare: a Resume
// whose pawn isn't tidying any more (its job was overwritten), cooldowns run
// out and benches gone. id is about to tidy.
func (t Tidy) prune(id entity.EntityID, now uint64) {
	s := t.s
	for _, other := range slices.Sorted(maps.Keys(s.Resume)) {
		if other != id && t.r.JobKind(other) != job.CleanRoom {
			t.Forget(other)
		}
	}
	maps.DeleteFunc(s.TidyAt, func(_ entity.EntityID, until uint64) bool { return until <= now })
	t.pruneBenches()
}

func (t Tidy) pruneBenches() {
	maps.DeleteFunc(t.s.NoClean, func(b entity.EntityID, _ bool) bool { return !t.r.BenchThere(b) })
}

// SetBench is CSE-019: enabled false opts the bench out of tidying. A
// building that isn't there changes nothing.
func (t Tidy) SetBench(bench entity.EntityID, enabled bool) {
	if t.r == nil {
		return
	}
	t.pruneBenches()
	if !t.r.BenchThere(bench) {
		return
	}
	if enabled {
		delete(t.s.NoClean, bench)
	} else {
		t.s.NoClean[bench] = true
	}
}

// TidyBench is CSE-018, called once when the last load lands at a bench:
// true means a tidy started and the bill j is what it resumes. The pawn keeps
// a load it holds through the tidy (j.Carry), and the bench, the work spot
// and the loads laid out stay claimed.
func (t Tidy) TidyBench(id, bench entity.EntityID, j job.Record) bool {
	s := t.s
	if !s.Rules.On(gameplay.CleanBeforeBill) || j.Forced {
		return false
	}
	now := t.r.Now()
	if now < s.TidyAt[id] || s.Rules.On(gameplay.BenchCleanToggle) && s.NoClean[bench] {
		return false
	}
	room, cells, dirty := t.dirtyRoom(j.DestX, j.DestY)
	if !dirty || !t.capable(id) {
		return false
	}
	s.TidyAt[id] = now + tidyCooldown
	rec, ok := t.route(id, room, cells, j.DestX, j.DestY, s.Rules.LimitOf(gameplay.TidyBench))
	if !ok {
		return false
	}
	t.prune(id, now)
	if !t.r.StartJob(id, rec) { // the search budget ran out: the bill goes on as it was
		t.reclaim(id, j)
		return false
	}
	t.reclaim(id, j)
	if j.Carry != 0 { // on with the load in hand: to the work spot, wherever it was going
		j.TX, j.TY = j.DestX, j.DestY
	}
	j.Path, j.PathI, j.State = nil, 0, job.Idle
	s.Resume[id] = j
	return true
}
