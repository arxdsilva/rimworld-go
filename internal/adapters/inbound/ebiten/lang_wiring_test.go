package ebiten

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
)

// TestLanguageSettingSwitchesTheCatalog: the Language option (OPT-001)
// loads its catalog from assets/lang; a code that does not load is English.
func TestLanguageSettingSwitchesTheCatalog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fr.json"), []byte(`{"_meta":{"name":"Français"},"Save":"Sauver"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := shellTexts{&lang.Live{}, dir}
	if got := s.T("Save"); got != "Save" {
		t.Fatalf("before: %q", got)
	}
	s.use("fr")
	if got := s.T("Save"); got != "Sauver" {
		t.Fatalf("fr: %q", got)
	}
	s.use("zz")
	if got := s.T("Save"); got != "Save" {
		t.Fatalf("missing catalog: %q", got)
	}
	if newTexts().dir == "" {
		t.Fatal("assets/lang not found from the test")
	}
}
