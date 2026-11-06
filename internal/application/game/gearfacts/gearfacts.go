// Package gearfacts is the Gear tab's summary numbers and what worn armour
// does to move speed (GEAR-1: GEAR-001, GEAR-002, GEAR-009 in
// .cursor/GEAR_GAP_ANALYSIS.md). It is pure: the game passes the outfit,
// what else is carried and the body's numbers, and gets values back.
package gearfacts

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/stat"
)

// CarryGrams is a healthy, human-sized carrier's capacity: 75 kg. wiki:
// Carrying capacity (75 × Manipulation × body size).
const CarryGrams = 75000

// Capacity is the carrying capacity, in grams, of a body with
// Manipulation manip and body size size (both permille).
func Capacity(manip, size int) int { return CarryGrams * max(manip, 0) / 1000 * max(size, 0) / 1000 }

// Grams is the mass of qty units of k (0 for nothing).
func Grams(k item.Kind, qty int) int {
	if k == 0 || qty <= 0 {
		return 0
	}
	return k.MassGrams() * qty
}

// Moves is what outfit o adds to move speed, in hundredths of a cell per
// second: every worn garment's armor.MoveOffset, added.
func Moves(o craft.Outfit) int {
	c := 0
	for _, p := range o.On {
		if p.Kind != 0 {
			c += armor.MoveOffset(p.Kind)
		}
	}
	return c
}

// MovePermille is Moves as permille of a human's base speed: the
// stat sheet's MoveSpeed offset.
func MovePermille(o craft.Outfit) int32 { return armor.SpeedPermille(Moves(o)) }

// Mods is the stat sheet's gear step for a MovePermille of move (nil for
// none). The game puts it before the body's capacities, so a lame wearer's
// Moving scales the armoured speed.
func Mods(move int32) []stat.Mod {
	if move == 0 {
		return nil
	}
	return []stat.Mod{{Stat: stat.MoveSpeed, Add: move}}
}

// Of is the Gear tab's summary of a pawn wearing o and carrying carried
// grams besides (weapons and pack), with Manipulation manip and body size
// size (permille): GearGrams, CapacityGrams, OverallArmor and MovePct.
// The apparel-policy fields stay for game/outfit.
func Of(o craft.Outfit, carried, manip, size int) gearview.Pawn {
	var worn [craft.Slots]armor.Layer
	n, grams := 0, carried
	for _, p := range o.On {
		if p.Kind == 0 {
			continue
		}
		grams += Grams(p.Kind, 1)
		if l, ok := armor.Piece(p.Kind, p.Stuff, p.Quality); ok {
			worn[n] = l
			n++
		}
	}
	return gearview.Pawn{GearGrams: grams, CapacityGrams: Capacity(manip, size), OverallArmor: armor.Overall(worn[:n]),
		MovePct: armor.SpeedPct(Moves(o))}
}
