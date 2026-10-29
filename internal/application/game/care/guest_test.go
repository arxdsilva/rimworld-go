package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	visiting entity.EntityID = 30 // a guest of faction guestFac
	wander   entity.EntityID = 31 // a stranger (no faction)
	guestBed entity.EntityID = 32
	guestFac uint64          = 3
)

// guestScene: doctor doc at (0, 0); guest visiting of guestFac and
// stranger wander standing at (5, 5) and (6, 5); one free bed at (8, 8).
func guestScene() (*Care, *fakeHost, *fakeGuestHost) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.doctors[doc] = true
	gu := newFakeGuestHost(h)
	gu.guest(visiting, guestFac, 5, 5)
	gu.guest(wander, 0, 6, 5)
	h.bed(guestBed, construction.Building{Kind: construction.Bed, X: 8, Y: 8})
	return New(h, Roles{Prison: newFakePrison(h), Guests: gu}), h, gu
}

// inGuestBed lays guest id in guestBed with healing injury healing left.
func inGuestBed(c *Care, h *fakeHost, id entity.EntityID, healing int) {
	c.layInBed(id, guestBed, true)
	h.healing[id] = healing
}

func TestGuestRescuable(t *testing.T) {
	c, h, gu := guestScene()
	h.downed(visiting, 50)
	if !c.Doctor.Rescuable(visiting) || !guestKind(c, visiting) {
		t.Fatal("a downed guest out of bed with a free bed is rescued")
	}
	h.downed(wander, 50)
	if !c.Doctor.Rescuable(wander) {
		t.Fatal("a downed stranger is rescued too")
	}
	h.outsider(raider, 7, 7)
	h.downed(raider, 50)
	if c.Doctor.Rescuable(raider) {
		t.Fatal("a downed raider is no guest")
	}
	h.prisoners[visiting] = true
	if c.Doctor.Rescuable(visiting) {
		t.Fatal("a prisoner is no guest")
	}
	delete(h.prisoners, visiting)
	if New(h, Roles{}).Doctor.Rescuable(visiting) || !gu.IsGuest(visiting) {
		t.Fatal("no Guests role: no guest is rescued")
	}
}

// guestKind: the order menu's kindOf sorts the test's guests right.
func guestKind(c *Care, id entity.EntityID) bool {
	k := c.kindOf(id)
	return k == targetGuest || k == targetStranger
}

func TestGuestRescueLaysInBed(t *testing.T) {
	c, h, _ := guestScene()
	h.downed(visiting, 50)
	j := job.Record{Kind: job.Rescue, Target: visiting, State: job.Do}
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	if c.Carry.Patient(doc) != visiting || j.Dest != guestBed {
		t.Fatalf("picked up for the free bed: carry %d job %+v", c.Carry.Patient(doc), j)
	}
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	if h.BedUnder(visiting) != guestBed || h.log[len(h.log)-1] != "rescued" {
		t.Fatalf("laid in bed: bed %d log %v", h.BedUnder(visiting), h.log)
	}
	if c.Doctor.Rescuable(visiting) {
		t.Fatal("a guest in bed is not rescued again")
	}
	if !c.Doctor.tendable(visiting, false) {
		t.Fatal("a guest in bed with a wound is tended")
	}
}

func TestGuestStaysInBedWhileHealing(t *testing.T) {
	c, h, gu := guestScene()
	hos := c.Guest
	inGuestBed(c, h, visiting, 50)
	h.vital[visiting] = health.Record{Status: health.Downed, Cause: health.CauseInjury}
	if !hos.Step(visiting) {
		t.Fatal("a downed guest in bed stays")
	}
	h.vital[visiting] = health.Record{Status: health.Alive}
	if !hos.Step(visiting) {
		t.Fatal("up but still hurt: it stays in bed")
	}
	if j := h.jobs[visiting]; j.Kind != job.Sleep || j.Target != guestBed || j.State != job.Do {
		t.Fatalf("it lies in its bed: %+v", j)
	}
	hos.Step(visiting) // the job is kept, not restarted
	h.healing[visiting] = RestWellInjury
	if hos.Step(visiting) {
		t.Fatal("well again: it leaves")
	}
	if !slices.Equal(gu.goodwill, []int{GoodwillHealed}) || gu.reasons[0] != ReasonHealed ||
		!slices.Equal(gu.left, []entity.EntityID{visiting}) || len(gu.offers) != 0 {
		t.Fatalf("goodwill %v reasons %v left %v offers %v", gu.goodwill, gu.reasons, gu.left, gu.offers)
	}
	if h.jobs[visiting].Kind == job.Sleep || h.BedUnder(visiting) != 0 || c.Rest.HeldBed(visiting) != 0 {
		t.Fatalf("it gets up and gives the bed back: job %+v bed %d", h.jobs[visiting], h.BedUnder(visiting))
	}
}

func TestGuestStaysWhileSickOrUntended(t *testing.T) {
	c, h, gu := guestScene()
	hos := c.Guest
	inGuestBed(c, h, visiting, 0)
	h.tend[visiting] = true
	if !hos.Step(visiting) {
		t.Fatal("a wound to tend keeps it in bed")
	}
	h.tend[visiting], h.sickStill[visiting] = false, true
	if !hos.Step(visiting) {
		t.Fatal("still sick: it stays")
	}
	h.sickStill[visiting] = false
	if hos.Step(visiting) || len(gu.left) != 1 {
		t.Fatal("well: it leaves")
	}
}

func TestGuestStepOutOfCare(t *testing.T) {
	c, h, gu := guestScene()
	hos := c.Guest
	if hos.Step(visiting) || len(gu.left) != 0 {
		t.Fatal("a guest not in bed is not in care")
	}
	h.outsider(raider, 8, 8)
	h.bldgs[guestBed] = h.bldgs[guestBed].Claim(uint64(raider))
	if hos.Step(raider) {
		t.Fatal("a raider in a bed is no guest")
	}
	inGuestBed(c, h, visiting, 50)
	h.vital[visiting] = health.Record{Status: health.Dead}
	if hos.Step(visiting) || len(gu.left) != 0 {
		t.Fatal("a dead guest neither stays nor leaves")
	}
}

func TestStrangerRecoveredMayJoin(t *testing.T) {
	c, h, gu := guestScene()
	inGuestBed(c, h, wander, 0)
	if c.Guest.Step(wander) {
		t.Fatal("a well stranger gets up")
	}
	if !slices.Equal(gu.offers, []entity.EntityID{wander}) || len(gu.goodwill) != 0 || len(gu.left) != 0 {
		t.Fatalf("a stranger is asked about joining, no goodwill: offers %v goodwill %v left %v", gu.offers, gu.goodwill, gu.left)
	}
}

func TestGuestDiedDowned(t *testing.T) {
	c, h, gu := guestScene()
	hos := c.Guest
	inGuestBed(c, h, visiting, 50)
	hos.Died(visiting)
	if !slices.Equal(gu.goodwill, []int{GoodwillDiedDowned}) || gu.reasons[0] != ReasonDiedInCare || c.Rest.HeldBed(visiting) != 0 {
		t.Fatalf("goodwill %v reasons %v", gu.goodwill, gu.reasons)
	}
	hos.Died(wander)
	if len(gu.goodwill) != 1 {
		t.Fatal("a stranger has no faction to mind")
	}
	New(h, Roles{}).Guest.Died(visiting) // no Guests role: nothing to tell
}

// TestGuestDoesNotFlap: a recovered guest leaves once, and its healing
// injury bobbing around the line afterwards never puts it back in care;
// only going down again makes it rescuable.
func TestGuestDoesNotFlap(t *testing.T) {
	c, h, gu := guestScene()
	hos := c.Guest
	inGuestBed(c, h, visiting, RestWellInjury+5)
	stays, leaves := 0, 0
	for tick := 0; tick < 400; tick++ {
		h.now = uint64(tick)
		h.healing[visiting] = RestWellInjury + 4 - tick%7 // 19 down to 13, again and again
		if hos.Step(visiting) {
			stays++
		}
		if c.Doctor.Rescuable(visiting) {
			t.Fatalf("tick %d: a standing guest is never rescued", tick)
		}
		if tick == 200 {
			leaves = len(gu.left)
		}
	}
	if len(gu.left) != 1 || leaves != 1 || len(gu.goodwill) != 1 {
		t.Fatalf("left %d times (goodwill %v), stayed %d ticks", len(gu.left), gu.goodwill, stays)
	}
	if stays == 0 || stays > 7 {
		t.Fatalf("it stays until first well, then never again: %d", stays)
	}
	h.downed(visiting, 60)
	if !c.Doctor.Rescuable(visiting) {
		t.Fatal("down again on its way home: rescued again")
	}
}
