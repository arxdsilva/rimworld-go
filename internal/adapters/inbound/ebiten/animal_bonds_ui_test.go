package ebiten

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// bondSim is a stub colony with two colonists and a tame dog, master Ada,
// bonded with Bo, its Obedience half learnt and Guard allowed.
func bondSim() *stubSim {
	dog := game.AnimalView{ID: 50, Species: "labrador", Label: "labrador", X: 6, Y: 6, Tame: true, Name: "Rook",
		Mode: "wandering", State: "wandering", Health: 100, Food: 80, Rest: 80,
		Master: "Ada", MasterID: 1, Bond: "Bo", BondID: 2,
		Trained: []bool{false, false, false},
		Training: []game.AnimalTrainingView{
			{Skill: 0, Label: "Obedience", Allowed: true, On: true, Steps: 2, Needed: 3},
			{Skill: 1, Label: "Guard", Allowed: true, Steps: 0, Needed: 3},
			{Skill: 2, Label: "Release", Needed: 2},
		},
		Mother: "Clover", Offspring: []string{"Ash", "Moss"}}
	return &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 2, Y: 2}, {ID: 2, Name: "Bo", X: 3, Y: 2}},
		world: game.WorldView{Animals: []game.AnimalView{dog}}}
}

func TestAnimalInspectorBondBlock(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	if !a.selectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.selectedAnimal()
	rows := strings.Join(animalBondRows(an), "|")
	for _, want := range []string{"Master: Ada", "Bond: Bo", "Mother: Clover", "Young: Ash, Moss"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("bond rows %q lack %q", rows, want)
		}
	}
	ids := map[string]button{}
	for _, b := range a.animalButtons() {
		ids[b.id] = b
	}
	if _, ok := ids["animal.train.2"]; ok {
		t.Fatal("a skill the species can't learn has no checkbox")
	}
	if !ids["animal.train.0"].selected || ids["animal.train.1"].selected || ids["animal.train.0"].label != "Obedience 2/3" {
		t.Fatalf("checkboxes %+v %+v", ids["animal.train.0"], ids["animal.train.1"])
	}
	panel := a.animalPanel()
	for _, b := range ids {
		if b.y+b.h > panel.y+panel.h {
			t.Fatalf("button %s falls out of the panel", b.id)
		}
	}
	click(t, a, "animal.train.1")
	click(t, a, "animal.train.0")
	if len(s.cmds) != 2 {
		t.Fatalf("cmds %+v", s.cmds)
	}
	if c := s.cmds[0]; c.Kind != command.SetTraining || c.Target != 50 || c.Index != 1 || !c.Enabled {
		t.Fatalf("guard on: %+v", c)
	}
	if c := s.cmds[1]; c.Kind != command.SetTraining || c.Index != 0 || c.Enabled {
		t.Fatalf("obedience off: %+v", c)
	}
	click(t, a, "animal.master")
	if !a.float.open || len(a.float.opts) != 3 {
		t.Fatalf("the master picker lists both colonists and none: %+v", a.float.opts)
	}
	if a.float.opts[0].Label != "Ada (master)" || a.float.opts[2].Cmd.Pawn != 0 {
		t.Fatalf("picker %+v", a.float.opts)
	}
	click(t, a, "float.1")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.SetAnimalMaster || c.Target != 50 || c.Pawn != 2 {
		t.Fatalf("pick Bo: %+v", c)
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawAnimalInspector(screen) // no art: must not panic
}

func TestWildAnimalHasNoMasterRow(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{ID: 9, Species: "deer", Mode: "wandering", State: "wandering", Mother: "a deer"}
	rows := animalBondRows(an)
	if len(rows) != 1 || rows[0] != "Mother: a deer" {
		t.Fatalf("wild rows %v", rows)
	}
	_, btns := animalBondLayout(an, button{w: 300})
	if len(btns) != 0 {
		t.Fatal("no master or training buttons for a wild animal")
	}
}

func TestAnimalsTabMasterAndTrainingColumns(t *testing.T) {
	t.Parallel()
	an := bondSim().world.Animals[0]
	if animalMasterCell(an) != "Ada" || animalMasterCell(game.AnimalView{}) != "-" {
		t.Fatal("master cell")
	}
	if got := animalTrainingCell(an); got != "Obedience 2/3" {
		t.Fatalf("training cell %q", got)
	}
	if colBtn+196 > 1280-2*hudPad {
		t.Fatal("the Colony list fits a 1280 px window")
	}
	s := bondSim()
	a, _ := colonyApp(t, s)
	click(t, a, "tab.animals")
	screen := ebiten.NewImage(1280, 720)
	a.drawAnimalsTab(screen, 1280, 720)
}
