package ebiten

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// TestProjShearMatchesMetadata keeps the renderer's shear in step with the
// art contract: every rendered sprite records projection.shear_k.
func TestProjShearMatchesMetadata(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "assets", "metadata", "furniture", "bed_basic_blanket_south.yaml"))
	if err != nil {
		t.Skip("no furniture metadata:", err)
	}
	m := regexp.MustCompile(`shear_k:\s*([0-9.]+)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("bed_basic_blanket_south.yaml has no projection.shear_k")
	}
	k, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	if k != projShearK {
		t.Fatalf("projShearK = %v, art metadata shear_k = %v", projShearK, k)
	}
}
