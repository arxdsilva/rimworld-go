package ebitenaudio

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The headless simulator must never link the audio stack.
func TestSimulateHasNoAudio(t *testing.T) {
	t.Parallel()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not in PATH")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		up := filepath.Dir(dir)
		if up == dir {
			t.Skip("module root not found")
		}
		dir = up
	}
	cmd := exec.Command(gobin, "list", "-deps", "./cmd/simulate")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, bad := range []string{"ebiten/v2/audio", "outbound/ebitenaudio"} {
		if strings.Contains(string(out), bad) {
			t.Errorf("cmd/simulate depends on %s", bad)
		}
	}
}
