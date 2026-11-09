package game

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/filesave"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
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
				s.Biome == "" || s.Colonists != 3 || s.Title != "New Hope 2" || s.Compat != int(savefmt.CompatOK) || s.Notice != "" {
				t.Fatalf("new slot %+v", s)
			}
			_, got, err := h.Load(s.Path)
			if err != nil || got != meta {
				t.Fatalf("load meta %+v, %v, want %+v", got, err, meta)
			}
		case "old": // the v1 fixture's header names it "premotion"
			if s.Err != "" || s.Colony != "slot:premotion" || s.ColonyName != "premotion" || s.Base != "premotion" || s.Autosave || s.Version != 1 ||
				s.Title != "premotion" || s.Compat != int(savefmt.CompatOlder) {
				t.Fatalf("old slot %+v", s)
			}
		default:
			t.Fatalf("unexpected slot %+v", s)
		}
	}
}

// SAV-021, SAV-022: how the list shows a save this build cannot read.
func TestSlotOfSaysWhyASaveCannotLoad(t *testing.T) {
	t.Parallel()
	rt := saveRuntime("v2", savefmt.Content{Hash: 5, Plugins: []string{"core"}})
	if rt.Version != appgame.SaveVersion || rt.Min != 1 {
		t.Fatalf("runtime %+v", rt)
	}
	newer := savefmt.Info{Version: appgame.SaveVersion + 1, Name: "Later", Colony: "c"}
	for _, c := range []struct {
		name string
		sl   filesave.Slot
		err  string
		c    savefmt.Compat
	}{
		{"newer", filesave.Slot{Info: newer, Err: fmt.Errorf("%w (v99)", appgame.ErrSaveVersion)}, "Saved by a newer version of the game", savefmt.CompatNewer},
		{"too old", filesave.Slot{Info: savefmt.Info{Version: 0}, Err: appgame.ErrSaveVersion}, "Saved by a version too old to load", savefmt.CompatTooOld},
		{"damaged", filesave.Slot{Info: savefmt.Info{Version: appgame.SaveVersion}, Err: fmt.Errorf("%w: crc", savefmt.ErrCorrupt)}, "damaged file", savefmt.CompatOK},
		{"unreadable", filesave.Slot{Err: errors.New("not a save file")}, "not a save file", savefmt.CompatOK},
		{"other content", filesave.Slot{Info: savefmt.Info{Version: appgame.SaveVersion, Build: "v2", Content: savefmt.Content{Hash: 6}}}, "", savefmt.CompatContent},
	} {
		s := slotOf(c.sl, rt)
		if s.Err != c.err || s.Compat != int(c.c) {
			t.Errorf("%s: Err %q compat %d, want %q %d", c.name, s.Err, s.Compat, c.err, c.c)
		}
	}
	if s := slotOf(filesave.Slot{Name: "Later", Info: newer, Err: appgame.ErrSaveVersion}, rt); s.Title != "Later" || s.Colony != "c" || s.Version != newer.Version {
		t.Fatalf("a newer save keeps its header: %+v", s)
	}
}

// SAV-025: a save played with a plugin this build lacks says which; one
// with other content says so; one like this build says nothing.
func TestMissingPluginNotice(t *testing.T) {
	t.Parallel()
	rt := savefmt.Runtime{Version: 16, Min: 1, Build: "v2", Content: savefmt.Content{Hash: 5, Plugins: []string{"core"}}}
	fauna := savefmt.Info{Version: 16, Build: "v2", Content: savefmt.Content{Hash: 9, Plugins: []string{"core", "moreFauna", "deepSea"}}}
	if n := slotNotice(fauna, rt, savefmt.CompatOf(fauna, rt)); !strings.Contains(n, "moreFauna, deepSea") {
		t.Fatalf("notice %q, want the missing plugins", n)
	}
	other := savefmt.Info{Version: 16, Build: "v2", Content: savefmt.Content{Hash: 9, Plugins: []string{"core"}}}
	if n := slotNotice(other, rt, savefmt.CompatOf(other, rt)); n == "" || strings.Contains(n, "plugins") {
		t.Fatalf("other content: notice %q", n)
	}
	same := savefmt.Info{Version: 16, Build: "v2", Content: rt.Content}
	if n := slotNotice(same, rt, savefmt.CompatOf(same, rt)); n != "" {
		t.Fatalf("this build's save: notice %q", n)
	}
	s := slotOf(filesave.Slot{Info: fauna}, rt)
	if s.Err != "" || s.Notice == "" || s.Compat != int(savefmt.CompatContent) {
		t.Fatalf("the list row %+v", s)
	}
}

// SAV-010: the hooks' Delete removes the save and its camera sidecar.
func TestDeleteHookRemovesTheSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := saveHooks(dir, "test")
	defer closeSaves(h)
	if err := h.Save(appgame.NewColony(appgame.Config{Seed: 4, Width: 30, Height: 30}), ports.SaveMeta{Name: "Gone soon"}); err != nil {
		t.Fatal(err)
	}
	if err := h.SaveView("Gone soon", nil); err != nil {
		t.Fatal(err)
	}
	if err := h.Delete("Gone soon"); err != nil {
		t.Fatal(err)
	}
	h.Queue.(saveQueue).Wait()
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left %v", left)
	}
}
