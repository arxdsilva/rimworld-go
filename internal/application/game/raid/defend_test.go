package raid

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// defendHost is the few roles defend needs; a call to any other panics.
type defendHost struct {
	Host
	groups  []Group
	r       Raider
	dist    int // the target's distance east of the defender (0: no target)
	started []job.Record
}

func (h *defendHost) Groups() []Group                           { return h.groups }
func (h *defendHost) SetRaider(_ entity.EntityID, r Raider)     { h.r = r }
func (h *defendHost) Interrupt(entity.EntityID, job.FailReason) {}
func (h *defendHost) RaidTarget(x, y int) (entity.EntityID, int, int, bool) {
	return 9, x + h.dist, y, h.dist > 0
}
func (h *defendHost) WalkableNear(x, y, _ int) (int, int, bool) { return x, y, true }
func (h *defendHost) StartJob(_ entity.EntityID, j job.Record) bool {
	h.started = append(h.started, j)
	return true
}

// TestDefendEngagesAndReleasesNoFlap: a holding defender engages inside
// sight, a fighting one lets go only past twice that; a target wandering
// across the sight line never flips it, and being shot from out of sight
// engages it.
func TestDefendEngagesAndReleasesNoFlap(t *testing.T) {
	t.Parallel()
	h := &defendHost{groups: []Group{{Plan: Defend, CampX: 50, CampY: 50}}}
	rs := &Raiders{h: h}
	const id = entity.EntityID(stageEvery) // looks on ticks divisible by stageEvery
	step := func(dist, hurt int) bool {
		h.dist = dist
		r := h.r
		held := rs.defend(id, &r, job.Record{}, hurt, 50, 50, stageEvery*10)
		h.r = r
		return held
	}
	if !step(SightRange+1, 0) || h.r.Mode != Arriving {
		t.Fatal("a target out of sight should not draw it")
	}
	if step(SightRange, 0) || h.r.Mode != Assaulting {
		t.Fatal("a target in sight should draw it")
	}
	flips := 0
	for d := SightRange - 3; d <= SightRange+3; d++ { // no flapping at the sight line
		for _, x := range [2]int{d, 2*SightRange - d} {
			if step(x, 0) {
				flips++
			}
		}
	}
	if flips != 0 || h.r.Mode != Assaulting {
		t.Fatalf("it flapped %d times around the sight line", flips)
	}
	if step(defendRelease, 0) {
		t.Fatal("it let go inside the release range")
	}
	if !step(defendRelease+1, 0) || h.r.Mode != Arriving {
		t.Fatal("past the release range it should go back to its post")
	}
	if !step(SightRange+1, 0) {
		t.Fatal("it should hold again")
	}
	if step(SightRange+5, 40) || h.r.Mode != Assaulting || h.r.Hurt != 40 {
		t.Fatal("being shot from out of sight should engage it")
	}
}

// TestDefendWalksBackAndNeverGivesUp: a defender strayed past the leash
// walks back; one on its post stays; and a day on the map never makes it
// give up, as it does a raider.
func TestDefendWalksBackAndNeverGivesUp(t *testing.T) {
	t.Parallel()
	h := &defendHost{groups: []Group{{Plan: Defend, CampX: 50, CampY: 50}, {Plan: Assault}}}
	rs := &Raiders{h: h}
	const id = entity.EntityID(stageEvery)
	r := Raider{}
	if !rs.defend(id, &r, job.Record{}, 0, 50+defendLeash, 50, stageEvery*10) || len(h.started) != 0 {
		t.Fatalf("on its post it should stay: %+v", h.started)
	}
	if !rs.defend(id, &r, job.Record{}, 0, 50+defendLeash+1, 50, stageEvery*10) || len(h.started) != 1 {
		t.Fatalf("past the leash it should walk back: %+v", h.started)
	}
	if j := h.started[0]; max(j.TX-50, 50-j.TX, j.TY-50, 50-j.TY) > defendPost {
		t.Fatalf("its post %d,%d is off the centre", j.TX, j.TY)
	}
	now := uint64(10 * clock.TicksPerDay)
	if rs.givenUp(Raider{Group: 0, Arrived: 1}, now) {
		t.Fatal("a defender gave up")
	}
	if !rs.givenUp(Raider{Group: 1, Arrived: 1}, now) {
		t.Fatal("a raider should still give up")
	}
	other := Raider{Group: 1}
	if rs.defend(id, &other, job.Record{}, 0, 0, 0, stageEvery*10) {
		t.Fatal("another plan is not defend's")
	}
}
