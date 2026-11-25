// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package simulate

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/replay"
)

// Main runs the tool with argv (program name first), writes the final colony
// state to out and flag errors to errOut, and returns the exit code.
func Main(argv []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet(argv[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	seed := fs.Uint64("seed", 1, "deterministic RNG seed")
	record := fs.String("record", "", "write a replay file to this path")
	every := fs.Int("every", 100, "ticks between replay checkpoints (with -record)")
	ticks := fs.Int("ticks", 100, "number of simulation ticks to run")
	width := fs.Int("width", 20, "map width in tiles")
	height := fs.Int("height", 20, "map height in tiles")
	if err := fs.Parse(argv[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	opts := replay.Options{Seed: *seed, Width: *width, Height: *height}
	cfg := game.Config{Seed: opts.Seed, Width: opts.Width, Height: opts.Height}
	if *record == "" {
		print(out, game.NewColony(cfg), *ticks)
		return 0
	}
	if *every < 0 {
		fmt.Fprintln(errOut, "simulate: -every must not be negative")
		return 2
	}
	if err := replay.CheckOptions(opts); err != nil {
		fmt.Fprintf(errOut, "simulate: %v\n", err)
		return 2
	}
	g := game.NewColony(cfg)
	rec := replay.NewRecorder(g, opts, uint64(*every))
	rec.Step(*ticks)
	print(out, g, 0)
	return writeReplay(*record, rec.File(), out, errOut)
}

// writeReplay writes f to path. A recording outside the limits replay.Decode
// enforces is refused before the file is created.
func writeReplay(path string, f replay.File, out, errOut io.Writer) int {
	err := replay.CheckLimits(f)
	var file *os.File
	if err == nil {
		file, err = os.Create(filepath.Clean(path))
	}
	if err == nil {
		err = replay.Encode(file, f)
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		fmt.Fprintf(errOut, "simulate: %v\n", err)
		return 1
	}
	hash := "-"
	if n := len(f.Checkpoints); n > 0 {
		hash = f.Checkpoints[n-1].Hash
	}
	fmt.Fprintf(out, "replay %s checkpoints=%d hash=%s\n", path, len(f.Checkpoints), hash)
	return 0
}

func print(out io.Writer, g *game.Game, ticks int) {
	g.Step(ticks)
	fmt.Fprintf(out, "tick=%d pawns=%d food=%d wood=%d\n", g.Tick(), len(g.Snapshot()), g.World().Food, g.World().Wood)
	for _, p := range g.Snapshot() {
		fmt.Fprintf(out, "pawn %s: (%d, %d) job=%s hunger=%d rest=%d %s\n", p.Name, p.X, p.Y, p.Job, p.Hunger, p.Rest, p.Status)
	}
	fmt.Fprintf(out, "hash=0x%016x\n", g.StateHash())
}
