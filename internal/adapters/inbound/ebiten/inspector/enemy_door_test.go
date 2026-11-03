package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// TestEnemyDoorPane (MAP-17): a settlement's door shows why it is shut and
// only a Claim button, refused with a reason while a defender stands. Hold
// open, forbid, move and deconstruct don't act on an enemy's door.
func TestEnemyDoorPane(t *testing.T) {
	t.Parallel()
	f := newFake()
	door := game.BuildingView{ID: 7, Kind: doorKind, X: 3, Y: 4, W: 1, H: 1, Hostile: true}
	f.world.Buildings = []game.BuildingView{door}
	f.hostiles = []game.PawnView{{ID: 9, Name: "Guard", Status: "alive"}}
	a := New(f).Building()

	got := a.BuildingCommands(7)
	if len(got) != 1 || got[0].ID != DoorClaimID {
		t.Fatalf("buttons = %+v, want Claim alone", got)
	}
	if !got[0].Disabled || got[0].Reason == "" {
		t.Fatalf("Claim before the defeat = %+v", got[0])
	}
	if lines := strings.Join(a.buildingLines(door), "|"); !strings.Contains(lines, EnemyDoorLine) || strings.Contains(lines, "Held open") {
		t.Fatalf("lines = %q", lines)
	}

	// Downed, fleeing and captured are the defeat Claim means.
	f.hostiles = []game.PawnView{{ID: 9, Status: "downed"}, {ID: 10, Status: "alive", Fleeing: true}, {ID: 11, Status: "alive", Prisoner: true}}
	got = a.BuildingCommands(7)
	if len(got) != 1 || got[0].Disabled {
		t.Fatalf("Claim after the defeat = %+v", got)
	}
	if len(got[0].Cmds) != 1 {
		t.Fatalf("Claim commands = %+v", got[0].Cmds)
	}
	c := got[0].Cmds[0]
	if c.Kind != command.DesignateClaim || c.X != 3 || c.Y != 4 || c.X2 != 3 || c.Y2 != 4 || !c.Enabled {
		t.Fatalf("Claim command = %+v", c)
	}
}

// TestSpentSettlementTurretSaysSo (MAP-20): out of ammo in plain words.
func TestSpentSettlementTurretSaysSo(t *testing.T) {
	t.Parallel()
	spent := game.BuildingView{ID: 7, Kind: turretKind, Hostile: true, Barrel: 0}
	if got := strings.Join(defenceLines(spent), "|"); got != HostileTurretSpentLine {
		t.Fatalf("spent lines = %q", got)
	}
	live := spent
	live.Barrel = 10
	if got := strings.Join(defenceLines(live), "|"); got != HostileTurretLine {
		t.Fatalf("live lines = %q", got)
	}
}
