package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"rimworld-go/pkg/audiomanifest"
	s "rimworld-go/pkg/sfxsynth"
)

// Placeholder animal voices (SND-003): the fallback layer under the
// per-species takes. Ids are animal.<group>.<event>, animal.generic.<event>
// and animal.burst; each group and event has 2 variants. Lengths follow
// the plan: call 0.8-1.6 s, attack 0.5 s, hurt 0.35 s, die 1.0 s. Recipes
// are starting points tuned by ear, like the rest of recipes.go.
//
// They ship as Ogg Vorbis (about 0.5 MB for all of them, against 6 MB of
// WAV) so the ElevenLabs species takes fit the audio budget. Like the
// music, they are encoded by a separate command, audiogen -animals (needs
// oggenc), and the Ogg files are committed; the default run (make audio)
// neither needs oggenc nor touches them, and keeps their sound list entries.

// animalGroups are the voice groups of the fallback chain (the renderer's
// voiceGroup table maps species onto them).
var animalGroups = []string{
	"small_mammal", "large_mammal", "canid", "felid", "bovid",
	"bird_small", "bird_large", "reptile", "insect",
}

var animalVolume = map[string]float64{"call": 0.7, "attack": 0.85, "hurt": 0.85, "die": 1}

// animalVoice builds the layers of one event of a group.
type animalVoice func(event string) []s.Layer

var animalVoices = map[string]animalVoice{
	"small_mammal": smallMammal, "large_mammal": largeMammal, "canid": canid,
	"felid": felid, "bovid": bovid, "bird_small": birdSmall, "bird_large": birdLarge,
	"reptile": reptile, "insect": insect, "generic": generic,
}

// animalSounds is every group x event recipe, the generic set and the
// burst. Seeds are 5000 + 10*group + event, 5099 for the burst.
func animalSounds() []sound {
	var out []sound
	for gi, g := range append(append([]string{}, animalGroups...), "generic") {
		for ei, ev := range []string{"call", "attack", "hurt", "die"} {
			out = append(out, sound{s.Recipe{
				ID: fmt.Sprintf("animal.%s.%s", g, ev), Seed: uint64(5000 + 10*gi + ei),
				Variants: 2, Jitter: 0.08, Layers: animalVoices[g](ev),
			}, animalVolume[ev]})
		}
	}
	return append(out, sound{s.Recipe{ID: "animal.burst", Seed: 5099, Layers: []s.Layer{
		{Wave: s.Sine, Freq: 90, FreqEnd: 38, Dur: 0.4, Decay: 0.18},
		{Wave: s.Brown, Dur: 0.5, LowPass: 350, Attack: 0.004, Decay: 0.25, Gain: 0.8},
		{Wave: s.White, Start: 0.05, Dur: 0.85, HighPass: 1500, Grains: 90, GrainLen: 0.01, Decay: 0.4, Gain: 0.35},
	}}, 1})
}

// vox is a voiced cry: a saw sliding f0 to f1 through a formant band.
func vox(f0, f1, at, dur, formant, gain float64) s.Layer {
	return s.Layer{Wave: s.Saw, Freq: f0, FreqEnd: f1, Start: at, Dur: dur, BandPass: formant, Q: 3,
		Attack: 0.03, Release: min(0.15, dur/3), Gain: gain}
}

// shape sets a layer's attack and release.
func shape(l s.Layer, attack, release float64) s.Layer {
	l.Attack, l.Release = attack, release
	return l
}

// wobble adds an amplitude wobble (a growl or a vibrato-ish warble).
func wobble(l s.Layer, rate, depth float64) s.Layer {
	l.Tremolo, l.TremoloDepth = rate, depth
	return l
}

// chirp is a short pure pitch sweep.
func chirp(f0, f1, at, dur, gain float64) s.Layer {
	return s.Layer{Wave: s.Sine, Freq: f0, FreqEnd: f1, Start: at, Dur: dur, Attack: 0.004, Release: dur / 3, Gain: gain}
}

// rasp is band-passed noise: a hiss, snort or breath.
func rasp(at, dur, centre, gain float64) s.Layer {
	return s.Layer{Wave: s.White, Start: at, Dur: dur, BandPass: centre, Q: 1.5, Attack: dur / 6, Release: dur / 3, Gain: gain}
}

// tick is a dry click.
func tick(at, gain float64) s.Layer {
	return s.Layer{Wave: s.White, Start: at, Dur: 0.02, HighPass: 3500, Attack: 0.001, Decay: 0.008, Gain: gain}
}

// sub is a low body under a voice.
func sub(f0, f1, dur, gain float64) s.Layer {
	return s.Layer{Wave: s.Sine, Freq: f0, FreqEnd: f1, Dur: dur, Attack: 0.03, Release: dur / 3, Gain: gain}
}

// canid: call is a howl sweep, attack a snarl plus a click, hurt a yelp,
// die a falling cry.
func canid(ev string) []s.Layer {
	switch ev {
	case "call":
		return []s.Layer{
			wobble(vox(350, 620, 0, 0.6, 900, 1), 6, 0.15),
			wobble(shape(vox(620, 420, 0.55, 1.0, 900, 1), 0.05, 0.4), 6, 0.2),
			s.Layer{Wave: s.Triangle, Freq: 700, FreqEnd: 1200, Dur: 0.6, Attack: 0.15, Release: 0.1, Gain: 0.2},
		}
	case "attack":
		return []s.Layer{
			wobble(vox(95, 80, 0, 0.45, 500, 1), 40, 0.8),
			rasp(0, 0.45, 1500, 0.4),
			tick(0.48, 0.8),
		}
	case "hurt":
		return []s.Layer{vox(950, 520, 0, 0.35, 1400, 1), chirp(1900, 1000, 0, 0.3, 0.2)}
	}
	return []s.Layer{wobble(vox(720, 170, 0, 1.0, 1000, 1), 8, 0.25), sub(300, 90, 1.0, 0.3)}
}

// felid: call is a yowl, attack a hiss and spit, hurt a shriek.
func felid(ev string) []s.Layer {
	switch ev {
	case "call":
		return []s.Layer{
			vox(500, 900, 0, 0.5, 1500, 1),
			wobble(shape(vox(900, 450, 0.45, 0.95, 1500, 1), 0.05, 0.3), 5, 0.2),
		}
	case "attack":
		return []s.Layer{rasp(0, 0.35, 5000, 0.9), wobble(vox(700, 400, 0.2, 0.3, 1400, 0.8), 30, 0.6), tick(0.48, 0.5)}
	case "hurt":
		return []s.Layer{vox(1250, 800, 0, 0.35, 1800, 1), chirp(2400, 1600, 0, 0.3, 0.15)}
	}
	return []s.Layer{wobble(vox(1000, 250, 0, 1.0, 1400, 1), 7, 0.2)}
}

// bovid: call is a low formant moo, attack a snort and bellow.
func bovid(ev string) []s.Layer {
	switch ev {
	case "call":
		return []s.Layer{
			shape(vox(120, 98, 0, 1.6, 420, 1), 0.2, 0.5),
			shape(vox(120, 98, 0, 1.6, 1100, 0.5), 0.25, 0.5),
			shape(sub(60, 50, 1.6, 0.4), 0.2, 0.5),
		}
	case "attack":
		return []s.Layer{rasp(0, 0.25, 600, 0.9), vox(135, 100, 0.1, 0.4, 600, 1)}
	case "hurt":
		return []s.Layer{vox(160, 115, 0, 0.35, 650, 1), vox(160, 115, 0, 0.35, 1200, 0.4)}
	}
	return []s.Layer{vox(150, 55, 0, 1.0, 500, 1), vox(150, 55, 0, 1.0, 1000, 0.4), sub(70, 35, 1.0, 0.4)}
}

// smallMammal: squeaks.
func smallMammal(ev string) []s.Layer {
	sq := func(at, f0, f1, dur float64) s.Layer {
		l := chirp(f0, f1, at, dur, 1)
		l.Wave = s.Triangle
		return l
	}
	switch ev {
	case "call":
		return []s.Layer{sq(0, 2400, 3200, 0.12), sq(0.22, 2500, 3300, 0.12), sq(0.44, 2400, 3100, 0.12), sq(0.65, 2600, 2000, 0.15)}
	case "attack":
		return []s.Layer{rasp(0, 0.35, 4500, 0.6), sq(0.3, 3000, 3800, 0.2)}
	case "hurt":
		return []s.Layer{sq(0, 3500, 2000, 0.35)}
	}
	return []s.Layer{wobble(sq(0, 3000, 700, 1.0), 14, 0.4)}
}

// largeMammal: grunts and bellows.
func largeMammal(ev string) []s.Layer {
	switch ev {
	case "call":
		return []s.Layer{
			shape(vox(100, 140, 0, 0.5, 500, 1), 0.08, 0.1),
			shape(vox(140, 70, 0.45, 0.95, 500, 1), 0.05, 0.4),
			sub(55, 45, 1.4, 0.4),
		}
	case "attack":
		return []s.Layer{wobble(vox(150, 110, 0, 0.5, 450, 1), 25, 0.5), rasp(0, 0.4, 350, 0.5)}
	case "hurt":
		return []s.Layer{vox(165, 110, 0, 0.35, 600, 1), rasp(0, 0.2, 400, 0.4)}
	}
	return []s.Layer{vox(180, 50, 0, 1.0, 450, 1), sub(80, 35, 1.0, 0.5)}
}

// birdSmall: chirps and twitters.
func birdSmall(ev string) []s.Layer {
	switch ev {
	case "call":
		out := []s.Layer{}
		for i := range 4 {
			out = append(out, chirp(3000, 4200, float64(i)*0.15, 0.08, 1))
		}
		return append(out, wobble(chirp(3500, 4500, 0.7, 0.3, 0.8), 30, 0.8))
	case "attack":
		out := []s.Layer{}
		for i := range 4 {
			out = append(out, chirp(4200, 3000, float64(i)*0.1, 0.08, 1))
		}
		return append(out, rasp(0.3, 0.2, 3500, 0.5))
	case "hurt":
		return []s.Layer{chirp(4000, 2500, 0, 0.35, 1)}
	}
	return []s.Layer{wobble(chirp(4500, 1200, 0, 1.0, 1), 20, 0.5)}
}

// birdLarge: caws and squawks.
func birdLarge(ev string) []s.Layer {
	caw := func(at, f0, f1, dur float64) s.Layer {
		l := vox(f0, f1, at, dur, 1500, 1)
		l.Q = 2
		return l
	}
	switch ev {
	case "call":
		return []s.Layer{caw(0, 700, 450, 0.3), caw(0.45, 720, 440, 0.3), caw(0.9, 700, 400, 0.3)}
	case "attack":
		return []s.Layer{caw(0, 1500, 900, 0.5), rasp(0, 0.5, 3000, 0.5)}
	case "hurt":
		return []s.Layer{caw(0, 950, 600, 0.35), rasp(0, 0.2, 2500, 0.3)}
	}
	return []s.Layer{wobble(caw(0, 900, 200, 1.0), 12, 0.35)}
}

// reptile: hisses.
func reptile(ev string) []s.Layer {
	hiss := func(at, dur, gain float64) s.Layer {
		return s.Layer{Wave: s.White, Start: at, Dur: dur, HighPass: 4000, Attack: dur / 5, Release: dur / 2, Gain: gain}
	}
	switch ev {
	case "call":
		return []s.Layer{hiss(0, 1.2, 1), wobble(s.Layer{Wave: s.Saw, Freq: 60, Dur: 1.2, LowPass: 400, Attack: 0.3, Release: 0.5, Gain: 0.4}, 30, 0.6)}
	case "attack":
		return []s.Layer{hiss(0, 0.45, 1), tick(0.48, 0.7)}
	case "hurt":
		return []s.Layer{hiss(0, 0.35, 0.9), vox(220, 150, 0, 0.3, 600, 0.4)}
	}
	return []s.Layer{hiss(0, 1.0, 0.9), vox(120, 50, 0, 1.0, 400, 0.5)}
}

// insect: chitters and clicks.
func insect(ev string) []s.Layer {
	chit := func(at, dur, rate, gain float64) s.Layer {
		return s.Layer{Wave: s.White, Start: at, Dur: dur, BandPass: 5000, Q: 6, Grains: rate, GrainLen: 0.01, Release: dur / 4, Gain: gain}
	}
	switch ev {
	case "call":
		return []s.Layer{chit(0, 1.0, 40, 1), wobble(s.Layer{Wave: s.Square, Freq: 4000, Dur: 1.0, LowPass: 5000, Attack: 0.1, Release: 0.3, Gain: 0.15}, 50, 0.9)}
	case "attack":
		return []s.Layer{chit(0, 0.5, 80, 1), tick(0.1, 0.6), tick(0.3, 0.6)}
	case "hurt":
		return []s.Layer{chit(0, 0.35, 60, 0.9), chirp(5200, 3000, 0, 0.3, 0.2)}
	}
	return []s.Layer{wobble(s.Layer{Wave: s.Saw, Freq: 300, FreqEnd: 80, Dur: 1.0, LowPass: 1500, Release: 0.3, Gain: 0.6}, 60, 0.8), chit(0, 1.0, 40, 0.6)}
}

// generic: a mid-sized, plain animal for anything without a group.
func generic(ev string) []s.Layer {
	switch ev {
	case "call":
		return []s.Layer{shape(vox(300, 380, 0, 0.5, 900, 1), 0.05, 0.1), shape(vox(380, 250, 0.45, 0.65, 900, 1), 0.04, 0.3)}
	case "attack":
		return []s.Layer{wobble(vox(180, 140, 0, 0.5, 700, 1), 30, 0.6), rasp(0, 0.4, 1200, 0.4)}
	case "hurt":
		return []s.Layer{vox(600, 350, 0, 0.35, 1100, 1)}
	}
	return []s.Layer{wobble(vox(500, 130, 0, 1.0, 900, 1), 8, 0.25)}
}

// isAnimalPlaceholder reports whether id is one of the procedural animal
// voices this file owns.
func isAnimalPlaceholder(id string) bool {
	for _, a := range animalSounds() {
		if a.recipe.ID == id {
			return true
		}
	}
	return false
}

// oggQuality is oggenc's -q for the placeholders.
const oggQuality = "3"

// encoder turns a WAV file into an Ogg Vorbis file; seed fixes the
// stream serial so the output is the same on every run.
type encoder func(wav, ogg string, seed uint64) error

func oggenc(wav, out string, seed uint64) error {
	cmd := exec.Command("oggenc", "-Q", "-q", oggQuality, "-s", fmt.Sprint(seed%1_000_000+1), "-o", out, wav)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("oggenc %s: %v: %s", filepath.Base(out), err, b)
	}
	return nil
}

// animals renders the animal placeholders to Ogg with oggenc (audiogen
// -animals).
func animals(assets string) error {
	if _, err := exec.LookPath("oggenc"); err != nil {
		return fmt.Errorf("-animals needs oggenc (vorbis-tools) to encode Ogg: %w", err)
	}
	n, err := generateAnimals(assets, oggenc)
	if err == nil {
		fmt.Printf("audiogen: %d animal placeholder sounds written\n", n)
	}
	return err
}

// generateAnimals renders every animal recipe through a temporary WAV and
// enc into generated/audio/sfx/animals/*.ogg, and lists (or replaces) the
// entries in the sound list, leaving every other entry alone.
func generateAnimals(assets string, enc encoder) (int, error) {
	m, err := readManifest(assets)
	if err != nil {
		return 0, err
	}
	tmp, err := os.MkdirTemp("", "audiogen-animals-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	root := filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir))
	all := animalSounds()
	for _, snd := range all {
		rc := snd.recipe
		e := audiomanifest.Entry{ID: rc.ID, Bus: audiomanifest.BusOf(rc.ID), Volume: snd.volume,
			Source: audiomanifest.SourceProcedural, Recipe: rc.ID, Seed: rc.Seed, License: proceduralLicense}
		for v := 0; v < max(rc.Variants, 1); v++ {
			rel := fmt.Sprintf("%s_%d.ogg", fileStem(rc.ID), v)
			var buf bytes.Buffer
			pcm := s.Quantize(s.Render(rc, v), rc.Seed+uint64(v))
			if err := s.WriteWAV(&buf, pcm, s.Rate); err != nil {
				return 0, err
			}
			wav := filepath.Join(tmp, fmt.Sprintf("%s_%d.wav", rc.ID, v))
			if err := os.WriteFile(wav, buf.Bytes(), 0o644); err != nil {
				return 0, err
			}
			out := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return 0, err
			}
			if err := enc(wav, out, rc.Seed+uint64(v)); err != nil {
				return 0, err
			}
			e.Files = append(e.Files, rel)
		}
		replaced := false
		for i := range m.Sounds {
			if m.Sounds[i].ID == rc.ID {
				m.Sounds[i], replaced = e, true
			}
		}
		if !replaced {
			m.Sounds = append(m.Sounds, e)
		}
	}
	if err := m.Validate(); err != nil {
		return 0, err
	}
	return len(all), writeManifest(assets, m)
}
