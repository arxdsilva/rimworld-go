package overlay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AX-4: the deep ore overlay shows by the drill and the scanner, each ore
// has its own marker colour, and the watermill's wheel keys and offsets
// match AR2's metadata.
func TestDeepOreOverlayAndWheel(t *testing.T) {
	if !DeepOreKind("deep drill") || !DeepOreKind("ground-penetrating scanner") || DeepOreKind("moisture pump") {
		t.Fatal("deep ore kinds")
	}
	if !DeepOreTool("deep_drill") || !DeepOreTool("deep_scanner") || DeepOreTool("moisture_pump") {
		t.Fatal("deep ore tools")
	}
	seen := map[[4]float32]string{}
	for _, ore := range []string{"steel", "silver", "plasteel", "uranium", "jade", "gold"} {
		c := OreColour(ore)
		if o, dup := seen[c]; dup {
			t.Fatalf("%s and %s share a colour", ore, o)
		}
		seen[c] = ore
	}
	root, _ := os.Getwd()
	for ; ; root = filepath.Dir(root) {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
	}
	for _, f := range []string{"south", "east", "north", "west"} {
		key := MillWheelKey(f, 1)
		meta, err := os.ReadFile(filepath.Join(root, "assets", "metadata", key+".yaml"))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		ox, oy := MillWheelOffset(f)
		want := "offset_tiles: [" + itoa(int(ox)) + ", " + itoa(int(oy)) + "]"
		if !strings.Contains(string(meta), want) {
			t.Fatalf("%s: metadata lacks %q", key, want)
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	s := ""
	for ; v > 0; v /= 10 {
		s = string(rune('0'+v%10)) + s
	}
	return s
}
