package armor

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Region is a set of the body-part groups a garment covers and a hit
// can strike (a bit set). wiki: Armor (coverage groups)
type Region uint16

const (
	CoverFullHead  Region = 1 << iota // the whole head: eyes, nose, jaw
	CoverUpperHead                    // skull, brain and ears
	CoverEyes
	CoverShoulders
	CoverArms
	CoverTorso // the torso and the organs in it
	CoverWaist
	CoverNeck
	CoverLegs

	// Head, Torso and Legs are the groups of the three body regions.
	Head  = CoverFullHead | CoverUpperHead | CoverEyes
	Torso = CoverShoulders | CoverArms | CoverTorso | CoverWaist | CoverNeck
	Legs  = CoverLegs
)

// Ply is a layer apparel is worn on, innermost first (a bit each in a
// garment's Plies). wiki: Apparel (layers Skin, Middle, Outer, Belt,
// Headgear, Eyes)
type Ply uint8

const (
	PlySkin Ply = iota
	PlyMiddle
	PlyOuter
	PlyBelt
	PlyHeadgear
	PlyEyes
)

// Layer is one worn garment: its ratings, the groups it covers and the
// plies it takes (a bit per Ply; none for a creature's own plating).
type Layer struct {
	Set
	Covers Region
	Plies  uint8
}

// outer is l's outermost ply (0 for none).
func (l Layer) outer() int {
	o := 0
	for p := l.Plies; p > 1; p >>= 1 {
		o++
	}
	return o
}

// Conflicts reports two garments that cannot be worn together: they
// share a ply and cover a group in common. wiki: Apparel
func (l Layer) Conflicts(o Layer) bool { return l.Plies&o.Plies != 0 && l.Covers&o.Covers != 0 }

// garment is a piece of apparel's armor: a stuffed garment (factor > 0)
// rates factor permille of its material's armor (stuff, the default when
// the stack names none); any other carries its fixed ratings.
type garment struct {
	fixed  Set
	covers Region
	plies  uint8
	factor int
	stuff  item.Kind
}

// The coverage of the garments. wiki: Apparel (parka, jacket, flak
// jacket: torso, neck, shoulders, arms; duster, plate and marine armor
// add the legs; flak vest: torso, neck; tuque and simple and flak
// helmets: head and ears; marine helmet: the whole head).
const (
	coatCover   = CoverTorso | CoverNeck | CoverShoulders | CoverArms
	longCover   = coatCover | CoverLegs
	vestCover   = CoverTorso | CoverNeck
	capCover    = CoverUpperHead
	helmetCover = CoverFullHead | CoverUpperHead | CoverEyes
)

// The plies the garments take. wiki: Apparel (parka, duster, flak jacket
// outer; flak vest and flak pants middle; plate and marine armor middle
// and outer; hats and helmets headgear).
const (
	outer    = 1 << PlyOuter
	middle   = 1 << PlyMiddle
	shell    = middle | outer
	headgear = 1 << PlyHeadgear
)

// apparel is each garment's armor at normal quality. wiki: Parka, Duster,
// Tuque (armor factor 0.2, 0.3, 0.2 of the material: plainleather by
// default; the tuque is sewn from leather here), Wool parka (sheep wool,
// fixed); Armor (DEF-2): flak vest, pants and jacket and marine armor and
// helmet are fixed; plate armor is 0.9, the simple helmet 0.5 and the
// advanced helmet (the flak helmet's older name) 0.7 of their metal,
// steel by default (steel plate armor: 81/40.5/54).
var apparel = map[item.Kind]garment{
	item.Parka:     {covers: coatCover, plies: outer, factor: 200, stuff: item.Leather},
	item.WoolParka: {fixed: Set{Sharp: 72, Blunt: 0, Heat: 220}, covers: coatCover, plies: outer},
	item.Duster:    {covers: longCover, plies: outer, factor: 300, stuff: item.Leather},
	item.Tuque:     {covers: capCover, plies: headgear, factor: 200, stuff: item.Leather},

	item.FlakVest:       {fixed: Set{Sharp: 1000, Blunt: 360, Heat: 270}, covers: vestCover, plies: middle},
	item.FlakPants:      {fixed: Set{Sharp: 550, Blunt: 80, Heat: 100}, covers: CoverLegs, plies: middle},
	item.FlakJacket:     {fixed: Set{Sharp: 550, Blunt: 80, Heat: 100}, covers: coatCover, plies: outer},
	item.PlateArmor:     {covers: longCover, plies: shell, factor: 900, stuff: item.Steel},
	item.MarineArmor:    {fixed: Set{Sharp: 1060, Blunt: 450, Heat: 540}, covers: longCover, plies: shell},
	item.MarineHelmet:   {fixed: Set{Sharp: 1060, Blunt: 450, Heat: 540}, covers: helmetCover, plies: headgear},
	item.SimpleHelmet:   {covers: capCover, plies: headgear, factor: 500, stuff: item.Steel},
	item.AdvancedHelmet: {covers: capCover, plies: headgear, factor: 700, stuff: item.Steel},
}

// layer is g made of stuff m (0: its default).
func (g garment) layer(m item.Kind) Layer {
	if g.factor == 0 {
		return Layer{g.fixed, g.covers, g.plies}
	}
	p, ok := item.StuffOf(m)
	if !ok || p.ArmorSharp+p.ArmorBlunt+p.ArmorHeat == 0 && m != g.stuff {
		p, _ = item.StuffOf(g.stuff) // no material, or one that gives no armor row: the default
	}
	f := Permille(g.factor)
	return Layer{Set{f * Permille(p.ArmorSharp) / 1000, f * Permille(p.ArmorBlunt) / 1000, f * Permille(p.ArmorHeat) / 1000}, g.covers, g.plies}
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

// MaxLayers is the most garments Struck resolves on one part.
const MaxLayers = 8

// Struck resolves hit on a part in groups on through worn (any order):
// the garments covering on take it outermost ply first (worn order
// breaks ties), one roll each from roll; nothing covering draws no roll.
// took, when as long as worn, gets each garment's share: the damage that
// reached it before it acted (its wear, wiki: Apparel). The result is
// not Placed: the caller names the part.
func Struck(worn []Layer, on Region, hit damage.Hit, roll func(int) int, took []int) Landed {
	var idx [MaxLayers]int
	n := 0
	for i, l := range worn {
		if l.Covers&on == 0 || n == MaxLayers {
			continue
		}
		j := n // insertion: outermost first, stable
		for j > 0 && worn[idx[j-1]].outer() < l.outer() {
			idx[j] = idx[j-1]
			j--
		}
		idx[j] = i
		n++
	}
	var sets [MaxLayers]Set
	for k := range n {
		sets[k] = worn[idx[k]].Set
	}
	var reached [MaxLayers]int
	l := absorb(sets[:n], hit, roll, reached[:n])
	if len(took) == len(worn) {
		for k := range n {
			took[idx[k]] = reached[k]
		}
	}
	return l
}
