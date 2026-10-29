package armor

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Region is a part of the body a hit lands on and a garment covers (a
// bit set).
type Region uint8

const (
	Head  Region = 1 << iota // head and ears
	Torso                    // torso, neck, shoulders and arms
	Legs
)

// Layer is one worn garment: its ratings and what it covers.
type Layer struct {
	Set
	Covers Region
}

// RegionOf is where a hit lands for roll in [0, RollMax): the head one
// time in ten, the legs two in ten, otherwise the torso and arms.
// original tuning: an approximation of the body's coverage shares
func RegionOf(roll int) Region {
	switch {
	case roll < 100:
		return Head
	case roll < 300:
		return Legs
	}
	return Torso
}

// garment is a piece of apparel's armor: a stuffed garment (factor > 0)
// rates factor permille of its material's armor (stuff, the default when
// the stack names none); any other carries its fixed ratings.
type garment struct {
	fixed  Set
	covers Region
	factor int
	stuff  item.Kind
}

// apparel is each garment's armor at normal quality. wiki: Parka, Duster,
// Tuque (armor factor 0.2, 0.3, 0.2 of the material: plainleather by
// default; the tuque is sewn from leather here), Wool parka (sheep wool,
// fixed); Armor (DEF-2): flak vest, pants and jacket and marine armor and
// helmet are fixed; plate armor is 0.9, the simple helmet 0.5 and the
// advanced helmet (the flak helmet's older name) 0.7 of their metal,
// steel by default (steel plate armor: 81/40.5/54).
var apparel = map[item.Kind]garment{
	item.Parka:     {covers: Torso, factor: 200, stuff: item.Leather},
	item.WoolParka: {fixed: Set{Sharp: 72, Blunt: 0, Heat: 220}, covers: Torso},
	item.Duster:    {covers: Torso | Legs, factor: 300, stuff: item.Leather},
	item.Tuque:     {covers: Head, factor: 200, stuff: item.Leather},

	item.FlakVest:       {fixed: Set{Sharp: 1000, Blunt: 360, Heat: 270}, covers: Torso},
	item.FlakPants:      {fixed: Set{Sharp: 550, Blunt: 80, Heat: 100}, covers: Legs},
	item.FlakJacket:     {fixed: Set{Sharp: 550, Blunt: 80, Heat: 100}, covers: Torso},
	item.PlateArmor:     {covers: Torso | Legs, factor: 900, stuff: item.Steel},
	item.MarineArmor:    {fixed: Set{Sharp: 1060, Blunt: 450, Heat: 540}, covers: Torso | Legs},
	item.MarineHelmet:   {fixed: Set{Sharp: 1060, Blunt: 450, Heat: 540}, covers: Head},
	item.SimpleHelmet:   {covers: Head, factor: 500, stuff: item.Steel},
	item.AdvancedHelmet: {covers: Head, factor: 700, stuff: item.Steel},
}

// layer is g made of stuff m (0: its default).
func (g garment) layer(m item.Kind) Layer {
	if g.factor == 0 {
		return Layer{g.fixed, g.covers}
	}
	p, ok := item.StuffOf(m)
	if !ok || p.ArmorSharp+p.ArmorBlunt+p.ArmorHeat == 0 && m != g.stuff {
		p, _ = item.StuffOf(g.stuff) // no material, or one that gives no armor row: the default
	}
	f := Permille(g.factor)
	return Layer{Set{f * Permille(p.ArmorSharp) / 1000, f * Permille(p.ArmorBlunt) / 1000, f * Permille(p.ArmorHeat) / 1000}, g.covers}
}

// Apparel is garment k's layer at normal quality in its default material
// (ok false: k gives no armor).
func Apparel(k item.Kind) (Layer, bool) {
	g, ok := apparel[k]
	if !ok {
		return Layer{}, false
	}
	return g.layer(0), true
}

// Rating is garment k's armor at normal quality in its default material
// (the gear tab's per-garment line; ok false: no armor).
func Rating(k item.Kind) (Set, bool) {
	l, ok := Apparel(k)
	return l.Set, ok
}

// Piece is garment k made of stuff (0: its default) at stored quality q
// (quality.Stored; 0: normal): its material's armor times the quality's
// apparel-protection factor. wiki: Apparel (quality multipliers)
func Piece(k, stuff item.Kind, q uint8) (Layer, bool) {
	g, ok := apparel[k]
	if !ok {
		return Layer{}, false
	}
	l := g.layer(stuff)
	if lv, ok := quality.FromStored(q); ok {
		f := Permille(quality.Factor(lv, quality.ApparelProtection))
		l.Sharp, l.Blunt, l.Heat = l.Sharp*f/1000, l.Blunt*f/1000, l.Heat*f/1000
	}
	return l, true
}

// AppendWorn appends the layers of the garments worn, given outermost
// first, to buf (garments without armor add nothing).
func AppendWorn(buf []Layer, worn ...item.Kind) []Layer {
	for _, k := range worn {
		if l, ok := Apparel(k); ok {
			buf = append(buf, l)
		}
	}
	return buf
}

// AppendPiece appends Piece(k, stuff, q) to buf when k gives armor.
func AppendPiece(buf []Layer, k, stuff item.Kind, q uint8) []Layer {
	if l, ok := Piece(k, stuff, q); ok {
		buf = append(buf, l)
	}
	return buf
}

// MaxLayers is the most layers Hit resolves.
const MaxLayers = 4

// Hit resolves a hit of amount (type t, penetration pen) on region r
// through the worn layers (outermost first): each layer covering r takes
// one roll from roll, in [0, RollMax). With nothing covering r no roll is
// drawn.
func Hit(layers []Layer, r Region, t damage.Type, amount int, pen Permille, roll func(int) int) Result {
	var sets [MaxLayers]Set
	var rolls [MaxLayers]int
	n := 0
	for _, l := range layers {
		if l.Covers&r != 0 && n < MaxLayers {
			sets[n], rolls[n] = l.Set, roll(RollMax)
			n++
		}
	}
	return ResolveLayers(sets[:n], t, amount, pen, rolls[:n])
}
