package filesave

import (
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
