// Package sfxsynth renders original sound effects from recipes: layers of
// oscillators and noise with envelopes, pitch slides, filters, tremolo and
// random grains, mixed, normalized and dithered to 16-bit PCM. Output is a
// pure function of the recipe and variant, byte-identical on every machine
// (cmd/audiogen, make audio).
package sfxsynth

import (
	"math"
)

// Rate is the sample rate of every rendered sound.
const Rate = 48000

// Wave is a layer's source.
type Wave int

// Sources.
const (
	Sine Wave = iota
	Square
	Saw
	Triangle
	White
	Pink
	Brown
)

// Layer is one voice of a recipe. Times are seconds, frequencies Hz; a
// zero field is "off" unless noted.
type Layer struct {
	Wave    Wave
	Freq    float64 // oscillators: start pitch
	FreqEnd float64 // exponential slide to this pitch over Dur
	Duty    float64 // Square: high fraction (0 means 0.5)
	Start   float64 // offset in the sound
	Dur     float64
	Gain    float64 // 0 means 1

	// Envelope: linear attack, exponential decay towards Sustain (Decay is
	// the time to fall most of the way), then a linear Release at the end.
	Attack, Decay, Sustain, Release float64

	LowPass, HighPass float64 // one-pole cutoffs
	BandPass, Q       float64 // band-pass centre (Q 0 means 1)
	BandEnd           float64 // band-pass centre slides here over Dur

	Tremolo, TremoloDepth float64 // amplitude wobble rate and depth (0..1)

	// Grains > 0 gates the layer with random bursts: Grains per second,
	// each GrainLen long (0 means 8 ms) with a random level.
	Grains, GrainLen float64
}

// Recipe is one sound.
type Recipe struct {
	ID       string
	Seed     uint64
	Layers   []Layer
	Variants int     // 0 means 1
	Jitter   float64 // per-variant pitch and timing spread (fraction)
	Delay    float64 // echo time (s), with DelayMix feedback
	DelayMix float64
	Loop     float64 // > 0: a seamless loop this long; the tail crossfades into the head
	Peak     float64 // normalize peak to this (0 means -1 dBFS)
}

// rng is splitmix64: tiny, seeded, identical everywhere.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// float is uniform in [0, 1).
func (r *rng) float() float64 { return float64(r.next()>>11) / (1 << 53) }

// bi is uniform in [-1, 1).
func (r *rng) bi() float64 { return 2*r.float() - 1 }

// Length is the recipe's duration in seconds (the loop length for loops).
func (rc Recipe) Length() float64 {
	if rc.Loop > 0 {
		return rc.Loop
	}
	end := 0.0
	for _, l := range rc.Layers {
		end = math.Max(end, l.Start+l.Dur)
	}
	if rc.Delay > 0 && rc.DelayMix > 0 {
		end += rc.Delay * 4
	}
	return end
}

// Render mixes variant v of the recipe to float samples in [-1, 1].
func Render(rc Recipe, v int) []float64 {
	r := &rng{s: rc.Seed*0x100000001B3 + uint64(v)*0x9E3779B97F4A7C15 + 1}
	jit := func() float64 { return 1 + rc.Jitter*r.bi() }
	length := rc.Length()
	fade := 0.0
	if rc.Loop > 0 {
		fade = math.Min(1, rc.Loop/4)
		length += fade
	}
	n := int(math.Ceil(length * Rate))
	out := make([]float64, n)
	for li, l := range rc.Layers {
		lr := &rng{s: r.next() + uint64(li)}
		pitch, stretch := 1.0, 1.0
		if v > 0 {
			pitch, stretch = jit(), 1+(jit()-1)*0.5
		}
		renderLayer(out, l, lr, pitch, stretch)
	}
	if rc.Delay > 0 && rc.DelayMix > 0 {
		d := int(rc.Delay * Rate)
		for i := d; i < n; i++ {
			out[i] += out[i-d] * rc.DelayMix
		}
	}
	if rc.Loop > 0 {
		out = loopFix(out, int(fade*Rate))
	}
	peak := rc.Peak
	if peak == 0 {
		peak = math.Pow(10, -1.0/20)
	}
	normalize(out, peak)
	return out
}

func renderLayer(out []float64, l Layer, r *rng, pitch, stretch float64) {
	dur := l.Dur * stretch
	start := int(l.Start * stretch * Rate)
	n := int(dur * Rate)
	gain := l.Gain
	if gain == 0 {
		gain = 1
	}
	duty := l.Duty
	if duty == 0 {
		duty = 0.5
	}
	q := l.Q
	if q == 0 {
		q = 1
	}
	glen := l.GrainLen
	if glen == 0 {
		glen = 0.008
	}
	var phase, lp, hpLow, brown float64
	var pink [7]float64
	var bp biquad
	var grain, grainLvl float64
	grainStep := 1 / (glen * Rate)
	lpA := onePole(l.LowPass)
	hpA := onePole(l.HighPass)
	for i := 0; i < n && start+i < len(out); i++ {
		t := float64(i) / Rate
		u := t / dur
		f := l.Freq * pitch
		if l.FreqEnd > 0 && l.Freq > 0 {
			f = l.Freq * pitch * math.Pow(l.FreqEnd/l.Freq, u)
		}
		var s float64
		switch l.Wave {
		case Sine:
			s = math.Sin(2 * math.Pi * phase)
		case Square:
			s = -1
			if phase < duty {
				s = 1
			}
		case Saw:
			s = 2*phase - 1
		case Triangle:
			s = 4*math.Abs(phase-0.5) - 1
		case White:
			s = r.bi()
		case Pink:
			w := r.bi()
			pink[0] = 0.99886*pink[0] + w*0.0555179
			pink[1] = 0.99332*pink[1] + w*0.0750759
			pink[2] = 0.96900*pink[2] + w*0.1538520
			pink[3] = 0.86650*pink[3] + w*0.3104856
			pink[4] = 0.55000*pink[4] + w*0.5329522
			pink[5] = -0.7616*pink[5] - w*0.0168980
			s = (pink[0] + pink[1] + pink[2] + pink[3] + pink[4] + pink[5] + pink[6] + w*0.5362) * 0.11
			pink[6] = w * 0.115926
		case Brown:
			brown = (brown + 0.02*r.bi()) / 1.02
			s = brown * 3.5
		}
		phase += f / Rate
		phase -= math.Floor(phase)
		if lpA > 0 {
			lp += lpA * (s - lp)
			s = lp
		}
		if hpA > 0 {
			hpLow += hpA * (s - hpLow)
			s -= hpLow
		}
		if l.BandPass > 0 {
			c := l.BandPass * pitch
			if l.BandEnd > 0 {
				c = l.BandPass * pitch * math.Pow(l.BandEnd/l.BandPass, u)
			}
			if i%32 == 0 || i == 0 {
				bp.bandPass(c, q)
			}
			s = bp.step(s)
		}
		if l.Grains > 0 {
			if grain <= 0 && r.float() < l.Grains/Rate {
				grain, grainLvl = 1, 0.3+0.7*r.float()
			}
			g := 0.0
			if grain > 0 {
				g = grainLvl * grain
				grain -= grainStep
			}
			s *= g
		}
		if l.Tremolo > 0 {
			s *= 1 - l.TremoloDepth*(0.5+0.5*math.Sin(2*math.Pi*l.Tremolo*t))
		}
		out[start+i] += s * gain * envelope(l, t*1, dur)
	}
}

// envelope is the layer's level at time t of a dur-long layer.
func envelope(l Layer, t, dur float64) float64 {
	e := 1.0
	switch {
	case l.Attack > 0 && t < l.Attack:
		e = t / l.Attack
	case l.Decay > 0:
		k := (t - l.Attack) / (l.Decay / 4)
		e = l.Sustain + (1-l.Sustain)*math.Exp(-k)
	}
	if l.Release > 0 && t > dur-l.Release {
		e *= math.Max(0, (dur-t)/l.Release)
	}
	// a 1 ms fade at both ends: no clicks
	edge := 0.001
	if t < edge && l.Attack == 0 {
		e *= t / edge
	}
	if dur-t < edge {
		e *= math.Max(0, (dur-t)/edge)
	}
	return e
}

func onePole(fc float64) float64 {
	if fc <= 0 {
		return 0
	}
	return 1 - math.Exp(-2*math.Pi*fc/Rate)
}

// biquad is an RBJ band-pass (0 dB peak).
type biquad struct{ b0, b2, a1, a2, x1, x2, y1, y2 float64 }

func (b *biquad) bandPass(fc, q float64) {
	fc = math.Min(fc, Rate*0.45)
	w := 2 * math.Pi * fc / Rate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha
	b.b0, b.b2 = alpha/a0, -alpha/a0
	b.a1, b.a2 = -2*math.Cos(w)/a0, (1-alpha)/a0
}

func (b *biquad) step(x float64) float64 {
	y := b.b0*x + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1 = b.x1, x
	b.y2, b.y1 = b.y1, y
	return y
}

// loopFix folds the last fade samples into the head with an equal-power
// crossfade, so the result loops without a seam.
func loopFix(x []float64, fade int) []float64 {
	n := len(x) - fade
	if fade <= 0 || n <= fade {
		return x
	}
	out := make([]float64, n)
	copy(out, x[:n])
	for i := 0; i < fade; i++ {
		k := float64(i) / float64(fade)
		out[i] = x[i]*math.Sin(k*math.Pi/2) + x[n+i]*math.Cos(k*math.Pi/2)
	}
	return out
}

// LoopFix makes 16-bit PCM loop seamlessly by crossfading its last fade
// samples into its head (audiogen -loopfix for imported loops).
func LoopFix(pcm []int16, fade int) []int16 {
	x := make([]float64, len(pcm))
	for i, v := range pcm {
		x[i] = float64(v) / 32768
	}
	y := loopFix(x, fade)
	out := make([]int16, len(y))
	for i, v := range y {
		out[i] = int16(math.Max(-32768, math.Min(32767, math.Round(v*32768))))
	}
	return out
}

func normalize(x []float64, peak float64) {
	m := 0.0
	for _, v := range x {
		m = math.Max(m, math.Abs(v))
	}
	if m == 0 {
		return
	}
	k := peak / m
	for i := range x {
		x[i] *= k
	}
}

// Quantize converts to 16-bit with seeded triangular dither.
func Quantize(x []float64, seed uint64) []int16 {
	r := &rng{s: seed ^ 0xD1B54A32D192ED03}
	out := make([]int16, len(x))
	for i, v := range x {
		d := (r.float() - r.float()) / 32768
		out[i] = int16(math.Max(-32768, math.Min(32767, math.Round((v+d)*32767))))
	}
	return out
}
