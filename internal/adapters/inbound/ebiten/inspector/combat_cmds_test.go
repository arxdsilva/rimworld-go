package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestCombatCommandsOnlyForADraftedColonist(t *testing.T) {
	t.Parallel()
	var a Pawn
	if got := a.combatCommands(game.PawnView{Status: "alive"}); len(got) != 0 {
		t.Fatalf("an undrafted colonist has %d combat buttons", len(got))
	}
	p := game.PawnView{ID: 4, Status: "alive", Drafted: true, FireAtWill: true}
	got := a.combatCommands(p)
	if len(got) != 2 {
		t.Fatalf("no grenades: %d buttons, want fire + hold", len(got))
	}
	fire, hold := got[0], got[1]
	if !strings.Contains(fire.ID, "fire") || !fire.On || fire.Cmds[0].Kind != command.SetFireAtWill || fire.Cmds[0].Enabled || fire.Cmds[0].Pawn != 4 {
		t.Errorf("fire at will = %+v", fire)
	}
	if !strings.Contains(hold.ID, "hold.position") || hold.On || hold.Cmds[0].Kind != command.SetHoldPosition || !hold.Cmds[0].Enabled {
		t.Errorf("hold position = %+v", hold)
	}
	p.Grenades = 2
	got = a.combatCommands(p)
	if len(got) != 3 || !strings.Contains(got[2].ID, "grenade") || got[2].Run == nil || got[2].Disabled {
		t.Errorf("with grenades = %+v", got)
	}
}
