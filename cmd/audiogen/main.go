// Command audiogen renders the game's procedural sound effects
// (recipes.go, pkg/sfxsynth) to assets/generated/audio as 48 kHz mono
// 16-bit WAV and updates the sound list (assets/metadata/audio/audio.json),
// keeping every non-procedural entry. Output is byte-identical on every
// machine.
//
//	audiogen [-assets assets]          render + update the sound list (make audio)
//	audiogen -music                    compose the music to Ogg (make music; needs ffmpeg)
//	audiogen -animals                  render the animal placeholder voices to Ogg (needs oggenc)
//	audiogen -check                    validate the sound list against the files
//	audiogen -loopfix in.wav out.wav   make an imported loop seamless
//	audiogen -looping music/menu.ogg   print whether prompts.json marks it a loop
//	audiogen -register ambience/day.ogg -credits 400 -license "..."
//	                                   list an imported ElevenLabs take
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// budget is the most audio the repo carries (SND-002).
const budget = 20 << 20

const proceduralLicense = "original, made in code (CC0, this repo)"

func main() {
	assets := flag.String("assets", "assets", "assets directory")
	animal := flag.Bool("animals", false, "render the animal placeholder voices (animal_recipes.go) and encode Ogg with oggenc")
	check := flag.Bool("check", false, "validate the sound list against the files")
	loopfix := flag.Bool("loopfix", false, "make the WAV loop seamless: -loopfix in.wav out.wav")
	fade := flag.Float64("fade", 1, "-loopfix crossfade, seconds")
	music := flag.Bool("music", false, "compose the music (scores in music.go) and encode Ogg with ffmpeg")
	looping := flag.String("looping", "", "print true/false: is this prompts.json file a loop")
	reg := flag.String("register", "", "add this imported file (under generated/audio) to the sound list")
	credits := flag.Int("credits", 0, "-register: ElevenLabs credits the take cost")
	license := flag.String("license", "", "-register: the plan and terms the take was made under")
	reviewer := flag.String("reviewer", "arxdsilva", "-register: who listened and approved it")
	flag.Parse()
	var err error
	switch {
	case *music:
		err = composeAll(*assets)
	case *animal:
		err = animals(*assets)
	case *looping != "":
		var p prompt
		if p, err = promptFor(*assets, *looping); err == nil {
			fmt.Println(p.Loop)
		}
	case *reg != "":
		if *license == "" {
			err = errors.New("-register needs -license (the ElevenLabs plan and terms)")
			break
		}
		var e audiomanifest.Entry
		if e, err = register(*assets, *reg, *credits, *license, *reviewer); err == nil {
			fmt.Printf("audiogen: %s now has %d file(s), %d credits\n", e.ID, len(e.Files), e.Credits)
		}
	case *loopfix:
		if flag.NArg() != 2 {
			err = errors.New("-loopfix needs in.wav out.wav")
			break
		}
		err = loopFix(flag.Arg(0), flag.Arg(1), *fade)
	case *check:
		var n int
		var size int64
		n, size, err = checkAll(*assets)
		if err == nil {
			fmt.Printf("audiogen: %d sounds ok, %.1f MB of %d MB\n", n, float64(size)/(1<<20), budget>>20)
		}
	default:
		var n int
		n, err = generate(*assets)
		if err == nil {
			fmt.Printf("audiogen: %d procedural sounds written\n", n)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "audiogen:", err)
		os.Exit(1)
	}
}

// fileStem is where an id's files go: ui.click -> ui/click,
// sfx.chop -> sfx/chop, amb.campfire -> ambience/campfire.
func fileStem(id string) string {
	kind, name, _ := strings.Cut(id, ".")
	switch kind {
	case "amb":
		kind = "ambience"
	case "animal":
		return animalStem(name)
	}
	return kind + "/" + name
}

// animalStem places animal ids under sfx/animals: animal.canid.call ->
// group_canid_call, animal.generic.die -> generic_die, animal.burst ->
// burst, any other key.event -> key_event.
func animalStem(name string) string {
	key, event, ok := strings.Cut(name, ".")
	switch {
	case !ok:
		return "sfx/animals/" + name
	case key == "generic":
		return "sfx/animals/generic_" + event
	case slices.Contains(animalGroups, key):
		return "sfx/animals/group_" + key + "_" + event
	}
	return "sfx/animals/" + key + "_" + event
}

func generate(assets string) (int, error) {
	root := filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir))
	m, err := readManifest(assets)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	keep := m.Sounds[:0]
	for _, e := range m.Sounds {
		if e.Source != audiomanifest.SourceProcedural || isAnimalPlaceholder(e.ID) { // -animals owns those
			keep = append(keep, e)
		}
	}
	m.Sounds = keep
	for _, snd := range sounds {
		rc := snd.recipe
		vars := max(rc.Variants, 1)
		e := audiomanifest.Entry{ID: rc.ID, Bus: audiomanifest.BusOf(rc.ID), Loop: rc.Loop > 0, Volume: snd.volume,
			Source: audiomanifest.SourceProcedural, Recipe: rc.ID, Seed: rc.Seed, License: proceduralLicense}
		if _, ok := m.Lookup(rc.ID); ok {
			return 0, fmt.Errorf("%s: also listed from another source; remove one", rc.ID)
		}
		for v := 0; v < vars; v++ {
			rel := fmt.Sprintf("%s_%d.wav", fileStem(rc.ID), v)
			if rc.Loop > 0 {
				rel = fileStem(rc.ID) + ".wav"
			}
			pcm := sfxsynth.Quantize(sfxsynth.Render(rc, v), rc.Seed+uint64(v))
			if err := writeWAV(filepath.Join(root, filepath.FromSlash(rel)), pcm); err != nil {
				return 0, err
			}
			e.Files = append(e.Files, rel)
		}
		m.Sounds = append(m.Sounds, e)
	}
	if err := m.Validate(); err != nil {
		return 0, err
	}
	return len(sounds), writeManifest(assets, m)
}

func writeWAV(path string, pcm []int16) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := sfxsynth.WriteWAV(&buf, pcm, sfxsynth.Rate); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func readManifest(assets string) (audiomanifest.Manifest, error) {
	f, err := os.Open(filepath.Join(assets, filepath.FromSlash(audiomanifest.Rel)))
	if err != nil {
		return audiomanifest.Manifest{}, err
	}
	defer f.Close()
	return audiomanifest.Decode(f)
}

func writeManifest(assets string, m audiomanifest.Manifest) error {
	path := filepath.Join(assets, filepath.FromSlash(audiomanifest.Rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := audiomanifest.Encode(&buf, m); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// checkAll validates the sound list, that every file exists (WAV files at
// 48 kHz), and the size budget.
func checkAll(assets string) (int, int64, error) {
	m, err := readManifest(assets)
	if err != nil {
		return 0, 0, err
	}
	root := filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir))
	var size int64
	for _, e := range m.Sounds {
		for _, rel := range e.Files {
			path := filepath.Join(root, filepath.FromSlash(rel))
			st, err := os.Stat(path)
			if err != nil {
				return 0, 0, fmt.Errorf("%s: %w", e.ID, err)
			}
			size += st.Size()
			if strings.HasSuffix(rel, ".ogg") {
				if err := checkOgg(path); err != nil {
					return 0, 0, fmt.Errorf("%s: %s: %w", e.ID, rel, err)
				}
			}
			if strings.HasSuffix(rel, ".wav") {
				f, err := os.Open(path)
				if err != nil {
					return 0, 0, err
				}
				_, rate, _, err := sfxsynth.ReadWAV(f)
				f.Close()
				if err != nil || rate != sfxsynth.Rate {
					return 0, 0, fmt.Errorf("%s: %s must be 16-bit PCM at %d Hz (%v)", e.ID, rel, sfxsynth.Rate, err)
				}
			}
		}
	}
	if err := checkAnimals(assets, m); err != nil {
		return 0, 0, err
	}
	if size > budget {
		return 0, 0, fmt.Errorf("audio is %.1f MB, over the %d MB budget", float64(size)/(1<<20), budget>>20)
	}
	return len(m.Sounds), size, nil
}

// checkOgg decodes an Ogg Vorbis file and wants it to be 48 kHz.
func checkOgg(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := vorbis.DecodeWithoutResampling(f)
	switch {
	case err != nil:
		return fmt.Errorf("not a decodable Ogg Vorbis file: %w", err)
	case st.SampleRate() != sfxsynth.Rate:
		return fmt.Errorf("Ogg must be %d Hz, is %d Hz", sfxsynth.Rate, st.SampleRate())
	case st.Length() <= 0:
		return errors.New("Ogg has no samples")
	}
	return nil
}

func loopFix(in, out string, fade float64) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	pcm, rate, _, err := sfxsynth.ReadWAV(f)
	f.Close()
	if err != nil {
		return err
	}
	fixed := sfxsynth.LoopFix(pcm, int(fade*float64(rate)))
	var buf bytes.Buffer
	if err := sfxsynth.WriteWAV(&buf, fixed, rate); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}
