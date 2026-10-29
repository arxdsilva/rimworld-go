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
		{"revolver", game.TraceView{Weapon: "revolver", Stage: projectile.Fired}, []string{"sfx.pistol_shot"}},
		{"machine pistol", game.TraceView{Weapon: "machine pistol", Stage: projectile.Fired}, []string{"sfx.smg_burst_shot"}},
		{"assault rifle", game.TraceView{Weapon: "assault rifle", Stage: projectile.Fired}, []string{"sfx.assault_rifle_shot"}},
		{"sniper", game.TraceView{Weapon: "sniper rifle", Stage: projectile.Fired}, []string{"sfx.sniper_shot"}},
		{"shotgun", game.TraceView{Weapon: "pump shotgun", Stage: projectile.Fired}, []string{"sfx.shotgun_blast"}},
		{"lmg", game.TraceView{Weapon: "lmg", Stage: projectile.Fired}, []string{"sfx.lmg_shot"}},
		{"minigun", game.TraceView{Weapon: "minigun", Stage: projectile.Fired}, []string{"sfx.minigun_shot"}},
		{"bow", game.TraceView{Weapon: "short bow", Stage: projectile.Fired}, []string{"sfx.bow_twang"}},
		{"pila", game.TraceView{Weapon: "pila", Stage: projectile.Fired}, []string{"sfx.pila_throw"}},
		{"grenade thrown", game.TraceView{Weapon: "frag grenade", Stage: projectile.Fired}, []string{"sfx.grenade_throw"}},
		{"molotov thrown", game.TraceView{Weapon: "molotov cocktail", Stage: projectile.Fired}, []string{"sfx.grenade_throw"}},
		{"emp grenade thrown", game.TraceView{Weapon: "EMP grenade", Stage: projectile.Fired}, []string{"sfx.grenade_throw"}},
		{"grenade landing is silent", game.TraceView{Weapon: "frag grenade", Stage: projectile.Landed}, nil},
		{"gladius", game.TraceView{Weapon: "gladius", Melee: true, Hit: true, Damage: 9}, []string{"sfx.melee_swing", "sfx.melee_heavy"}},
		{"mace", game.TraceView{Weapon: "mace", Melee: true, Hit: true, Damage: 9}, []string{"sfx.melee_swing", "sfx.melee_heavy"}},
		{"longsword", game.TraceView{Weapon: "longsword", Melee: true, Hit: true, Damage: 9}, []string{"sfx.melee_swing", "sfx.melee_heavy"}},
		{"weaponless shot", game.TraceView{}, []string{"sfx.gunshot"}},
		{"weaponless melee", game.TraceView{Melee: true, Hit: true}, []string{"sfx.melee_hit"}},
	}
	for _, c := range cases {
		if got := combatIDs(c.tr); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: cues %v, want %v", c.name, got, c.want)
		}
	}
}

func TestListenBlastsCuesEachBurstOnce(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	evs := []BlastEvent{{X: 3, Y: 4, Tick: 9, Blast: "bomb"}, {X: 5, Y: 5, Tick: 9, Blast: "flame"}, {X: 6, Y: 6, Tick: 9, Blast: "EMP"}, {Blast: "other"}}
	r.b.ListenBlasts(evs)
	r.b.ListenBlasts(evs)
	var ids []string
	for _, c := range r.b.Take() {
		ids = append(ids, c.ID)
	}
	want := []string{"sfx.explosion", "sfx.incendiary_burst", "sfx.emp_burst"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("cues %v, want %v", ids, want)
	}
}

// TestListenBurstsCuesEachNewBlastOnce (DEF-2): the world's bursts with
// a Blast are heard once each, louder the wider they are, across frames
// and ListenWorld's memo rebuild; animal and turret bursts are not.
func TestListenBurstsCuesEachNewBlastOnce(t *testing.T) { // not parallel: AllocsPerRun
	r := newBus(1)
	bursts := []game.BurstView{{X: 3, Y: 4, Tick: 9, Blast: "bomb", Radius: 19}, {X: 1, Y: 1, Tick: 9, Species: "emberbuck"},
		{X: 2, Y: 2, Tick: 9, Turret: true}, {X: 6, Y: 6, Tick: 9, Blast: "EMP", Radius: 11}, {X: 7, Y: 7, Tick: 9, Blast: "smoke", Radius: 30}}
	r.b.ListenBursts(bursts)
	got := r.b.Take()
	if len(got) != 2 || got[0].ID != "sfx.explosion" || got[1].ID != "sfx.emp_burst" {
		t.Fatalf("cues %+v, want the bomb and the EMP", got)
	}
	if got[0].X != 3.5 || got[0].Y != 4.5 || got[1].Vol >= got[0].Vol || got[0].Vol > 1 {
		t.Errorf("placement or volume: %+v", got)
	}
	r.b.ListenWorld(r.v, game.WorldView{}, nil) // rebuilds the memo
	r.b.ListenBursts(bursts)
	if again := r.b.Take(); len(again) != 0 {
		t.Fatalf("a burst still in the world was heard again: %+v", again)
	}
	r.b.ListenBursts(append(bursts, game.BurstView{X: 3, Y: 4, Tick: 30, Blast: "flame", Radius: 11}))
	if next := r.b.Take(); len(next) != 1 || next[0].ID != "sfx.incendiary_burst" {
		t.Fatalf("a new burst: %+v", next)
	}
	if n := testing.AllocsPerRun(50, func() { r.b.ListenBursts(bursts) }); n != 0 {
		t.Errorf("hearing old bursts allocates %.1f times a frame", n)
	}
}
