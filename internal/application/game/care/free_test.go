package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

const heldID entity.EntityID = 31

// fakeCaptives holds the quest prisoners and records who freed whom.
type fakeCaptives struct {
	held  map[entity.EntityID]bool
	freed [][2]entity.EntityID
	// help: the helpable person and who helped them
	help   map[entity.EntityID]bool
	helped [][2]entity.EntityID
}

func (f *fakeCaptives) Helpable(id entity.EntityID) bool { return f.help[id] }
func (f *fakeCaptives) Helped(by, id entity.EntityID) {
	f.helped = append(f.helped, [2]entity.EntityID{by, id})
	delete(f.help, id)
}

func (f *fakeCaptives) Held(id entity.EntityID) bool { return f.held[id] }
func (f *fakeCaptives) Freed(by, id entity.EntityID) {
	f.freed = append(f.freed, [2]entity.EntityID{by, id})
	delete(f.held, id)
}

// freeScene: doctor doc at (0, 0) and a held captive at (5, 5), listed with
// the outsiders.
func freeScene() (*Care, *fakeHost, *fakeCaptives) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.outsider(heldID, 5, 5)
	cp := &fakeCaptives{held: map[entity.EntityID]bool{heldID: true}}
	return New(h, Roles{Prison: newFakePrison(h), Captives: cp}), h, cp
}

// runFree walks doer beside the captive and runs its job for n ticks.
func runFree(c *Care, h *fakeHost, doer entity.EntityID, n int) {
	h.Place(doer, 6, 5)
	j := h.jobs[doer]
	j.State = job.Do
	h.jobs[doer] = j
	for i := 0; i < n && h.jobs[doer].State == job.Do; i++ {
		c.Do(doer, h.jobs[doer])
	}
}

func TestFreeOrderReachesTargetAndFreesOnce(t *testing.T) {
	c, h, cp := freeScene()
	o, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID)
	if !ok || o.Disabled || o.Label != "Free N31" {
		t.Fatalf("the menu offers Free on a held captive: %+v %v", o, ok)
	}
	if why := c.Force(doc, command.OrderFreePrisoner, heldID); why != "" {
		t.Fatalf("force: %q", why)
	}
	if j := h.jobs[doc]; j.Kind != job.Strip || j.Count != freeMark || j.Target != heldID || !j.Forced || j.TX != 6 || j.TY != 5 {
		t.Fatalf("job %+v: beside the captive, forced", j)
	}
	runFree(c, h, doc, FreeWork-1)
	if len(cp.freed) != 0 || h.jobs[doc].State != job.Do {
		t.Fatal("freed before the work is done")
	}
	runFree(c, h, doc, 10)
	if len(cp.freed) != 1 || cp.freed[0] != [2]entity.EntityID{doc, heldID} || h.jobs[doc].State != job.Complete {
		t.Fatalf("freed once and the job complete: %+v %+v", cp.freed, h.jobs[doc])
	}
	// Stepping a finished job again frees nothing more, and the option is gone.
	c.Do(doc, h.jobs[doc])
	if _, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); ok || len(cp.freed) != 1 {
		t.Fatalf("a freed captive is offered again or freed twice: %+v", cp.freed)
	}
	if why := c.Force(doc, command.OrderFreePrisoner, heldID); why != ReasonNoLonger {
		t.Fatalf("a second order: %q", why)
	}
}

func TestFreeOrderRefusals(t *testing.T) {
	c, h, cp := freeScene()
	h.noPath = true
	if o, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); !ok || o.Reason != ReasonUnreachable {
		t.Fatalf("walled off: %+v", o)
	}
	if why := c.Force(doc, command.OrderFreePrisoner, heldID); why != ReasonUnreachable {
		t.Fatalf("force walled off: %q", why)
	}
	h.noPath = false
	h.downed(doc, 50)
	if o, _ := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); o.Reason != ReasonIncapable {
		t.Fatalf("incapable doer: %+v", o)
	}
	h.vital[doc] = health.Record{Status: health.Alive}
	h.owners[heldID] = 7
	if o, _ := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); o.Reason == "" {
		t.Fatalf("held by someone else: %+v", o)
	}
	delete(h.owners, heldID)
	// Not a quest prisoner: no option, no order. Nor any role: no order.
	cp.held[heldID] = false
	if _, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); ok {
		t.Fatal("offered on a pawn that is not held")
	}
	bare := New(h, Roles{Prison: newFakePrison(h)})
	if why := bare.Force(doc, command.OrderFreePrisoner, heldID); why != ReasonNoLonger {
		t.Fatalf("no Captives role: %q", why)
	}
	bare.Free.Do(doc, job.Record{Kind: job.Strip, Count: freeMark, Target: heldID, State: job.Do})
}

// A job that loses its captive, or its adjacency, ends without freeing; a
// strip with Count 0 is still a strip.
func TestFreeJobEndsWhenCaptiveGoneOrFar(t *testing.T) {
	c, h, cp := freeScene()
	c.Force(doc, command.OrderFreePrisoner, heldID)
	j := h.jobs[doc]
	j.State = job.Do // still at (0, 0): not beside the captive
	h.jobs[doc] = j
	c.Do(doc, j)
	if h.jobs[doc].State != job.Complete || len(cp.freed) != 0 {
		t.Fatalf("far from the captive: %+v", h.jobs[doc])
	}
}
