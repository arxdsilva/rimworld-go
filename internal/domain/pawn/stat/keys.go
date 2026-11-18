// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package stat

// keys are the stats' stable names in plugin files (PLG-026).
var keys = [NumStats]string{
	WorkSpeed: "work_speed", MoveSpeed: "move_speed", HungerRate: "hunger_rate", RestRate: "rest_rate",
	MoodOffset: "mood_offset", BreakThreshold: "break_threshold", ColdComfort: "cold_comfort",
	HeatComfort: "heat_comfort", ShootAcc: "shoot_acc", MeleeAcc: "melee_acc", AimCooldown: "aim_cooldown",
	PainTolerance: "pain_tolerance", LearnRate: "learn_rate", RecreationRate: "recreation_rate",
	SocialImpact: "social_impact", NegotiationAbility: "negotiation_ability",
	TradePriceImprovement: "trade_price_improvement", ArrestSuccess: "arrest_success",
	Comfort: "comfort", RestEffectiveness: "rest_effectiveness", Beauty: "beauty",
}

// Key is the stable name ("work_speed"); "" for an unknown stat.
func (id ID) Key() string {
	if id >= NumStats {
		return ""
	}
	return keys[id]
}

// ParseKey finds a stat by its key.
func ParseKey(key string) (ID, bool) {
	for i, k := range keys {
		if k == key && k != "" {
			return ID(i), true
		}
	}
	return 0, false
}
