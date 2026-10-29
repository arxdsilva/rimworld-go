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

// weaponVoice is a ranged weapon's sound at the muzzle and, for a
// launcher or grenade (its burst is cued by ListenBlasts, not here).
type weaponVoice struct{ fire, land string }

// weaponVoices maps a weapon name (as game.TraceView.Weapon carries it)
// to its voice, one line per weapon.
var weaponVoices = map[string]weaponVoice{
	"revolver":            {fire: "sfx.pistol_shot"},
	"autopistol":          {fire: "sfx.pistol_shot"},
	"machine pistol":      {fire: "sfx.smg_burst_shot"},
	"heavy smg":           {fire: "sfx.smg_burst_shot"},
	"assault rifle":       {fire: "sfx.assault_rifle_shot"},
	"bolt-action":         {fire: "sfx.rifle_shot"},
	"sniper rifle":        {fire: "sfx.sniper_shot"},
	"pump shotgun":        {fire: "sfx.shotgun_blast"},
	"chain shotgun":       {fire: "sfx.shotgun_blast"},
	"lmg":                 {fire: "sfx.lmg_shot"},
	"minigun":             {fire: "sfx.minigun_shot"},
	"short bow":           {fire: "sfx.bow_twang"},
	"recurve bow":         {fire: "sfx.bow_twang"},
	"greatbow":            {fire: "sfx.bow_twang"},
	"pila":                {fire: "sfx.pila_throw"},
	"frag grenade":        {fire: "sfx.grenade_throw"},
	"molotov cocktail":    {fire: "sfx.grenade_throw"},
	"EMP grenade":         {fire: "sfx.grenade_throw"},
	"incendiary launcher": {fire: "sfx.gunshot"},
	"emp launcher":        {fire: "sfx.gunshot"},
}

// heavyMelee are the melee weapons with the heavy hit sound.
var heavyMelee = map[string]bool{"mace": true, "gladius": true, "longsword": true}

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
			if v, ok := weaponVoices[t.Weapon]; ok {
				b.Cue(v.fire, fx, fy, 1)
			} else {
				b.Cue("sfx.gunshot", fx, fy, 1)
			}
		}
		return true
	case projectile.Landed: // the round arrived: miss, wall, flesh or armor
		switch {
		case weaponVoices[t.Weapon].fire == "sfx.grenade_throw" || t.Weapon == "incendiary launcher" || t.Weapon == "emp launcher":
			// the blast is voiced by ListenBlasts, from the world's bursts
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
	case !t.Melee && weaponVoices[t.Weapon].fire != "":
		b.Cue(weaponVoices[t.Weapon].fire, fx, fy, 1)
	case t.Melee && (bladeWeapons[t.Weapon] || bluntWeapons[t.Weapon] || heavyMelee[t.Weapon]):
		b.Cue("sfx.melee_swing", fx, fy, 1)
		vol := 1.0
		if voiced {
			vol = VolMeleeUnder
		}
		if t.Building {
			b.Cue("sfx.wood_hit", tx, ty, 1)
		} else if heavyMelee[t.Weapon] {
			impact("sfx.melee_heavy", vol)
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

// BlastEvent is one explosion in the world's bursts (BurstView.Blast:
// "bomb", "flame" or "EMP"; Tick stamps it, so it is heard once) and its
// radius in tenths of a tile (BurstView.Radius; 0 unknown).
type BlastEvent struct {
	X, Y   int
	Tick   uint64
	Blast  string
	Radius int
}

// blastSounds is the sound of each blast kind.
var blastSounds = map[string]string{"bomb": "sfx.explosion", "flame": "sfx.incendiary_burst", "EMP": "sfx.emp_burst"}

// ListenBlasts cues each new explosion once, at its tile.
func (b *Bus) ListenBlasts(evs []BlastEvent) {
	for _, e := range evs {
		b.hearBlast(e)
	}
}

// ListenBursts cues the world's new explosions (WorldView.Bursts with a
// Blast; an animal's or a turret's burst has its own bang), each once,
// mapping every burst to a BlastEvent in place: nothing is allocated per
// frame.
func (b *Bus) ListenBursts(bursts []game.BurstView) {
	for i := range bursts {
		if bu := &bursts[i]; bu.Blast != "" {
			b.hearBlast(BlastEvent{X: bu.X, Y: bu.Y, Tick: bu.Tick, Blast: bu.Blast, Radius: bu.Radius})
		}
	}
}

// Blast volume by radius: a small burst (a launcher's 1.1 tiles) a little
// quieter than a grenade's, which plays at full volume.
const (
	blastVolMin       = 0.6
	blastVolPerRadius = 0.02 // per tenth of a tile
)

// hearBlast cues explosion e unless it was heard already.
func (b *Bus) hearBlast(e BlastEvent) {
	id, ok := blastSounds[e.Blast]
	m := &b.Memo
	if !ok || m.blasts[e] {
		return
	}
	if m.blasts == nil || len(m.blasts) > 256 {
		m.blasts = map[BlastEvent]bool{}
	}
	m.blasts[e] = true
	vol := 1.0
	if e.Radius > 0 {
		vol = min(1, blastVolMin+blastVolPerRadius*float64(e.Radius))
	}
	b.Cue(id, float64(e.X)+0.5, float64(e.Y)+0.5, vol)
}
