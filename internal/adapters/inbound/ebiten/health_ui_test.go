package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// healthSimView is a colonist missing its left leg and foot with a cut
// right arm, a peg leg queued.
func healthSimView() game.HealthView {
	parts := make([]game.PartView, body.NumParts)
	for p := body.Part(0); p < body.NumParts; p++ {
		parts[p] = game.PartView{Label: p.Label(), Status: game.PartHealthy, HP: body.DefOf(p).MaxHP}
	}
	parts[body.LegL].Status, parts[body.LegL].HP = game.PartMissing, 0
	parts[body.FootL].Status, parts[body.FootL].HP = game.PartMissing, 0
	parts[body.ArmR].Status, parts[body.ArmR].HP = "wounded: 1 wound", 25
	parts[body.Jaw].Status = game.PartArtificial + ": denture"
	return game.HealthView{
		Capacities: []game.CapacityView{{Label: "Consciousness", Pct: 96}, {Label: "Moving", Pct: 50}, {Label: "Talking", Pct: 77}},
		Parts:      parts, Pain: 12,
		Operations: []game.OperationView{{Label: "Install peg leg (left leg)", Slot: 0}},
	}
}

func healthApp(t *testing.T) (*App, *fakeInput, *stubSim) {
	t.Helper()
	s := bioSim()
	s.health = map[entity.EntityID]game.HealthView{1: healthSimView()}
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.frame(t, a)
	click(t, a, "inspector.health")
	f.frame(t, a)
	return a, f, s
}

func TestHealthTabListsPartsAndOperations(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	for _, id := range []string{"health.part.16", "health.part.18", "health.part.13", "health.part.6", healthRemoveID(0), healthAddID} {
		if !hasWidget(a, id) {
			t.Fatalf("widget %s missing", id)
		}
	}
	if hasWidget(a, "health.part.0") {
		t.Fatal("a healthy brain is not listed")
	}
	if w, _ := a.uiSt().find("health.part.16"); w.label != "left leg: missing" {
		t.Fatalf("leg row = %q", w.label)
	}
	press(t, a, f, healthRemoveID(0))
	if n := len(s.cmds); n == 0 || s.cmds[n-1].Kind != command.RemoveOperation || s.cmds[n-1].Index != 0 || s.cmds[n-1].Pawn != 1 {
		t.Fatalf("cancel sends RemoveOperation: %+v", s.cmds)
	}
	drawFrame(t, a, f)
}

func TestHealthOptionsFollowTheBody(t *testing.T) {
	t.Parallel()
	opts := healthOptions(healthSimView())
	labels := map[string]healthOption{}
	for _, o := range opts {
		labels[o.label] = o
	}
	if o, ok := labels["Install prosthetic leg (left leg)"]; !ok || o.op != body.InstallImplant || o.part != body.LegL || o.item != item.ProstheticLeg {
		t.Fatalf("prosthetic leg option = %+v (%v)", o, ok)
	}
	if _, ok := labels["Install peg leg (left leg)"]; ok {
		t.Fatal("a queued operation is not offered again")
	}
	if _, ok := labels["Remove implant (jaw)"]; !ok {
		t.Fatal("an implant can be removed")
	}
	if _, ok := labels["Amputate right arm"]; !ok {
		t.Fatal("a limb can be amputated")
	}
	for _, o := range opts {
		if o.part == body.Heart || o.part == body.Brain || o.part == body.Torso {
			t.Fatalf("vital parts are never amputated: %+v", o)
		}
		if o.part == body.FootL && o.op == body.InstallImplant {
			t.Fatalf("nothing fits a missing foot: %+v", o)
		}
	}
}

func TestHealthPickerSendsAddOperation(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	press(t, a, f, healthAddID)
	f.frame(t, a)
	if a.uiModalTop() != healthPickID {
		t.Fatalf("picker not open (top %q)", a.uiModalTop())
	}
	if w, ok := a.uiSt().find("modal." + healthPickID + ".health.addop.ok"); !ok || !w.disabled {
		t.Fatalf("Add waits for a pick: %+v", w)
	}
	opts := healthOptions(s.health[1])
	want := -1
	for i, o := range opts {
		if o.item == item.ProstheticLeg {
			want = i
		}
	}
	press(t, a, f, optID(healthPickList, want))
	press(t, a, f, "modal."+healthPickID+".health.addop.ok")
	var got *command.Command
	for i := range s.cmds {
		if s.cmds[i].Kind == command.AddOperation {
			got = &s.cmds[i]
		}
	}
	if got == nil || got.Pawn != 1 || got.Index != int(body.InstallImplant) || got.Build != uint8(body.LegL) || got.Stuff != uint8(item.ProstheticLeg) {
		t.Fatalf("AddOperation = %+v", got)
	}
	if a.uiModalTop() == healthPickID {
		t.Fatal("Add closes the picker")
	}
}

func TestHealthTabStates(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	s.health = nil
	drawFrame(t, a, f)
	if hasWidget(a, healthAddID) {
		t.Fatal("no read model: No data yet, no controls")
	}
	h := healthSimView()
	h.Operations = nil
	for i := range h.Parts {
		h.Parts[i].Status = game.PartHealthy
	}
	s.health = map[entity.EntityID]game.HealthView{1: h}
	s.pawns[0].Status = "dead"
	drawFrame(t, a, f)
	if w, ok := a.uiSt().find(healthAddID); !ok || !w.disabled || w.reason == "" {
		t.Fatalf("a dead colonist's Add operation explains why: %+v", w)
	}
	if hasWidget(a, healthRemoveID(0)) || hasWidget(a, "health.part.16") {
		t.Fatal("nothing queued, nothing hurt: no rows")
	}
}
