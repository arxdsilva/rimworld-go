package musicsynth

import (
	"math"
	"testing"
)

func testScore() Score {
	var notes []Note
	for i, inst := range []Inst{Guitar, Pad, Strings, Tremolo, Horn, Bass, Frame, Taiko, Shaker} {
		notes = append(notes, Note{Inst: inst, Start: 0.2 * float64(i), Dur: 0.5, Pitch: 57 + i, Vel: 0.6, Pan: -0.5 + 0.1*float64(i)})
	}
	return Score{Length: 4, Notes: notes, Reverb: 0.3, Seed: 3}
}

func TestRenderDeterministicAndFinite(t *testing.T) {
	t.Parallel()
	l1, r1 := Render(testScore())
	l2, r2 := Render(testScore())
	if len(l1) != 4*Rate || len(r1) != len(l1) {
		t.Fatalf("length %d, want %d", len(l1), 4*Rate)
	}
	for i := range l1 {
		if l1[i] != l2[i] || r1[i] != r2[i] {
			t.Fatalf("sample %d differs between renders", i)
		}
		if math.IsNaN(l1[i]) || math.IsInf(l1[i], 0) || math.IsNaN(r1[i]) {
			t.Fatalf("sample %d is not finite", i)
		}
	}
}

func TestEveryInstrumentSounds(t *testing.T) {
	t.Parallel()
	for inst := Guitar; inst <= Shaker; inst++ {
		l, _ := Render(Score{Length: 2, Notes: []Note{{Inst: inst, Dur: 0.6, Pitch: 57, Vel: 0.8}}, Seed: 1})
		peak := 0.0
		for _, v := range l {
			peak = math.Max(peak, math.Abs(v))
		}
		if peak < 0.01 {
			t.Fatalf("instrument %d is silent (peak %v)", inst, peak)
		}
	}
}

func TestPanAndTailWrap(t *testing.T) {
	t.Parallel()
	l, r := Render(Score{Length: 1, Notes: []Note{{Inst: Pad, Start: 0.5, Dur: 0.5, Pitch: 60, Vel: 1, Pan: 1}}})
	el, er := 0.0, 0.0
	for i := range l {
		el += l[i] * l[i]
		er += r[i] * r[i]
	}
	if el > er*1e-6 {
		t.Fatalf("hard right pan leaks left: %v vs %v", el, er)
	}
	head := 0.0
	for _, v := range r[:Rate/4] {
		head += math.Abs(v)
	}
	if head == 0 {
		t.Fatal("a tail past the end should ring into the start (loops)")
	}
}

func TestMasterLimitsAndInterleaves(t *testing.T) {
	t.Parallel()
	l, r := Render(testScore())
	pcm := Master(l, r, 0.13, 0.89, 1)
	if len(pcm) != 2*len(l) {
		t.Fatalf("interleaved length %d, want %d", len(pcm), 2*len(l))
	}
	for _, s := range pcm {
		if s > 29491 || s < -29491 { // 0.9 of full scale
			t.Fatalf("sample %d over the ceiling", s)
		}
	}
}
