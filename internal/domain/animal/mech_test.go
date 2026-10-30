package animal

import (
	"testing"

	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/world/biome"
)

// TestMechRows (wave E contract): every mechanoid has its wiki row and
// fights with its own weapon, is EMP-vulnerable, has no needs, never
// lives wild, breeds or goes in a pen; animals have no row.
func TestMechRows(t *testing.T) {
	t.Parallel()
	n := 0
	for _, s := range All() {
		m, ok := MechOf(s)
		if !ok {
			continue
		}
		n++
		r := RosterOf(s)
		if r.Archetype != ArchMech || m.Role == "" || m.HealthPct <= 0 || m.BodySize <= 0 || m.Speed <= 0 ||
			m.CombatPower <= 0 || m.Armor.Sharp <= 0 || !m.EMPVulnerable {
			t.Errorf("%s: incomplete row %+v", s, m)
		}
		if DefOf(s).Attack != m.Weapon {
			t.Errorf("%s attacks with %v, want its %v", s, DefOf(s).Attack, m.Weapon)
		}
		if !NeedsFree(s) || Breeds(s) || r.Penned || r.HerdMin != 0 {
			t.Errorf("%s: a machine has no needs, never breeds, herds or is penned", s)
		}
		for _, b := range biome.All() {
			if Weight(s, b.ID) != 0 {
				t.Errorf("%s lives wild in %s", s, b.Key)
			}
		}
	}
	if n != 4 {
		t.Fatalf("%d mechanoids, want 4", n)
	}
	if _, ok := MechOf(Deer); ok {
		t.Error("a deer is no machine")
	}
	if b, _ := MechOf(Bulwark); b.BodySize != 30 || RosterOf(Bulwark).Size != SizeXL {
		t.Error("the heavy gunner is the big one (body size 3)")
	}
}

// TestMechDefs (DEF-4): a mechanoid's def is its wiki row: health from a
// human's, pace from its move speed, its weapon's numbers, no meat or
// leather, never downed, a wreck that shreds into salvage.
func TestMechDefs(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		m, ok := MechOf(s)
		d := DefOf(s)
		if !ok {
			if d.Mech || IsMech(s) || WreckYield(s) != (Salvage{}) {
				t.Errorf("%s is no machine", s)
			}
			continue
		}
		if !d.Mech || d.MaxHP != humanHP*m.HealthPct/100 || d.Meat != 0 || d.Leather != 0 || d.TameChance != 0 || d.Revenge != 0 {
			t.Errorf("%s: def %+v", s, d)
		}
		if wd := weapon.DefOf(m.Weapon); d.Damage != wd.Damage || d.Cooldown != wd.Cooldown {
			t.Errorf("%s: %d dmg / %d cd, want its weapon's %d / %d", s, d.Damage, d.Cooldown, wd.Damage, wd.Cooldown)
		}
		if y := WreckYield(s); y.Steel <= 0 {
			t.Errorf("%s: wreck yields %+v", s, y)
		}
		if l := m.Layer(); l.Set != m.Armor || l.Covers != armor.Head|armor.Torso|armor.Legs {
			t.Errorf("%s: plating %+v", s, l)
		}
		st := State{Species: s, Injury: d.MaxHP - 1}
		if st.DownsNow() {
			t.Errorf("%s went down: a machine fights on", s)
		}
	}
	if DefOf(Harrow).StepTicks >= DefOf(Bulwark).StepTicks || DefOf(Quill).StepTicks <= DefOf(Spindle).StepTicks {
		t.Error("the blade and the sniper are fast, the gunner and the skirmisher slow")
	}
	if DefOf(Harrow).Accuracy != MechMeleeHit || WreckYield(Bulwark) != (Salvage{Steel: 30, Plasteel: 10}) {
		t.Error("wiki: Scyther melee 62 %, Centipede shreds to 30 steel and 10 plasteel")
	}
}
