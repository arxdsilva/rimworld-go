package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestDefenceLines(t *testing.T) {
	t.Parallel()
	m := game.BuildingView{Kind: "mortar", Barrel: 800, Shells: 3, HasTarget: true, TargetX: 4, TargetY: 9}
	got := strings.Join(defenceLines(m), "|")
	for _, want := range []string{"Barrel  80%", "Shells loaded  3", "Target  (4, 9)"} {
		if !strings.Contains(got, want) {
			t.Errorf("mortar lines %q lack %q", got, want)
		}
	}
	if l := defenceLines(game.BuildingView{Kind: "mortar"}); l[len(l)-1] != "No target set" {
		t.Errorf("untargeted mortar lines = %v", l)
	}
	bench := game.BuildingView{Kind: "stove", Linked: []string{"hopper"}, WorkSpeed: 1250}
	if g := strings.Join(defenceLines(bench), "|"); g != "Linked  hopper|Work speed  125%" {
		t.Errorf("bench lines = %q", g)
	}
	if l := defenceLines(game.BuildingView{Kind: "stove"}); len(l) != 0 {
		t.Errorf("plain bench lines = %v", l)
	}
}

func TestDefenceCommandsToggle(t *testing.T) {
	t.Parallel()
	var a Building
	door := a.defenceCommands(game.BuildingView{Kind: "door", X: 2, Y: 3, HoldOpen: true})
	if len(door) != 1 || door[0].Cmds[0].Kind != command.SetDoorHoldOpen || door[0].Cmds[0].Enabled || !door[0].On {
		t.Errorf("door hold = %+v", door)
	}
	tur := a.defenceCommands(game.BuildingView{Kind: "autocannon", X: 1, Y: 1})
	if len(tur) != 1 || tur[0].Cmds[0].Kind != command.SetTurretHold || !tur[0].Cmds[0].Enabled {
		t.Errorf("turret hold = %+v", tur)
	}
	mo := a.defenceCommands(game.BuildingView{Kind: "mortar"})
	if len(mo) != 2 || mo[0].ID != MortarAimID || !mo[1].Disabled || mo[1].Cmds[0].Kind != command.ClearMortarTarget {
		t.Errorf("mortar buttons = %+v", mo)
	}
	if got := a.defenceCommands(game.BuildingView{Kind: "wall"}); got != nil {
		t.Errorf("wall buttons = %+v", got)
	}
}

func TestDropWeaponCommand(t *testing.T) {
	t.Parallel()
	armed := dropWeaponCommand(game.PawnView{ID: 4, Status: "alive", Weapon: "rifle"})
	if armed.Disabled || len(armed.Cmds) != 1 || armed.Cmds[0].Kind != command.DropWeapon || armed.Cmds[0].Pawn != 4 {
		t.Errorf("armed = %+v", armed)
	}
	if c := dropWeaponCommand(game.PawnView{Status: "alive", Weapon: "fists"}); !c.Disabled {
		t.Errorf("unarmed should be disabled: %+v", c)
	}
}

func TestShelfUnlinkNeedsAGroup(t *testing.T) {
	t.Parallel()
	var a Building
	b := game.BuildingView{ID: 9, Kind: "shelf"}
	if c := unlinkCommand(a, b, game.ZoneView{}); !c.Disabled {
		t.Errorf("alone: %+v", c)
	}
	c := unlinkCommand(a, b, game.ZoneView{Group: 2})
	if c.Disabled || c.Cmds[0].Kind != command.UnlinkStorage || c.Cmds[0].Target != 9 {
		t.Errorf("grouped: %+v", c)
	}
}

func TestBillSourceIsZeroWithoutTheField(t *testing.T) {
	t.Parallel()
	if got := billSource(game.BillView{}); got != 0 {
		t.Errorf("source = %d", got)
	}
}
