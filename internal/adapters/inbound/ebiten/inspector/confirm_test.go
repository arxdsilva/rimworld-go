package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestDeconstructConfirmOnAndOff(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	a.ApplyBuildingClick("bldg.deconstruct")
	if c := lastCmd(t, f); c.Kind != command.DesignateDeconstruct {
		t.Fatalf("off: deconstruct is enqueued at once: %+v", c)
	}
	f.cmds = nil
	f.Kit().Access.Confirm = true
	f.sel.HasBuilding, f.sel.SelBuilding = true, 7
	a.ApplyBuildingClick("bldg.deconstruct")
	if len(f.cmds) != 0 || !f.Kit().ModalOpen() {
		t.Fatalf("on: asks first (cmds %v, modal %v)", f.cmds, f.Kit().ModalOpen())
	}
	f.Kit().Modals[0].Buttons[0].OnClick()
	if c := lastCmd(t, f); c.Kind != command.DesignateDeconstruct || c.X != 3 || c.Y != 4 {
		t.Fatalf("OK sends the deconstruct: %+v", c)
	}
}

func TestReleaseAnimalConfirmOnAndOff(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.world.Animals = []game.AnimalView{{ID: 60, Species: "dustbuck", Tame: true}}
	f.sel.HasAnimal, f.sel.SelAnimal = true, 60
	a := New(f).Animal()
	a.ApplyAnimalClick("animal.release")
	if c := lastCmd(t, f); c.Kind != command.ReleaseAnimal || c.Pawn != 60 {
		t.Fatalf("off: release at once: %+v", c)
	}
	f.cmds = nil
	f.Kit().Access.Confirm = true
	a.ApplyAnimalClick("animal.release")
	if len(f.cmds) != 0 || !f.Kit().ModalOpen() {
		t.Fatalf("on: asks first (cmds %v)", f.cmds)
	}
	f.Kit().Modals[0].Buttons[0].OnClick()
	if c := lastCmd(t, f); c.Kind != command.ReleaseAnimal || c.Pawn != 60 {
		t.Fatalf("OK sends the release: %+v", c)
	}
}
