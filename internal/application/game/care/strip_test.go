package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

const guestID entity.EntityID = 8

// stripScene: doctor doc at (0, 0); downed colonist patient on (5, 5)
// wearing gear; the Gear and Guests roles are fakes.
func stripScene() (*Care, *fakeHost, *fakePrison, *fakeGear, *fakeGuests) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.downed(patient, 50)
	pr := newFakePrison(h)
	g := &fakeGear{h: h, worn: map[entity.EntityID]bool{patient: true}}
	gu := &fakeGuests{guests: map[entity.EntityID]uint64{}}
	return New(h, Roles{Prison: pr, Gear: g, Guests: gu}), h, pr, g, gu
}

// runStrip does the doer's strip job beside the target until it ends.
func runStrip(c *Care, h *fakeHost, doer, target entity.EntityID) {
	h.Place(doer, 6, 5)
	j := h.jobs[doer]
	j.State = job.Do
	h.jobs[doer] = j
	for n := 0; h.jobs[doer].State == job.Do && n < 10*StripWork; n++ {
		c.Do(doer, h.jobs[doer])
	}
}

func TestStripOrderOnDownedAndPrisoner(t *testing.T) {
	c, h, pr, g, _ := stripScene()
	h.outsider(raider, 7, 7)
	h.vital[raider] = health.Record{Status: health.Downed, Cause: health.CauseInjury}
	pr.raiders[raider] = true
	g.worn[raider] = true
	h.outsider(prisonID, 2, 2)
	h.prisoners[prisonID] = true
	h.vital[prisonID] = health.Record{Status: health.Alive}
	g.worn[prisonID] = true
	h.outsider(visitor, 4, 4)
	h.vital[visitor] = health.Record{Status: health.Alive}
	pr.visitors[visitor] = true
	for _, tg := range []entity.EntityID{patient, raider, prisonID} {
		o, ok := optionFor(c.PawnOrders(doc, h.pos[tg][0], h.pos[tg][1]), command.OrderStrip, tg)
		if !ok || o.Disabled {
			t.Errorf("Strip %d: a downed pawn of any kind or a prisoner may be stripped: %+v", tg, o)
		}
	}
	if _, ok := optionFor(c.PawnOrders(doc, 4, 4), command.OrderStrip, visitor); ok {
		t.Error("a standing visitor can't be stripped")
	}
}

func TestStripDisabledReasons(t *testing.T) {
	c, h, _, g, _ := stripScene()
	delete(g.worn, patient)
	if o, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderStrip, patient); !ok || o.Reason != ReasonNothingToStrip {
		t.Errorf("nothing to strip: %+v", o)
	}
	g.worn[patient] = true
	h.noPath = true
	if o, _ := optionFor(c.PawnOrders(doc, 5, 5), command.OrderStrip, patient); o.Reason != ReasonUnreachable {
		t.Errorf("unreachable: %+v", o)
	}
	h.noPath = false
	h.blocked[doc] = false
	h.drafted[doc] = false
	h.vital[doc] = health.Record{Status: health.Downed}
	if o, _ := optionFor(c.PawnOrders(doc, 5, 5), command.OrderStrip, patient); o.Reason != ReasonIncapable {
		t.Errorf("a doer that can't act: %+v", o)
	}
}

func TestForceStripDropsGear(t *testing.T) {
	c, h, _, g, _ := stripScene()
	if why := c.Force(doc, command.OrderStrip, patient); why != "" {
		t.Fatalf("force strip: %q", why)
	}
	if j := h.jobs[doc]; j.Kind != job.Strip || j.Target != patient || !j.Forced || j.TX != 6 {
		t.Fatalf("strip job %+v: beside the target, forced", j)
	}
	runStrip(c, h, doc, patient)
	if g.strip != 1 || h.jobs[doc].State != job.Complete {
		t.Fatalf("StripGear once, job complete: %d, %+v", g.strip, h.jobs[doc])
	}
}

func TestStripGuestGoodwill(t *testing.T) {
	for _, tc := range []struct {
		name    string
		faction uint64
		status  health.Status
		want    []int
	}{
		{"downed guest of a faction", 7, health.Downed, []int{StripGoodwill}},
		{"downed stranger without a faction", 0, health.Downed, nil},
	} {
		c, h, _, g, gu := stripScene()
		h.outsider(guestID, 5, 6)
		h.vital[guestID] = health.Record{Status: tc.status, Cause: health.CauseInjury}
		gu.guests[guestID] = tc.faction
		g.worn[guestID] = true
		if why := c.Force(doc, command.OrderStrip, guestID); why != "" {
			t.Fatalf("%s: %q", tc.name, why)
		}
		h.pos[guestID] = [2]int{5, 5}
		runStrip(c, h, doc, guestID)
		if g.strip != 1 || len(gu.goodwill) != len(tc.want) || len(tc.want) > 0 && gu.goodwill[0] != tc.want[0] {
			t.Errorf("%s: goodwill %v, want %v (strips %d)", tc.name, gu.goodwill, tc.want, g.strip)
		}
	}
}

func TestStripNothingLeftSaysSo(t *testing.T) {
	c, h, _, g, _ := stripScene()
	c.Force(doc, command.OrderStrip, patient)
	delete(g.worn, patient) // someone else got there first
	runStrip(c, h, doc, patient)
	if g.strip != 0 || len(h.log) == 0 || h.log[len(h.log)-1] != "message: P2 has nothing to strip." {
		t.Fatalf("log %q", h.log)
	}
}

func TestStripEndsWhenTheTargetStandsUp(t *testing.T) {
	c, h, _, g, _ := stripScene()
	c.Force(doc, command.OrderStrip, patient)
	h.vital[patient] = health.Record{Status: health.Alive}
	runStrip(c, h, doc, patient)
	if g.strip != 0 || h.jobs[doc].State != job.Complete {
		t.Fatalf("a colonist on its feet is not stripped: %+v", h.jobs[doc])
	}
}
