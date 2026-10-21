package ebitenaudio

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

func TestLoadRepoSoundList(t *testing.T) {
	t.Parallel()
	root, err := LocateAudio()
	if err != nil {
		t.Skip(err)
	}
	m, pcm, ogg, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sounds) == 0 {
		t.Fatal("empty sound list")
	}
	for _, e := range m.Sounds {
		if got := len(pcm[e.ID]) + len(ogg[e.ID]); got != len(e.Files) {
			t.Errorf("%s: %d of %d files decoded", e.ID, got, len(e.Files))
		}
	}
}

func TestLoadSkipsBadFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel string, data []byte) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wav := func(rate int) []byte {
		f, err := os.CreateTemp(t.TempDir(), "*.wav")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := sfxsynth.WriteWAV(f, []int16{1, 2, 3}, rate); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(f.Name())
		return b
	}
	write(audiomanifest.Dir+"/ui/ok.wav", wav(sfxsynth.Rate))
	write(audiomanifest.Dir+"/ui/slow.wav", wav(22050))
	write(audiomanifest.Dir+"/music/bad.ogg", []byte("not vorbis"))
	write(audiomanifest.Rel, []byte(`{"version":1,"sounds":[
		{"id":"ui.a","bus":"ui","volume":1,"files":["ui/ok.wav","ui/slow.wav","ui/none.wav"],"source":"procedural","license":"CC0"},
		{"id":"music.b","bus":"music","volume":1,"files":["music/bad.ogg"],"source":"procedural","license":"CC0"}]}`))
	_, pcm, ogg, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm["ui.a"]) != 1 || len(ogg["music.b"]) != 0 {
		t.Fatalf("kept %d WAVs, %d OGGs", len(pcm["ui.a"]), len(ogg["music.b"]))
	}

	write(audiomanifest.Rel, []byte(`{"version":1,"sounds":[{"id":"x","bus":"loud"}]}`))
	if _, _, _, err := load(root); err == nil {
		t.Fatal("bad sound list accepted")
	}
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("New without a sound list succeeded")
	}
}
