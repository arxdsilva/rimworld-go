package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestAnimalsReleaseConfirmOnAndOff(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.Animals = []game.AnimalView{{ID: 50, Species: "dustbuck", Tame: true}}
	w.Show("animals")
	w.Animals().Click("animals.release.50")
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.ReleaseAnimal {
		t.Fatalf("off: release at once: %+v", sh.cmds)
	}
	sh.cmds = nil
	sh.kit.Access.Confirm = true
	w.Animals().Click("animals.release.50")
	if len(sh.cmds) != 0 || !sh.kit.ModalOpen() {
		t.Fatalf("on: asks first (%+v)", sh.cmds)
	}
	sh.kit.Modals[0].Buttons[0].OnClick()
	if len(sh.cmds) != 1 || sh.cmds[0].Pawn != 50 {
		t.Fatalf("OK sends the release: %+v", sh.cmds)
	}
}
