package ebiten

import (
	"strconv"
	"testing"
)

func TestAnimalsTabListsTameAnimals(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, err := g.SpawnAnimal("dustbuck", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Clover")
	click(t, a, "tab.animals")
	if !a.workOpen || a.panelMode != "animals" {
		t.Fatal("animals tab opens the animals panel")
	}
	sid := strconv.FormatUint(uint64(id), 10)
	click(t, a, "animals.slaughter."+sid)
	click(t, a, "animals.release."+sid)
	g.Step(1)
	if an := g.World().Animals[0]; an.Tame || !an.Designated {
		t.Fatalf("slaughter marks it for hunting, release lets it go: %+v", an)
	}
}
