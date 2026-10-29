package main

import s "rimworld-go/pkg/sfxsynth"

// anvilHit is one hammer blow on hot metal at time at: a sharp tick over a
// ringing clang.
func anvilHit(at, f, gain float64) []s.Layer {
	return []s.Layer{
		{Wave: s.White, Start: at, Dur: 0.02, HighPass: 2500, Attack: 0.0005, Decay: 0.008, Gain: gain},
		{Wave: s.Sine, Start: at, Freq: f, FreqEnd: f * 0.98, Dur: 0.22, Attack: 0.0005, Decay: 0.09, Gain: gain * 0.6},
		{Wave: s.Sine, Start: at, Freq: f * 2.76, Dur: 0.12, Attack: 0.0005, Decay: 0.05, Gain: gain * 0.3},
		{Wave: s.Sine, Start: at, Freq: 140, FreqEnd: 80, Dur: 0.08, Decay: 0.03, Gain: gain * 0.5},
	}
}

// chiselTap is one tap of a chisel on stone at time at.
func chiselTap(at, f, gain float64) []s.Layer {
	return []s.Layer{
		{Wave: s.White, Start: at, Dur: 0.025, BandPass: f, Q: 1.5, Attack: 0.0005, Decay: 0.01, Gain: gain},
		{Wave: s.Sine, Start: at, Freq: f / 2, FreqEnd: f / 3, Dur: 0.06, Decay: 0.022, Gain: gain * 0.5},
		{Wave: s.White, Start: at + 0.01, Dur: 0.07, HighPass: 4000, Grains: 120, GrainLen: 0.003, Decay: 0.03, Gain: gain * 0.25},
	}
}

// structureSounds are the sounds of wave D's defences and workshops
// (DEF-3, PRD-3): two heavy turrets, a mortar, a charge arming, the
// powered door and six bench work loops. Ids are sfx.<name> for the
// one-shots and amb.<name> for the loops (heard where a crafter works).
// Seeds are 930+.
func structureSounds() []sound {
	return []sound{
		// The autocannon: a rapid, hard thump with a metallic bark.
		{s.Recipe{ID: "sfx.autocannon_shot", Seed: 930, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.03, HighPass: 1500, Attack: 0.0005, Decay: 0.012, Gain: 1},
			{Wave: s.Sine, Freq: 180, FreqEnd: 60, Dur: 0.16, Attack: 0.001, Decay: 0.07, Gain: 1},
			{Wave: s.Square, Freq: 520, FreqEnd: 240, Dur: 0.05, Attack: 0.0005, Decay: 0.02, LowPass: 3000, Gain: 0.35},
			{Wave: s.White, Dur: 0.14, BandPass: 1800, Q: 0.9, Attack: 0.001, Decay: 0.06, Gain: 0.5},
			{Wave: s.Brown, Dur: 0.2, LowPass: 550, Decay: 0.09, Gain: 0.7},
		}}, 0.8},
		// The uranium slug turret: a deep, heavy boom with a long ring.
		{s.Recipe{ID: "sfx.slug_turret_shot", Seed: 931, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.04, HighPass: 900, Attack: 0.0005, Decay: 0.018, Gain: 1},
			{Wave: s.Sine, Freq: 110, FreqEnd: 36, Dur: 0.4, Attack: 0.002, Decay: 0.17, Gain: 1},
			{Wave: s.Brown, Dur: 0.45, LowPass: 420, Decay: 0.2, Gain: 0.9},
			{Wave: s.Sine, Freq: 1250, FreqEnd: 1180, Dur: 0.35, Attack: 0.0005, Decay: 0.14, Gain: 0.12},
			{Wave: s.White, Start: 0.04, Dur: 0.3, BandPass: 900, BandEnd: 350, Q: 0.6, Decay: 0.12, Gain: 0.5},
		}}, 0.9},
		// A mortar firing: a hollow thump from the tube and a rising whoosh of
		// the shell leaving.
		{s.Recipe{ID: "sfx.mortar_launch", Seed: 932, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 130, FreqEnd: 45, Dur: 0.35, Attack: 0.001, Decay: 0.14, Gain: 1},
			{Wave: s.Brown, Dur: 0.3, LowPass: 500, Decay: 0.12, Gain: 0.8},
			{Wave: s.White, Dur: 0.03, HighPass: 1200, Attack: 0.0005, Decay: 0.012, Gain: 0.7},
			{Wave: s.Pink, Start: 0.05, Dur: 0.5, BandPass: 500, BandEnd: 2600, Q: 0.9, Attack: 0.05, Decay: 0.2, Release: 0.1, Gain: 0.4},
			{Wave: s.Sine, Freq: 420, FreqEnd: 190, Dur: 0.1, Decay: 0.04, Gain: 0.3},
		}}, 0.85},
		// A shell coming down: a whistle sliding from high to low, growing.
		{s.Recipe{ID: "sfx.mortar_incoming", Seed: 933, Variants: 2, Jitter: 0.04, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 2400, FreqEnd: 700, Dur: 1.1, Attack: 0.25, Decay: 0.8, Sustain: 0.6, Release: 0.08, Gain: 0.6},
			{Wave: s.Sine, Freq: 3600, FreqEnd: 1050, Dur: 1.1, Attack: 0.3, Decay: 0.8, Sustain: 0.4, Release: 0.08, Gain: 0.2},
			{Wave: s.White, Dur: 1.1, BandPass: 2200, BandEnd: 800, Q: 3, Attack: 0.4, Decay: 0.9, Sustain: 0.5, Release: 0.08, Gain: 0.35},
		}}, 0.55},
		// A turret's spent barrel dropping out and a new one locking in:
		// a heavy clunk, a scrape and a latch.
		{s.Recipe{ID: "sfx.turret_rearm", Seed: 934, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 150, FreqEnd: 70, Dur: 0.18, Attack: 0.001, Decay: 0.07, Gain: 0.9},
			{Wave: s.White, Dur: 0.025, BandPass: 1800, Q: 1.2, Attack: 0.0005, Decay: 0.01, Gain: 0.7},
			{Wave: s.White, Start: 0.12, Dur: 0.18, BandPass: 2500, BandEnd: 1500, Q: 2, Attack: 0.02, Decay: 0.07, Gain: 0.25},
			{Wave: s.Sine, Start: 0.34, Freq: 210, FreqEnd: 120, Dur: 0.14, Attack: 0.001, Decay: 0.05, Gain: 1},
			{Wave: s.White, Start: 0.34, Dur: 0.03, HighPass: 2000, Attack: 0.0005, Decay: 0.012, Gain: 0.8},
			{Wave: s.Sine, Start: 0.34, Freq: 1900, Dur: 0.12, Attack: 0.0005, Decay: 0.045, Gain: 0.2},
		}}, 0.7},
		// A buried charge arming: two small sharp clicks and a faint beep.
		{s.Recipe{ID: "sfx.ied_arm", Seed: 935, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.White, Dur: 0.012, HighPass: 3500, Attack: 0.0003, Decay: 0.005, Gain: 0.9},
			{Wave: s.Sine, Freq: 2200, FreqEnd: 1800, Dur: 0.02, Decay: 0.008, Gain: 0.4},
			{Wave: s.White, Start: 0.11, Dur: 0.012, HighPass: 3000, Attack: 0.0003, Decay: 0.005, Gain: 0.7},
			{Wave: s.Sine, Start: 0.2, Freq: 1500, Dur: 0.07, Attack: 0.002, Decay: 0.03, Gain: 0.18},
		}}, 0.5},
		// The powered door sliding open: a quick servo whine rising, a soft
		// slide of metal and a stop.
		{s.Recipe{ID: "sfx.autodoor_open", Seed: 936, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Saw, Freq: 260, FreqEnd: 520, Dur: 0.4, Attack: 0.03, Decay: 0.3, Release: 0.08, LowPass: 1800, Gain: 0.35},
			{Wave: s.Pink, Dur: 0.4, BandPass: 1500, BandEnd: 2200, Q: 0.8, Attack: 0.04, Decay: 0.25, Release: 0.08, Gain: 0.35},
			{Wave: s.Sine, Start: 0.38, Freq: 180, FreqEnd: 100, Dur: 0.1, Decay: 0.04, Gain: 0.5},
			{Wave: s.White, Start: 0.38, Dur: 0.015, BandPass: 1600, Q: 1, Decay: 0.007, Gain: 0.4},
		}}, 0.5},
		// And closing: the same whine falling, ending in a firmer clack.
		{s.Recipe{ID: "sfx.autodoor_close", Seed: 937, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Saw, Freq: 520, FreqEnd: 250, Dur: 0.4, Attack: 0.03, Decay: 0.3, Release: 0.08, LowPass: 1800, Gain: 0.35},
			{Wave: s.Pink, Dur: 0.4, BandPass: 2200, BandEnd: 1400, Q: 0.8, Attack: 0.04, Decay: 0.25, Release: 0.08, Gain: 0.35},
			{Wave: s.Sine, Start: 0.38, Freq: 200, FreqEnd: 90, Dur: 0.12, Decay: 0.05, Gain: 0.7},
			{Wave: s.White, Start: 0.38, Dur: 0.02, BandPass: 1400, Q: 1, Decay: 0.009, Gain: 0.55},
		}}, 0.55},

		// Bench work loops: positioned, one per bench while a crafter works.
		// Smithy: hammer blows on an anvil with the fire's low roar under.
		{s.Recipe{ID: "amb.bench_smithy", Seed: 940, Loop: 1.4, Layers: cat(
			anvilHit(0.05, 1150, 0.9), anvilHit(0.55, 1230, 0.75), anvilHit(0.95, 1150, 0.85),
			[]s.Layer{{Wave: s.Brown, Dur: span(1.4), LowPass: 380, Gain: 0.35}})}, 0.4},
		// Machining: a lathe's whir, a cutting hiss and a lower motor.
		{s.Recipe{ID: "amb.bench_machining", Seed: 941, Loop: 0.5, Layers: cat(steady(0.5,
			s.Layer{Wave: s.Saw, Freq: 220, LowPass: 900, Tremolo: 22, TremoloDepth: 0.2, Gain: 0.35},
			s.Layer{Wave: s.Sine, Freq: 440, Tremolo: 22, TremoloDepth: 0.2, Gain: 0.2},
			s.Layer{Wave: s.Sine, Freq: 70, Gain: 0.3}),
			[]s.Layer{{Wave: s.White, Dur: span(0.5), BandPass: 3200, Q: 1.5, Gain: 0.2}})}, 0.3},
		// Smelter: a furnace's deep roar with a rumble and flickering hiss.
		{s.Recipe{ID: "amb.bench_smelter", Seed: 942, Loop: 1.2, Layers: []s.Layer{
			{Wave: s.Brown, Dur: span(1.2), LowPass: 300, Gain: 1},
			{Wave: s.Pink, Dur: span(1.2), BandPass: 450, Q: 0.5, Gain: 0.6},
			{Wave: s.White, Dur: span(1.2), HighPass: 2500, Grains: 70, GrainLen: 0.004, Gain: 0.2},
			{Wave: s.Pink, Dur: span(1.2), BandPass: 1100, Q: 0.7, Tremolo: 3, TremoloDepth: 0.5, Gain: 0.25},
		}}, 0.35},
		// Refinery: liquid bubbling over a pump's low hum.
		{s.Recipe{ID: "amb.bench_refinery", Seed: 943, Loop: 1.2, Layers: cat(steady(1.2,
			s.Layer{Wave: s.Sine, Freq: 90, Tremolo: 6, TremoloDepth: 0.2, Gain: 0.3}),
			[]s.Layer{
				{Wave: s.Pink, Dur: span(1.2), BandPass: 500, Q: 0.7, Gain: 0.3},
				{Wave: s.Pink, Dur: span(1.2), BandPass: 700, BandEnd: 1400, Q: 4, Grains: 9, GrainLen: 0.05, Gain: 0.7},
				{Wave: s.Sine, Dur: span(1.2), Freq: 600, FreqEnd: 1200, Grains: 7, GrainLen: 0.04, Gain: 0.2},
			})}, 0.3},
		// Sculpting: chisel taps on stone with a patient rhythm.
		{s.Recipe{ID: "amb.bench_sculpt", Seed: 944, Loop: 1.6, Layers: cat(
			chiselTap(0.05, 2600, 0.8), chiselTap(0.45, 2300, 0.7), chiselTap(0.75, 2700, 0.8),
			chiselTap(1.2, 2400, 0.7))}, 0.3},
		// Fabrication: a clean electronic hum, a servo chirp now and then.
		{s.Recipe{ID: "amb.bench_fabrication", Seed: 945, Loop: 1.0, Layers: cat(steady(1.0,
			s.Layer{Wave: s.Sine, Freq: 120, Gain: 0.4},
			s.Layer{Wave: s.Sine, Freq: 240, Tremolo: 4, TremoloDepth: 0.2, Gain: 0.25},
			s.Layer{Wave: s.Triangle, Freq: 480, Gain: 0.08}),
			[]s.Layer{
				{Wave: s.White, Dur: span(1.0), BandPass: 4500, Q: 1, Gain: 0.05},
				{Wave: s.Sine, Start: 0.3, Freq: 900, FreqEnd: 1500, Dur: 0.12, Attack: 0.02, Decay: 0.05, Gain: 0.15},
				{Wave: s.Sine, Start: 0.7, Freq: 1500, FreqEnd: 800, Dur: 0.1, Attack: 0.02, Decay: 0.04, Gain: 0.12},
			})}, 0.25},
	}
}
