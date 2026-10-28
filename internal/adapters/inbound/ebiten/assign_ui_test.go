package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// policySim is the stub with the drug policy read model.
type policySim struct {
	*stubSim
	policy map[entity.EntityID]uint8
}

func (s *policySim) DrugPolicy(id entity.EntityID) uint8 { return s.policy[id] }

func assignStub() *policySim {
	s := &stubSim{pawns: []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive"},
		{ID: 2, Name: "Bo", Status: "alive"},
		{ID: 3, Name: "Cy", Status: "dead"},
	}}
	return &policySim{stubSim: s, policy: map[entity.EntityID]uint8{1: uint8(drug.SocialDrinking), 2: uint8(drug.NoDrugs)}}
}

func TestAssignTabOpensPolicyTable(t *testing.T) {
	t.Parallel()
	s := assignStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s // the stub with the read model

	var tab button
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		if b.id == "tab.assign" {
			tab = b
		}
	}
	if tab.disabled || tab.label != "Assign" {
		t.Fatalf("Assign tab = %+v, want enabled", tab)
	}
	press(t, a, f, "tab.assign")
	f.Frame(t, a)
	if a.uiModalTop() != assignModalID {
		t.Fatalf("top modal = %q, want %q", a.uiModalTop(), assignModalID)
	}
	if !hasWidget(a, assignDrugID(1)) || !hasWidget(a, assignDrugID(2)) {
		t.Fatal("a drug policy button per living colonist")
	}
	if hasWidget(a, assignDrugID(3)) {
		t.Fatal("the dead get no row")
	}
	if w, _ := a.uiSt().find(assignDrugID(1)); w.label != "Social drinking" {
		t.Fatalf("Ada's button %q", w.label)
	}

	// A click cycles to the next preset through a typed command.
	press(t, a, f, assignDrugID(2))
	var got []command.Command
	for _, c := range s.cmds {
		if c.Kind == command.SetDrugPolicy {
			got = append(got, c)
		}
	}
	if len(got) != 1 || got[0].Pawn != 2 || drug.Preset(got[0].Mode) != drug.SocialDrinking {
		t.Fatalf("commands %+v, want Bo to social drinking", got)
	}

	// Its key (F3) closes the window again.
	a.toggleAssign()
	f.Frame(t, a)
	if a.uiModalTop() == assignModalID {
		t.Fatal("the Assign key toggles the window")
	}
}

// Without the read model the column is disabled and says why.
func TestAssignTabWithoutPolicies(t *testing.T) {
	t.Parallel()
	s := assignStub()
	a, f := colonyApp(t, s.stubSim)
	a.openAssign()
	f.Frame(t, a)
	w, ok := a.uiSt().find(assignDrugID(1))
	if !ok || !w.disabled || w.reason == "" {
		t.Fatalf("button %+v: disabled with a reason", w)
	}
	press(t, a, f, assignDrugID(1))
	for _, c := range s.cmds {
		if c.Kind == command.SetDrugPolicy {
			t.Fatal("a disabled button sends nothing")
		}
	}
}

func TestAssignEmptyAndDraws(t *testing.T) {
	t.Parallel()
	s := assignStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.openAssign()
	drawFrame(t, a, f)
	s.pawns = nil
	drawFrame(t, a, f)
	if hasWidget(a, assignDrugID(1)) {
		t.Fatal("no colonists: no rows")
	}
}

func TestNextPresetCycles(t *testing.T) {
	t.Parallel()
	p := drug.NoDrugs
	seen := map[drug.Preset]bool{}
	for i := 0; i < len(assignPresets); i++ {
		seen[p] = true
		p = nextPreset(p)
	}
	if p != drug.NoDrugs || len(seen) != 3 {
		t.Fatalf("cycle ends at %v after %v", p, seen)
	}
	if presetLabel(drug.Unrestricted) != "Unrestricted" {
		t.Fatal(presetLabel(drug.Unrestricted))
	}
}
