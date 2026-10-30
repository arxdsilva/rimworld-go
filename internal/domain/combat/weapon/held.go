package weapon

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Held is how a wielded weapon was made (CBT-3): its Quality (stored,
// quality.Level+1; 0 reads as Normal) and its Stuff (0: the material the
// def's numbers are for, steel). The zero value changes nothing.
type Held struct {
	Quality uint8
	Stuff   item.Kind
}

// level is the quality the weapon fights at (Normal when none is stored).
func (m Held) level() quality.Level {
	if l, ok := quality.FromStored(m.Quality); ok {
		return l
	}
	return quality.Normal
}

// stuffFactor is the weapon's material's melee damage factor (permille)
// on a blow of type t, against the steel the defs are for: its sharp or
// blunt multiplier (1000 with no material or no table row). wiki: Wood,
// Steel, Silver, Gold, Granite blocks (melee stat modifiers)
func (m Held) stuffFactor(t damage.Type) int {
	p, ok := item.StuffOf(m.Stuff)
	if m.Stuff == 0 || !ok {
		return 1000
	}
	f := p.BluntDamage
	if t == damage.Sharp {
		f = p.SharpDamage
	}
	if f <= 0 {
		return 1000
	}
	return f
}

// Tuned is profile d as made. A gun's accuracy (every band, never past
// certain), damage and armor penetration scale by its quality; a melee
// weapon's damage by its material and its quality, its penetration by its
// quality. wiki: Quality (weapons), Mace (per-material damage)
func (m Held) Tuned(d Def) Def {
	if m == (Held{}) {
		return d
	}
	l := m.level()
	if d.Ranged {
		acc := quality.Factor(l, quality.RangedAccuracy)
		for _, b := range [...]*int{&d.Bands.Touch.Acc, &d.Bands.Short.Acc, &d.Bands.Medium.Acc, &d.Bands.Long.Acc} {
			*b = min(*b*acc/1000, 1000)
		}
		if d.Legacy() {
			d.Accuracy = min(d.Accuracy*acc/1000, 100)
		}
		dmg := quality.Factor(l, quality.RangedDamage)
		d.ArmorPen = Penetration(d, d.Damage) * dmg / 1000
		d.Damage = d.Damage * dmg / 1000
		return d
	}
	q := quality.Factor(l, quality.MeleeDamage)
	d.ArmorPen = Penetration(d, d.Damage) * q / 1000
	d.Damage = d.Damage * m.stuffFactor(d.DamageType) / 1000 * q / 1000
	return d
}

// Tool is melee tool t of a weapon made as m: its damage by the
// material's factor for the tool's form and by quality, its penetration
// by quality (a gun's stock has no material: quality alone).
func (m Held) Tool(t Tool) Tool {
	if m == (Held{}) {
		return t
	}
	q := quality.Factor(m.level(), quality.MeleeDamage)
	t.Damage = t.Damage * m.stuffFactor(t.Form.Type()) / 1000 * q / 1000
	t.Pen = t.Pen * q / 1000
	return t
}
