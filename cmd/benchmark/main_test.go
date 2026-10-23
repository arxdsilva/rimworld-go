package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

func TestParse(t *testing.T) {
	o, err := parse([]string{"-sizes", "120, 250", "-speeds", "3", "-fixtures", "grown", "-ticks", "10"})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.sizes) != 2 || o.sizes[1] != 250 || o.speeds[0] != 3 || o.ticks != 10 || o.fixtures[0] != "grown" {
		t.Fatalf("parsed %+v", o)
	}
	for _, bad := range [][]string{{"-sizes", "x"}, {"-speeds", "0"}, {"-fixtures", "nope"}, {"-ticks", "0"}} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%v: want error", bad)
		}
	}
}

func TestMeasureAndReport(t *testing.T) {
	g := game.NewColony(game.Config{Seed: 1, Width: 48, Height: 40})
	r := measure(g, 7, 3)
	if len(r.ticks) != 7 || len(r.frames) != 3 {
		t.Fatalf("ticks %d frames %d", len(r.ticks), len(r.frames))
	}
	for _, p := range game.Phases() {
		if len(r.phases[p]) != 7 {
			t.Fatalf("phase %s: %d samples", p, len(r.phases[p]))
		}
	}
	var buf bytes.Buffer
	report(&buf, "x", r)
	if !strings.Contains(buf.String(), "movement") || !strings.Contains(buf.String(), "frame") {
		t.Fatalf("report:\n%s", buf.String())
	}
}

func TestPct(t *testing.T) {
	ds := []time.Duration{5, 1, 3, 2, 4}
	if pct(ds, 50) != 3 || pct(ds, 100) != 5 || pct(nil, 50) != 0 || mean(ds) != 3 || mean(nil) != 0 {
		t.Fatal("pct/mean")
	}
}

// TestRunFromCache runs the whole flow on a cached fixture: a fresh
// colony saved where the cache keeps a 120² grown one (building that
// takes a two-day warm-up).
func TestRunFromCache(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { scenario.CacheDir = "" })
	scenario.CacheDir = dir
	var save bytes.Buffer
	if err := game.NewColony(game.Config{Seed: 1, Width: 48, Height: 40}).Save(&save, "fx"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scenario.CacheFile(9, 120, 120, false), save.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := run([]string{"-seed", "9", "-sizes", "120", "-fixtures", "grown", "-ticks", "4", "-speeds", "2", "-hash", "-cache", dir}, &buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fixture grown 120x120", "grown/120/x2", "state hash after 4 ticks"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, buf.String())
		}
	}
	buf.Reset()
	if err := run([]string{"-seed", "9", "-sizes", "120", "-fixtures", "grown", "-build-only", "-cache", dir}, &buf); err != nil || strings.Contains(buf.String(), "tick") {
		t.Fatalf("build-only: %v\n%s", err, buf.String())
	}
}
