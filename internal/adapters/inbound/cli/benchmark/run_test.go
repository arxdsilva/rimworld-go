// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package benchmark

import (
	"bytes"
	"os"
	"runtime"
	"slices"
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
	if !slices.Equal(o.colonists, []int{scenario.DefaultColonists}) || scenario.DefaultColonists != 20 {
		t.Fatalf("default colonists %v", o.colonists)
	}
	o, err = parse([]string{"-colonists", "3, 100"})
	if err != nil || !slices.Equal(o.colonists, []int{3, 100}) {
		t.Fatalf("colonists %v, %v", o.colonists, err)
	}
	for _, bad := range [][]string{{"-sizes", "x"}, {"-speeds", "0"}, {"-fixtures", "nope"}, {"-ticks", "0"},
		{"-colonists", "2"}, {"-colonists", "0"}, {"-colonists", "x"}, {"-colonists", "20,2"}} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%v: want error", bad)
		}
	}
	if _, err := parse([]string{"-colonists", "2"}); err == nil || !strings.Contains(err.Error(), "-colonists: 2 below 3") {
		t.Errorf("below-minimum error: %v", err)
	}
}

func TestPerTick(t *testing.T) {
	before := runtime.MemStats{Mallocs: 100, TotalAlloc: 1000}
	after := runtime.MemStats{Mallocs: 400, TotalAlloc: 4000}
	m, b := allocDelta(&before, &after)
	if m != 300 || b != 3000 {
		t.Fatalf("allocDelta = %d, %d", m, b)
	}
	for _, c := range []struct {
		m, b  uint64
		ticks int
		o, by float64
	}{{300, 3000, 3, 100, 1000}, {5, 10, 4, 1.25, 2.5}, {7, 7, 0, 0, 0}, {0, 0, 5, 0, 0}} {
		if o, by := perTick(c.m, c.b, c.ticks); o != c.o || by != c.by {
			t.Errorf("perTick(%d,%d,%d) = %v, %v; want %v, %v", c.m, c.b, c.ticks, o, by, c.o, c.by)
		}
	}
	var buf bytes.Buffer
	report(&buf, "x", result{ticks: make([]time.Duration, 3), mallocs: 300, allocBytes: 3000})
	if !strings.Contains(buf.String(), "  allocs/tick 100.0 objects, 1000 B\n") {
		t.Fatalf("report:\n%s", buf.String())
	}
}

// TestMeasurePreallocates pins that the sample slices are sized up front
// (capacity, not length), so the harness adds no allocations in the loop.
func TestMeasurePreallocates(t *testing.T) {
	g := game.NewColony(game.Config{Seed: 1, Width: 48, Height: 40})
	r := measure(g, 7, 3)
	if cap(r.ticks) != 7 || cap(r.frames) != 3 || cap(r.views) != 3 {
		t.Fatalf("caps ticks %d frames %d views %d", cap(r.ticks), cap(r.frames), cap(r.views))
	}
	for _, p := range game.Phases() {
		if cap(r.phases[p]) != 7 {
			t.Fatalf("phase %s cap %d", p, cap(r.phases[p]))
		}
	}
	if r.mallocs == 0 || r.allocBytes == 0 {
		t.Fatalf("no allocations seen: %d, %d", r.mallocs, r.allocBytes)
	}
}

func TestMainFixtureErrors(t *testing.T) {
	t.Cleanup(func() { scenario.CacheDir = "" })
	for _, args := range [][]string{
		{"-colonists", "99999", "-sizes", "110", "-fixtures", "grown", "-cache", t.TempDir()},
		{"-sizes", "100", "-fixtures", "grown", "-cache", t.TempDir()},
		{"-colonists", "2"},
	} {
		var out, errOut bytes.Buffer
		if code := Main(args, &out, &errOut); code != 1 {
			t.Errorf("%v: exit %d, want 1", args, code)
		}
		if !strings.HasPrefix(errOut.String(), "benchmark: ") {
			t.Errorf("%v: errOut %q", args, errOut.String())
		}
		if args[0] != "-colonists" || args[1] != "2" {
			if !strings.Contains(errOut.String(), "benchmark: grown colony:") {
				t.Errorf("%v: errOut %q lacks grown colony prefix", args, errOut.String())
			}
		}
	}
}

func TestMeasureAndReport(t *testing.T) {
	g := game.NewColony(game.Config{Seed: 1, Width: 48, Height: 40})
	r := measure(g, 7, 3)
	if len(r.ticks) != 7 || len(r.frames) != 3 || len(r.views) != 3 {
		t.Fatalf("ticks %d frames %d views %d", len(r.ticks), len(r.frames), len(r.views))
	}
	for _, p := range game.Phases() {
		if len(r.phases[p]) != 7 {
			t.Fatalf("phase %s: %d samples", p, len(r.phases[p]))
		}
	}
	var buf bytes.Buffer
	report(&buf, "x", r)
	if !strings.Contains(buf.String(), "movement") || !strings.Contains(buf.String(), "frame") || !strings.Contains(buf.String(), "views") {
		t.Fatalf("report:\n%s", buf.String())
	}
}

func TestOverBudget(t *testing.T) {
	fs := []time.Duration{frameBudget, frameBudget + 1, 8 * time.Millisecond, time.Millisecond}
	if got := overBudget(fs); got != 2 {
		t.Fatalf("overBudget = %d, want 2", got)
	}
	var buf bytes.Buffer
	report(&buf, "x", result{frames: fs})
	if !strings.Contains(buf.String(), "frames > 5ms: 2 (50.0%)") {
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
	// a different colony under each count's name, so each run's hash shows
	// which file it read: this also pins that the colonist count is part of
	// scenario's memo key and cache name (a scenario test cannot assign its
	// own package's CacheDir, SOLID §9)
	for i, n := range []int{3, scenario.DefaultColonists} {
		var save bytes.Buffer
		if err := game.NewColony(game.Config{Seed: uint64(i + 1), Width: 48, Height: 40}).Save(&save, "fx"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scenario.CacheFile(9, 120, 120, n, false), save.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := run([]string{"-seed", "9", "-sizes", "120", "-colonists", "3,20", "-fixtures", "grown", "-ticks", "4", "-speeds", "2", "-hash", "-cache", dir}, &buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fixture grown 120x120 c3", "fixture grown 120x120 c20", "grown/120/c3/x2", "grown/120/c20/x2", "state hash after 4 ticks"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, buf.String())
		}
	}
	if got := strings.Count(buf.String(), "allocs/tick"); got != 2 {
		t.Fatalf("allocs/tick lines = %d, want 2:\n%s", got, buf.String())
	}
	var hashes []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "state hash after") {
			hashes = append(hashes, l)
		}
	}
	if len(hashes) != 2 || hashes[0] == hashes[1] {
		t.Fatalf("c3 and c20 runs must read their own cache files, hash lines %q", hashes)
	}
	buf.Reset()
	if err := run([]string{"-seed", "9", "-sizes", "120", "-fixtures", "grown", "-build-only", "-cache", dir}, &buf); err != nil || strings.Contains(buf.String(), "tick") {
		t.Fatalf("build-only: %v\n%s", err, buf.String())
	}
}
