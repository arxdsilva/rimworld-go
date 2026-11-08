package audiogen

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// budget is the most audio the repo carries (SND-002; raised from 20 MB
// to 30 MB by user decision, 2026-10-29, for the BED/PRD/DEF sounds).
const budget = 30 << 20

const proceduralLicense = "original, made in code (CC0, this repo)"

// options are the parsed command line.
type options struct {
	assets   string
	animal   bool
	check    bool
	loopfix  bool
	fade     float64
	music    bool
	looping  string
	reg      string
	credits  int
	license  string
	reviewer string
	seed     uint64
	args     []string
}

func parse(args []string, stderr io.Writer) (options, error) {
	var o options
	set := flag.NewFlagSet("audiogen", flag.ExitOnError)
	set.SetOutput(stderr)
	set.StringVar(&o.assets, "assets", "assets", "assets directory")
	set.BoolVar(&o.animal, "animals", false, "render the animal placeholder voices (animal_recipes.go) and encode Ogg with oggenc")
	set.BoolVar(&o.check, "check", false, "validate the sound list against the files")
	set.BoolVar(&o.loopfix, "loopfix", false, "make the WAV loop seamless: -loopfix in.wav out.wav")
	set.Float64Var(&o.fade, "fade", 1, "-loopfix crossfade, seconds")
	set.BoolVar(&o.music, "music", false, "compose the music (scores in music.go) and encode Ogg with ffmpeg")
	set.StringVar(&o.looping, "looping", "", "print true/false: is this prompts.json file a loop")
	set.StringVar(&o.reg, "register", "", "add this imported file (under generated/audio) to the sound list")
	set.IntVar(&o.credits, "credits", 0, "-register: ElevenLabs credits the take cost")
	set.StringVar(&o.license, "license", "", "-register: the plan and terms the take was made under")
	set.StringVar(&o.reviewer, "reviewer", "arxdsilva", "-register: who listened and approved it")
	set.Uint64Var(&o.seed, "seed", 0, "-register: the MOSS seed of the take (0: an ElevenLabs take)")
	err := set.Parse(args)
	o.args = set.Args()
	return o, err
}

// Run executes audiogen with the given arguments and returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args, stderr)
	if err == nil {
		err = dispatch(o, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "audiogen:", err)
		return 1
	}
	return 0
}

func dispatch(o options, stdout io.Writer) error {
	var err error
	switch {
	case o.music:
		err = composeAll(o.assets)
	case o.animal:
		err = animals(o.assets)
	case o.looping != "":
		var p prompt
		if p, err = promptFor(o.assets, o.looping); err == nil {
			fmt.Fprintln(stdout, p.Loop)
		}
	case o.reg != "":
		if o.license == "" {
			return errors.New("-register needs -license (the model's or plan's terms)")
		}
		var e audiomanifest.Entry
		if e, err = register(o.assets, o.reg, o.credits, o.license, o.reviewer, o.seed); err == nil {
			fmt.Fprintf(stdout, "audiogen: %s now has %d file(s), %d credits\n", e.ID, len(e.Files), e.Credits)
		}
	case o.loopfix:
		if len(o.args) != 2 {
			return errors.New("-loopfix needs in.wav out.wav")
		}
		err = loopFix(o.args[0], o.args[1], o.fade)
	case o.check:
		var n int
		var size int64
		n, size, err = checkAll(o.assets)
		if err == nil {
			fmt.Fprintf(stdout, "audiogen: %d sounds ok, %.1f MB of %d MB\n", n, float64(size)/(1<<20), budget>>20)
		}
	default:
		var n int
		n, err = generate(o.assets)
		if err == nil {
			fmt.Fprintf(stdout, "audiogen: %d procedural sounds written\n", n)
		}
	}
	return err
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
		return fmt.Errorf("ogg must be %d Hz, is %d Hz", sfxsynth.Rate, st.SampleRate())
	case st.Length() <= 0:
		return errors.New("ogg has no samples")
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
