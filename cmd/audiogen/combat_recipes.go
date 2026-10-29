package main

import s "rimworld-go/pkg/sfxsynth"

// combatSounds are the weapon and impact sounds of combat (wave B). Ids
// are sfx.<name>; the weapon-less fallbacks sfx.gunshot and
// sfx.melee_hit stay for traces that name no weapon. Seeds are 900+.
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

		// Wave C: the firearm, bow, thrown and explosive voices. Seeds 910+.
		// A pistol: a sharp, small crack with a short snap of body.
		{s.Recipe{ID: "sfx.pistol_shot", Seed: 910, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 2200, Attack: 0.0005, Decay: 0.012, Gain: 1},
			{Wave: s.White, Dur: 0.1, BandPass: 1800, Q: 0.8, Attack: 0.001, Decay: 0.04, Gain: 0.5},
			{Wave: s.Sine, Freq: 260, FreqEnd: 110, Dur: 0.1, Attack: 0.001, Decay: 0.045, Gain: 0.7},
		}}, 0.75},
		// One round of a burst: lighter and drier than the pistol.
		{s.Recipe{ID: "sfx.smg_burst_shot", Seed: 911, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.025, HighPass: 2600, Attack: 0.0005, Decay: 0.009, Gain: 0.9},
			{Wave: s.Sine, Freq: 320, FreqEnd: 150, Dur: 0.06, Attack: 0.001, Decay: 0.025, Gain: 0.55},
			{Wave: s.White, Dur: 0.06, BandPass: 2000, Q: 1, Decay: 0.025, Gain: 0.3},
		}}, 0.6},
		// An assault rifle: a snappy crack over a mid thump, short tail.
		{s.Recipe{ID: "sfx.assault_rifle_shot", Seed: 912, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.035, HighPass: 2000, Attack: 0.0005, Decay: 0.013, Gain: 1},
			{Wave: s.White, Dur: 0.14, BandPass: 1500, Q: 0.7, Attack: 0.001, Decay: 0.055, Gain: 0.55},
			{Wave: s.Sine, Freq: 190, FreqEnd: 70, Dur: 0.14, Attack: 0.001, Decay: 0.06, Gain: 0.8},
			{Wave: s.Brown, Dur: 0.16, LowPass: 600, Decay: 0.07, Gain: 0.45},
		}}, 0.8},
		// A sniper rifle: a heavy boom, a crack and a long echo.
		{s.Recipe{ID: "sfx.sniper_shot", Seed: 913, Jitter: 0.04, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.05, HighPass: 1500, Attack: 0.0005, Decay: 0.02, Gain: 1},
			{Wave: s.Sine, Freq: 110, FreqEnd: 38, Dur: 0.5, Attack: 0.001, Decay: 0.22, Gain: 1},
			{Wave: s.Brown, Dur: 0.6, LowPass: 450, Decay: 0.25, Gain: 0.8},
			{Wave: s.White, Start: 0.12, Dur: 0.5, BandPass: 700, Q: 0.6, Decay: 0.25, Gain: 0.25},
		}}, 0.95},
		// A shotgun: a wide, noisy blast with a deep thump.
		{s.Recipe{ID: "sfx.shotgun_blast", Seed: 914, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.05, HighPass: 1000, Attack: 0.0005, Decay: 0.02, Gain: 1},
			{Wave: s.White, Dur: 0.25, BandPass: 900, Q: 0.5, Attack: 0.001, Decay: 0.1, Gain: 0.8},
			{Wave: s.Sine, Freq: 120, FreqEnd: 45, Dur: 0.3, Attack: 0.001, Decay: 0.13, Gain: 1},
			{Wave: s.Brown, Dur: 0.35, LowPass: 500, Decay: 0.14, Gain: 0.6},
		}}, 0.9},
		// A light machine gun: a thick, flat chug.
		{s.Recipe{ID: "sfx.lmg_shot", Seed: 915, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 1700, Attack: 0.0005, Decay: 0.012, Gain: 0.9},
			{Wave: s.Sine, Freq: 170, FreqEnd: 65, Dur: 0.12, Attack: 0.001, Decay: 0.05, Gain: 0.9},
			{Wave: s.Brown, Dur: 0.12, LowPass: 700, Decay: 0.05, Gain: 0.6},
		}}, 0.7},
		// A minigun round: a short, buzzy rattle, meant to be heard at 25 a burst.
		{s.Recipe{ID: "sfx.minigun_shot", Seed: 916, Jitter: 0.07, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.02, HighPass: 2400, Attack: 0.0003, Decay: 0.008, Gain: 0.8},
			{Wave: s.Saw, Freq: 240, FreqEnd: 120, Dur: 0.05, Attack: 0.001, Decay: 0.02, LowPass: 2000, Gain: 0.5},
			{Wave: s.Sine, Freq: 150, FreqEnd: 70, Dur: 0.05, Decay: 0.02, Gain: 0.5},
		}}, 0.5},
		// A bow: the string's twang and the arrow's brief hiss.
		{s.Recipe{ID: "sfx.bow_twang", Seed: 917, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Triangle, Freq: 420, FreqEnd: 300, Dur: 0.25, Attack: 0.001, Decay: 0.12, Tremolo: 38, TremoloDepth: 0.35, Gain: 0.7},
			{Wave: s.Sine, Freq: 840, FreqEnd: 600, Dur: 0.18, Attack: 0.001, Decay: 0.08, Gain: 0.25},
			{Wave: s.White, Dur: 0.02, BandPass: 1800, Q: 1, Decay: 0.008, Gain: 0.4},
			{Wave: s.White, Start: 0.03, Dur: 0.18, BandPass: 3500, BandEnd: 1800, Q: 2, Attack: 0.02, Decay: 0.08, Gain: 0.15},
		}}, 0.6},
		// A heavy javelin thrown: a long, rising whoosh.
		{s.Recipe{ID: "sfx.pila_throw", Seed: 918, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.Pink, Dur: 0.32, BandPass: 500, BandEnd: 2200, Q: 1, Attack: 0.1, Decay: 0.14, Release: 0.06, Gain: 0.9},
			{Wave: s.White, Dur: 0.25, HighPass: 3000, Attack: 0.1, Decay: 0.1, Gain: 0.12},
			{Wave: s.Sine, Freq: 90, FreqEnd: 60, Dur: 0.1, Decay: 0.04, Gain: 0.3},
		}}, 0.55},
		// A grenade thrown: a short whoosh and the clink of the spoon.
		{s.Recipe{ID: "sfx.grenade_throw", Seed: 919, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Pink, Dur: 0.22, BandPass: 600, BandEnd: 1800, Q: 1, Attack: 0.07, Decay: 0.09, Release: 0.04, Gain: 0.6},
			{Wave: s.Sine, Freq: 3100, FreqEnd: 2900, Dur: 0.15, Attack: 0.0005, Decay: 0.05, Gain: 0.4},
			{Wave: s.Sine, Freq: 4650, Dur: 0.1, Attack: 0.0005, Decay: 0.03, Gain: 0.2},
			{Wave: s.White, Dur: 0.01, HighPass: 4000, Decay: 0.004, Gain: 0.5},
		}}, 0.55},
		// A frag explosion: a flash of crack, a deep boom, a rumbling debris tail.
		{s.Recipe{ID: "sfx.explosion", Seed: 920, Variants: 2, Jitter: 0.07, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.06, HighPass: 800, Attack: 0.0005, Decay: 0.025, Gain: 1},
			{Wave: s.Sine, Freq: 95, FreqEnd: 28, Dur: 0.8, Attack: 0.002, Decay: 0.35, Gain: 1},
			{Wave: s.Brown, Dur: 0.9, LowPass: 400, Decay: 0.4, Gain: 0.9},
			{Wave: s.White, Dur: 0.5, BandPass: 1200, BandEnd: 400, Q: 0.5, Attack: 0.002, Decay: 0.22, Gain: 0.6},
			{Wave: s.White, Start: 0.1, Dur: 0.7, HighPass: 2500, Grains: 50, GrainLen: 0.004, Decay: 0.35, Gain: 0.3},
		}}, 0.95},
		// An incendiary burst: a soft whoomp of igniting fuel and a crackle.
		{s.Recipe{ID: "sfx.incendiary_burst", Seed: 921, Jitter: 0.07, Layers: []s.Layer{
			{Wave: s.Brown, Dur: 0.5, LowPass: 600, Attack: 0.06, Decay: 0.25, Gain: 1},
			{Wave: s.Sine, Freq: 70, FreqEnd: 40, Dur: 0.45, Attack: 0.04, Decay: 0.2, Gain: 0.8},
			{Wave: s.White, Dur: 0.4, BandPass: 1000, BandEnd: 500, Q: 0.6, Attack: 0.05, Decay: 0.2, Gain: 0.5},
			{Wave: s.White, Start: 0.1, Dur: 0.6, HighPass: 3000, Grains: 90, GrainLen: 0.003, Decay: 0.35, Gain: 0.5},
		}}, 0.8},
		// An EMP burst: a falling electric zap with a crackle.
		{s.Recipe{ID: "sfx.emp_burst", Seed: 922, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Saw, Freq: 2400, FreqEnd: 160, Dur: 0.3, Attack: 0.001, Decay: 0.14, LowPass: 5000, Gain: 0.7},
			{Wave: s.Square, Freq: 1200, FreqEnd: 90, Dur: 0.25, Attack: 0.001, Decay: 0.1, Gain: 0.3},
			{Wave: s.White, Dur: 0.3, HighPass: 3000, Grains: 100, GrainLen: 0.003, Decay: 0.15, Gain: 0.6},
			{Wave: s.Sine, Freq: 100, FreqEnd: 50, Dur: 0.2, Decay: 0.08, Gain: 0.5},
		}}, 0.7},
		// A mace or longsword landing: a heavy clang over a crunching thud.
		{s.Recipe{ID: "sfx.melee_heavy", Seed: 923, Variants: 1, Jitter: 0.07, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 2500, Attack: 0.0005, Decay: 0.012, Gain: 0.7},
			{Wave: s.Sine, Freq: 95, FreqEnd: 45, Dur: 0.25, Attack: 0.001, Decay: 0.11, Gain: 1},
			{Wave: s.Brown, Dur: 0.2, LowPass: 800, Decay: 0.09, Gain: 0.9},
			{Wave: s.Sine, Freq: 1700, FreqEnd: 1620, Dur: 0.22, Attack: 0.0005, Decay: 0.09, Gain: 0.25},
		}}, 0.9},
	}
}
