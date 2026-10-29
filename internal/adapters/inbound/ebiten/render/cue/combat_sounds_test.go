package cue

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/projectile"
)

func combatIDs(t game.TraceView) []string {
	r := newBus(1)
	r.b.TraceSound(t)
	var ids []string
	for _, c := range r.b.Take() {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestCombatTraceCues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		tr   game.TraceView
		want []string
	}{
		{"rifle hit flesh", game.TraceView{Weapon: "rifle", Hit: true, Damage: 9}, []string{"sfx.rifle_shot", "sfx.bullet_flesh"}},
		{"rifle miss", game.TraceView{Weapon: "rifle"}, []string{"sfx.rifle_shot", "sfx.bullet_miss"}},
		{"rifle hit wall", game.TraceView{Weapon: "rifle", Hit: true, Building: true, Damage: 5}, []string{"sfx.rifle_shot", "sfx.bullet_wall"}},
		{"rifle armor", game.TraceView{Weapon: "rifle", Hit: true}, []string{"sfx.rifle_shot", "sfx.armor_deflect"}},
		{"sentry", game.TraceView{Weapon: "sentry gun", FromBuilding: true, Hit: true, Damage: 4}, []string{"sfx.sentry_shot", "sfx.bullet_flesh"}},
		{"knife", game.TraceView{Weapon: "knife", Melee: true, Hit: true, Damage: 6}, []string{"sfx.melee_swing", "sfx.blade_hit"}},
		{"club", game.TraceView{Weapon: "club", Melee: true, Hit: true, Damage: 6}, []string{"sfx.melee_swing", "sfx.blunt_hit"}},
		{"club armor", game.TraceView{Weapon: "club", Melee: true, Hit: true}, []string{"sfx.melee_swing", "sfx.armor_deflect"}},
		{"spear whiff", game.TraceView{Weapon: "spear", Melee: true}, []string{"sfx.melee_swing"}},
		{"fired", game.TraceView{Weapon: "rifle", Stage: projectile.Fired}, []string{"sfx.rifle_shot"}},
		{"fired other", game.TraceView{Stage: projectile.Fired}, []string{"sfx.gunshot"}},
		{"landed miss", game.TraceView{Weapon: "rifle", Stage: projectile.Landed}, []string{"sfx.bullet_miss"}},
		{"landed wall", game.TraceView{Stage: projectile.Landed, Hit: true, Building: true, Damage: 3}, []string{"sfx.bullet_wall"}},
		{"landed flesh", game.TraceView{Stage: projectile.Landed, Hit: true, Damage: 3}, []string{"sfx.bullet_flesh"}},
		{"landed armor", game.TraceView{Stage: projectile.Landed, Hit: true}, []string{"sfx.armor_deflect"}},
		{"weaponless shot", game.TraceView{}, []string{"sfx.gunshot"}},
		{"weaponless melee", game.TraceView{Melee: true, Hit: true}, []string{"sfx.melee_hit"}},
	}
	for _, c := range cases {
		if got := combatIDs(c.tr); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: cues %v, want %v", c.name, got, c.want)
		}
	}
}
