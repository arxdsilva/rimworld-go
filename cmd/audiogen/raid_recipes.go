package main

import s "rimworld-go/pkg/sfxsynth"

// raidSounds are the sounds of wave E's raids (DEF-4): a hostile drop pod
// falling and landing, and the hum of a crashed ship part. The grenade
// throw is sfx.grenade_throw (combatSounds). Seeds are 950+.
func raidSounds() []sound {
	return []sound{
		// A raid's pod coming in: a long descending roar and whistle that
		// swells into the ground, heavier and longer than the colony's own
		// landing pod (about 2 s).
		{s.Recipe{ID: "sfx.droppod_incoming", Seed: 950, Variants: 2, Jitter: 0.04, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 2600, FreqEnd: 380, Dur: 2.0, Attack: 0.5, Decay: 1.3, Sustain: 0.7, Release: 0.1, Tremolo: 7, TremoloDepth: 0.15, Gain: 0.5},
			{Wave: s.Saw, Freq: 1300, FreqEnd: 190, Dur: 2.0, Attack: 0.6, Decay: 1.2, Sustain: 0.5, Release: 0.1, LowPass: 2200, Gain: 0.15},
			{Wave: s.White, Dur: 2.0, BandPass: 3200, BandEnd: 600, Q: 1.1, Attack: 0.7, Decay: 1.2, Sustain: 0.6, Release: 0.1, Gain: 0.5},
			{Wave: s.Brown, Dur: 2.0, LowPass: 380, Attack: 1.2, Release: 0.1, Gain: 1},
		}}, 0.85},
		// The impact: a heavy thud, a crack of metal and a spray of debris.
		{s.Recipe{ID: "sfx.droppod_impact", Seed: 951, Variants: 2, Jitter: 0.05, Layers: []s.Layer{
			{Wave: s.Sine, Freq: 85, FreqEnd: 30, Dur: 0.7, Attack: 0.002, Decay: 0.3, Gain: 1},
			{Wave: s.Brown, Dur: 0.9, LowPass: 400, Attack: 0.002, Decay: 0.4, Gain: 1},
			{Wave: s.White, Dur: 0.04, HighPass: 1200, Attack: 0.0005, Decay: 0.018, Gain: 0.8},
			{Wave: s.Sine, Freq: 330, FreqEnd: 210, Dur: 0.3, Attack: 0.001, Decay: 0.12, Gain: 0.25},
			{Wave: s.White, Start: 0.05, Dur: 1.2, HighPass: 1500, Grains: 140, GrainLen: 0.012, Attack: 0.01, Decay: 0.6, Gain: 0.35},
			{Wave: s.Pink, Start: 0.08, Dur: 1.2, BandPass: 1800, BandEnd: 800, Q: 0.6, Attack: 0.05, Decay: 0.6, Gain: 0.3},
		}}, 0.95},
		// A crashed ship part: an ominous, low machine drone with a slow
		// beat between two near notes and a faint electrical whine.
		{s.Recipe{ID: "amb.ship_part_hum", Seed: 952, Loop: 4.0, Layers: cat(steady(4.0,
			s.Layer{Wave: s.Sine, Freq: 55, Tremolo: 0.5, TremoloDepth: 0.2, Gain: 0.5},
			s.Layer{Wave: s.Sine, Freq: 58, Gain: 0.4},
			s.Layer{Wave: s.Saw, Freq: 110, LowPass: 400, Tremolo: 1.5, TremoloDepth: 0.3, Gain: 0.18},
			s.Layer{Wave: s.Sine, Freq: 1480, Tremolo: 0.75, TremoloDepth: 0.6, Gain: 0.03}),
			[]s.Layer{{Wave: s.Brown, Dur: span(4.0), LowPass: 260, Gain: 0.4}})}, 0.3},
	}
}
