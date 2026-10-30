package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/weapon"
)

func TestWeaponTitleAndTip(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Weapon: "longsword", WeaponStuff: "steel", WeaponQuality: 4, WeaponStats: weapon.Stats(weapon.Longsword)}
	if got := WeaponTitle(p); got != "Steel longsword (good)" {
		t.Fatalf("title %q", got)
	}
	tip := WeaponTip(p)
	if !strings.Contains(tip, "Damage 23") || !strings.Contains(tip, "cooldown 2.6s") || !strings.Contains(tip, "per second") {
		t.Fatalf("tip %q", tip)
	}
	if WeaponTip(game.PawnView{}) != "" || WeaponTitle(game.PawnView{Weapon: "fists"}) != "fists" {
		t.Fatal("unarmed")
	}
}

func TestCombatExtraLines(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Kills: 2, DamageDealt: 55}
	if got := strings.Join(CombatExtraLines(p), "|"); got != "Kills 2   Damage dealt 55" {
		t.Fatalf("lines %q", got)
	}
	if len(CombatExtraLines(game.PawnView{})) != 0 {
		t.Fatal("no record, no lines")
	}
}

func TestDoorForbidToggle(t *testing.T) {
	t.Parallel()
	d := game.BuildingView{ID: 5, Kind: "door"}
	if !forbiddable(d) {
		t.Fatal("a door is forbiddable")
	}
	if c := forbidCommand(d); c.Label != "Forbid" || c.On || !c.Cmds[0].Enabled {
		t.Fatalf("open door %+v", c)
	}
	d.Forbidden = true
	if c := forbidCommand(d); c.Label != "Allow" || !c.On || c.Cmds[0].Enabled {
		t.Fatalf("forbidden door %+v", c)
	}
}
