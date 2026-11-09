package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/filesave"
	appgame "rimworld-go/internal/application/game"
)

// -save-dir: saves and autosaves go to the given folder, never the
// player's (probe runs cross 06:00 and autosave).
func TestSaveDirReplacesThePlayersSaves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.rwsave"), []byte("not a save"), 0o644); err != nil {
		t.Fatal(err)
	}
	slots, err := saveHooks(dir, "test").List()
	if err != nil && len(slots) == 0 {
		t.Fatal(err)
	}
	if len(slots) != 1 || !strings.HasPrefix(slots[0].Path, dir) {
		t.Fatalf("slots %+v, want the one in %s", slots, dir)
	}
}

// SAV-023: the Options "Saves folder" row opens the folder the saves go
// to, -save-dir's when one is given.
func TestSaveDirReachesTheSavesFolderRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := saveHooks(dir, "test").Dir; got != dir {
		t.Fatalf("Dir %q, want -save-dir's %q", got, dir)
	}
	def, err := filesave.Default()
	if err != nil {
		t.Skip("no per-user config folder here")
	}
	if got := saveHooks("", "test").Dir; got != def.Dir {
		t.Fatalf("Dir %q without -save-dir, want the per-user folder %q", got, def.Dir)
	}
}

// SAV-003: the client's meta and the build reach the header, List shows
// every header field, and Load gives the meta back; an older save gets
// its colony from the name in its header.
func TestSaveHeaderRoundTripsThroughTheHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old, err := os.ReadFile("../../application/game/testdata/premotion_v1.sav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.rwsave"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	h := saveHooks(dir, "v1.2.3")
	meta := ports.SaveMeta{Name: "New Hope 2", Colony: "col-1", ColonyName: "New Hope", Base: "New Hope", Season: "Aprimay"}
	if err := h.Save(appgame.NewColony(appgame.Config{Seed: 4, Width: 40, Height: 30}), meta); err != nil {
		t.Fatal(err)
	}
	closeSaves(h)
	slots, err := h.List()
	if err != nil || len(slots) != 2 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	for _, s := range slots {
		switch s.Name {
		case "New-Hope-2":
			if s.Err != "" || s.Colony != "col-1" || s.ColonyName != "New Hope" || s.Base != "New Hope" || s.Autosave ||
				s.Season != "Aprimay" || s.Build != "v1.2.3" || s.Version != appgame.SaveVersion || s.MapSize != 40 ||
				s.Biome == "" || s.Colonists != 3 {
				t.Fatalf("new slot %+v", s)
			}
			_, got, err := h.Load(s.Path)
			if err != nil || got != meta {
				t.Fatalf("load meta %+v, %v, want %+v", got, err, meta)
			}
		case "old": // the v1 fixture's header names it "premotion"
			if s.Err != "" || s.Colony != "slot:premotion" || s.ColonyName != "premotion" || s.Base != "premotion" || s.Autosave || s.Version != 1 {
				t.Fatalf("old slot %+v", s)
			}
		default:
			t.Fatalf("unexpected slot %+v", s)
		}
	}
}
