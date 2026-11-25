// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Limits bound what an untrusted replay file may ask for.
const (
	// MaxTicks bounds the run length: Ticks may exceed the first
	// checkpoint's tick (20000 when there is none, a colony's start) by at
	// most MaxTicks.
	MaxTicks = 1 << 24
	// MaxSide bounds Options.Width and Options.Height, each in [1, MaxSide].
	MaxSide = 1024
	// MaxCommands bounds len(File.Commands).
	MaxCommands = 1 << 20
	// MaxCheckpoints bounds len(File.Checkpoints).
	MaxCheckpoints = 1 << 16

	// defaultStartTick is the span base when a file has no checkpoint.
	defaultStartTick = 20000
)

// CheckOptions rejects a map size outside [1, MaxSide], naming the field. A
// recorder calls it before building the run, so it never records a size
// Decode rejects.
func CheckOptions(o Options) error {
	if o.Width < 1 || o.Width > MaxSide {
		return fmt.Errorf("replay: options.width %d outside [1, %d]", o.Width, MaxSide)
	}
	if o.Height < 1 || o.Height > MaxSide {
		return fmt.Errorf("replay: options.height %d outside [1, %d]", o.Height, MaxSide)
	}
	return nil
}

// CheckLimits rejects a File outside the limits above, naming the field. It
// is the limit check Decode applies, so a recorder calls it before writing a
// file that could not be read back (for example 65536 ticks checkpointed
// every tick: 65537 checkpoints).
func CheckLimits(f File) error {
	if err := CheckOptions(f.Options); err != nil {
		return err
	}
	if len(f.Commands) > MaxCommands {
		return fmt.Errorf("replay: commands has %d entries, max %d", len(f.Commands), MaxCommands)
	}
	if len(f.Checkpoints) > MaxCheckpoints {
		return fmt.Errorf("replay: checkpoints has %d entries, max %d", len(f.Checkpoints), MaxCheckpoints)
	}
	base := uint64(defaultStartTick)
	if len(f.Checkpoints) > 0 {
		base = f.Checkpoints[0].Tick
	}
	if f.Ticks > base && f.Ticks-base > MaxTicks {
		return fmt.Errorf("replay: ticks %d is more than %d past the start tick %d", f.Ticks, uint64(MaxTicks), base)
	}
	return nil
}

func hashString(h uint64) string { return fmt.Sprintf("%016x", h) }

func utoa(v uint64) string { return strconv.FormatUint(v, 10) }

func validHash(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Encode writes f as indented JSON with a trailing newline; the same File
// always gives the same bytes.
func Encode(w io.Writer, f File) error {
	if f.Commands == nil {
		f.Commands = []Entry{}
	}
	if f.Checkpoints == nil {
		f.Checkpoints = []Checkpoint{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("replay: encode: %w", err)
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// Decode reads a File and rejects an unknown version, unknown fields,
// unsorted or out-of-range entries and checkpoints, and malformed hashes.
func Decode(r io.Reader) (File, error) {
	var f File
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("replay: decode: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return File{}, errors.New("replay: decode: trailing data after the JSON object")
	}
	if f.Version != Version {
		return File{}, fmt.Errorf("replay: unsupported version %d, want %d", f.Version, Version)
	}
	if err := CheckLimits(f); err != nil {
		return File{}, err
	}
	for i, c := range f.Checkpoints {
		if !validHash(c.Hash) {
			return File{}, fmt.Errorf("replay: checkpoint %d: malformed hash %q", i, c.Hash)
		}
		if c.Tick > f.Ticks {
			return File{}, fmt.Errorf("replay: checkpoint %d: tick %d exceeds ticks %d", i, c.Tick, f.Ticks)
		}
		if i > 0 && c.Tick <= f.Checkpoints[i-1].Tick {
			return File{}, fmt.Errorf("replay: checkpoint %d: tick %d not strictly increasing", i, c.Tick)
		}
	}
	for i, e := range f.Commands {
		if e.At > f.Ticks {
			return File{}, fmt.Errorf("replay: command %d: at %d exceeds ticks %d", i, e.At, f.Ticks)
		}
		if i > 0 {
			p := f.Commands[i-1]
			if e.At < p.At || (e.At == p.At && e.Seq <= p.Seq) {
				return File{}, fmt.Errorf("replay: command %d: not sorted by (at, seq)", i)
			}
		}
	}
	if len(f.Commands) == 0 {
		f.Commands = nil
	}
	if len(f.Checkpoints) == 0 {
		f.Checkpoints = nil
	}
	return f, nil
}
