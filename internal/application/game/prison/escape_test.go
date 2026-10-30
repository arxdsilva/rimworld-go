package prison

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeEscapes: rooms by pawn, and who escaped.
type fakeEscapes struct {
	rooms    map[entity.EntityID]int
	escaped  []entity.EntityID
	breaks   []int
	departed int
}

func (f *fakeEscapes) RoomOf(id entity.EntityID) int     { return f.rooms[id] }
func (f *fakeEscapes) Escape(id entity.EntityID)         { f.escaped = append(f.escaped, id) }
func (f *fakeEscapes) Breakout(_ entity.EntityID, n int) { f.breaks = append(f.breaks, n) }

// Exit: the edge is x 0; the way out is (0, y).
func (f *fakeEscapes) Exit(x, y int) (int, int, bool, bool) { return 0, y, x == 0, true }
func (f *fakeEscapes) Depart(entity.EntityID)               { f.departed++ }

// breakRig: prisoners 5 and 6 share room 1, 7 is in room 2 close by, 8
// in room 3 far off.
func breakRig(t *testing.T) (*Prison, *fakeHost) {
	t.Helper()
	p, f := rig(t)
	f.put(rid2, 11, 10)
	f.put(7, 14, 10)
	f.put(8, 60, 10)
	f.raiders = []entity.EntityID{rid, rid2, 7, 8}
	for _, id := range f.raiders[1:] {
		p.Take(id, warden, 0)
	}
	f.rooms = map[entity.EntityID]int{rid: 1, rid2: 1, 7: 2, 8: 3}
	return p, f
}

// hourFor is the first tick at or after from on prisoner id's hourly
// escape check.
func hourFor(id entity.EntityID, from uint64) uint64 {
	for now := from; ; now++ {
		if (now+uint64(id)*escapeStagger)%clock.TicksPerHour == 0 {
			return now
		}
	}
}

// TestPrisonBreakSpreadsByRoom: the starter's room always joins, a room
// within 20 tiles joins on a coin toss, a far one never; escapees stop
// being prisoners.
func TestPrisonBreakSpreadsByRoom(t *testing.T) {
	p, f := breakRig(t)
	now := hourFor(rid, EscapeGrace)
	if p.breakOut(rid, now-clock.TicksPerHour) {
		t.Fatal("no break inside the grace after a capture")
	}
	f.rolls = []int{0, 0} // the MTB roll fires; room 2's coin joins
	if !p.breakOut(rid, now) {
		t.Fatal("the roll starts a break")
	}
	if len(f.escaped) != 3 || f.escaped[0] != rid || f.escaped[1] != rid2 || f.escaped[2] != 7 {
		t.Fatalf("escaped %v", f.escaped)
	}
	if p.Is(rid) || p.Is(7) || !p.Is(8) || len(f.breaks) != 1 || f.breaks[0] != 3 {
		t.Fatalf("prisoners left: 8 only; breaks %v", f.breaks)
	}
}

// TestPrisonBreakCalm: after a break, the prisoners left try nothing
// for EscapeCalm, however the dice fall (no break after break).
func TestPrisonBreakCalm(t *testing.T) {
	p, f := breakRig(t)
	now := hourFor(rid, EscapeGrace)
	f.rolls = []int{0, 1} // room 2 stays
	if !p.breakOut(rid, now) || !p.Is(7) || !p.Is(8) {
		t.Fatal("break without room 2")
	}
	breaks := 0
	for tick := now + 1; tick < now+EscapeCalm-clock.TicksPerHour; tick += 7 {
		for _, id := range []entity.EntityID{7, 8} {
			f.ri = 0
			f.rolls = []int{0}
			if p.breakOut(id, hourFor(id, tick)) {
				breaks++
			}
		}
	}
	if breaks != 0 {
		t.Fatalf("%d breaks inside the calm", breaks)
	}
	f.ri, f.rolls = 0, []int{int(clock.TicksPerHour)} // the MTB roll misses
	if p.breakOut(8, hourFor(8, now+EscapeCalm)) {
		t.Fatal("a missed roll breaks nothing")
	}
	f.ri, f.rolls = 0, []int{0}
	if !p.breakOut(8, hourFor(8, now+EscapeCalm)) {
		t.Fatal("past the calm a break may come again")
	}
}

// TestWalkOut: a released pawn on the edge (or after a day) departs; a
// pawn that can't start its walk waits retry ticks.
func TestWalkOut(t *testing.T) {
	p, f := rig(t)
	f.pos[rid] = [2]int{0, 4}
	p.walkOut(rid, Held{Leaving: true}, job.Record{}, 10)
	if f.departed != 1 {
		t.Fatal("on the edge it goes home")
	}
	f.pos[rid] = [2]int{9, 4}
	p.walkOut(rid, Held{Leaving: true}, job.Record{}, giveUp)
	if f.departed != 2 {
		t.Fatal("a day of trying: it goes home")
	}
	f.failStart = true
	p.walkOut(rid, Held{Leaving: true, Idle: 5}, job.Record{}, 10)
	if f.Held(rid).RetryAt != 10+retry {
		t.Fatalf("no way out: it waits, %+v", f.Held(rid))
	}
}
