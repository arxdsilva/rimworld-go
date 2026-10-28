package animal

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// repoRoot is the directory holding go.mod, found by walking up from the
// package under test.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			t.Fatal("no go.mod above the package")
		}
		dir = up
	}
}

// TestSizeLengthMatchesArtRoster: Go's sizeLength and the art pipeline's
// SIZE_LENGTH (animal_roster.py) are one table written twice (PLAY-022:
// the area targets XS 1, S 1.5, M 2, L 4, XL 6 tiles^2). The sim never
// reads the length, but the picture it is drawn at does, so they must
// agree.
func TestSizeLengthMatchesArtRoster(t *testing.T) {
	t.Parallel()
	path := filepath.Join(repoRoot(t), "assets", "source", "blender", "artlib", "families", "animal_roster.py")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?m)^SIZE_LENGTH\s*=\s*\{([^}]*)\}`).FindSubmatch(src)
	if block == nil {
		t.Fatalf("%s has no SIZE_LENGTH = {...} line", path)
	}
	art := map[string]float64{}
	for _, m := range regexp.MustCompile(`["'](XS|S|M|L|XL)["']\s*:\s*([0-9]+(?:\.[0-9]+)?)`).FindAllSubmatch(block[1], -1) {
		v, err := strconv.ParseFloat(string(m[2]), 64)
		if err != nil {
			t.Fatal(err)
		}
		art[string(m[1])] = v
	}
	classes := map[string]Size{"XS": SizeXS, "S": SizeS, "M": SizeM, "L": SizeL, "XL": SizeXL}
	if len(art) != len(classes) {
		t.Fatalf("SIZE_LENGTH has %d classes (%v), want %d", len(art), art, len(classes))
	}
	for name, size := range classes {
		if got, ok := art[name]; !ok || got != sizeLength[size] {
			t.Errorf("class %s: animal_roster.py says %v, Go sizeLength says %v", name, got, sizeLength[size])
		}
	}
}
