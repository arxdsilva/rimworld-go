// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package simulate

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func run(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Main(append([]string{"simulate"}, args...), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	return out.String()
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestMainPrintsColony(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Main([]string{"simulate", "-ticks", "5"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "tick=") {
		t.Fatalf("first line %q", lines[0])
	}
	pawns := 0
	for _, l := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(l, "pawn ") {
			t.Fatalf("middle line %q", l)
		}
		pawns++
	}
	if pawns == 0 {
		t.Fatalf("no pawn lines: %q", out.String())
	}
	if code := Main([]string{"simulate", "-nope"}, &out, &errOut); code != 2 {
		t.Fatalf("bad flag exit %d, want 2", code)
	}
}

func TestHashLineMatchesIndependentColony(t *testing.T) {
	got := lastLine(run(t, "-seed", "3", "-ticks", "30", "-width", "24", "-height", "22"))
	if !regexp.MustCompile(`^hash=0x[0-9a-f]{16}$`).MatchString(got) {
		t.Fatalf("last line %q", got)
	}
	g := game.NewColony(game.Config{Seed: 3, Width: 24, Height: 22})
	g.Step(30)
	if want := fmt.Sprintf("hash=0x%016x", g.StateHash()); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHashLineDeterministic(t *testing.T) {
	a := run(t, "-seed", "1", "-ticks", "50")
	if b := run(t, "-seed", "1", "-ticks", "50"); a != b {
		t.Fatalf("runs differ:\n%s\n%s", a, b)
	}
	if c := run(t, "-seed", "2", "-ticks", "50"); lastLine(c) == lastLine(a) {
		t.Fatalf("seed 2 hash equals seed 1: %q", lastLine(c))
	}
}
