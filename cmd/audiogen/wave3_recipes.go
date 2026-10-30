package main

import s "rimworld-go/pkg/sfxsynth"

// wave3Sounds are the sounds of the storage and combat wave (CBT-3, CBT-4,
// STO-2): four new launchers and energy weapons, the shield belt, a
// smokepop, a stagger, a gun-stock blow, the paste dispenser and a
// tantrum. All procedural. Ids are sfx.<name>. Seeds are 960+.
func wave3Sounds() []sound {
	return []sound{
		// A rocket leaving the tube: a sharp ignition pop, then a roaring
		// whoosh that rises and fades as the rocket flies off.
		{s.Recipe{ID: "sfx.rocket_launch", Seed: 960, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 1500, Attack: 0.0005, Decay: 0.012, Gain: 0.8},
			{Wave: s.Sine, Freq: 140, FreqEnd: 50, Dur: 0.25, Attack: 0.001, Decay: 0.1, Gain: 0.9},
			{Wave: s.White, Start: 0.02, Dur: 0.9, BandPass: 700, BandEnd: 3200, Q: 0.8, Attack: 0.1, Decay: 0.6, Release: 0.15, Gain: 0.8},
			{Wave: s.Brown, Start: 0.02, Dur: 0.9, LowPass: 600, Attack: 0.05, Decay: 0.6, Release: 0.15, Gain: 1},
		}}, 0.9},
		// The smoke launcher: a hollow thump of a canister leaving the
		// barrel and a short puff of gas.
		{s.Recipe{ID: "sfx.smoke_launcher", Seed: 961, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 110, FreqEnd: 60, Dur: 0.2, Attack: 0.002, Decay: 0.09, Gain: 1},
			{Wave: s.Brown, Dur: 0.2, LowPass: 500, Attack: 0.002, Decay: 0.08, Gain: 0.8},
			{Wave: s.Pink, Start: 0.02, Dur: 0.18, BandPass: 1800, Q: 0.6, Attack: 0.01, Decay: 0.09, Gain: 0.4},
		}}, 0.8},
		// A charge rifle: a snapping electric crack that drops in pitch.
		{s.Recipe{ID: "sfx.charge_rifle_shot", Seed: 962, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.025, HighPass: 3000, Attack: 0.0005, Decay: 0.01, Gain: 0.8},
			{Wave: s.Saw, Freq: 3600, FreqEnd: 420, Dur: 0.14, Attack: 0.001, Decay: 0.06, LowPass: 6000, Gain: 0.7},
			{Wave: s.Square, Freq: 1800, FreqEnd: 300, Dur: 0.1, Attack: 0.001, Decay: 0.04, Gain: 0.3},
			{Wave: s.Sine, Freq: 200, FreqEnd: 80, Dur: 0.12, Decay: 0.05, Gain: 0.6},
			{Wave: s.White, Start: 0.01, Dur: 0.1, HighPass: 4000, Grains: 60, GrainLen: 0.003, Decay: 0.05, Gain: 0.3},
		}}, 0.75},
		// A charge lance: a longer, heavier bolt with a low throb under the
		// crack and a crackling tail.
		{s.Recipe{ID: "sfx.charge_lance_shot", Seed: 963, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.04, HighPass: 2200, Attack: 0.0005, Decay: 0.016, Gain: 0.9},
			{Wave: s.Saw, Freq: 2400, FreqEnd: 180, Dur: 0.3, Attack: 0.002, Decay: 0.14, LowPass: 5000, Gain: 0.8},
			{Wave: s.Square, Freq: 900, FreqEnd: 120, Dur: 0.25, Attack: 0.002, Decay: 0.1, Tremolo: 40, TremoloDepth: 0.5, Gain: 0.35},
			{Wave: s.Sine, Freq: 120, FreqEnd: 40, Dur: 0.35, Attack: 0.002, Decay: 0.15, Gain: 1},
			{Wave: s.White, Start: 0.03, Dur: 0.3, HighPass: 3000, Grains: 120, GrainLen: 0.004, Decay: 0.15, Gain: 0.35},
		}}, 0.85},
		// The shield belt giving way: a rising shimmer that cracks and
		// scatters into glassy shards.
		{s.Recipe{ID: "sfx.shield_pop", Seed: 964, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 900, FreqEnd: 2400, Dur: 0.18, Attack: 0.02, Decay: 0.08, Gain: 0.4},
			{Wave: s.Sine, Freq: 1350, FreqEnd: 3600, Dur: 0.18, Attack: 0.02, Decay: 0.08, Gain: 0.25},
			{Wave: s.White, Start: 0.17, Dur: 0.03, HighPass: 3500, Attack: 0.0005, Decay: 0.012, Gain: 0.8},
			{Wave: s.Sine, Start: 0.17, Freq: 3200, FreqEnd: 700, Dur: 0.22, Attack: 0.001, Decay: 0.09, Gain: 0.4},
			{Wave: s.White, Start: 0.18, Dur: 0.3, HighPass: 5000, Grains: 70, GrainLen: 0.004, Decay: 0.18, Gain: 0.3},
		}}, 0.7},
		// A smokepop bursting: a soft pop and a long hiss of gas.
		{s.Recipe{ID: "sfx.smokepop_burst", Seed: 965, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 160, FreqEnd: 70, Dur: 0.12, Attack: 0.001, Decay: 0.05, Gain: 0.8},
			{Wave: s.White, Start: 0.01, Dur: 1.0, HighPass: 2500, BandPass: 5000, Q: 0.5, Attack: 0.03, Decay: 0.7, Release: 0.2, Gain: 0.6},
			{Wave: s.Pink, Start: 0.02, Dur: 0.8, BandPass: 2000, Q: 0.5, Attack: 0.05, Decay: 0.5, Release: 0.15, Gain: 0.35},
		}}, 0.65},
		// A stagger: the heavy thud of a body knocked off balance and a
		// scuff of boots.
		{s.Recipe{ID: "sfx.stagger_thud", Seed: 966, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 90, FreqEnd: 40, Dur: 0.22, Attack: 0.002, Decay: 0.1, Gain: 1},
			{Wave: s.Brown, Dur: 0.2, LowPass: 450, Attack: 0.002, Decay: 0.09, Gain: 0.9},
			{Wave: s.Pink, Start: 0.06, Dur: 0.14, BandPass: 1200, Q: 0.6, Attack: 0.01, Decay: 0.06, Gain: 0.3},
		}}, 0.75},
		// A blow with a gun stock: a hard, dry wooden-and-metal knock, less
		// soft than a fist.
		{s.Recipe{ID: "sfx.gunstock_hit", Seed: 967, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.02, BandPass: 1800, Q: 1, Attack: 0.0005, Decay: 0.008, Gain: 0.6},
			{Wave: s.Sine, Freq: 180, FreqEnd: 80, Dur: 0.12, Attack: 0.001, Decay: 0.05, Gain: 1},
			{Wave: s.Sine, Freq: 620, FreqEnd: 400, Dur: 0.08, Attack: 0.001, Decay: 0.03, Gain: 0.3},
			{Wave: s.Brown, Dur: 0.1, LowPass: 800, Decay: 0.045, Gain: 0.7},
		}}, 0.8},
		// A nutrient paste dispenser: a mechanical clunk, a wet glug of
		// paste and a squelch as the meal drops.
		{s.Recipe{ID: "sfx.paste_dispense", Seed: 968, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.Square, Freq: 300, FreqEnd: 160, Dur: 0.05, Attack: 0.001, Decay: 0.02, LowPass: 1500, Gain: 0.5},
			{Wave: s.Sine, Start: 0.08, Freq: 220, FreqEnd: 120, Dur: 0.12, Attack: 0.01, Decay: 0.06, Gain: 0.8},
			{Wave: s.Sine, Start: 0.22, Freq: 180, FreqEnd: 100, Dur: 0.1, Attack: 0.01, Decay: 0.05, Gain: 0.7},
			{Wave: s.Sine, Start: 0.34, Freq: 260, FreqEnd: 140, Dur: 0.09, Attack: 0.008, Decay: 0.04, Gain: 0.6},
			{Wave: s.Pink, Start: 0.08, Dur: 0.4, BandPass: 500, Q: 1.5, Tremolo: 14, TremoloDepth: 0.5, Attack: 0.02, Decay: 0.2, Gain: 0.4},
			{Wave: s.Brown, Start: 0.4, Dur: 0.15, LowPass: 400, Decay: 0.07, Gain: 0.6},
		}}, 0.65},
		// A tantrum: furniture and crockery smashed, a crash of splintering
		// and a rattle of fragments.
		{s.Recipe{ID: "sfx.tantrum_smash", Seed: 969, Variants: 2, Jitter: 0.08, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.05, HighPass: 1500, Attack: 0.0005, Decay: 0.02, Gain: 0.9},
			{Wave: s.Sine, Freq: 130, FreqEnd: 50, Dur: 0.3, Attack: 0.001, Decay: 0.13, Gain: 1},
			{Wave: s.Brown, Dur: 0.35, LowPass: 700, Attack: 0.001, Decay: 0.15, Gain: 0.8},
			{Wave: s.White, Start: 0.03, Dur: 0.5, HighPass: 2500, Grains: 160, GrainLen: 0.005, Decay: 0.3, Gain: 0.6},
			{Wave: s.Sine, Start: 0.04, Freq: 1700, FreqEnd: 1500, Dur: 0.2, Attack: 0.0005, Decay: 0.08, Gain: 0.2},
		}}, 0.9},
	}
}
