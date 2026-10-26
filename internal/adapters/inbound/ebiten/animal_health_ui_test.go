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
	if care.Label != "Care: Herbal" || care.Icon != "care_herbal" || len(care.Options(a)) != int(medcare.NumPolicies) {
		t.Fatalf("care command %+v", care)
	}
	click(t, a, "cmd.animal.care")
	if !a.float.open || len(a.float.opts) != int(medcare.NumPolicies) ||
		a.float.opts[medcare.HerbalMax].Label != medcare.HerbalMax.Label()+" (now)" {
		t.Fatalf("the care picker lists the five levels, the current marked: %+v", a.float.opts)
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
