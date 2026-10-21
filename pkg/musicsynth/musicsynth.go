// Package musicsynth renders original music from a score: notes played by
// a few synthesized instruments (plucked guitar, string pads, violins,
// horn, bass, frame drum, taiko, shaker), mixed in stereo through a small
// reverb. Output is a pure function of the score and seed, so a track is
// byte-identical on every machine (cmd/audiogen -music).
package musicsynth

import (
	"math"
)

// Rate is the sample rate of rendered music.
const Rate = 48000

// Inst is an instrument.
type Inst int

// Instruments.
const (
	Guitar  Inst = iota // plucked string (Karplus-Strong)
	Pad                 // soft string pad, slow swell
	Strings             // bowed violins/cellos, faster attack, vibrato
	Tremolo             // strings with a fast amplitude tremolo
	Horn                // warm brass
	Bass                // round low bass
	Frame               // frame drum
	Taiko               // big low drum
	Shaker              // short high noise
)

// Note is one event of a score. Start and Dur are seconds; Pitch is a
// MIDI note number (ignored by drums); Vel is 0..1; Pan is -1..1.
type Note struct {
	Inst   Inst
	Start  float64
	Dur    float64
	Pitch  int
	Vel    float64
	Pan    float64
	Reverb float64 // send level 0..1 (0 means the instrument's default)
}

// Score is a whole track.
type Score struct {
	Length float64 // seconds
	Notes  []Note
	Reverb float64 // overall wet level
	Seed   uint64
}

func hz(pitch int) float64 { return 440 * math.Pow(2, float64(pitch-69)/12) }

type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (r *rng) float() float64 { return float64(r.next()>>11) / (1 << 53) }
func (r *rng) bi() float64    { return 2*r.float() - 1 }

// tail is how long an instrument rings past its note, seconds.
func tail(i Inst) float64 {
	switch i {
	case Guitar:
		return 2.5
	case Pad:
		return 1.6
	case Strings, Tremolo:
		return 0.8
	case Horn:
		return 0.4
	case Bass:
		return 0.3
	case Taiko:
		return 1.4
	case Frame:
		return 0.6
	}
	return 0.1
}

func defaultSend(i Inst) float64 {
	switch i {
	case Pad, Strings, Tremolo, Horn:
		return 0.55
	case Guitar:
		return 0.35
	case Taiko, Frame:
		return 0.25
	}
	return 0.1
}

// Render mixes the score to stereo float samples (unnormalized).
func Render(sc Score) (l, r []float64) {
	n := int(sc.Length * Rate)
	l, r = make([]float64, n), make([]float64, n)
	wl, wr := make([]float64, n), make([]float64, n)
	for i, nt := range sc.Notes {
		voice := renderNote(nt, &rng{s: sc.Seed*0x100000001B3 + uint64(i)*0x9E37 + 7})
		gl, gr := math.Cos((nt.Pan+1)*math.Pi/4), math.Sin((nt.Pan+1)*math.Pi/4)
		send := nt.Reverb
		if send == 0 {
			send = defaultSend(nt.Inst)
		}
		start := int(nt.Start * Rate)
		for k, v := range voice {
			j := start + k
			if j < 0 {
				continue
			}
			j %= n // past the end wraps to the start: tails ring into a loop's head
			l[j] += v * gl
			r[j] += v * gr
			wl[j] += v * gl * send
			wr[j] += v * gr * send
		}
	}
	if sc.Reverb > 0 {
		rl, rr := reverb(wl, wr)
		for i := range l {
			l[i] += rl[i] * sc.Reverb
			r[i] += rr[i] * sc.Reverb
		}
	}
	return l, r
}

func renderNote(nt Note, rg *rng) []float64 {
	total := nt.Dur + tail(nt.Inst)
	n := int(total * Rate)
	out := make([]float64, n)
	f := hz(nt.Pitch)
	vel := nt.Vel
	switch nt.Inst {
	case Guitar:
		pluck(out, f, vel, nt.Dur, rg)
	case Pad, Strings, Tremolo:
		bowed(out, f, vel, nt, rg)
	case Horn:
		horn(out, f, vel, nt.Dur)
	case Bass:
		bass(out, f, vel, nt.Dur)
	case Frame:
		drum(out, 95, 52, 0.35, 0.12, 700, vel, rg)
	case Taiko:
		drum(out, 62, 38, 0.9, 0.2, 400, vel, rg)
	case Shaker:
		shaker(out, vel, rg)
	}
	return out
}

// pluck is a Karplus-Strong string with a short damped release after dur.
func pluck(out []float64, f, vel, dur float64, rg *rng) {
	period := int(math.Max(2, Rate/f))
	buf := make([]float64, period)
	prev := 0.0
	for i := range buf { // a slightly lowpassed noise burst: a softer, fingered attack
		v := rg.bi()
		prev = 0.6*prev + 0.4*v
		buf[i] = prev
	}
	decay := 0.996
	end := int(dur * Rate)
	p := 0
	lp := 0.0
	for i := range out {
		if i == end {
			decay = 0.97 // the string is damped after the note
		}
		a, b := buf[p], buf[(p+1)%period]
		v := decay * 0.5 * (a + b)
		buf[p] = v
		p = (p + 1) % period
		lp += 0.35 * (a - lp) // body: soften the brightness
		out[i] = lp * vel * 0.9
	}
}

func saw(ph float64) float64 { return 2*(ph-math.Floor(ph)) - 1 }

// bowed are detuned saws through a lowpass, with a swell, vibrato and
// (Tremolo) a fast amplitude pulse.
func bowed(out []float64, f, vel float64, nt Note, rg *rng) {
	attack, cutoff, vib := 0.9, 1400.0, 0.0
	switch nt.Inst {
	case Strings:
		attack, cutoff, vib = 0.18, 2600, 0.004
	case Tremolo:
		attack, cutoff, vib = 0.12, 3000, 0.003
	}
	release := tail(nt.Inst)
	detune := []float64{-0.006, 0, 0.0065}
	phase := []float64{rg.float(), rg.float(), rg.float()}
	a := 1 - math.Exp(-2*math.Pi*cutoff/Rate)
	lp1, lp2 := 0.0, 0.0
	for i := range out {
		t := float64(i) / Rate
		fv := f * (1 + vib*math.Sin(2*math.Pi*5.2*t))
		s := 0.0
		for k, d := range detune {
			phase[k] += fv * (1 + d) / Rate
			s += saw(phase[k])
		}
		s /= 3
		lp1 += a * (s - lp1)
		lp2 += a * (lp1 - lp2)
		env := math.Min(1, t/attack)
		if t > nt.Dur {
			env *= math.Max(0, 1-(t-nt.Dur)/release)
		}
		if nt.Inst == Tremolo {
			env *= 0.55 + 0.45*math.Sin(2*math.Pi*9*t)
		}
		out[i] = lp2 * env * vel * 0.7
	}
}

// horn is a saw-and-square brass tone whose filter opens with the attack.
func horn(out []float64, f, vel, dur float64) {
	ph := 0.0
	lp := 0.0
	for i := range out {
		t := float64(i) / Rate
		fv := f * (1 + 0.003*math.Sin(2*math.Pi*4.8*t)*math.Min(1, t/0.4))
		ph += fv / Rate
		sq := -1.0
		if ph-math.Floor(ph) < 0.5 {
			sq = 1
		}
		s := 0.7*saw(ph) + 0.3*sq
		env := math.Min(1, t/0.08)
		if t > dur {
			env *= math.Max(0, 1-(t-dur)/0.4)
		}
		cut := 400 + 900*env*vel
		lp += (1 - math.Exp(-2*math.Pi*cut/Rate)) * (s - lp)
		out[i] = lp * env * vel * 0.8
	}
}

// bass is a triangle with its octave below, gently filtered.
func bass(out []float64, f, vel, dur float64) {
	ph := 0.0
	for i := range out {
		t := float64(i) / Rate
		ph += f / Rate
		x := ph - math.Floor(ph)
		tri := 4*math.Abs(x-0.5) - 1
		sub := math.Sin(math.Pi * ph) // an octave below
		env := math.Min(1, t/0.01) * math.Exp(-t/2.5)
		if t > dur {
			env *= math.Max(0, 1-(t-dur)/0.3)
		}
		out[i] = (0.6*tri + 0.5*sub) * env * vel * 0.8
	}
}

// drum is a pitched membrane thump plus a short filtered noise slap.
func drum(out []float64, f0, f1, decay, noiseDecay, noiseCut, vel float64, rg *rng) {
	ph, brown, lp := 0.0, 0.0, 0.0
	a := 1 - math.Exp(-2*math.Pi*noiseCut/Rate)
	for i := range out {
		t := float64(i) / Rate
		f := f1 + (f0-f1)*math.Exp(-t/0.06)
		ph += f / Rate
		body := math.Sin(2*math.Pi*ph) * math.Exp(-t/decay*3)
		brown = (brown + 0.02*rg.bi()) / 1.02
		lp += a * (brown*3.5 - lp)
		slap := lp * math.Exp(-t/noiseDecay*3)
		out[i] = (body + 0.6*slap) * vel * math.Min(1, t/0.002)
	}
}

func shaker(out []float64, vel float64, rg *rng) {
	hp := 0.0
	for i := range out {
		t := float64(i) / Rate
		w := rg.bi()
		hp += 0.5 * (w - hp)
		out[i] = (w - hp) * math.Exp(-t/0.035) * math.Min(1, t/0.004) * vel * 0.5
	}
}

// reverb is a small Schroeder reverb: parallel feedback combs into series
// allpasses, with different delays per side for width.
func reverb(inL, inR []float64) (l, r []float64) {
	return schroeder(inL, []int{1557, 1617, 1491, 1422}, []int{225, 556}),
		schroeder(inR, []int{1580, 1640, 1514, 1445}, []int{248, 579})
}

func schroeder(in []float64, combs, alls []int) []float64 {
	n := len(in)
	out := make([]float64, n)
	for _, d := range combs {
		d = d * Rate / 44100
		buf := make([]float64, d)
		p, damp := 0, 0.0
		for i := 0; i < n; i++ {
			y := buf[p]
			damp = 0.8*y + 0.2*damp
			buf[p] = in[i] + damp*0.84
			p = (p + 1) % d
			out[i] += y * 0.25
		}
	}
	for _, d := range alls {
		d = d * Rate / 44100
		buf := make([]float64, d)
		p := 0
		for i := 0; i < n; i++ {
			b := buf[p]
			y := -out[i] + b
			buf[p] = out[i] + b*0.5
			p = (p + 1) % d
			out[i] = y
		}
	}
	return out
}

// Master normalizes the mix to rms (linear, e.g. 0.12 ≈ -18 dBFS) and
// soft-limits peaks below ceiling, returning interleaved 16-bit stereo.
func Master(l, r []float64, rms, ceiling float64, seed uint64) []int16 {
	sum := 0.0
	for i := range l {
		sum += l[i]*l[i] + r[i]*r[i]
	}
	cur := math.Sqrt(sum / float64(2*max(len(l), 1)))
	k := 1.0
	if cur > 0 {
		k = rms / cur
	}
	rg := &rng{s: seed ^ 0xD1B54A32D192ED03}
	out := make([]int16, 2*len(l))
	for i := range l {
		for c, v := range [2]float64{l[i], r[i]} {
			x := ceiling * math.Tanh(v*k/ceiling)
			d := (rg.float() - rg.float()) / 32768
			out[2*i+c] = int16(math.Max(-32768, math.Min(32767, math.Round((x+d)*32767))))
		}
	}
	return out
}
