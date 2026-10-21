package ebitenaudio

import (
	"bytes"
	"io"
	"math"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

// sampleRate is the device rate and the rate of every sound file.
const sampleRate = 48000

// panGains are the left and right channel gains for pan (-1 left .. 1
// right): the near side stays at full level, the far side drops to 0.
func panGains(pan float64) (l, r float64) {
	pan = clamp(pan, -1, 1)
	return math.Min(1, 1-pan), math.Min(1, 1+pan)
}

func clamp(v, lo, hi float64) float64 {
	if v != v { // NaN
		return lo
	}
	return math.Max(lo, math.Min(hi, v))
}

func scale(s int16, g float64) int16 {
	return int16(clamp(math.Round(float64(s)*g), math.MinInt16, math.MaxInt16))
}

// stereoBytes is mono pcm as 16-bit little-endian stereo with pan baked in.
func stereoBytes(pcm []int16, pan float64) []byte {
	l, r := panGains(pan)
	out := make([]byte, 4*len(pcm))
	for i, s := range pcm {
		putFrame(out[4*i:], scale(s, l), scale(s, r))
	}
	return out
}

func putFrame(b []byte, l, r int16) {
	b[0], b[1] = byte(uint16(l)), byte(uint16(l)>>8)
	b[2], b[3] = byte(uint16(r)), byte(uint16(r)>>8)
}

// atomicPan is a pan written by the game loop and read by the audio
// goroutine.
type atomicPan struct{ bits atomic.Uint64 }

func (p *atomicPan) set(v float64) { p.bits.Store(math.Float64bits(clamp(v, -1, 1))) }
func (p *atomicPan) get() float64  { return math.Float64frombits(p.bits.Load()) }

// loopReader plays mono pcm forever as 16-bit stereo, panned by pan
// (read once per Read). It never returns an error.
type loopReader struct {
	pcm   []int16
	pos   int
	pan   atomicPan
	frame [4]byte
	off   int // bytes of frame already handed out
}

func newLoopReader(pcm []int16, pan float64) *loopReader {
	lr := &loopReader{pcm: pcm, off: 4}
	lr.pan.set(pan)
	return lr
}

func (lr *loopReader) Read(p []byte) (int, error) {
	if len(lr.pcm) == 0 {
		return 0, io.EOF
	}
	l, r := panGains(lr.pan.get())
	n := 0
	for n < len(p) {
		if lr.off == 4 {
			s := lr.pcm[lr.pos]
			lr.pos = (lr.pos + 1) % len(lr.pcm)
			putFrame(lr.frame[:], scale(s, l), scale(s, r))
			lr.off = 0
		}
		c := copy(p[n:], lr.frame[lr.off:])
		n += c
		lr.off += c
	}
	return n, nil
}

// decodeOGG opens an Ogg/Vorbis file as a 48 kHz 16-bit stereo stream
// and its length in bytes.
func decodeOGG(data []byte) (io.ReadSeeker, int64, error) {
	s, err := vorbis.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	return s, s.Length(), nil
}

// splitmix is a small deterministic generator for variant picks and
// music gaps (presentation only, never the simulation's RNG).
type splitmix struct{ s uint64 }

func (r *splitmix) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// intn is uniform in [0, n); n must be > 0.
func (r *splitmix) intn(n int) int { return int(r.next() % uint64(n)) }
