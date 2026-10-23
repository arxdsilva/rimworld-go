package main

import s "rimworld-go/pkg/sfxsynth"

// sound is a recipe plus its place in the mix.
type sound struct {
	recipe s.Recipe
	volume float64
}

// knock is a short wooden hit at time at, pitch f.
func knock(at, f, gain float64) []s.Layer {
	return []s.Layer{
		{Wave: s.Sine, Freq: f, FreqEnd: f * 0.75, Start: at, Dur: 0.12, Decay: 0.05, Gain: gain},
		{Wave: s.White, Start: at, Dur: 0.03, BandPass: f * 4.5, Q: 1, Decay: 0.015, Gain: gain * 0.5},
	}
}

func cat(parts ...[]s.Layer) []s.Layer {
	var out []s.Layer
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// sounds are every procedural effect (SND-002). Recipes are starting
// points tuned by ear; change one, run make audio, listen.
var sounds = []sound{
	// --- interface
	{s.Recipe{ID: "ui.click", Seed: 101, Variants: 3, Jitter: 0.03, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 1800, FreqEnd: 1500, Dur: 0.06, Attack: 0.002, Decay: 0.03},
		{Wave: s.White, Dur: 0.01, HighPass: 4000, Decay: 0.005, Gain: 0.3},
	}}, 0.55},
	{s.Recipe{ID: "ui.deny", Seed: 102, Layers: []s.Layer{
		{Wave: s.Square, Freq: 220, Dur: 0.07, LowPass: 1800, Release: 0.02, Gain: 0.6},
		{Wave: s.Square, Freq: 180, Start: 0.08, Dur: 0.08, LowPass: 1800, Release: 0.03, Gain: 0.6},
	}}, 0.5},
	{s.Recipe{ID: "ui.toggle", Seed: 103, Variants: 3, Jitter: 0.03, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.008, HighPass: 3000, Gain: 0.5},
		{Wave: s.Sine, Freq: 900, Dur: 0.08, Attack: 0.003, Decay: 0.04},
	}}, 0.5},
	{s.Recipe{ID: "ui.speed", Seed: 104, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 660, Dur: 0.06, Attack: 0.003, Decay: 0.05},
		{Wave: s.Sine, Freq: 880, Start: 0.06, Dur: 0.07, Attack: 0.003, Decay: 0.06},
	}}, 0.45},
	{s.Recipe{ID: "ui.pause", Seed: 105, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 880, Dur: 0.06, Attack: 0.003, Decay: 0.05},
		{Wave: s.Sine, Freq: 660, Start: 0.06, Dur: 0.09, Attack: 0.003, Decay: 0.07},
	}}, 0.45},
	{s.Recipe{ID: "ui.resume", Seed: 106, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 660, Dur: 0.06, Attack: 0.003, Decay: 0.05},
		{Wave: s.Sine, Freq: 990, Start: 0.06, Dur: 0.09, Attack: 0.003, Decay: 0.07},
	}}, 0.45},
	{s.Recipe{ID: "ui.notice", Seed: 107, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 660, Dur: 0.4, Attack: 0.005, Decay: 0.3},
		{Wave: s.Sine, Freq: 1320, Dur: 0.25, Attack: 0.005, Decay: 0.12, Gain: 0.2},
		{Wave: s.Sine, Freq: 880, Start: 0.12, Dur: 0.4, Attack: 0.005, Decay: 0.3},
	}}, 0.5},
	{s.Recipe{ID: "ui.threat", Seed: 108, Layers: []s.Layer{
		{Wave: s.Saw, Freq: 110, Dur: 0.9, Attack: 0.02, Decay: 0.8, Sustain: 0.3, Release: 0.2, LowPass: 1100, Tremolo: 7, TremoloDepth: 0.35},
		{Wave: s.Saw, Freq: 164.8, Dur: 0.9, Attack: 0.03, Decay: 0.8, Sustain: 0.3, Release: 0.2, LowPass: 1100, Gain: 0.6},
		{Wave: s.Sine, Freq: 55, Dur: 0.9, Attack: 0.02, Decay: 0.9, Release: 0.2, Gain: 0.6},
	}}, 0.7},

	// --- work
	{s.Recipe{ID: "sfx.chop", Seed: 201, Variants: 3, Jitter: 0.08, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.12, BandPass: 1200, Q: 1.2, Attack: 0.001, Decay: 0.05},
		{Wave: s.Sine, Freq: 180, FreqEnd: 120, Dur: 0.12, Decay: 0.06, Gain: 0.9},
		{Wave: s.White, Dur: 0.02, HighPass: 5000, Gain: 0.3},
	}}, 0.8},
	{s.Recipe{ID: "sfx.tree_fall", Seed: 202, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
		{Wave: s.Saw, Freq: 90, FreqEnd: 60, Dur: 0.9, Attack: 0.1, Release: 0.2, BandPass: 700, BandEnd: 450, Q: 6, Tremolo: 28, TremoloDepth: 0.6, Gain: 0.5},
		{Wave: s.White, Start: 0.85, Dur: 0.6, HighPass: 2000, Grains: 220, Decay: 0.4, Gain: 0.3},
		{Wave: s.Brown, Start: 0.9, Dur: 0.7, LowPass: 300, Attack: 0.005, Decay: 0.4},
		{Wave: s.Sine, Freq: 70, FreqEnd: 40, Start: 0.9, Dur: 0.5, Decay: 0.3, Gain: 0.9},
	}}, 0.85},
	{s.Recipe{ID: "sfx.pick", Seed: 203, Variants: 3, Jitter: 0.06, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 2100, Dur: 0.25, Attack: 0.001, Decay: 0.12, Gain: 0.7},
		{Wave: s.Sine, Freq: 3300, Dur: 0.15, Attack: 0.001, Decay: 0.06, Gain: 0.35},
		{Wave: s.White, Dur: 0.015, HighPass: 3000, Gain: 0.6},
		{Wave: s.Brown, Dur: 0.1, LowPass: 600, Decay: 0.05, Gain: 0.5},
	}}, 0.7},
	{s.Recipe{ID: "sfx.rock_break", Seed: 204, Variants: 2, Jitter: 0.06, Layers: []s.Layer{
		{Wave: s.Brown, Dur: 0.7, LowPass: 800, Grains: 60, GrainLen: 0.03, Release: 0.2},
		{Wave: s.White, Dur: 0.5, BandPass: 2500, Grains: 40, Release: 0.15, Gain: 0.4},
		{Wave: s.Sine, Freq: 80, FreqEnd: 50, Dur: 0.3, Decay: 0.15, Gain: 0.6},
	}}, 0.8},
	{s.Recipe{ID: "sfx.hammer", Seed: 205, Variants: 3, Jitter: 0.08, Layers: knock(0, 400, 1)}, 0.75},
	{s.Recipe{ID: "sfx.build_done", Seed: 206, Layers: cat(knock(0, 420, 1), knock(0.12, 400, 0.9), knock(0.24, 440, 1),
		[]s.Layer{
			{Wave: s.Sine, Freq: 660, Start: 0.38, Dur: 0.35, Attack: 0.005, Decay: 0.25, Gain: 0.5},
			{Wave: s.Sine, Freq: 990, Start: 0.45, Dur: 0.35, Attack: 0.005, Decay: 0.25, Gain: 0.5},
		})}, 0.7},
	{s.Recipe{ID: "sfx.saw", Seed: 207, Variants: 3, Jitter: 0.06, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.45, BandPass: 2500, Q: 2, Tremolo: 7, TremoloDepth: 0.9, Release: 0.08},
		{Wave: s.Saw, Freq: 180, Dur: 0.45, BandPass: 1000, Release: 0.08, Gain: 0.15},
	}}, 0.6},
	{s.Recipe{ID: "sfx.harvest", Seed: 208, Variants: 3, Jitter: 0.08, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.3, HighPass: 2500, Grains: 300, GrainLen: 0.01, Release: 0.1},
		{Wave: s.Pink, Dur: 0.2, BandPass: 1500, Decay: 0.1, Gain: 0.3},
	}}, 0.6},
	{s.Recipe{ID: "sfx.soil", Seed: 209, Variants: 3, Jitter: 0.08, Layers: []s.Layer{
		{Wave: s.Brown, Dur: 0.2, LowPass: 600, Attack: 0.005, Decay: 0.1},
		{Wave: s.White, Dur: 0.08, BandPass: 900, Q: 0.8, Decay: 0.04, Gain: 0.4},
	}}, 0.65},
	{s.Recipe{ID: "sfx.cleave", Seed: 210, Variants: 3, Jitter: 0.08, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 120, FreqEnd: 70, Dur: 0.2, Decay: 0.08},
		{Wave: s.White, Dur: 0.1, BandPass: 700, Q: 1, Decay: 0.05, Gain: 0.7},
		{Wave: s.Pink, Dur: 0.05, LowPass: 2000, Gain: 0.4},
	}}, 0.7},
	{s.Recipe{ID: "sfx.wood_drop", Seed: 211, Variants: 2, Jitter: 0.08, Layers: cat(knock(0, 300, 1), knock(0.14, 260, 0.6))}, 0.7},
	// A broom: two brushy strokes of filtered noise (SIM-021).
	{s.Recipe{ID: "sfx.sweep", Seed: 212, Variants: 3, Jitter: 0.1, Layers: []s.Layer{
		{Wave: s.Pink, Dur: 0.22, BandPass: 3200, BandEnd: 2400, Q: 0.6, Attack: 0.04, Decay: 0.12, Gain: 0.55},
		{Wave: s.White, Start: 0.26, Dur: 0.16, BandPass: 4200, BandEnd: 3000, Q: 0.7, Attack: 0.03, Decay: 0.09, Gain: 0.35},
	}}, 0.45},

	// --- doors and hits
	{s.Recipe{ID: "sfx.door_open", Seed: 301, Variants: 2, Jitter: 0.05, Layers: cat([]s.Layer{
		{Wave: s.Saw, Freq: 140, FreqEnd: 190, Dur: 0.35, Attack: 0.03, Release: 0.06, BandPass: 900, Q: 8, Tremolo: 25, TremoloDepth: 0.5, Gain: 0.7},
	}, knock(0.32, 260, 0.6))}, 0.55},
	{s.Recipe{ID: "sfx.door_close", Seed: 302, Variants: 2, Jitter: 0.05, Layers: cat(knock(0, 220, 1), []s.Layer{
		{Wave: s.Brown, Dur: 0.1, LowPass: 400, Decay: 0.05, Gain: 0.6},
	})}, 0.6},
	{s.Recipe{ID: "sfx.melee_hit", Seed: 303, Variants: 3, Jitter: 0.08, Layers: []s.Layer{
		{Wave: s.Pink, Dur: 0.12, LowPass: 1500, Decay: 0.05},
		{Wave: s.Sine, Freq: 140, FreqEnd: 80, Dur: 0.12, Decay: 0.06, Gain: 0.8},
	}}, 0.8},
	{s.Recipe{ID: "sfx.wood_hit", Seed: 304, Variants: 3, Jitter: 0.08, Layers: knock(0, 500, 1)}, 0.7},
	{s.Recipe{ID: "sfx.gunshot", Seed: 305, Variants: 3, Jitter: 0.05, Delay: 0.09, DelayMix: 0.25, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.05, HighPass: 1000, Decay: 0.02},
		{Wave: s.Brown, Dur: 0.4, LowPass: 400, Decay: 0.2, Gain: 0.9},
		{Wave: s.Sine, Freq: 60, FreqEnd: 40, Dur: 0.3, Decay: 0.15, Gain: 0.7},
	}}, 0.85},

	// --- weather and fire
	{s.Recipe{ID: "sfx.thunder", Seed: 401, Variants: 3, Jitter: 0.15, Layers: []s.Layer{
		{Wave: s.Pink, Dur: 0.25, LowPass: 1200, Decay: 0.15, Gain: 0.5},
		{Wave: s.Brown, Dur: 4, LowPass: 250, Attack: 0.05, Decay: 2.5, Sustain: 0.2, Release: 0.6, Gain: 0.6},
		{Wave: s.Brown, Dur: 4, LowPass: 300, Grains: 6, GrainLen: 0.3, Release: 0.8},
	}}, 0.9},
	{s.Recipe{ID: "amb.campfire", Seed: 402, Loop: 8, Layers: []s.Layer{
		{Wave: s.Brown, Dur: 10, LowPass: 500, Gain: 0.5},
		{Wave: s.White, Dur: 10, HighPass: 1500, Grains: 25, GrainLen: 0.006},
		{Wave: s.White, Dur: 10, BandPass: 3000, Grains: 8, GrainLen: 0.015, Gain: 0.7},
	}}, 0.7},

	// --- new game (NG-1 §7.4)
	// A soft tick for a slider moving one step.
	{s.Recipe{ID: "ui.slider_tick", Seed: 109, Variants: 2, Jitter: 0.04, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 2600, FreqEnd: 2100, Dur: 0.03, Attack: 0.001, Decay: 0.012, Gain: 0.8},
		{Wave: s.White, Dur: 0.006, HighPass: 5000, Decay: 0.003, Gain: 0.2},
	}}, 0.4},
	// A page turn: a short breath of filtered noise.
	{s.Recipe{ID: "ui.page", Seed: 110, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
		{Wave: s.Pink, Dur: 0.22, BandPass: 1500, BandEnd: 3400, Q: 0.7, Attack: 0.05, Decay: 0.09, Gain: 0.6},
		{Wave: s.White, Start: 0.03, Dur: 0.15, HighPass: 4500, Attack: 0.04, Decay: 0.06, Gain: 0.18},
	}}, 0.45},

	// --- landing pod
	// The descent: a whistle climbing over a rumble and rushing air.
	{s.Recipe{ID: "sfx.pod_fall", Seed: 501, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 320, FreqEnd: 1900, Dur: 1.4, Attack: 0.5, Release: 0.15, Tremolo: 9, TremoloDepth: 0.15, Gain: 0.5},
		{Wave: s.Triangle, Freq: 640, FreqEnd: 3700, Dur: 1.4, Attack: 0.6, Release: 0.15, Gain: 0.15},
		{Wave: s.White, Dur: 1.4, BandPass: 700, BandEnd: 3600, Q: 1.2, Attack: 0.7, Release: 0.15, Gain: 0.45},
		{Wave: s.Brown, Dur: 1.4, LowPass: 320, Attack: 0.9, Release: 0.1, Gain: 0.9},
	}}, 0.8},
	// The impact: a thud, rocks thrown up, dust settling.
	{s.Recipe{ID: "sfx.pod_land", Seed: 502, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 95, FreqEnd: 38, Dur: 0.5, Attack: 0.002, Decay: 0.18, Gain: 0.9},
		{Wave: s.Brown, Dur: 0.6, LowPass: 450, Attack: 0.002, Decay: 0.25},
		{Wave: s.White, Dur: 0.9, HighPass: 1800, Grains: 90, GrainLen: 0.012, Attack: 0.01, Decay: 0.4, Gain: 0.25},
		{Wave: s.Pink, Start: 0.05, Dur: 1.0, BandPass: 2200, BandEnd: 1200, Q: 0.6, Attack: 0.05, Decay: 0.45, Gain: 0.3},
	}}, 0.85},
	// The hatch: pressure hisses out, then a metal clank.
	{s.Recipe{ID: "sfx.pod_open", Seed: 503, Layers: []s.Layer{
		{Wave: s.White, Dur: 0.8, HighPass: 2400, Attack: 0.03, Decay: 0.4, Gain: 0.4},
		{Wave: s.Pink, Dur: 0.7, BandPass: 1400, Q: 0.8, Attack: 0.04, Decay: 0.3, Gain: 0.3},
		{Wave: s.Sine, Freq: 1250, Start: 0.6, Dur: 0.5, Attack: 0.001, Decay: 0.2, Gain: 0.4},
		{Wave: s.Sine, Freq: 1960, Start: 0.6, Dur: 0.3, Attack: 0.001, Decay: 0.1, Gain: 0.25},
		{Wave: s.Sine, Freq: 310, FreqEnd: 240, Start: 0.6, Dur: 0.25, Attack: 0.001, Decay: 0.1, Gain: 0.6},
		{Wave: s.White, Start: 0.6, Dur: 0.02, HighPass: 3000, Gain: 0.5},
	}}, 0.7},

	// --- water (NG-1 §7.4)
	// A babbling brook: pinkish noise through several band-passes that drift
	// slowly, with fast gurgling wobble and splashing grains.
	{s.Recipe{ID: "amb.water_river", Seed: 601, Loop: 8, Layers: []s.Layer{
		{Wave: s.Pink, Dur: 10, BandPass: 450, BandEnd: 800, Q: 1.4, Tremolo: 7, TremoloDepth: 0.45, Gain: 0.8},
		{Wave: s.Pink, Dur: 10, BandPass: 1300, BandEnd: 850, Q: 1.6, Tremolo: 11, TremoloDepth: 0.5, Gain: 0.7},
		{Wave: s.Pink, Dur: 10, BandPass: 2300, BandEnd: 3100, Q: 2, Tremolo: 5, TremoloDepth: 0.6, Gain: 0.45},
		{Wave: s.White, Dur: 10, BandPass: 3400, BandEnd: 2600, Q: 2.5, Grains: 30, GrainLen: 0.02, Gain: 0.3},
		{Wave: s.Brown, Dur: 10, LowPass: 260, Gain: 0.35},
	}}, 0.6},
	// Waves washing a shore: a slow swell of noise with a fizz of foam at
	// the crest.
	{s.Recipe{ID: "amb.water_shore", Seed: 602, Loop: 8, Layers: []s.Layer{
		{Wave: s.Brown, Dur: 10, LowPass: 700, Tremolo: 0.25, TremoloDepth: 0.85, Gain: 0.8},
		{Wave: s.Pink, Dur: 10, BandPass: 900, BandEnd: 1500, Q: 0.6, Tremolo: 0.25, TremoloDepth: 0.9, Gain: 0.6},
		{Wave: s.White, Dur: 10, HighPass: 3500, Tremolo: 0.25, TremoloDepth: 0.95, Grains: 120, GrainLen: 0.01, Gain: 0.25},
	}}, 0.6},
}
