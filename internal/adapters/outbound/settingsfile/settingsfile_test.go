package settingsfile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/platform/config"
)

func TestMissingFileIsDefaults(t *testing.T) {
	t.Parallel()
	s := Store{Path: filepath.Join(t.TempDir(), "settings.json")}
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, config.Defaults()) {
		t.Fatalf("got %+v, %v; want defaults", got, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := Store{Path: filepath.Join(dir, "nested", "settings.json")}
	want := config.Defaults()
	want.Audio.Master = 40
	want.Gameplay.EdgeScroll = true
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
	entries, _ := os.ReadDir(filepath.Dir(s.Path))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("left a temp file: %s", e.Name())
		}
	}
}

func TestCorruptFileSetAside(t *testing.T) {
	t.Parallel()
	s := Store{Path: filepath.Join(t.TempDir(), "settings.json")}
	if err := os.WriteFile(s.Path, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err == nil || !reflect.DeepEqual(got, config.Defaults()) {
		t.Fatalf("got %+v, %v; want defaults and an error", got, err)
	}
	if _, err := os.Stat(s.Path + ".bad"); err != nil {
		t.Fatalf("corrupt file not kept as .bad: %v", err)
	}
	if _, err := os.Stat(s.Path); !os.IsNotExist(err) {
		t.Fatal("corrupt file left in place")
	}
}
