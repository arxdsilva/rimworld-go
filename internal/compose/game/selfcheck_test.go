package game

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cligame "rimworld-go/internal/adapters/inbound/cli/game"
)

func TestVersionPrintsAndExitsZero(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	code, done := Report(cligame.Options{Version: true}, "v1.2.3", &out)
	if !done || code != 0 || out.String() != "v1.2.3\n" {
		t.Fatalf("-version: done %v code %d out %q", done, code, out.String())
	}
	out.Reset()
	if _, done := Report(cligame.Options{}, "v1.2.3", &out); done || out.Len() != 0 {
		t.Fatalf("no flag must run the game: done %v out %q", done, out.String())
	}
}

func TestConsoleOutKeepsARedirectedStdout(t *testing.T) {
	t.Parallel()
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if w := ConsoleOut(f); w != io.Writer(f) {
		t.Fatalf("a file as stdout must be kept, got %T", w)
	}
}

// From the repository every asset root resolves; the settings folder is
// only resolved, never made.
func TestSelfCheckFindsTheRepositoryAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
	var out bytes.Buffer
	code, done := Report(cligame.Options{SelfCheck: true}, "dev", &out)
	if !done || code != 0 {
		t.Fatalf("selfcheck: code %d\n%s", code, out.String())
	}
	for _, name := range []string{"translations", "art", "menu splash", "sounds", "window icon", "settings folder"} {
		if !strings.Contains(out.String(), "ok       "+name+": ") {
			t.Errorf("no %q line in\n%s", name, out.String())
		}
	}
	dir, err := configDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, home) {
		t.Fatalf("settings folder %q is not under the test home %q", dir, home)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("selfcheck made the settings folder %q (%v)", dir, err)
	}
	if !strings.Contains(out.String(), "(made on first run)") {
		t.Errorf("a fresh settings folder should read as made on first run:\n%s", out.String())
	}
}

func TestSelfCheckFailsOnAMissingRoot(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	found := func() (string, error) { return "/x/assets", nil }
	lost := func() (string, error) { return "", errors.New("assets/lang not found") }
	code := selfCheck(&out, "v1", []fileCheck{{"art", found}, {"translations", lost}}, func() (string, error) { return t.TempDir(), nil })
	if code != 1 || !strings.Contains(out.String(), "MISSING  translations: assets/lang not found") || !strings.Contains(out.String(), "ok       art: /x/assets") {
		t.Fatalf("code %d\n%s", code, out.String())
	}
	out.Reset()
	code = selfCheck(&out, "v1", []fileCheck{{"art", found}}, func() (string, error) { return "", errors.New("$HOME is not defined") })
	if code != 1 || !strings.Contains(out.String(), "MISSING  settings folder") {
		t.Fatalf("an unresolvable settings folder must fail: code %d\n%s", code, out.String())
	}
}
