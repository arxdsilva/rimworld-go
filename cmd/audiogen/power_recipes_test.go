package main

import (
	"math"
	"strings"
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// PLAY-021 electricity sounds: five quiet machine loops and three
// one-shots, all made in code.
var powerSoundWant = map[string]struct {
	loop        bool
	minLen, max float64
	bus         string
}{
	"amb.generator_wood": {true, 0.3, 1, audiomanifest.BusAmbience},
	"amb.generator_geo":  {true, 0.3, 1, audiomanifest.BusAmbience},
	"amb.wind_turbine":   {true, 0.3, 1, audiomanifest.BusAmbience},
	"amb.heater":         {true, 0.3, 1, audiomanifest.BusAmbience},
	"amb.cooler":         {true, 0.25, 1, audiomanifest.BusAmbience},
	"sfx.switch_click":   {false, 0.04, 0.2, audiomanifest.BusEffects},
	"sfx.short_circuit":  {false, 0.25, 1, audiomanifest.BusEffects},
	"ui.solar_flare":     {false, 0.4, 1.4, audiomanifest.BusUI},
}

// powerAudioBudget is the most the eight sounds may weigh together (WAV,
// 48 kHz mono 16-bit, headers included): the repo's audio budget (20 MB then, 30 MB since 2026-10-29) was
// nearly spent, with about 290 KB left when these were made.
const powerAudioBudget = 280_000

func TestPowerRecipesDeterministicAndListed(t *testing.T) {
	t.Parallel()
	seen, bytes := 0, 0
	for _, snd := range sounds {
		w, ok := powerSoundWant[snd.recipe.ID]
		if !ok {
			continue
		}
		seen++
		a, b := sfxsynth.Render(snd.recipe, 0), sfxsynth.Render(snd.recipe, 0)
		if len(a) == 0 {
			t.Fatalf("%s renders nothing", snd.recipe.ID)
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s differs between renders at sample %d", snd.recipe.ID, i)
			}
		}
		if got := snd.recipe.Length(); got < w.minLen || got > w.max {
			t.Fatalf("%s is %.2f s, want %.2f..%.2f", snd.recipe.ID, got, w.minLen, w.max)
		}
		if (snd.recipe.Loop > 0) != w.loop {
			t.Fatalf("%s loop = %v, want %v", snd.recipe.ID, snd.recipe.Loop > 0, w.loop)
		}
		for v := 0; v < max(snd.recipe.Variants, 1); v++ {
			bytes += 2*len(sfxsynth.Render(snd.recipe, v)) + 44
		}
	}
	if seen != len(powerSoundWant) {
		t.Fatalf("found %d of %d power recipes", seen, len(powerSoundWant))
	}
	if bytes > powerAudioBudget {
		t.Fatalf("the power sounds weigh %d bytes, over %d", bytes, powerAudioBudget)
	}
	dir := t.TempDir()
	if _, err := generate(dir); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id, w := range powerSoundWant {
		e, ok := m.Lookup(id)
		if !ok {
			t.Fatalf("%s missing from the sound list", id)
		}
		if e.Bus != w.bus || e.Loop != w.loop || e.Source != audiomanifest.SourceProcedural {
			t.Fatalf("%s listed as %+v, want bus %s loop %v procedural", id, e, w.bus, w.loop)
		}
	}
}

// rmsOf is the root-mean-square level of x[from:to].
func rmsOf(x []float64, from, to int) float64 {
	sum := 0.0
	for _, f := range x[from:to] {
		sum += f * f
	}
	return math.Sqrt(sum / float64(to-from))
}

// A loop is heard over and over, so its seam must not dip or burst: the
// level of the 30 ms window across the join is as typical as the level of
// any other 30 ms window of the loop (a beat on the seam is fine when the
// other beats are as loud).
func TestPowerLoopsHaveNoSeam(t *testing.T) {
	t.Parallel()
	win, hop := sfxsynth.Rate*30/1000, sfxsynth.Rate*10/1000
	for _, snd := range sounds {
		if _, ok := powerSoundWant[snd.recipe.ID]; !ok || snd.recipe.Loop <= 0 {
			continue
		}
		x := sfxsynth.Render(snd.recipe, 0)
		if want := int(math.Round(snd.recipe.Loop * sfxsynth.Rate)); max(len(x)-want, want-len(x)) > 1 {
			t.Fatalf("%s has %d samples, want one loop of %d", snd.recipe.ID, len(x), want)
		}
		if whole := rmsOf(x, 0, len(x)); whole < 0.02 {
			t.Errorf("%s is nearly silent (rms %.4f)", snd.recipe.ID, whole)
		}
		lo, hi := math.Inf(1), 0.0
		for at := 0; at+win <= len(x); at += hop {
			r := rmsOf(x, at, at+win)
			lo, hi = math.Min(lo, r), math.Max(hi, r)
		}
		seam := append(append([]float64{}, x[len(x)-win/2:]...), x[:win/2]...)
		if near := rmsOf(seam, 0, len(seam)); near < lo*0.7 || near > hi*1.3 {
			t.Errorf("%s: the seam is at %.3f, the rest of the loop between %.3f and %.3f", snd.recipe.ID, near, lo, hi)
		}
	}
}

// The heater and cooler hum sits under the generator's engine, the loudest
// of the machines (the generator's recipe is mostly bass, so its list
// volume is high for a bed that is heard at about a campfire's level).
func TestPowerLoopVolumesAreOrdered(t *testing.T) {
	t.Parallel()
	vol := map[string]float64{}
	for _, snd := range sounds {
		if _, ok := powerSoundWant[snd.recipe.ID]; ok {
			vol[snd.recipe.ID] = snd.volume
		}
	}
	if vol["amb.heater"] > vol["amb.generator_wood"] || vol["amb.cooler"] > vol["amb.generator_wood"] {
		t.Fatalf("heater and cooler hum must sit under the generator: %v", vol)
	}
	for id := range powerSoundWant {
		if !strings.Contains(id, ".") || vol[id] <= 0 {
			t.Fatalf("%s has no volume", id)
		}
	}
}
