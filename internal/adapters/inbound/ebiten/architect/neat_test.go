package architect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// TestEveryNeatStorageRowHasACard: NEAT_STORAGE_GAP_ANALYSIS.md lists the 27
// pieces. Each row's build name has a card in the Storage category whose
// Build is that name; a done or partial row's card is not a placeholder; the
// counts table agrees with the rows; every Storage card with a Neat build
// name has a row; the rotation hint on the card matches the piece.
func TestEveryNeatStorageRowHasACard(t *testing.T) {
	t.Parallel()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for ; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		if filepath.Dir(dir) == dir {
			t.Fatal("no go.mod above the package")
		}
	}
	doc, err := os.ReadFile(filepath.Join(dir, ".cursor", "NEAT_STORAGE_GAP_ANALYSIS.md"))
	if err != nil {
		t.Fatal(err)
	}
	idRe := regexp.MustCompile(`^NST-\d{3}$`)
	nameRe := regexp.MustCompile("\\(`([^`]+)`\\)\\s*$")
	rows, want := 0, -1
	builds := map[string]bool{}
	for _, ln := range strings.Split(string(doc), "\n") {
		cols := strings.Split(ln, "|")
		if len(cols) > 2 && strings.TrimSpace(cols[1]) == "Architect cards" {
			var n int
			if _, err := fmt.Sscan(strings.Fields(cols[2])[0], &n); err == nil {
				want = n
			}
		}
		if len(cols) < 11 || !idRe.MatchString(strings.TrimSpace(cols[1])) {
			continue
		}
		m := nameRe.FindStringSubmatch(strings.TrimSpace(cols[2]))
		if m == nil {
			continue // the card-text table has no build name
		}
		id, build, status := strings.TrimSpace(cols[1]), m[1], strings.TrimSpace(cols[9])
		rows++
		builds[build] = true
		var card ToolDef
		for _, d := range Tools {
			if d.Build == build {
				card = d
			}
		}
		switch {
		case card.ID == "":
			t.Errorf("%s: no card builds %q", id, build)
			continue
		case card.Category != CatStorage:
			t.Errorf("%s: card %s is in %q, want %q", id, card.ID, card.Category, CatStorage)
		case card.Planned && (status == "done" || status == "partial"):
			t.Errorf("%s is %s but its card is a placeholder", id, status)
		case card.Desc == "" || card.Icon == "":
			t.Errorf("%s: card %s has no text or icon", id, card.ID)
		}
		k, ok := kindByBuild(build)
		if !ok {
			t.Errorf("%s: no construction kind named %q", id, build)
			continue
		}
		if !k.IsNeatStorage() {
			t.Errorf("%s: %q is not a Neat Storage kind", id, build)
		}
		if hint := strings.Contains(card.Desc, "Q/E"); hint != construction.DefOf(k).Rotatable {
			t.Errorf("%s: card rotation hint %v, piece rotatable %v", id, hint, construction.DefOf(k).Rotatable)
		}
	}
	if rows != 27 {
		t.Fatalf("%d rows parsed, want 27", rows)
	}
	if want != rows {
		t.Errorf("counts table says %d cards, the rows are %d", want, rows)
	}
	for _, d := range Tools {
		if k, ok := kindByBuild(d.Build); ok && k.IsNeatStorage() && !builds[d.Build] {
			t.Errorf("card %s builds Neat piece %q with no row", d.ID, d.Build)
		}
	}
}

func kindByBuild(name string) (construction.Kind, bool) {
	for k := construction.Kind(0); k < 255; k++ {
		if k.Valid() && construction.DefOf(k).Name == name {
			return k, true
		}
	}
	return 0, false
}
