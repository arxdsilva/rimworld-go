package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"testing"

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
	labelled := false
	for _, b := range a.animalButtons() {
		if b.id == "animal.hunt" && b.label == "Cancel hunt" {
			labelled = true
		}
	}
	if !labelled {
		t.Fatal("button flips to Cancel hunt")
	}
	// drawing the inspector and the world must not panic without art
	screen := ebiten.NewImage(640, 480)
	a.drawAnimalInspector(screen)
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
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 9, 9
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
