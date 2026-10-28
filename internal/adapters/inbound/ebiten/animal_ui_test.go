package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestAnimalInspectorTogglesHunt(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnAnimal("dustbuck", 10, 10); err != nil {
		t.Fatal(err)
	}
	if !a.selectAnimalAt(10, 10) {
		t.Fatal("clicking an animal selects it")
	}
	click(t, a, "animal.hunt")
	g.Step(1)
	an, ok := a.selectedAnimal()
	if !ok || !an.Designated {
		t.Fatalf("hunt button should mark it: %+v", an)
	}
	if hunt := thingCmd(t, a.animalCommands(an.ID), "animal.hunt"); hunt.Label != "Cancel hunt" || !hunt.On {
		t.Fatalf("button flips to Cancel hunt: %+v", hunt)
	}
	// the pane has the close button the click below uses
	if sp := a.animalPane(an.ID); sp.Close != "animal.close" {
		t.Fatalf("pane close = %q", sp.Close)
	}
	// drawing the inspector and the world must not panic without art
	screen := ebiten.NewImage(640, 480)
	a.drawAnimalMark(screen)
	a.drawAlerts(screen, 640)
	click(t, a, "animal.hunt")
	g.Step(1)
	if an, _ := a.selectedAnimal(); an.Designated {
		t.Fatal("cancel hunt clears the mark")
	}
	click(t, a, "animal.close")
	if _, ok := a.selectedAnimal(); ok {
		t.Fatal("close deselects")
	}
}

func TestHuntToolMarksAnimals(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnAnimal("ridgewolf", 8, 8); err != nil {
		t.Fatal(err)
	}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.orders")
	click(t, a, "arch.tool.hunt")
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 6, 9, 9
	a.submitArchitect()
	g.Step(1)
	if an := g.World().Animals; len(an) != 1 || !an[0].Designated {
		t.Fatalf("animals = %+v", an)
	}
}

// TestCancelHuntKeepsPen: the panel's "Cancel hunt" clears only the mark;
// the pen the animal stands in stays.
func TestCancelHuntKeepsPen(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	a.renderer.speed = speedPaused
	a.renderer.enqueue(command.Command{Kind: command.DesignatePen, X: 6, Y: 6, X2: 12, Y2: 12})
	aid, err := g.SpawnAnimal("dustbuck", 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	a.selectAnimalAt(9, 9)
	click(t, a, "animal.hunt") // mark
	click(t, a, "animal.hunt") // cancel
	w := g.World()
	if countZones(w, game.ZonePen) != 1 {
		t.Fatal("cancelling a hunt must not delete the pen")
	}
	for _, an := range w.Animals {
		if an.ID == aid && an.Designated {
			t.Fatal("the hunt mark must clear")
		}
	}
}
