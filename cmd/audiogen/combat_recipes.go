package main

import s "rimworld-go/pkg/sfxsynth"

// combatSounds are the weapon and impact sounds of combat (wave B). Ids
// are sfx.<name>; the weapon-less fallbacks sfx.gunshot and
// sfx.melee_hit stay for traces that name no weapon. Seeds are 900+.
// There is no bow_twang: bows are not in the game yet (DEF-2), and an
// unplayed sound is an orphan.
func combatSounds() []sound {
	return []sound{
		// A crisp rifle report: sharp crack, a thump of powder and a tail
		// of echo.
		{s.Recipe{ID: "sfx.rifle_shot", Seed: 901, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.04, HighPass: 1800, Attack: 0.0005, Decay: 0.015, Gain: 1},
			{Wave: s.White, Dur: 0.18, BandPass: 1400, Q: 0.7, Attack: 0.001, Decay: 0.07, Gain: 0.6},
			{Wave: s.Sine, Freq: 150, FreqEnd: 55, Dur: 0.22, Attack: 0.001, Decay: 0.1, Gain: 0.9},
			{Wave: s.Brown, Dur: 0.25, LowPass: 500, Decay: 0.1, Gain: 0.6},
		}}, 0.85},
		// The sentry turret: a lighter, mechanical clack-and-pop.
		{s.Recipe{ID: "sfx.sentry_shot", Seed: 902, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Square, Freq: 900, FreqEnd: 400, Dur: 0.03, Attack: 0.0005, Decay: 0.012, LowPass: 3500, Gain: 0.5},
			{Wave: s.White, Dur: 0.05, BandPass: 2600, Q: 1.2, Attack: 0.0005, Decay: 0.02, Gain: 0.8},
			{Wave: s.Sine, Freq: 220, FreqEnd: 90, Dur: 0.1, Attack: 0.001, Decay: 0.04, Gain: 0.7},
			{Wave: s.White, Start: 0.05, Dur: 0.02, BandPass: 1500, Q: 2, Decay: 0.008, Gain: 0.3},
		}}, 0.7},
		// A short whoosh of air.
		{s.Recipe{ID: "sfx.melee_swing", Seed: 903, Variants: 2, Jitter: 0.1, Layers: []s.Layer{
			{Wave: s.Pink, Dur: 0.2, BandPass: 600, BandEnd: 2400, Q: 1.1, Attack: 0.06, Decay: 0.1, Release: 0.04, Gain: 0.9},
			{Wave: s.White, Dur: 0.16, HighPass: 3000, Attack: 0.06, Decay: 0.08, Gain: 0.15},
		}}, 0.5},
		// A knife or spear going into flesh: a wet slice and a short thud.
		{s.Recipe{ID: "sfx.blade_hit", Seed: 904, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.08, BandPass: 2800, BandEnd: 1200, Q: 0.9, Attack: 0.001, Decay: 0.035, Gain: 0.8},
			{Wave: s.Pink, Dur: 0.12, LowPass: 1100, Decay: 0.05, Gain: 0.7},
			{Wave: s.Sine, Freq: 170, FreqEnd: 90, Dur: 0.09, Decay: 0.04, Gain: 0.6},
		}}, 0.75},
		// A club or fist on flesh: a heavy low thud with little top end.
		{s.Recipe{ID: "sfx.blunt_hit", Seed: 905, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 120, FreqEnd: 55, Dur: 0.16, Attack: 0.001, Decay: 0.08, Gain: 1},
			{Wave: s.Brown, Dur: 0.14, LowPass: 700, Attack: 0.001, Decay: 0.06, Gain: 0.9},
			{Wave: s.White, Dur: 0.02, BandPass: 900, Q: 0.8, Decay: 0.01, Gain: 0.4},
		}}, 0.85},
		// A bullet into a creature: a dull, wet thump.
		{s.Recipe{ID: "sfx.bullet_flesh", Seed: 906, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 190, FreqEnd: 70, Dur: 0.12, Attack: 0.001, Decay: 0.05, Gain: 0.9},
			{Wave: s.Pink, Dur: 0.1, LowPass: 900, Decay: 0.045, Gain: 0.8},
			{Wave: s.White, Dur: 0.015, BandPass: 2200, Q: 1, Decay: 0.006, Gain: 0.3},
		}}, 0.7},
		// A bullet into a wall: a sharp crack and a spray of debris.
		{s.Recipe{ID: "sfx.bullet_wall", Seed: 907, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 2500, Attack: 0.0005, Decay: 0.012, Gain: 0.9},
			{Wave: s.Sine, Freq: 420, FreqEnd: 200, Dur: 0.07, Decay: 0.03, Gain: 0.6},
			{Wave: s.White, Start: 0.02, Dur: 0.15, HighPass: 3000, Grains: 40, GrainLen: 0.004, Decay: 0.08, Gain: 0.35},
			{Wave: s.Brown, Dur: 0.1, LowPass: 600, Decay: 0.045, Gain: 0.5},
		}}, 0.7},
		// A round passing close or glancing off: a whiz sweeping down.
		{s.Recipe{ID: "sfx.bullet_miss", Seed: 908, Variants: 2, Jitter: 0.1, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.22, BandPass: 4200, BandEnd: 1200, Q: 4, Attack: 0.01, Decay: 0.1, Release: 0.06, Gain: 0.8},
			{Wave: s.Sine, Freq: 2600, FreqEnd: 900, Dur: 0.2, Attack: 0.01, Decay: 0.1, Gain: 0.2},
		}}, 0.5},
		// Armor turning a blow: a metallic glance, ringing briefly.
		{s.Recipe{ID: "sfx.armor_deflect", Seed: 909, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.02, HighPass: 3500, Attack: 0.0005, Decay: 0.008, Gain: 0.6},
			{Wave: s.Sine, Freq: 2300, FreqEnd: 2150, Dur: 0.3, Attack: 0.0005, Decay: 0.14, Gain: 0.5},
			{Wave: s.Sine, Freq: 3450, Dur: 0.2, Attack: 0.0005, Decay: 0.08, Gain: 0.3},
			{Wave: s.Sine, Freq: 1430, Dur: 0.22, Attack: 0.0005, Decay: 0.1, Gain: 0.3},
		}}, 0.6},
	}
}
