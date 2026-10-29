package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
)

// MED-09: the animal inspector's health lines and care selector, and the
// Animals table's Health cell, all from the view.
func TestAnimalInspectorHealthBlock(t *testing.T) {
	t.Parallel()
	s := bondSim()
	dog := &s.world.Animals[0]
	dog.Health = 62
	dog.HealthState = "Bleeding 25%/day · death in 31h"
	dog.BleedPerDay, dog.BleedDeathHours = 250, 31
	dog.BloodLossPct, dog.BloodStage = 18, "minor"
	dog.InfectionPct, dog.ImmunityPct, dog.InfectionLosing = 34, 20, true
	dog.NeedsTend = true
	dog.CareLevel = uint8(medcare.HerbalMax)
	a, _ := colonyApp(t, s)
	if !a.selectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.selectedAnimal()
	lines := thingLines(a.animalPane(an.ID))
	for _, want := range []string{"Bleeding 25%/day · death in 31h", "Blood loss 18% (minor)",
		"Infection 34% · immunity 20% (losing)", "Needs tending"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("pane lines %q lack %q", lines, want)
		}
	}
	if c := animalHealthCell(an); c != "62%!" {
		t.Fatalf("Animals table cell %q", c)
	}
	care := thingCmd(t, a.animalCommands(an.ID), "cmd.animal.care")
	if care.Label != "Care: Herbal" || care.Icon != "care_herbal" || len(care.Options()) != int(medcare.NumPolicies) {
		t.Fatalf("care command %+v", care)
	}
	click(t, a, "cmd.animal.care")
	if !a.ord.Float.Open || len(a.ord.Float.Opts) != int(medcare.NumPolicies) ||
		a.ord.Float.Opts[medcare.HerbalMax].Label != medcare.HerbalMax.Label()+" (now)" {
		t.Fatalf("the care picker lists the five levels, the current marked: %+v", a.ord.Float.Opts)
	}
	click(t, a, "float.0")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.SetMedicalCare || c.Pawn != 50 || c.Mode != uint8(medcare.NoCare) {
		t.Fatalf("pick No care: %+v", c)
	}
	// a healthy animal shows no health lines and a plain cell; a wild one
	// has no care command
	calm := game.AnimalView{ID: 9, Species: "deer", Health: 100, HealthState: "Healthy", BleedDeathHours: -1}
	if rows := animalHealthRows(calm); len(rows) != 0 || animalHealthCell(calm) != "100%" {
		t.Fatalf("healthy: rows %q cell %q", rows, animalHealthCell(calm))
	}
	s.world.Animals = append(s.world.Animals, calm)
	for _, c := range a.animalCommands(9) {
		if c.ID == "cmd.animal.care" {
			t.Fatal("a wild animal has no care command")
		}
	}
}

// MED-13: the pane lists the animal's hurt and lost parts and its moving
// while it is down, from the view; a long list folds into "+N more".
func TestAnimalInspectorPartLines(t *testing.T) {
	t.Parallel()
	s := bondSim()
	dog := &s.world.Animals[0]
	dog.Health, dog.HealthState = 70, "Injured"
	dog.MovingPct = 75
	dog.Parts = []game.AnimalPartView{
		{Label: "torso", HP: 12, MaxHP: 28, Bleeding: true},
		{Label: "front left leg", MaxHP: 17, Missing: true},
	}
	a, _ := colonyApp(t, s)
	if !a.selectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.selectedAnimal()
	lines := thingLines(a.animalPane(an.ID))
	for _, want := range []string{"Moving 75%", "Torso: 12/28 HP, bleeding", "Front left leg: missing"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("pane lines %q lack %q", lines, want)
		}
	}
	// a body with every part whole moving at full shows no part lines; a
	// long list is capped
	if rows := animalPartRows(game.AnimalView{MovingPct: 100}); rows != nil {
		t.Fatalf("whole: %q", rows)
	}
	many := game.AnimalView{MovingPct: 100}
	for i := 0; i < animalPartRowsMax+3; i++ {
		many.Parts = append(many.Parts, game.AnimalPartView{Label: "tail", HP: 1, MaxHP: 5})
	}
	rows := animalPartRows(many)
	if len(rows) != animalPartRowsMax+1 || rows[len(rows)-1] != "+3 more" || rows[0] != "Tail: 1/5 HP" {
		t.Fatalf("capped rows %q", rows)
	}
}
