package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// JoyRoles is the game's side of TopOff and Relax (adapters:
// game/sense_joy_host.go), values only.
type JoyRoles interface {
	JoyPawns
	JoyDrinks
}

// JoyPawns is the pawns TopOff and Relax ask about and task.
type JoyPawns interface {
	// Colonist: one of the colony's own.
	Colonist(id entity.EntityID) bool
	// SeatWanted: id is sitting down to rest, or low comfort has it looking for a
	// seat it may still find: its free hours have this to do first.
	SeatWanted(id entity.EntityID) bool
	Joy(id entity.EntityID) int
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, rec job.Record) bool
	// Seat is the free cell beside (x, y) that id can reach.
	Seat(id entity.EntityID, x, y int) (int, int, bool)
}

// JoyDrinks is the drink a relaxing colonist takes along (game/dose).
type JoyDrinks interface {
	// Drink is what id's policy lets it take along: kind k, from its pack (stack
	// 0) or from the nearest stack at (x, y).
	Drink(id entity.EntityID) (k item.Kind, stack entity.EntityID, x, y int, ok bool)
	// DoseJoy is the recreation a dose of k gives.
	DoseJoy(k item.Kind) int
	// Unpack brings one k out of id's pack into its hands.
	Unpack(id entity.EntityID, k item.Kind) (entity.EntityID, bool)
	// TakeOne has id, at stack, pick up one dose in its hands.
	TakeOne(id, stack entity.EntityID) (entity.EntityID, bool)
	// HeldJoy is the recreation the dose id holds as stack gives; false: it holds
	// none now.
	HeldJoy(id, stack entity.EntityID) (int, bool)
	// Swallow has id take the dose it holds; true: the dose killed it.
	Swallow(id, stack entity.EntityID) bool
}

// The topped-off band (CSE-002): the 95 % and 80 % of the brief.
const (
	topOffHigh = recreation.Full * 95 / 100
	topOffLow  = recreation.Full * 80 / 100
)

// TopOff has a colonist above full recreation work its free hours (CSE-002).
// Topped off above the high mark, it wants no recreation again until it falls
// under the low one; what it was doing in between keeps it on one side.
type TopOff struct {
	s *Sense
	r JoyRoles
}

func newTopOff(s *Sense, r JoyRoles) TopOff { return TopOff{s: s, r: r} }

// Wants reports id, with this much recreation, wants recreation; false sends
// it to look for work first in a free hour. It is read wherever a free hour is
// spent, so the flag follows joy there, and its map is written only when the
// flag turns. Asleep or at work in a Work hour nothing reads it: the mark is
// taken the next free hour.
func (t TopOff) Wants(id entity.EntityID, joy int) bool {
	if !t.s.Rules.On(gameplay.JoyTopOff) || !t.r.Colonist(id) {
		return true
	}
	on := t.s.TopOffOn[id]
	switch {
	case !on && joy > topOffHigh:
		t.s.TopOffOn[id], on = true, true
	case on && joy < topOffLow:
		delete(t.s.TopOffOn, id)
		on = false
	}
	return !on || t.r.SeatWanted(id)
}

// relaxSit is the least a relax with a drink lasts once seated (ours, the
// shortest free-time linger): one that came close to the mark still sits down.
const relaxSit = 240

// Relax has a colonist relaxing in company bring a drink and finish it at the
// end, so its recreation isn't wasted at full (CSE-008). The job is a plain
// job.SocialRelax with two more fields: Target the stack it fetches first, Carry
// the drink in its hands; a bare relax has neither. Count is the WorkLeft from
// which it may finish early. The dose gap in the policy keeps it to one drink in
// many relaxes; an interrupted job drops the drink, or it goes back to the pack.
type Relax struct {
	s *Sense
	r JoyRoles
}

func newRelax(s *Sense, r JoyRoles) Relax { return Relax{s: s, r: r} }

// finishMark is the recreation a drink giving doseJoy is finished at.
func finishMark(doseJoy int) int { return recreation.Full*99/100 - doseJoy }

// Assign starts relax job seat with a drink; false: none wanted, or none to be
// had, and the caller starts it bare. A drink that has left the pack stays on
// the job even if it could not start: the job's end sees to it.
func (r Relax) Assign(id entity.EntityID, seat job.Record) bool {
	if !r.s.Rules.On(gameplay.RelaxDrink) {
		return false
	}
	k, stack, x, y, ok := r.r.Drink(id)
	if !ok || r.r.Joy(id) >= finishMark(r.r.DoseJoy(k)) {
		return false
	}
	seat.Count = max(seat.WorkLeft-relaxSit, 0)
	if stack != 0 {
		seat.Target, seat.TX, seat.TY = stack, x, y
		return r.r.StartJob(id, seat)
	}
	if seat.Carry, ok = r.r.Unpack(id, k); !ok {
		return false
	}
	r.r.StartJob(id, seat)
	return true
}

// Do is one tick of a relax before its linger: it returns the job, and whether
// the linger goes on this tick. At the stack the drink comes into hand and the
// walk to the seat begins; seated, the drink is finished at the mark or when the
// time is up, and the job ends with it.
func (r Relax) Do(id entity.EntityID, j job.Record) (job.Record, bool) {
	if j.Target == 0 && j.Carry == 0 {
		return j, true
	}
	if j.Carry == 0 {
		r.fetched(id, j)
		return j, false
	}
	doseJoy, held := r.r.HeldJoy(id, j.Carry)
	if !held { // put down meanwhile (a work giver tried, tick.go startJob)
		j.Carry = 0
		return j, true
	}
	if j.WorkLeft > 1 && (j.WorkLeft > j.Count || r.r.Joy(id) < finishMark(doseJoy)) {
		return j, true
	}
	stack := j.Carry
	j.Carry, j.WorkLeft = 0, 1
	r.r.SetJob(id, j)
	return j, !r.r.Swallow(id, stack)
}

// fetched is the arrival at the stack: pick up a dose (a stack gone means a bare
// relax) and walk to a seat, or end there, the drink falling with the job.
func (r Relax) fetched(id entity.EntityID, j job.Record) {
	rec := job.Record{Kind: j.Kind, DestX: j.DestX, DestY: j.DestY, WorkLeft: j.WorkLeft, Count: j.Count}
	if carry, ok := r.r.TakeOne(id, j.Target); ok {
		rec.Carry = carry
	}
	x, y, ok := r.r.Seat(id, j.DestX, j.DestY)
	if !ok {
		rec.State = job.Complete
		r.r.SetJob(id, rec)
		return
	}
	rec.TX, rec.TY = x, y
	r.r.StartJob(id, rec)
}
