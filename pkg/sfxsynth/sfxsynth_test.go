package sfxsynth

import (
	"bytes"
	"math"
	"testing"
)

func testRecipe() Recipe {
	return Recipe{ID: "t", Seed: 7, Variants: 3, Jitter: 0.05, Layers: []Layer{
		{Wave: Sine, Freq: 440, FreqEnd: 330, Dur: 0.2, Decay: 0.1},
		{Wave: White, Dur: 0.05, BandPass: 1500, Q: 2},
		{Wave: Brown, Dur: 0.2, LowPass: 400, Grains: 50},
	}}
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()
	a := Quantize(Render(testRecipe(), 1), 7)
	b := Quantize(Render(testRecipe(), 1), 7)
	if len(a) != len(b) {
		t.Fatal("length differs")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sample %d differs", i)
		}
	}
}

func TestVariantsDiffer(t *testing.T) {
	t.Parallel()
	a, b := Render(testRecipe(), 0), Render(testRecipe(), 2)
	same := len(a) == len(b)
	for i := 0; same && i < len(a); i++ {
		same = a[i] == b[i]
	}
	if same {
		t.Fatal("variants 0 and 2 are identical")
	}
}

func TestRenderNormalizedAndSilentEnds(t *testing.T) {
	t.Parallel()
	x := Render(testRecipe(), 0)
	peak := 0.0
	for _, v := range x {
		peak = math.Max(peak, math.Abs(v))
	}
	if want := math.Pow(10, -1.0/20); math.Abs(peak-want) > 1e-9 {
		t.Fatalf("peak %v, want %v", peak, want)
	}
	if math.Abs(x[0]) > 0.01 || math.Abs(x[len(x)-1]) > 0.01 {
		t.Fatalf("ends not faded: %v .. %v", x[0], x[len(x)-1])
	}
}

func TestLoopLengthAndSeam(t *testing.T) {
	t.Parallel()
	rc := Recipe{Seed: 3, Loop: 2, Layers: []Layer{{Wave: Brown, Dur: 3, LowPass: 600}}}
	x := Render(rc, 0)
	if len(x) != 2*Rate {
		t.Fatalf("loop length %d, want %d", len(x), 2*Rate)
	}
	// the wrap from the last sample to the first is no bigger than a
	// typical step inside the loop
	jump := math.Abs(x[0] - x[len(x)-1])
	typical := 0.0
	for i := 1; i < len(x); i++ {
		typical = math.Max(typical, math.Abs(x[i]-x[i-1]))
	}
	if jump > typical {
		t.Fatalf("seam jump %v > largest inner step %v", jump, typical)
	}
}

func TestWAVRoundTrip(t *testing.T) {
	t.Parallel()
	pcm := Quantize(Render(testRecipe(), 0), 1)
	var buf bytes.Buffer
	if err := WriteWAV(&buf, pcm, Rate); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 44+2*len(pcm) {
		t.Fatalf("wav size %d", buf.Len())
	}
	got, rate, ch, err := ReadWAV(&buf)
	if err != nil || rate != Rate || ch != 1 || len(got) != len(pcm) {
		t.Fatalf("read back rate %d ch %d len %d err %v", rate, ch, len(got), err)
	}
	for i := range pcm {
		if got[i] != pcm[i] {
			t.Fatalf("sample %d differs", i)
		}
	}
}
