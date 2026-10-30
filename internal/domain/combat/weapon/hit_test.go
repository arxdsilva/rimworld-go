package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

// TestHitOf (CBT-1): a weapon's hit carries its own name, form and
// stopping power. wiki: Weapons
func TestHitOf(t *testing.T) {
	t.Parallel()
	for k, want := range map[Kind]struct {
		form damage.Form
		stop int
	}{
		Rifle: {damage.FormBullet, 1500}, AssaultRifle: {damage.FormBullet, 500}, PumpShotgun: {damage.FormBullet, 3000},
		ShortBow: {damage.FormArrow, 500}, Pila: {damage.FormArrow, 2500}, Knife: {damage.FormCut, 0},
		Spear: {damage.FormStab, 0}, Club: {damage.FormBlunt, 0}, Fists: {damage.FormBlunt, 0}, Bite: {damage.FormBite, 0},
		Claw: {damage.FormScratch, 0}, Gore: {damage.FormStab, 0}, FragGrenade: {damage.FormBomb, 500},
		Molotov: {damage.FormFlame, 0}, EMPGrenade: {damage.FormEMP, 0}, SentryGun: {damage.FormBullet, 500},
	} {
		h := HitOf(k, DefOf(k))
		if h.Form != want.form || h.Stopping != want.stop || h.Weapon != DefOf(k).Name || h.Amount != DefOf(k).Damage ||
			h.Pen != Penetration(DefOf(k), DefOf(k).Damage) {
			t.Errorf("%v: %+v", DefOf(k).Name, h)
		}
	}
	if h := HitOf(numKinds, Def{Name: "odd", Damage: 3, DamageType: damage.Sharp}); h.Form != damage.FormCut || h.Stopping != 0 {
		t.Fatalf("an unknown melee kind cuts: %+v", h)
	}
}
