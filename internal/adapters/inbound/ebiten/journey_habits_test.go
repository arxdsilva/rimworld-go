package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
)

// openHabits opens Options › Habits from the pause menu of a running colony.
func openHabits(j *journey) {
	j.t.Helper()
	j.key("Escape", 0)
	j.clickID("pause.options")
	j.clickID("options.tab.habits")
	j.update(2)
	j.draw(1) // the tab paints without a panic
}

// habitsScrollTo turns the mouse wheel over the tab until the widget is
// declared (rows outside the view are not).
func habitsScrollTo(j *journey, id string) {
	j.t.Helper()
	for i := 0; i < 40; i++ {
		if _, ok := j.a.uiSt().Find(id); ok {
			return
		}
		j.f.Cur.Mx, j.f.Cur.My = int(float64(300)*j.a.scale.S), int(float64(300)*j.a.scale.S)
		j.f.Cur.WheelY = -1
		j.update(1)
		j.f.Cur.WheelY = 0
	}
	j.t.Fatalf("no widget %q after scrolling the Habits tab", id)
}

func TestJourneyHabitsToggle(t *testing.T) {
	j := newJourney(t)
	if !j.world().Gameplay.On(gameplay.OutdoorsJoy) {
		t.Fatal("outdoors joy starts off")
	}
	openHabits(j)
	j.reset()
	j.clickID("opt.habits.t" + string(rune('0'+int(gameplay.OutdoorsJoy))))
	c, ok := j.lastCmd(command.SetGameplayToggle)
	if !ok || c.Index != int(gameplay.OutdoorsJoy) || c.Enabled {
		t.Fatalf("command %+v (sent %v), want SetGameplayToggle OutdoorsJoy off", c, ok)
	}
	j.update(1) // the colony is paused behind Options: the command applies at once
	if j.world().Gameplay.On(gameplay.OutdoorsJoy) {
		t.Fatal("the read model still has outdoors joy on after the command")
	}
	// The box follows the colony: clicking again turns it back on.
	j.reset()
	j.clickID("opt.habits.t" + string(rune('0'+int(gameplay.OutdoorsJoy))))
	if c, ok := j.lastCmd(command.SetGameplayToggle); !ok || !c.Enabled {
		t.Fatalf("second click sent %+v (sent %v), want enabled", c, ok)
	}
	j.update(1) // the colony is paused behind Options: the command applies at once
	if !j.world().Gameplay.On(gameplay.OutdoorsJoy) {
		t.Fatal("outdoors joy did not come back on")
	}
}

func TestJourneyHabitsLimit(t *testing.T) {
	j := newJourney(t)
	if got := j.world().Gameplay.LimitOf(gameplay.TidyStation); got != 5 {
		t.Fatalf("tidy station starts at %d, want 5", got)
	}
	openHabits(j)
	id := "opt.habits.l0"
	habitsScrollTo(j, id+".inc")
	j.reset()
	j.clickID(id + ".inc")
	c, ok := j.lastCmd(command.SetGameplayLimit)
	if !ok || c.Index != int(gameplay.TidyStation) || c.Count != 6 {
		t.Fatalf("command %+v (sent %v), want SetGameplayLimit TidyStation 6", c, ok)
	}
	j.update(1) // the colony is paused behind Options: the command applies at once
	if got := j.world().Gameplay.LimitOf(gameplay.TidyStation); got != 6 {
		t.Fatalf("tidy station is %d after +, want 6", got)
	}
	habitsScrollTo(j, id+".dec")
	for i := 0; i < 8; i++ {
		j.clickID(id + ".dec")
		j.update(1)
	}
	if got := j.world().Gameplay.LimitOf(gameplay.TidyStation); got != 0 {
		t.Fatalf("tidy station is %d after stepping down, want 0 (never below)", got)
	}
	if j.clickID(id + ".dec") {
		t.Error("the minus button works at 0")
	}
}

func TestJourneyHabitsBackToDefaults(t *testing.T) {
	j := newJourney(t)
	openHabits(j)
	j.clickID("opt.habits.t0")
	j.update(1) // the colony is paused behind Options: the command applies at once
	j.reset()
	j.clickID("opt.habits.defaults")
	j.update(1) // the colony is paused behind Options: the command applies at once
	if r := j.world().Gameplay; r != (gameplay.Settings{}) {
		t.Fatalf("rules after Back to defaults: %+v", r)
	}
}
