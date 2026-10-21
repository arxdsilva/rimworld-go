package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestThreatPausesOnlyForNewDanger(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 4, h: 4})
	old := game.WorldView{Messages: []game.MessageView{{Tick: 5, Text: "Raid!", Danger: true}}}
	r.watchThreats(old) // already there when the session starts
	if r.speed == speedPaused {
		t.Fatal("old threats must not pause")
	}
	w := game.WorldView{Messages: append(old.Messages,
		game.MessageView{Tick: 9, Text: "crops died"},
		game.MessageView{Tick: 10, Text: "Raid!", Danger: true})}
	r.watchThreats(w)
	if r.speed != speedPaused || r.speedRate() != "paused (threat)" {
		t.Fatalf("a new raid pauses: %v %q", r.speed, r.speedRate())
	}
	r.speed = speedNormal
	r.watchThreats(w)
	if r.speed != speedNormal || r.speedRate() == "paused (threat)" {
		t.Fatal("the same threat pauses once")
	}
	r.pauseOnThreat = false
	w.Messages = append(w.Messages, game.MessageView{Tick: 20, Text: "wolves", Danger: true})
	r.watchThreats(w)
	if r.speed == speedPaused {
		t.Fatal("option off: no pause")
	}
}

func TestOptionsTogglePauseOnThreat(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.options")
	click(t, a, "options.tab.gameplay")
	click(t, a, "options.pausethreat")
	if a.settings.Gameplay.PauseOnThreat {
		t.Fatal("toggle off")
	}
	click(t, a, "options.pausethreat")
	if !a.settings.Gameplay.PauseOnThreat {
		t.Fatal("toggle back on")
	}
}

func TestFloatTraceNumbers(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 4, h: 4})
	r.floatTrace(game.TraceView{ToX: 1, ToY: 1, Hit: true, Damage: 12}, 1)
	r.floatTrace(game.TraceView{ToX: 1, ToY: 1}, 2) // a missed shot
	r.floatTrace(game.TraceView{ToX: 1, ToY: 1, Melee: true}, 3)
	if len(r.floaters) != 2 || r.floaters[0].text != "-12" || r.floaters[1].text != "miss" {
		t.Fatalf("floaters %+v", r.floaters)
	}
}
