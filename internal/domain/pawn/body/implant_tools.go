// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
)

// NaturalFists counts fully organic hands available for barehanded blows.
// wiki: Wooden hand, Power claw, Bionic arm. A pawn without body state
// has the normal two fists.
func NaturalFists(b *Body) int {
	if b == nil {
		return 2
	}
	n := 0
	if !b.isMissing(ArmL) && !b.isMissing(HandL) && !b.hasReplacement(ArmL) && !b.hasReplacement(HandL) {
		n++
	}
	if !b.isMissing(ArmR) && !b.isMissing(HandR) && !b.hasReplacement(ArmR) && !b.hasReplacement(HandR) {
		n++
	}
	return n
}

// MeleeTools supplies the installed arms' or claws' natural attacks.
// Each arm contributes at most one tool, on the combat tick with no heap use.
func MeleeTools(b *Body) (tools [2]weapon.Tool, n int) {
	if b == nil {
		return
	}
	for _, p := range [...]Part{ArmL, ArmR} {
		k := b.ReplacementAt(p)
		var attack weapon.Tool
		switch k {
		case item.ProstheticArm: // wiki: Prosthetic arm; 8.2 damage rounded to this game's integer scale.
			attack = weapon.Tool{Name: "prosthetic fist", Damage: 8, Pen: 120, Cooldown: 120, Form: damage.FormBlunt, Chance: 1000}
		case item.ServoArm: // wiki: Bionic arm; original tuning: weighted melee chance.
			attack = weapon.Tool{Name: "bionic fist", Damage: 12, Pen: 180, Cooldown: 120, Form: damage.FormBlunt, Chance: 1000}
		case item.ApexArm: // wiki: Archotech arm; original tuning: weighted melee chance.
			attack = weapon.Tool{Name: "apex strike", Damage: 14, Pen: 210, Cooldown: 120, Form: damage.FormBlunt, Chance: 1000}
		default:
			hand := HandL
			if p == ArmR {
				hand = HandR
			}
			if b.ReplacementAt(hand) == item.ServoClaw { // wiki: Power claw; original tuning: weighted melee chance.
				attack = weapon.Tool{Name: "servo claw", Damage: 22, Pen: 330, Cooldown: 120, Form: damage.FormScratch, Chance: 1000, NoSocialFight: true}
			}
		}
		if attack.Damage > 0 {
			tools[n] = attack
			n++
		}
	}
	return
}
