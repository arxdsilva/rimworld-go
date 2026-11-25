// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"rimworld-go/internal/application/game"
	appreplay "rimworld-go/internal/application/replay"
)

const usage = "usage: replay verify <file>"

// Main runs the tool with argv (program name first), writes the result to
// out and errors to errOut, and returns the exit code: 0 when the replay
// matches, 1 on the first mismatching checkpoint, 2 on a usage error, an
// unreadable file or an unknown version.
func Main(argv []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet(argv[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	if err := fs.Parse(argv[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	args := fs.Args()
	if len(args) != 2 || args[0] != "verify" {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	return verify(args[1], maxReplayBytes, out, errOut)
}

// maxReplayBytes caps the replay file read into memory (64 MiB).
const maxReplayBytes = 64 << 20

func verify(path string, limit int64, out, errOut io.Writer) int {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(errOut, "replay: %v\n", err)
		return 2
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		fmt.Fprintf(errOut, "replay: %v\n", err)
		return 2
	}
	if int64(len(data)) > limit {
		fmt.Fprintf(errOut, "replay: file exceeds %d bytes\n", limit)
		return 2
	}
	f, err := appreplay.Decode(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(errOut, "replay: %v\n", err)
		return 2
	}
	err = appreplay.Verify(f, startColony)
	var m *appreplay.Mismatch
	switch {
	case errors.As(err, &m):
		fmt.Fprintf(out, "replay: checkpoint tick %d: hash %s, want %s\n", m.Tick, m.Got, m.Want)
		return 1
	case err != nil:
		fmt.Fprintf(errOut, "replay: %v\n", err)
		return 2
	}
	tick, hash := f.Ticks, "-"
	if n := len(f.Checkpoints); n > 0 {
		tick, hash = f.Checkpoints[n-1].Tick, f.Checkpoints[n-1].Hash
	}
	fmt.Fprintf(out, "ok: %d checkpoints, %d commands, tick %d, hash %s\n", len(f.Checkpoints), len(f.Commands), tick, hash)
	return 0
}

func startColony(o appreplay.Options) appreplay.Sim {
	return game.NewColony(game.Config{Seed: o.Seed, Width: o.Width, Height: o.Height})
}
