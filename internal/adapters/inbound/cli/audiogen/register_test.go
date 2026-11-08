package audiogen

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/pkg/audiomanifest"
)

func registerEnv(t *testing.T) string {
	t.Helper()
	assets := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(assets, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(promptsRel, `{"sounds":[{"id":"amb.day","file":"ambience/day.ogg","tool":"text_to_sound_effects","seconds":20,"loop":true,"volume":0.6,"prompt":"wind"}]}`)
	write(audiomanifest.Dir+"/ambience/day.ogg", "ogg")
	return assets
}

func TestRegisterMossTakeReplacesElevenLabs(t *testing.T) {
	t.Parallel()
	assets := registerEnv(t)
	e, err := register(assets, "ambience/day.ogg", 200, "ElevenLabs Creator plan", "arxdsilva", 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.Source != audiomanifest.SourceElevenLabs || e.Credits != 200 || e.Seed != 0 {
		t.Fatalf("elevenlabs entry = %+v", e)
	}
	if e, err = register(assets, "ambience/day.ogg", 100, "ElevenLabs Creator plan", "arxdsilva", 0); err != nil || e.Credits != 300 {
		t.Fatalf("a second ElevenLabs take adds its credits: %+v, %v", e, err)
	}
	const lic = "MOSS-SoundEffect v2.0 model (Apache-2.0), generated locally"
	if e, err = register(assets, "ambience/day.ogg", 50, lic, "tester", 7); err != nil {
		t.Fatal(err)
	}
	if e.Source != audiomanifest.SourceMOSS || e.Tool != mossTool || e.Seed != 7 || e.Credits != 0 ||
		e.License != lic || e.Review != "tester" || len(e.Files) != 1 || !e.Loop || e.Prompt != "wind" {
		t.Fatalf("moss entry = %+v", e)
	}
	m, err := readManifest(assets)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := m.Lookup("amb.day"); !ok || got.Source != audiomanifest.SourceMOSS || len(m.Sounds) != 1 {
		t.Fatalf("sound list = %+v", m.Sounds)
	}
}

func TestRegisterRefusesNonCommercialAndProcedural(t *testing.T) {
	t.Parallel()
	assets := registerEnv(t)
	if _, err := register(assets, "ambience/day.ogg", 200, "ElevenLabs free tier: non-commercial use", "arxdsilva", 0); err == nil {
		t.Fatal("a non-commercial take was listed")
	}
	if err := writeManifest(assets, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{{ID: "amb.day", Bus: audiomanifest.BusAmbience,
		Volume: 0.6, Files: []string{"ambience/day.wav"}, Source: audiomanifest.SourceProcedural, License: proceduralLicense}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := register(assets, "ambience/day.ogg", 0, "MOSS", "arxdsilva", 7); err == nil {
		t.Fatal("a procedural sound was replaced")
	}
}
