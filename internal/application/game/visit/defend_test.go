package visit

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeDefence is one visitor at (0, 0) and one foe on the x axis.
type fakeDefence struct {
	foeX    int
	j       job.Record
	hit     uint64
	trader  bool
	strikes int
}

const fakeFoe entity.EntityID = 99

func (f *fakeDefence) Pos(id entity.EntityID) (int, int) {
	if id == fakeFoe {
		return f.foeX, 0
	}
	return 0, 0
}
func (f *fakeDefence) SetJob(_ entity.EntityID, rec job.Record)   { f.j = rec }
func (f *fakeDefence) Release(entity.EntityID)                    {}
func (f *fakeDefence) Strike(entity.EntityID, job.Record, uint64) { f.strikes++ }
func (f *fakeDefence) HitAt(entity.EntityID) uint64               { return f.hit }
func (f *fakeDefence) Trader(entity.EntityID) bool                { return f.trader }
func (f *fakeDefence) Engage(_ entity.EntityID, x, y, r int, _ uint64) bool {
	if !within(x, y, f.foeX, 0, r) {
		return false
	}
	f.j = job.Record{Kind: job.Attack, Target: fakeFoe, State: job.Do}
	return true
}

// TestVisitorDefenceNoFlap (EAI-011): a visitor starts a fight only with a
// foe within DefendSight, keeps it while the foe stays within DefendKeep
// (and for DefendHold whatever happens), and once it lets go does not
// start again for a foe hovering between the two ranges.
func TestVisitorDefenceNoFlap(t *testing.T) {
	t.Parallel()
	f := &fakeDefence{foeX: DefendSight + 2}
	r := &Rec{ID: 7}
	fighting, changes, joins := false, 0, 0
	var now uint64
	run := func(ticks int, foe func(now uint64) int) {
		for end := now + uint64(ticks); now < end; now++ {
			f.foeX = foe(now)
			fight, joined := defend(f, r, 0, 0, f.j, now)
			if fight != fighting {
				fighting, changes = fight, changes+1
			}
			if joined {
				joins++
			}
		}
	}
	run(600, func(uint64) int { return DefendSight + 2 })
	if fighting || changes != 0 {
		t.Fatal("a visitor fought a foe out of sight")
	}
	// The foe comes in, then weaves between the two ranges.
	run(3000, func(n uint64) int {
		if n < 700 {
			return DefendSight - 1
		}
		if n/45%2 == 0 {
			return DefendSight - 1
		}
		return DefendKeep - 1
	})
	if !fighting || changes != 1 || joins != 1 || f.strikes == 0 {
		t.Fatalf("weaving foe: fighting %v, %d changes, %d joins, %d strikes: want one fight held", fighting, changes, joins, f.strikes)
	}
	// It runs off: the visitor holds on until the hold is over, then lets go.
	run(DefendHold+2*defendScan, func(uint64) int { return DefendKeep + 4 })
	if fighting || changes != 2 || r.Defend != 0 {
		t.Fatalf("after the foe fled: fighting %v, %d changes, defend %d", fighting, changes, r.Defend)
	}
	// Hovering between sight and keep never starts another fight.
	run(3000, func(n uint64) int { return DefendSight + 1 + int(n/30%(DefendKeep-DefendSight)) })
	if changes != 2 || joins != 1 {
		t.Fatalf("hovering foe: %d changes, %d joins: the visitor flapped", changes, joins)
	}
	// A hurt visitor looks as far as DefendKeep; a trader only when hurt.
	f.trader = true
	f.foeX = DefendSight - 2
	f.j = job.Record{}
	scan := now + defendScan - (now+uint64(r.ID))%defendScan
	if fight, _ := defend(f, r, 0, 0, f.j, scan); fight {
		t.Fatal("an unhurt trader joined a fight")
	}
	f.hit, f.foeX = scan+defendScan-1, DefendKeep-1 // hurt a tick before its next look
	if fight, joined := defend(f, r, 0, 0, f.j, scan+defendScan); !fight || !joined {
		t.Fatal("a hurt trader did not defend itself")
	}
}

// TestDefendStaggered: a visitor looks only on its own scan tick, so two
// visitors with neighbouring ids look on different ticks.
func TestDefendStaggered(t *testing.T) {
	t.Parallel()
	first := map[entity.EntityID]uint64{}
	for _, id := range []entity.EntityID{3, 4} {
		f := &fakeDefence{foeX: 1}
		r := &Rec{ID: id}
		for now := uint64(0); now < defendScan; now++ {
			if _, joined := defend(f, r, 0, 0, f.j, now); joined {
				first[id] = now
				break
			}
		}
		if (first[id]+uint64(id))%defendScan != 0 {
			t.Fatalf("visitor %d looked first at tick %d, off its scan tick", id, first[id])
		}
	}
	if first[3] == first[4] {
		t.Fatal("two visitors looked on the same tick")
	}
}
