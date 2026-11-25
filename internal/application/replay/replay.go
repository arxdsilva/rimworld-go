// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay

import (
	"fmt"
	"sort"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// Version is the replay file format version this package writes and reads.
const Version = 1

// Sim is the simulation a replay records and re-runs.
type Sim interface {
	Apply(command.Command)
	Step(n int)
	Tick() clock.Tick
	StateHash() uint64
}

// Start builds a fresh Sim from recorded Options.
type Start func(Options) Sim

// Options are the inputs that build the recorded Sim.
type Options struct {
	Seed   uint64 `json:"seed"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Entry is one submitted command: At is the tick it was submitted at and Seq
// its recorder ordinal, starting at 1.
type Entry struct {
	At  uint64          `json:"at"`
	Seq uint64          `json:"seq"`
	Cmd command.Command `json:"cmd"`
}

// Checkpoint is the Sim's state hash at Tick, as 16 lowercase hex digits.
type Checkpoint struct {
	Tick uint64 `json:"tick"`
	Hash string `json:"hash"`
}

// File is a recorded run: entries sorted by (At, Seq) and checkpoints with
// strictly increasing ticks, none after Ticks.
type File struct {
	Version     int          `json:"version"`
	Options     Options      `json:"options"`
	Ticks       uint64       `json:"ticks"`
	Every       uint64       `json:"every"`
	Commands    []Entry      `json:"commands"`
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// Recorder wraps a Sim, logging submitted commands and checkpoints.
type Recorder struct {
	sim     Sim
	opts    Options
	every   uint64
	seq     uint64
	entries []Entry
	points  []Checkpoint
}

// NewRecorder wraps sim, checkpointing its current tick at once and then
// every ticks (0: only the first and the final tick).
func NewRecorder(sim Sim, o Options, every uint64) *Recorder {
	r := &Recorder{sim: sim, opts: o, every: every}
	r.checkpoint()
	return r
}

func (r *Recorder) checkpoint() {
	r.points = append(r.points, Checkpoint{Tick: uint64(r.sim.Tick()), Hash: hashString(r.sim.StateHash())})
}

// Apply records c at the current tick and forwards it unchanged to the Sim;
// a command of Kind None is skipped.
func (r *Recorder) Apply(c command.Command) {
	if c.Kind == command.None {
		return
	}
	r.seq++
	r.entries = append(r.entries, Entry{At: uint64(r.sim.Tick()), Seq: r.seq, Cmd: c})
	r.sim.Apply(c)
}

// Step advances the Sim n ticks one at a time, checkpointing on every
// multiple of the recorder's interval.
func (r *Recorder) Step(n int) {
	for i := 0; i < n; i++ {
		r.sim.Step(1)
		if t := uint64(r.sim.Tick()); r.every > 0 && t%r.every == 0 && r.lastPoint() != t {
			r.checkpoint()
		}
	}
}

func (r *Recorder) lastPoint() uint64 {
	if len(r.points) == 0 {
		return 0
	}
	return r.points[len(r.points)-1].Tick
}

// File returns the recorded run with Ticks set to the current tick. A final
// checkpoint is added only when that tick has none and no command was
// submitted at it (its hash would include still-pending commands).
func (r *Recorder) File() File {
	ticks := uint64(r.sim.Tick())
	f := File{
		Version:     Version,
		Options:     r.opts,
		Ticks:       ticks,
		Every:       r.every,
		Commands:    append([]Entry(nil), r.entries...),
		Checkpoints: append([]Checkpoint(nil), r.points...),
	}
	if r.lastPoint() == ticks {
		return f
	}
	for _, e := range r.entries {
		if e.At == ticks {
			return f
		}
	}
	f.Checkpoints = append(f.Checkpoints, Checkpoint{Tick: ticks, Hash: hashString(r.sim.StateHash())})
	return f
}

// Mismatch is the first checkpoint whose re-run hash differs.
type Mismatch struct {
	Tick      uint64
	Want, Got string
}

// Error names the mismatching tick and both hashes.
func (m *Mismatch) Error() string {
	return "replay: state hash mismatch at tick " + utoa(m.Tick) + ": want " + m.Want + ", got " + m.Got
}

// Verify re-runs f on a Sim built by start and returns a *Mismatch at the
// first checkpoint whose hash differs, or nil when every checkpoint matches.
func Verify(f File, start Start) error {
	if err := checkLimits(f); err != nil {
		return err
	}
	sim := start(f.Options)
	entries := append([]Entry(nil), f.Commands...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].At != entries[j].At {
			return entries[i].At < entries[j].At
		}
		return entries[i].Seq < entries[j].Seq
	})
	first := uint64(sim.Tick())
	if f.Ticks < first {
		return fmt.Errorf("replay: ticks %d is before the start tick %d", f.Ticks, first)
	}
	if f.Ticks-first > MaxTicks {
		return fmt.Errorf("replay: ticks %d is more than %d past the start tick %d", f.Ticks, uint64(MaxTicks), first)
	}
	for i, c := range f.Checkpoints {
		if c.Tick < first {
			return fmt.Errorf("replay: checkpoint %d at tick %d is before the start tick %d", i, c.Tick, first)
		}
	}
	for i, e := range entries {
		if e.At < first {
			return fmt.Errorf("replay: command %d at tick %d is before the start tick %d", i, e.At, first)
		}
	}
	ei, ci := 0, 0
	// The Sim may start past tick 0 (a colony starts mid-morning), so the
	// walk begins at its own first tick, where the first checkpoint was taken.
	// The loop breaks at Ticks before incrementing, so it cannot wrap.
	for t := first; t <= f.Ticks; t++ {
		for ci < len(f.Checkpoints) && f.Checkpoints[ci].Tick < t {
			ci++
		}
		if ci < len(f.Checkpoints) && f.Checkpoints[ci].Tick == t {
			if got := hashString(sim.StateHash()); got != f.Checkpoints[ci].Hash {
				return &Mismatch{Tick: t, Want: f.Checkpoints[ci].Hash, Got: got}
			}
		}
		for ei < len(entries) && entries[ei].At == t {
			sim.Apply(entries[ei].Cmd)
			ei++
		}
		if t == f.Ticks {
			break
		}
		sim.Step(1)
	}
	return nil
}
