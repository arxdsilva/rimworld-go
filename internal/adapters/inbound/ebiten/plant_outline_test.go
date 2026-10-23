package ebiten

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const outlineChecker = "../../../../assets/source/tools/check_plant_outline.py"

func runOutlineChecker(t *testing.T, files ...string) (string, error) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	out, err := exec.Command(py, append([]string{"-I", outlineChecker}, files...)...).CombinedOutput()
	return string(out), err
}

// TestPlantRecipesHaveNoOutline: grass and plants (ground plants, bushes,
// crops, trees) never have a contour; every plant recipe must render with
// outline=False (AGENTS.md, ART spec Housekeeping PLAY-010).
func TestPlantRecipesHaveNoOutline(t *testing.T) {
	if out, err := runOutlineChecker(t); err != nil {
		t.Fatalf("plant recipes render with an outline:\n%s", out)
	}
}

// TestOutlineCheckerCatchesOutline proves the checker fails on a recipe that
// asks for an outline, or forgets to turn the default (on) off.
func TestOutlineCheckerCatchesOutline(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.py")
	src := "def a():\n    return render_asset('plant', 'x')\ndef b():\n    return _asset(f, 'x', {}, m, outline=True)\n" +
		"def c():\n    return render_asset('plant', 'y', outline=False, outline_width=2)\n"
	if err := os.WriteFile(bad, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runOutlineChecker(t, bad)
	if err == nil {
		t.Fatal("checker accepted outlined plant recipes")
	}
	for _, want := range []string{":2:", ":4:", ":6:"} {
		if !strings.Contains(out, want) {
			t.Errorf("no violation reported at line %s:\n%s", want, out)
		}
	}
}
