package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/filesave"
)

// -save-dir: saves and autosaves go to the given folder, never the
// player's (probe runs cross 06:00 and autosave).
func TestSaveDirReplacesThePlayersSaves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.rwsave"), []byte("not a save"), 0o644); err != nil {
		t.Fatal(err)
	}
	slots, err := saveHooks(dir).List()
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
	if got := saveHooks(dir).Dir; got != dir {
		t.Fatalf("Dir %q, want -save-dir's %q", got, dir)
	}
	def, err := filesave.Default()
	if err != nil {
		t.Skip("no per-user config folder here")
	}
	if got := saveHooks("").Dir; got != def.Dir {
		t.Fatalf("Dir %q without -save-dir, want the per-user folder %q", got, def.Dir)
	}
}
