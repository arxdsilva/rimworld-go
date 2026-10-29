package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"strings"
	"testing"
)

// A downed animal (from the view): the inspector's state line and the
// Animals table say "Downed", and the figure draws the rest pose tinted.
func TestAnimalInspectorDownedLine(t *testing.T) {
	t.Parallel()
	s := bondSim()
	dog := &s.world.Animals[0]
	dog.Downed, dog.Sleeping, dog.State, dog.Designated = true, true, "downed", true
	a, _ := colonyApp(t, s)
	if !a.insp.Animal().SelectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.insp.SelectedAnimal()
	if got := a.insp.Animal().AnimalStateLine(an); got != "Downed" {
		t.Fatalf("tame state line %q", got)
	}
	if lines := thingLines(a.insp.Animal().AnimalPane(an.ID)); !strings.Contains(lines, "Downed") {
		t.Fatalf("pane lines %q lack Downed", lines)
	}
	if got := windows.AnimalDoing(an); got != "Downed" {
		t.Fatalf("Animals table status %q", got)
	}
	wild := an
	wild.Tame = false
	if got := a.insp.Animal().AnimalStateLine(wild); got != "Wild | Downed" {
		t.Fatalf("wild state line %q", got)
	}
	cs, ok := a.renderer.animalFigureScale(an)
	if !ok || cs == sprite.NoCS || cs[3] != 1 {
		t.Fatalf("a downed figure is tinted, opaque: %v %v", cs, ok)
	}
	an.Downed = false
	if cs, _ := a.renderer.animalFigureScale(an); cs != sprite.NoCS && !a.renderer.animalRestFallback(an) {
		t.Fatalf("a standing figure is untinted: %v", cs)
	}
}
