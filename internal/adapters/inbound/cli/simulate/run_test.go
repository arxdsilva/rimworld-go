// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package simulate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/replay"
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

func TestMainRecordWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.replay")
	var plain, rec, errOut bytes.Buffer
	Main([]string{"simulate", "-ticks", "120"}, &plain, &errOut)
	if code := Main([]string{"simulate", "-ticks", "120", "-record", path, "-every", "50"}, &rec, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	body, extra, _ := strings.Cut(rec.String(), "replay "+path)
	if body != plain.String() {
		t.Fatalf("colony output changed:\n%q\n%q", body, plain.String())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	file, err := replay.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	n := len(file.Checkpoints)
	want := fmt.Sprintf(" checkpoints=%d hash=%s\n", n, file.Checkpoints[n-1].Hash)
	if extra != want {
		t.Fatalf("line %q, want %q", extra, want)
	}
}

func TestMainRecordErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	bad := filepath.Join(t.TempDir(), "no", "dir", "r.replay")
	if code := Main([]string{"simulate", "-ticks", "3", "-record", bad}, &out, &errOut); code != 1 || errOut.Len() == 0 {
		t.Fatalf("unwritable: exit %d err %q", code, errOut.String())
	}
	errOut.Reset()
	if code := Main([]string{"simulate", "-record", bad, "-every", "-1"}, &out, &errOut); code != 2 {
		t.Fatalf("negative every: exit %d", code)
	}
}

// TestMainRecordRejectsOversizedMap: -record refuses a map size the replay
// decoder would reject, before building the colony or creating the file.
func TestMainRecordRejectsOversizedMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.replay")
	for _, args := range [][]string{
		{"-width", fmt.Sprint(replay.MaxSide + 1)},
		{"-height", fmt.Sprint(replay.MaxSide + 1)},
		{"-width", "0"},
	} {
		var out, errOut bytes.Buffer
		argv := append([]string{"simulate", "-ticks", "1", "-record", path}, args...)
		if code := Main(argv, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "options.") {
			t.Fatalf("%v: exit %d err %q", args, code, errOut.String())
		}
		if out.Len() != 0 {
			t.Fatalf("%v: colony ran: %q", args, out.String())
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%v: replay file created", args)
		}
	}
}

// TestWriteReplayRefusesUndecodable: a recording past the decoder's
// checkpoint limit is refused and no file is written.
func TestWriteReplayRefusesUndecodable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.replay")
	f := replay.File{Version: replay.Version, Options: replay.Options{Width: 1, Height: 1}, Ticks: replay.MaxCheckpoints}
	for i := 0; i <= replay.MaxCheckpoints; i++ {
		f.Checkpoints = append(f.Checkpoints, replay.Checkpoint{Tick: uint64(i), Hash: "0000000000000000"})
	}
	var out, errOut bytes.Buffer
	if code := writeReplay(path, f, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "checkpoints") {
		t.Fatalf("exit %d err %q", code, errOut.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("replay file created")
	}
}
