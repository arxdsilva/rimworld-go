package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The ratchet only lets the budget shrink.
func TestSOLIDBudgetRatchet(t *testing.T) {
	t.Parallel()
	app := measure{"methods", "x/ebiten.App", 950}
	initHit := measure{"init", "x/ebiten/menu.go", 0}
	budget := map[string]int{"methods x/ebiten.App": 950, "init x/ebiten/menu.go": 0}
	order := []string{"methods x/ebiten.App", "init x/ebiten/menu.go"}

	if bad := judge([]measure{app, initHit}, budget, order); len(bad) != 0 {
		t.Fatalf("a budget that matches the tree must pass, got %v", bad)
	}
	// Shrinking within the slack passes; shrinking past it asks for a lower line.
	if bad := judge([]measure{{"methods", "x/ebiten.App", 920}, initHit}, budget, order); len(bad) != 0 {
		t.Fatalf("5%% under the ceiling is within the slack, got %v", bad)
	}
	bad := judge([]measure{{"methods", "x/ebiten.App", 800}, initHit}, budget, order)
	if len(bad) != 1 || !strings.Contains(bad[0], "stale; lower it to 800") {
		t.Fatalf("a stale ceiling must ask to be lowered, got %v", bad)
	}
	// Growth fails even under the ceiling's slack.
	bad = judge([]measure{{"methods", "x/ebiten.App", 951}, initHit}, budget, order)
	if len(bad) != 1 || !strings.Contains(bad[0], "grew to 951") {
		t.Fatalf("growth must fail, got %v", bad)
	}
	// A new violation without a line prints the line to paste; a paid debt must be deleted.
	bad = judge([]measure{app, initHit, {"iface", "x/ebiten.Sim", 32}}, budget, order)
	if len(bad) != 1 || !strings.Contains(bad[0], "iface x/ebiten.Sim 32") {
		t.Fatalf("a new debt must name its line, got %v", bad)
	}
	bad = judge([]measure{app}, budget, order)
	if len(bad) != 1 || !strings.Contains(bad[0], "init x/ebiten/menu.go no longer violates") {
		t.Fatalf("a paid presence debt must be deleted, got %v", bad)
	}
	bad = judge([]measure{initHit, {"global", "x/ebiten.cur", 0}}, budget, order)
	if len(bad) != 2 {
		t.Fatalf("a new global and a paid methods line are two violations, got %v", bad)
	}
}

// The budget file rejects malformed and duplicate lines.
func TestSOLIDBudgetFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "b.txt")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	budget, order := readBudget(t, write("# note\nmethods a.B 50  # why\ninit a/f.go\n\n"))
	if budget["methods a.B"] != 50 || budget["init a/f.go"] != 0 || len(order) != 2 {
		t.Fatalf("budget = %v order = %v", budget, order)
	}
	if b, o := readBudget(t, filepath.Join(dir, "missing.txt")); len(b) != 0 || o != nil {
		t.Fatalf("a missing file is an empty budget, got %v %v", b, o)
	}
}
