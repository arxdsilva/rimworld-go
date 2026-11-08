package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// helpScene is freeScene with the captive a helpable refugee instead.
func helpScene() (*Care, *fakeHost, *fakeCaptives) {
	c, h, cp := freeScene()
	cp.held, cp.help = nil, map[entity.EntityID]bool{heldID: true}
	return c, h, cp
}

func TestOfferHelpOrderWalksAndHelpsOnce(t *testing.T) {
	t.Parallel()
	c, h, cp := helpScene()
	o, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderOfferHelp, heldID)
	if !ok || o.Disabled || o.Label != "Offer help to N31" {
		t.Fatalf("the menu offers help: %+v %v", o, ok)
	}
	if _, free := optionFor(c.PawnOrders(doc, 5, 5), command.OrderFreePrisoner, heldID); free {
		t.Fatal("a refugee is not a prisoner to free")
	}
	if why := c.Force(doc, command.OrderOfferHelp, heldID); why != "" {
		t.Fatalf("force refused: %q", why)
	}
	if j := h.jobs[doc]; j.Kind != job.Strip || j.Count != helpMark || j.Target != heldID || !j.Forced {
		t.Fatalf("the job: %+v", j)
	}
	runFree(c, h, doc, 1)
	if len(cp.helped) != 1 || cp.helped[0] != [2]entity.EntityID{doc, heldID} {
		t.Fatalf("helped %+v", cp.helped)
	}
	runFree(c, h, doc, 5)
	if len(cp.helped) != 1 {
		t.Fatalf("helped again: %+v", cp.helped)
	}
	if _, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderOfferHelp, heldID); ok {
		t.Fatal("helped, the option is still offered")
	}
	if why := c.Force(doc, command.OrderOfferHelp, heldID); why != ReasonNoLonger {
		t.Fatalf("force on a helped person: %q", why)
	}
}

func TestOfferHelpNotOfferedToHeldPrisoner(t *testing.T) {
	t.Parallel()
	c, _, _ := freeScene()
	if _, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderOfferHelp, heldID); ok {
		t.Fatal("help offered to a quest prisoner")
	}
}
