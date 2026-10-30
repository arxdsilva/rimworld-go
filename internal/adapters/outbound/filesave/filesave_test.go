package filesave

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestSaveListLoad(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	g := game.NewColony(game.Config{Seed: 4, Width: 40, Height: 30})
	g.Step(3000)
	path, err := s.Save(g, "My colony!")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "My-colony"+Ext {
		t.Fatalf("slot file %s", path)
	}
	os.WriteFile(filepath.Join(s.Dir, "broken"+Ext), []byte("nope"), 0o644)
	slots, err := s.List()
	if err != nil || len(slots) != 2 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	var good Slot
	for _, sl := range slots {
		if sl.Name == "My-colony" {
			good = sl
		} else if sl.Err == nil {
			t.Fatal("a broken file should be listed with an error")
		}
	}
	if good.Err != nil || good.Info.Colonists != 3 {
		t.Fatalf("slot %+v", good)
	}
	h, _, err := s.Load(good.Path)
	if err != nil || h.Tick() != g.Tick() {
		t.Fatalf("load: %v", err)
	}
	// no temp files left behind
	ents, _ := os.ReadDir(s.Dir)
	for _, e := range ents {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}

func TestFilesaveAtomic(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	g := game.NewColony(game.Config{Seed: 2, Width: 30, Height: 30})
	path, err := s.Save(g, "only")
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A save that dies half-way (disk full, a crash in the encoder): the
	// partial bytes never reach the slot.
	boom := errors.New("disk full")
	_, err = s.write("only", func(w io.Writer) error {
		_, _ = w.Write([]byte("half a save"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("write error %v, want %v", err, boom)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, old) {
		t.Fatal("a failed save changed the existing file")
	}
	ents, _ := os.ReadDir(s.Dir)
	if len(ents) != 1 {
		t.Fatalf("folder after a failed save: %v", ents)
	}
	if h, _, err := s.Load(path); err != nil || h.Tick() != g.Tick() {
		t.Fatalf("old save no longer loads: %v", err)
	}
	// A slot that cannot be replaced (here a folder in the way) fails the
	// same way: error, nothing left behind.
	if err := os.Mkdir(filepath.Join(s.Dir, "blocked"+Ext), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(g, "blocked"); err == nil {
		t.Fatal("saving over a folder should fail")
	}
	ents, _ = os.ReadDir(s.Dir)
	for _, e := range ents {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}

func TestListAcceptsV1AndV2(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	v1, err := os.ReadFile("../../../application/game/testdata/premotion_v1.sav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "old"+Ext), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(game.NewColony(game.Config{Seed: 5, Width: 30, Height: 30}), "new"); err != nil {
		t.Fatal(err)
	}
	slots, err := s.List()
	if err != nil || len(slots) != 2 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	for _, sl := range slots {
		if sl.Err != nil {
			t.Fatalf("%s: %v", sl.Name, sl.Err)
		}
		want := map[string]int{"old": 1, "new": game.SaveVersion}[sl.Name]
		if sl.Info.Version != want {
			t.Fatalf("%s: version %d, want %d", sl.Name, sl.Info.Version, want)
		}
		if _, _, err := s.Load(sl.Path); err != nil {
			t.Fatalf("%s: %v", sl.Name, err)
		}
	}
}

func TestViewSidecarRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	g := game.NewColony(game.Config{Seed: 4, Width: 40, Height: 30})
	path, err := s.Save(g, "My colony")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.LoadView(path); ok {
		t.Fatal("an old save with no sidecar has no view")
	}
	if _, _, err := s.Load(path); err != nil {
		t.Fatalf("a save without a sidecar loads: %v", err)
	}
	want := ViewFile{CX: 12.25, CY: 7.5, Zoom: 1.75}
	if err := s.SaveView("My colony", want); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.LoadView(path); !ok || got != want {
		t.Fatalf("view %+v %v, want %+v", got, ok, want)
	}
	slots, err := s.List()
	if err != nil || len(slots) != 1 {
		t.Fatalf("the sidecar is not a save: %+v %v", slots, err)
	}
	os.WriteFile(filepath.Join(s.Dir, "My-colony"+ViewExt), []byte("{bad"), 0o644)
	if _, ok := s.LoadView(path); ok {
		t.Fatal("a corrupt sidecar reads as missing")
	}
	os.WriteFile(filepath.Join(s.Dir, "My-colony"+ViewExt), []byte(`{"CX":1,"CY":1,"Zoom":0}`), 0o644)
	if _, ok := s.LoadView(path); ok {
		t.Fatal("a zero zoom is invalid")
	}
	if left, _ := filepath.Glob(filepath.Join(s.Dir, "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}
