package armor

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
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

// apparel is each garment's ratings at normal quality in the stuff the
// colony makes it from. wiki: Parka, Duster, Tuque (plainleather and
// sheep wool rows); the tuque is sewn from leather here and its armor
// factor equals the parka's (0.2), so it rates as a leather parka.
var apparel = map[item.Kind]Layer{
	item.Parka:     {Set{Sharp: 162, Blunt: 48, Heat: 300}, Torso},
	item.WoolParka: {Set{Sharp: 72, Blunt: 0, Heat: 220}, Torso},
	item.Duster:    {Set{Sharp: 243, Blunt: 72, Heat: 450}, Torso | Legs},
	item.Tuque:     {Set{Sharp: 162, Blunt: 48, Heat: 300}, Head},
}

// Apparel is garment k's layer (ok false: k gives no armor).
func Apparel(k item.Kind) (Layer, bool) {
	l, ok := apparel[k]
	return l, ok
}

// AppendWorn appends the layers of the garments worn, given outermost
// first, to buf (garments without armor add nothing).
func AppendWorn(buf []Layer, worn ...item.Kind) []Layer {
	for _, k := range worn {
		if l, ok := apparel[k]; ok {
			buf = append(buf, l)
		}
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
