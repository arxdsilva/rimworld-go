package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	raider   entity.EntityID = 4 // a raider (R4)
	visitor  entity.EntityID = 5 // a visitor (R5)
	mate     entity.EntityID = 6 // a second colonist (P6)
	carrier  entity.EntityID = 7 // a colonist that carries (P7)
	prisonID entity.EntityID = 11
)

// orderScene: doctor doc at (0, 0); on tile (5, 5) the downed colonist
// patient, the downed raider, the standing visitor and the standing
// colonist mate; one free bed and one free prisoner bed.
func orderScene() (*Care, *fakeHost, *orderPrison) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.pawn(mate, 5, 5)
	h.pawn(other, 9, 9)
	h.pawn(carrier, 9, 9)
	h.doctors[doc] = true
	h.downed(patient, 50)
	h.outsider(raider, 5, 5)
	h.vital[raider] = health.Record{Status: health.Downed, Cause: health.CauseInjury, Injury: 50}
	h.tend[raider] = true
	h.outsider(visitor, 5, 5)
	h.vital[visitor] = health.Record{Status: health.Alive}
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 8, Y: 8})
	pr := newOrderPrison(h)
	pr.raider[raider] = true
	pr.visitor[visitor] = true
	pr.prisonBd = prisonID
	return New(h, pr), h, pr
}

func TestPawnOrdersMenu(t *testing.T) {
	c, h, _ := orderScene()
	want := []string{"Rescue P2", "Capture R4", "Arrest P6", "Arrest R5", "Prioritize tending P2"}
	opts := c.PawnOrders(doc, 5, 5)
	if got := labels(opts); !slices.Equal(got, want) {
		t.Fatalf("undrafted: rescue, capture, arrest, tend, each in roster order:\n got %q\nwant %q", got, want)
	}
	if opts[0].Cmd != (command.Command{Kind: command.Prioritize, Order: command.OrderRescue, Pawn: uint64(doc),
		Target: uint64(patient), X: 5, Y: 5}) {
		t.Fatalf("the rescue command names the doer, the target and the tile: %+v", opts[0].Cmd)
	}
	h.drafted[doc] = true
	want = []string{"Rescue P2", "Capture R4", "Arrest P6", "Arrest R5", "Tend P2"}
	opts = c.PawnOrders(doc, 5, 5)
	if got := labels(opts); !slices.Equal(got, want) {
		t.Fatalf("drafted: today's Tend replaces Prioritize tending:\n got %q\nwant %q", got, want)
	}
	if opts[4].Cmd.Kind != command.EmergencyTend {
		t.Fatalf("drafted Tend is an EmergencyTend: %+v", opts[4].Cmd)
	}
	if c.PawnOrders(doc, 1, 1) != nil {
		t.Fatal("nobody there: no entries")
	}
	if got := labels(c.PawnOrders(mate, 5, 5)); slices.Contains(got, "Arrest P6") {
		t.Fatalf("a pawn never targets itself: %q", got)
	}
}

// TestPawnOrdersDraftedTend is today's drafted "Tend <name>" (moved from
// policy_test.go, SIM-062): colonists, then prisoners; no entry for one
// carried; a held one disabled with its holder.
func TestPawnOrdersDraftedTend(t *testing.T) {
	c, h, _ := orderScene()
	h.raiders = []entity.EntityID{13, 14} // the raider and the visitor are gone
	h.pos[13], h.pos[14] = [2]int{5, 5}, [2]int{5, 5}
	h.vital[13] = health.Record{Status: health.Downed}
	h.tend[13], h.tend[14] = true, true
	h.prisoners[13] = true // raider 14 is no prisoner
	tends := func() []Option {
		var out []Option
		for _, o := range c.PawnOrders(doc, 5, 5) {
			if o.Cmd.Kind == command.EmergencyTend {
				out = append(out, o)
			}
		}
		return out
	}
	if tends() != nil {
		t.Fatal("undrafted: no Tend order")
	}
	h.drafted[doc] = true
	opts := tends()
	if len(opts) != 2 || opts[0].Cmd.Target != uint64(patient) || opts[1].Cmd.Target != uint64(13) ||
		opts[0].Disabled || opts[0].Label != "Tend "+h.Name(patient) {
		t.Fatalf("Tend the colonist, then the prisoner: %+v", opts)
	}
	h.owners[13] = carrier
	c.Carry.Pick(8, patient)
	opts = tends()
	if len(opts) != 1 || !opts[0].Disabled || opts[0].Reason != ReasonBusyTarget+h.Name(carrier) {
		t.Fatalf("carried: no entry; held: disabled with the reason: %+v", opts)
	}
}

// orderCase is one fixture for an order on a target: hidden (no option),
// or offered and disabled with why ("" enabled).
type orderCase struct {
	name   string
	o      command.Order
	doer   entity.EntityID
	target entity.EntityID
	set    func(c *Care, h *fakeHost, pr *orderPrison)
	hidden bool
	why    string
}

func orderCases() []orderCase {
	none := func(*Care, *fakeHost, *orderPrison) {}
	resc, capt, arr, tend := command.OrderRescue, command.OrderCapture, command.OrderArrest, command.OrderTendPatient
	return []orderCase{
		{"rescue", resc, doc, patient, none, false, ""},
		{"rescue drafted", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.drafted[doc] = true }, false, ""},
		{"rescue without Doctor priority", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.doctors[doc] = false }, false, ""},
		{"rescue no bed", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.bldgIDs = nil }, false, ReasonNoBed},
		{"rescue by a raider", resc, raider, patient, none, false, ReasonIncapable},
		{"rescue by a downed doer", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.downed(doc, 40) }, false, ReasonIncapable},
		{"rescue by a breaking doer", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.breaking = map[entity.EntityID]bool{doc: true} }, false, ReasonIncapable},
		{"rescue incapable of Doctor", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.blocked[doc] = true }, false, ReasonIncapable},
		{"rescue carried", resc, doc, patient, func(c *Care, _ *fakeHost, _ *orderPrison) { c.Carry.Pick(carrier, patient) }, false, ReasonCarried + "P7"},
		{"rescue held", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.owners[patient] = other }, false, ReasonBusyTarget + "P3"},
		{"rescue held by the doer", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.owners[patient] = doc }, false, ""},
		{"rescue unreachable", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.noPath = true }, false, ReasonUnreachable},
		{"rescue standing", resc, doc, mate, none, true, ""},
		{"rescue in bed", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) {
			h.bldgs[bedID] = construction.Building{Kind: construction.Bed, X: 5, Y: 5, Claimant: uint64(patient)}
		}, true, ""},
		{"rescue a prisoner", resc, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.prisoners[patient] = true }, true, ""},
		{"rescue a raider", resc, doc, raider, none, true, ""},
		{"rescue itself", resc, patient, patient, none, true, ""},

		{"capture", capt, doc, raider, none, false, ""},
		{"capture a downed visitor", capt, doc, visitor, func(_ *Care, h *fakeHost, _ *orderPrison) { h.downed(visitor, 40) }, false, ""},
		{"capture incapable of Doctor", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.blocked[doc] = true }, false, ""},
		{"capture no prison bed", capt, doc, raider, func(_ *Care, _ *fakeHost, pr *orderPrison) { pr.prisonBd = 0 }, false, ReasonNoPrisonBed},
		{"capture by a breaking doer", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.breaking = map[entity.EntityID]bool{doc: true} }, false, ReasonIncapable},
		{"capture unreachable", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.noPath = true }, false, ReasonUnreachable},
		{"capture held", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.owners[raider] = other }, false, ReasonBusyTarget + "P3"},
		{"capture being captured", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) {
			h.owners[raider] = other
			h.jobs[other] = jobOn(job.Capture, raider)
		}, true, ""},
		{"capture a standing raider", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.vital[raider] = health.Record{Status: health.Alive} }, true, ""},
		{"capture a prisoner", capt, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.prisoners[raider] = true }, true, ""},
		{"capture a colonist", capt, doc, patient, none, true, ""},

		{"arrest a visitor", arr, doc, visitor, none, false, ""},
		{"arrest a colonist", arr, doc, mate, none, false, ""},
		{"arrest no prison bed", arr, doc, visitor, func(_ *Care, _ *fakeHost, pr *orderPrison) { pr.prisonBd = 0 }, false, ReasonNoPrisonBed},
		{"arrest by a downed doer", arr, doc, visitor, func(_ *Care, h *fakeHost, _ *orderPrison) { h.downed(doc, 40) }, false, ReasonIncapable},
		{"arrest unreachable", arr, doc, visitor, func(_ *Care, h *fakeHost, _ *orderPrison) { h.noPath = true }, false, ReasonUnreachable},
		{"arrest held", arr, doc, visitor, func(_ *Care, h *fakeHost, _ *orderPrison) { h.owners[visitor] = other }, false, ReasonBusyTarget + "P3"},
		{"arrest being arrested", arr, doc, visitor, func(_ *Care, h *fakeHost, _ *orderPrison) {
			h.owners[visitor] = other
			h.jobs[other] = jobOn(job.Arrest, visitor)
		}, true, ""},
		{"arrest a downed colonist", arr, doc, patient, none, true, ""},
		{"arrest a raider", arr, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.vital[raider] = health.Record{Status: health.Alive} }, true, ""},
		{"arrest itself", arr, mate, mate, none, true, ""},

		{"tend", tend, doc, patient, none, false, ""},
		{"tend without Doctor priority", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.doctors[doc] = false }, false, ""},
		{"tend a prisoner", tend, doc, raider, func(_ *Care, h *fakeHost, _ *orderPrison) { h.prisoners[raider] = true }, false, ""},
		{"tend incapable of Doctor", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.blocked[doc] = true }, false, ReasonIncapable},
		{"tend by a visitor", tend, visitor, patient, none, false, ReasonIncapable},
		{"tend held", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.owners[patient] = other }, false, ReasonBusyTarget + "P3"},
		{"tend unreachable", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.noPath = true }, false, ReasonUnreachable},
		{"tend drafted", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.drafted[doc] = true }, true, ""},
		{"tend nothing to tend", tend, doc, patient, func(_ *Care, h *fakeHost, _ *orderPrison) { h.tend[patient] = false }, true, ""},
		{"tend no care", tend, doc, patient, func(c *Care, _ *fakeHost, _ *orderPrison) { c.Policy.Set(patient, uint8(medcare.NoCare)) }, true, ""},
		{"tend carried", tend, doc, patient, func(c *Care, _ *fakeHost, _ *orderPrison) { c.Carry.Pick(carrier, patient) }, true, ""},
		{"tend a raider", tend, doc, raider, none, true, ""},
		{"tend itself", tend, patient, patient, none, true, ""},
	}
}

// TestPawnOrdersReasons: each fixture's option is present, hidden or
// disabled with its reason.
func TestPawnOrdersReasons(t *testing.T) {
	for _, tc := range orderCases() {
		c, h, pr := orderScene()
		tc.set(c, h, pr)
		x, y, _ := h.Pos(tc.target)
		op, ok := optionFor(c.PawnOrders(tc.doer, x, y), tc.o, tc.target)
		switch {
		case tc.hidden && ok:
			t.Errorf("%s: want no option, got %+v", tc.name, op)
		case tc.hidden:
		case !ok:
			t.Errorf("%s: no option", tc.name)
		case op.Disabled != (tc.why != "") || op.Reason != tc.why:
			t.Errorf("%s: disabled %v reason %q, want %q", tc.name, op.Disabled, op.Reason, tc.why)
		}
	}
}
