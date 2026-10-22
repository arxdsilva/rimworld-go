package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

var animalEvents = []string{"call", "attack", "hurt", "die"}

func soundByID(id string) (sound, bool) {
	for _, s := range animalSounds() {
		if s.recipe.ID == id {
			return s, true
		}
	}
	return sound{}, false
}

func TestAnimalRecipeSet(t *testing.T) {
	t.Parallel()
	volume := map[string]float64{"call": 0.7, "attack": 0.85, "hurt": 0.85, "die": 1}
	groups := append(append([]string{}, animalGroups...), "generic")
	if len(animalGroups) != 9 {
		t.Fatalf("%d voice groups, want 9", len(animalGroups))
	}
	for _, g := range groups {
		for _, ev := range animalEvents {
			id := fmt.Sprintf("animal.%s.%s", g, ev)
			s, ok := soundByID(id)
			if !ok {
				t.Fatalf("no recipe for %s", id)
			}
			if s.volume != volume[ev] || s.recipe.Variants != 2 || s.recipe.Loop != 0 || s.recipe.Seed == 0 {
				t.Errorf("%s: volume %v, variants %d, loop %v, seed %d", id, s.volume, s.recipe.Variants, s.recipe.Loop, s.recipe.Seed)
			}
		}
	}
	b, ok := soundByID("animal.burst")
	if !ok || b.volume != 1 || b.recipe.Variants > 1 {
		t.Fatalf("animal.burst: %+v ok=%v", b, ok)
	}
}

func TestAnimalLengths(t *testing.T) {
	t.Parallel()
	limits := map[string][2]float64{"call": {0.8, 1.6}, "attack": {0.45, 0.55}, "hurt": {0.3, 0.4}, "die": {0.95, 1.05}}
	for _, g := range append(append([]string{}, animalGroups...), "generic") {
		for _, ev := range animalEvents {
			id := fmt.Sprintf("animal.%s.%s", g, ev)
			s, _ := soundByID(id)
			l := limits[ev]
			if got := s.recipe.Length(); got < l[0] || got > l[1] {
				t.Errorf("%s is %.2f s, want %.2f-%.2f", id, got, l[0], l[1])
			}
		}
	}
	if b, _ := soundByID("animal.burst"); b.recipe.Length() < 0.5 || b.recipe.Length() > 1.5 {
		t.Errorf("animal.burst is %.2f s", b.recipe.Length())
	}
}

func TestAnimalSeedsAreUniqueAndRenderIsDeterministic(t *testing.T) {
	t.Parallel()
	seen := map[uint64]string{}
	for _, s := range animalSounds() {
		if other, dup := seen[s.recipe.Seed]; dup {
			t.Fatalf("%s and %s share seed %d", s.recipe.ID, other, s.recipe.Seed)
		}
		seen[s.recipe.Seed] = s.recipe.ID
	}
	s, _ := soundByID("animal.canid.call")
	a, b := sfxsynth.Render(s.recipe, 0), sfxsynth.Render(s.recipe, 0)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatal("render length differs")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("render differs at %d", i)
		}
	}
	c := sfxsynth.Render(s.recipe, 1)
	same := len(c) == len(a)
	for i := 0; same && i < len(a); i++ {
		same = a[i] == c[i]
	}
	if same {
		t.Fatal("variants 0 and 1 are identical")
	}
}

func TestAnimalVoicesAreDistinctPerGroup(t *testing.T) {
	t.Parallel()
	// Different groups must not render the same call.
	seen := map[string]string{}
	for _, g := range animalGroups {
		s, _ := soundByID("animal." + g + ".call")
		x := sfxsynth.Render(s.recipe, 0)
		sum := 0.0
		for i, f := range x {
			sum += f * float64(i%97+1)
		}
		key := fmt.Sprintf("%d/%.3f", len(x), math.Round(sum*1000)/1000)
		if other, dup := seen[key]; dup {
			t.Fatalf("%s and %s render the same call", g, other)
		}
		seen[key] = g
	}
}

func TestAnimalFileStem(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"animal.canid.call":      "sfx/animals/group_canid_call",
		"animal.bird_small.die":  "sfx/animals/group_bird_small_die",
		"animal.generic.attack":  "sfx/animals/generic_attack",
		"animal.burst":           "sfx/animals/burst",
		"animal.timber_wolf.die": "sfx/animals/timber_wolf_die", // species takes (cmd/sfxgen) keep their own names
	} {
		if got := fileStem(id); got != want {
			t.Errorf("fileStem(%s) = %s, want %s", id, got, want)
		}
	}
}

// copyEncoder stands in for oggenc: the "OGG" is the WAV bytes.
func copyEncoder(wav, out string, _ uint64) error {
	b, err := os.ReadFile(wav)
	if err != nil {
		return err
	}
	return os.WriteFile(out, b, 0o644)
}

func speciesEntry() audiomanifest.Entry {
	return audiomanifest.Entry{ID: "animal.timber_wolf.call", Bus: audiomanifest.BusEffects, Volume: 0.7,
		Files: []string{"sfx/animals/timber_wolf_call.ogg"}, Source: audiomanifest.SourceElevenLabs, License: "x"}
}

func TestDefaultGenerateLeavesAnimalsAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writeManifest(dir, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{speciesEntry()}}); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "generated/audio/sfx/animals")); err == nil {
		t.Fatal("make audio wrote animal files; those come from -animals")
	}
	if _, err := generateAnimals(dir, copyEncoder); err != nil {
		t.Fatal(err)
	}
	// A later default run keeps the animal placeholders listed.
	if _, err := generate(dir); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Lookup("animal.canid.call"); !ok {
		t.Fatal("the default run dropped an animal placeholder entry")
	}
	if _, ok := m.Lookup("animal.timber_wolf.call"); !ok {
		t.Fatal("a species entry was dropped")
	}
}

func TestGenerateAnimalsWritesOggEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writeManifest(dir, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{speciesEntry()}}); err != nil {
		t.Fatal(err)
	}
	n, err := generateAnimals(dir, copyEncoder)
	if err != nil || n != len(animalSounds()) {
		t.Fatalf("generateAnimals: %d, %v", n, err)
	}
	m, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Lookup("animal.timber_wolf.call"); !ok {
		t.Fatal("a species entry was dropped")
	}
	e, ok := m.Lookup("animal.canid.attack")
	if !ok {
		t.Fatal("no animal.canid.attack entry")
	}
	if e.Bus != audiomanifest.BusEffects || e.Source != audiomanifest.SourceProcedural || e.Recipe != e.ID ||
		e.Seed == 0 || e.License != proceduralLicense || e.Volume != 0.85 || e.Loop {
		t.Fatalf("entry %+v", e)
	}
	want := []string{"sfx/animals/group_canid_attack_0.ogg", "sfx/animals/group_canid_attack_1.ogg"}
	if fmt.Sprint(e.Files) != fmt.Sprint(want) {
		t.Fatalf("files %v, want %v", e.Files, want)
	}
	if b, _ := m.Lookup("animal.burst"); fmt.Sprint(b.Files) != "[sfx/animals/burst_0.ogg]" {
		t.Fatalf("burst files %v", b.Files)
	}
	for _, rel := range append(want, "sfx/animals/generic_die_1.ogg", "sfx/animals/burst_0.ogg") {
		if _, err := os.Stat(filepath.Join(dir, "generated/audio", rel)); err != nil {
			t.Fatal(err)
		}
	}
	entries := len(m.Sounds)
	if _, err := generateAnimals(dir, copyEncoder); err != nil { // rerun replaces, never duplicates
		t.Fatal(err)
	}
	if m2, _ := readManifest(dir); len(m2.Sounds) != entries {
		t.Fatalf("rerun changed the entry count %d -> %d", entries, len(m2.Sounds))
	}
}

func TestGenerateAnimalsEncoderFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writeManifest(dir, audiomanifest.Manifest{}); err != nil {
		t.Fatal(err)
	}
	boom := func(string, string, uint64) error { return fmt.Errorf("boom") }
	if _, err := generateAnimals(dir, boom); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err %v", err)
	}
}

func TestAnimalsNeedsOggenc(t *testing.T) { // not parallel: changes PATH
	t.Setenv("PATH", t.TempDir())
	err := animals(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "oggenc") {
		t.Fatalf("err %v, want a message naming oggenc", err)
	}
}

func TestAnimalsWithRealOggenc(t *testing.T) {
	if _, err := exec.LookPath("oggenc"); err != nil {
		t.Skip("oggenc not installed")
	}
	dir := t.TempDir()
	if err := writeManifest(dir, audiomanifest.Manifest{}); err != nil {
		t.Fatal(err)
	}
	if err := animals(dir); err != nil {
		t.Fatal(err)
	}
	n, size, err := checkAll(dir)
	if err != nil || n != len(animalSounds()) {
		t.Fatalf("checkAll: %d sounds, %v", n, err)
	}
	if size > 1<<20 {
		t.Fatalf("animal placeholders are %d bytes, want under 1 MiB", size)
	}
	first, _ := os.ReadFile(filepath.Join(dir, "generated/audio/sfx/animals/group_canid_call_0.ogg"))
	if err := animals(dir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "generated/audio/sfx/animals/group_canid_call_0.ogg"))
	if len(first) == 0 || !bytes.Equal(first, second) {
		t.Fatal("the encoded Ogg differs between runs")
	}
}

func TestCheckAllValidatesOgg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rel := "sfx/animals/bad_0.ogg"
	path := filepath.Join(dir, "generated/audio", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not an ogg"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := audiomanifest.Entry{ID: "animal.bad.call", Bus: audiomanifest.BusEffects, Volume: 0.7, Files: []string{rel},
		Source: audiomanifest.SourceProcedural, License: "x"}
	if err := writeManifest(dir, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{e}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := checkAll(dir); err == nil {
		t.Fatal("checkAll accepted a corrupt .ogg")
	}
}

func TestCheckAllRejectsOtherRateOgg(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("oggenc"); err != nil {
		t.Skip("oggenc not installed")
	}
	dir := t.TempDir()
	tmp := t.TempDir()
	var buf bytes.Buffer
	if err := sfxsynth.WriteWAV(&buf, make([]int16, 4410), 44100); err != nil {
		t.Fatal(err)
	}
	wav := filepath.Join(tmp, "a.wav")
	if err := os.WriteFile(wav, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := "sfx/animals/slow_0.ogg"
	out := filepath.Join(dir, "generated/audio", rel)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("oggenc", "-Q", "-o", out, wav).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	e := audiomanifest.Entry{ID: "animal.slow.call", Bus: audiomanifest.BusEffects, Volume: 0.7, Files: []string{rel},
		Source: audiomanifest.SourceProcedural, License: "x"}
	if err := writeManifest(dir, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{e}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := checkAll(dir); err == nil || !strings.Contains(err.Error(), "48000") {
		t.Fatalf("err %v, want a 48000 Hz complaint", err)
	}
}
