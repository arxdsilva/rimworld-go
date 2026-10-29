package cue

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/projectile"
)

// Weapon-specific combat sounds (wave B). A trace that names a weapon the
// colony fights with is voiced here; any other (weapon-less, creature
// attacks) falls through to TraceSound's generic gunshot and melee_hit.

// bladeWeapons and bluntWeapons are the melee weapons by the sound of
// their hit (weapon names, as game.TraceView.Weapon carries them).
var (
	bladeWeapons = map[string]bool{"knife": true, "spear": true}
	bluntWeapons = map[string]bool{"fists": true, "club": true}
)

// Weapon names with their own shot sound.
const (
	weaponRifle  = "rifle"
	weaponSentry = "sentry gun"
)

// combatTrace cues the shot or swing and its impact, a bullet that
// missed, or armor turning the blow, for a trace of a named weapon. It
// reports whether it voiced the trace. A hit that dealt no damage was
// stopped by armor.
func (b *Bus) combatTrace(t game.TraceView, voiced bool) bool {
	fx, fy := float64(t.FromX)+0.5, float64(t.FromY)+0.5
	tx, ty := float64(t.ToX)+0.5, float64(t.ToY)+0.5
	impact := func(hitID string, vol float64) {
		switch {
		case !t.Hit:
		case t.Damage <= 0:
			b.Cue("sfx.armor_deflect", tx, ty, 1)
		default:
			b.Cue(hitID, tx, ty, vol)
		}
	}
	switch t.Stage {
	case projectile.Fired: // the muzzle report, heard where it was fired
		switch t.Weapon {
		case weaponSentry:
			b.Cue("sfx.sentry_shot", fx, fy, 1)
		case weaponRifle:
			b.Cue("sfx.rifle_shot", fx, fy, 1)
		default:
			b.Cue("sfx.gunshot", fx, fy, 1)
		}
		return true
	case projectile.Landed: // the round arrived: miss, wall, flesh or armor
		switch {
		case !t.Hit:
			b.Cue("sfx.bullet_miss", tx, ty, 1)
		case t.Building:
			b.Cue("sfx.bullet_wall", tx, ty, 1)
		default:
			impact("sfx.bullet_flesh", 1)
		}
		return true
	}
	switch {
	case t.Weapon == weaponSentry || t.FromBuilding && !t.Melee:
		b.Cue("sfx.sentry_shot", fx, fy, 1)
	case t.Weapon == weaponRifle:
		b.Cue("sfx.rifle_shot", fx, fy, 1)
	case t.Melee && (bladeWeapons[t.Weapon] || bluntWeapons[t.Weapon]):
		b.Cue("sfx.melee_swing", fx, fy, 1)
		vol := 1.0
		if voiced {
			vol = VolMeleeUnder
		}
		if t.Building {
			b.Cue("sfx.wood_hit", tx, ty, 1)
		} else if bladeWeapons[t.Weapon] {
			impact("sfx.blade_hit", vol)
		} else {
			impact("sfx.blunt_hit", vol)
		}
		return true
	default:
		return false
	}
	switch {
	case !t.Hit:
		b.Cue("sfx.bullet_miss", tx, ty, 1)
	case t.Building:
		b.Cue("sfx.bullet_wall", tx, ty, 1)
	default:
		impact("sfx.bullet_flesh", 1)
	}
	return true
}
