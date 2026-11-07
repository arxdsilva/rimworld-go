package filesave

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
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
	want := ViewFile{CX: 12.25, CY: 7.5, Zoom: 1.75, Version: ViewVersion} // SaveView stamps the version
	if err := s.SaveView("My colony", ViewFile{CX: 12.25, CY: 7.5, Zoom: 1.75}); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.LoadView(path); !ok || !reflect.DeepEqual(got, want) {
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

// TestViewSidecarPerMap: each open map's camera round-trips (MAP-1); map 0
// stays at the top level, so a sidecar written before maps still loads as
// map 0's, and one holding only another map's camera loads too.
func TestViewSidecarPerMap(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	path := filepath.Join(s.Dir, "Two-maps"+Ext)
	views := map[game.MapID]ViewFile{0: {CX: 10, CY: 20, Zoom: 1}, 3: {CX: 5.5, CY: 6.5, Zoom: 0.5}}
	if err := s.SaveView("Two maps", ViewsOf(views)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "Two-maps"+ViewExt))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil || raw["CX"] != 10.0 || raw["maps"] == nil {
		t.Fatalf("map 0 at the top level, the others under maps: %s", data)
	}
	got, ok := s.LoadView(path)
	if !ok || !reflect.DeepEqual(got.PerMap(), views) {
		t.Fatalf("per map %+v %v, want %+v", got.PerMap(), ok, views)
	}

	os.WriteFile(filepath.Join(s.Dir, "Two-maps"+ViewExt), []byte(`{"CX":3,"CY":4,"Zoom":2,"v":1}`), 0o644)
	got, ok = s.LoadView(path)
	if want := map[game.MapID]ViewFile{0: {CX: 3, CY: 4, Zoom: 2}}; !ok || !reflect.DeepEqual(got.PerMap(), want) {
		t.Fatalf("an old sidecar is map 0's: %+v %v", got.PerMap(), ok)
	}

	os.WriteFile(filepath.Join(s.Dir, "Two-maps"+ViewExt),
		[]byte(`{"CX":0,"CY":0,"Zoom":0,"maps":{"2":{"CX":1,"CY":2,"Zoom":1},"x":{"CX":1,"CY":1,"Zoom":1},"4":{"CX":1,"CY":1,"Zoom":-1}}}`), 0o644)
	got, ok = s.LoadView(path)
	if want := map[game.MapID]ViewFile{2: {CX: 1, CY: 2, Zoom: 1}}; !ok || !reflect.DeepEqual(got.PerMap(), want) {
		t.Fatalf("only the valid cameras load: %+v %v", got.PerMap(), ok)
	}
}

// TestViewSidecarOldVersionLoadsAtDefaultZoom (UIS-087): a sidecar written
// before the zoom followed the window (no "v") loads every valid camera at
// the default zoom with its centre kept; a current one keeps its zoom.
func TestViewSidecarOldVersionLoadsAtDefaultZoom(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	path := filepath.Join(s.Dir, "Old"+Ext)
	side := filepath.Join(s.Dir, "Old"+ViewExt)
	os.WriteFile(side, []byte(`{"CX":30.5,"CY":12.25,"Zoom":2.5,"maps":{"3":{"CX":4,"CY":5,"Zoom":0.3},"4":{"CX":1,"CY":1,"Zoom":0}}}`), 0o644)
	got, ok := s.LoadView(path)
	if !ok || got.Version != ViewVersion {
		t.Fatalf("old sidecar: %+v %v, want version %d", got, ok, ViewVersion)
	}
	want := map[game.MapID]ViewFile{0: {CX: 30.5, CY: 12.25, Zoom: DefaultZoom}, 3: {CX: 4, CY: 5, Zoom: DefaultZoom}}
	if !reflect.DeepEqual(got.PerMap(), want) {
		t.Fatalf("old cameras %+v, want the default zoom at the saved centres %+v", got.PerMap(), want)
	}
	os.WriteFile(side, []byte(`{"CX":30.5,"CY":12.25,"Zoom":1.8,"v":1}`), 0o644)
	if got, ok := s.LoadView(path); !ok || got.PerMap()[0].Zoom != 1.8 {
		t.Fatalf("a current sidecar keeps its zoom: %+v %v", got, ok)
	}
	if err := s.SaveView("Old", ViewFile{CX: 2, CY: 3, Zoom: 0.5}); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.LoadView(path); !ok || got.Version != ViewVersion || got.Zoom != 0.5 {
		t.Fatalf("a saved sidecar is current: %+v %v", got, ok)
	}
}
