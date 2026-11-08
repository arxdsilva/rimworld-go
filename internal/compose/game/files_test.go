package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
