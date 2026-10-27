package ebiten

import (
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
	if !a.selectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.selectedAnimal()
	if got := a.animalStateLine(an); got != "Downed" {
		t.Fatalf("tame state line %q", got)
	}
	if lines := thingLines(a.animalPane(an.ID)); !strings.Contains(lines, "Downed") {
		t.Fatalf("pane lines %q lack Downed", lines)
	}
	if got := animalDoing(an); got != "Downed" {
		t.Fatalf("Animals table status %q", got)
	}
	wild := an
	wild.Tame = false
	if got := a.animalStateLine(wild); got != "Wild | Downed" {
		t.Fatalf("wild state line %q", got)
	}
	cs, ok := a.renderer.animalFigureScale(an)
	if !ok || cs == noCS || cs[3] != 1 {
		t.Fatalf("a downed figure is tinted, opaque: %v %v", cs, ok)
	}
	an.Downed = false
	if cs, _ := a.renderer.animalFigureScale(an); cs != noCS && !a.renderer.animalRestFallback(an) {
		t.Fatalf("a standing figure is untinted: %v", cs)
	}
}
