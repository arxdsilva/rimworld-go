// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/replay"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// fakeSim hashes its tick and the commands applied so far, order sensitive.
type fakeSim struct {
	tick uint64
	h    uint64
}

func (s *fakeSim) Apply(c command.Command) {
	s.h = s.h*1099511628211 + uint64(c.Kind)*131 + uint64(c.X) + 7
}
func (s *fakeSim) Step(n int)             { s.tick += uint64(n) }
func (s *fakeSim) Tick() clock.Tick       { return clock.Tick(s.tick) }
func (s *fakeSim) StateHash() uint64      { return s.h ^ s.tick*0x9e3779b97f4a7c15 }
func startFake(replay.Options) replay.Sim { return &fakeSim{} }

func ticksOf(f replay.File) []uint64 {
	var out []uint64
	for _, c := range f.Checkpoints {
		out = append(out, c.Tick)
	}
	return out
}

func eq(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRecordVerifyFake(t *testing.T) {
	mv := func(x int) command.Command { return command.Command{Kind: command.MovePawn, X: x} }
	tests := []struct {
		name  string
		every uint64
		run   func(r *replay.Recorder)
		want  []uint64
		cmds  int
	}{
		{"ticks not multiple", 10, func(r *replay.Recorder) { r.Apply(mv(1)); r.Step(25) }, []uint64{0, 10, 20, 25}, 1},
		{"apply after last step", 10, func(r *replay.Recorder) { r.Step(20); r.Apply(mv(2)); r.Apply(mv(3)) }, []uint64{0, 10, 20}, 2},
		{"apply after non-multiple step", 10, func(r *replay.Recorder) { r.Step(15); r.Apply(mv(2)) }, []uint64{0, 10}, 1},
		{"every zero", 0, func(r *replay.Recorder) { r.Apply(mv(1)); r.Step(7); r.Apply(mv(2)); r.Step(3) }, []uint64{0, 10}, 2},
		{"none skipped", 5, func(r *replay.Recorder) { r.Apply(command.Command{}); r.Step(5); r.Apply(mv(4)) }, []uint64{0, 5}, 1},
		{"no steps", 5, func(r *replay.Recorder) {}, []uint64{0}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := replay.NewRecorder(&fakeSim{}, replay.Options{Seed: 1, Width: 1, Height: 1}, tc.every)
			tc.run(r)
			f := r.File()
			if got := ticksOf(f); !eq(got, tc.want) {
				t.Fatalf("checkpoints %v, want %v", got, tc.want)
			}
			if len(f.Commands) != tc.cmds {
				t.Fatalf("commands %d, want %d", len(f.Commands), tc.cmds)
			}
			for i, e := range f.Commands {
				if e.Seq != uint64(i+1) {
					t.Fatalf("entry %d seq %d", i, e.Seq)
				}
			}
			if f.Version != replay.Version || f.Every != tc.every {
				t.Fatalf("header %+v", f)
			}
			if err := replay.Verify(f, startFake); err != nil {
				t.Fatalf("verify: %v", err)
			}
			var buf bytes.Buffer
			if err := replay.Encode(&buf, f); err != nil {
				t.Fatal(err)
			}
			if _, err := replay.Decode(&buf); err != nil {
				t.Fatalf("decode: %v", err)
			}
		})
	}
}

// TestRecordCheckpointLimit pins the boundary: checkpointing every tick,
// 65535 steps give exactly MaxCheckpoints (the initial one included) and
// pass CheckLimits and Decode; one more step is refused by CheckLimits.
func TestRecordCheckpointLimit(t *testing.T) {
	r := replay.NewRecorder(&fakeSim{}, replay.Options{Seed: 1, Width: 1, Height: 1}, 1)
	r.Step(replay.MaxCheckpoints - 1)
	f := r.File()
	if len(f.Checkpoints) != replay.MaxCheckpoints {
		t.Fatalf("checkpoints %d, want %d", len(f.Checkpoints), replay.MaxCheckpoints)
	}
	if err := replay.CheckLimits(f); err != nil {
		t.Fatalf("at the limit: %v", err)
	}
	var buf bytes.Buffer
	if err := replay.Encode(&buf, f); err != nil {
		t.Fatal(err)
	}
	if _, err := replay.Decode(&buf); err != nil {
		t.Fatalf("decode at the limit: %v", err)
	}
	r.Step(1)
	if err := replay.CheckLimits(r.File()); err == nil || !strings.Contains(err.Error(), "checkpoints") {
		t.Fatalf("past the limit: err %v, want a checkpoints error", err)
	}
}

func TestCheckOptions(t *testing.T) {
	for _, o := range []replay.Options{{Width: 1, Height: 1}, {Width: replay.MaxSide, Height: replay.MaxSide}} {
		if err := replay.CheckOptions(o); err != nil {
			t.Fatalf("%+v: %v", o, err)
		}
	}
	for _, o := range []replay.Options{{Width: 0, Height: 1}, {Width: replay.MaxSide + 1, Height: 1}, {Width: 1, Height: replay.MaxSide + 1}} {
		if replay.CheckOptions(o) == nil {
			t.Fatalf("%+v accepted", o)
		}
	}
}

func TestRecordEntryTicks(t *testing.T) {
	r := replay.NewRecorder(&fakeSim{}, replay.Options{}, 10)
	r.Apply(command.Command{Kind: command.MovePawn, X: 1})
	r.Step(12)
	r.Apply(command.Command{Kind: command.MovePawn, X: 2})
	f := r.File()
	if f.Commands[0].At != 0 || f.Commands[1].At != 12 || f.Ticks != 12 {
		t.Fatalf("%+v", f)
	}
}

// orderSim records the exact call sequence Verify makes.
type orderSim struct {
	fakeSim
	calls *[]string
}

func (s *orderSim) Apply(c command.Command) {
	*s.calls = append(*s.calls, "A"+string(rune('0'+c.X)))
	s.fakeSim.Apply(c)
}
func (s *orderSim) Step(n int) { *s.calls = append(*s.calls, "S"); s.fakeSim.Step(n) }

func TestVerifyReplayOrder(t *testing.T) {
	f := replay.File{Version: 1, Options: replay.Options{Width: 1, Height: 1}, Ticks: 2, Commands: []replay.Entry{
		{At: 1, Seq: 2, Cmd: command.Command{Kind: command.MovePawn, X: 2}},
		{At: 0, Seq: 1, Cmd: command.Command{Kind: command.MovePawn, X: 1}},
		{At: 1, Seq: 3, Cmd: command.Command{Kind: command.MovePawn, X: 3}},
		{At: 2, Seq: 4, Cmd: command.Command{Kind: command.MovePawn, X: 4}},
	}}
	var calls []string
	_ = replay.Verify(f, func(replay.Options) replay.Sim { return &orderSim{calls: &calls} })
	got := ""
	for _, c := range calls {
		got += c
	}
	if want := "A1SA2A3SA4"; got != want {
		t.Fatalf("calls %s, want %s", got, want)
	}
}

func TestVerifyBeforeStartTick(t *testing.T) {
	late := func(replay.Options) replay.Sim { return &fakeSim{tick: 50} }
	mv := command.Command{Kind: command.MovePawn}
	o := replay.Options{Width: 1, Height: 1}
	for name, f := range map[string]replay.File{
		"entry":      {Version: 1, Options: o, Ticks: 60, Commands: []replay.Entry{{At: 10, Seq: 1, Cmd: mv}}},
		"checkpoint": {Version: 1, Options: o, Ticks: 60, Checkpoints: []replay.Checkpoint{{Tick: 10, Hash: "0000000000000000"}}},
	} {
		err := replay.Verify(f, late)
		var m *replay.Mismatch
		if err == nil || errors.As(err, &m) {
			t.Fatalf("%s: err %v, want a start-tick error", name, err)
		}
	}
}

func TestVerifyHugeTicksNoWrap(t *testing.T) {
	f := replay.File{Version: 1, Options: replay.Options{Width: 1, Height: 1}, Ticks: math.MaxUint64}
	started := false
	err := replay.Verify(f, func(replay.Options) replay.Sim { started = true; return &fakeSim{} })
	if err == nil || !strings.Contains(err.Error(), "ticks") || started {
		t.Fatalf("err %v started %v", err, started)
	}
	// Within the limit but ending at the top of the range must also stop.
	f = replay.File{Version: 1, Options: replay.Options{Width: 1, Height: 1}, Ticks: 3}
	if err := replay.Verify(f, startFake); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyHugeFirstCheckpoint(t *testing.T) {
	base := uint64(1) << 60
	f := replay.File{
		Version: 1, Options: replay.Options{Width: 1, Height: 1}, Ticks: base + 5,
		Checkpoints: []replay.Checkpoint{{Tick: base, Hash: "0000000000000000"}},
	}
	err := replay.Verify(f, func(replay.Options) replay.Sim { return &fakeSim{tick: 20000} })
	if err == nil || !strings.Contains(err.Error(), "past the start tick") {
		t.Fatalf("err %v", err)
	}
	f.Ticks = 10 // before the start tick
	f.Checkpoints = nil
	if err := replay.Verify(f, func(replay.Options) replay.Sim { return &fakeSim{tick: 20000} }); err == nil {
		t.Fatal("ticks before start must fail")
	}
}

func TestMismatchError(t *testing.T) {
	var err error = &replay.Mismatch{Tick: 3, Want: "a", Got: "b"}
	var m *replay.Mismatch
	if !errors.As(err, &m) || err.Error() == "" {
		t.Fatal("bad mismatch")
	}
}

func colonyStart(o replay.Options) replay.Sim {
	return game.NewColony(game.Config{Seed: o.Seed, Width: o.Width, Height: o.Height})
}

func recordColony(t *testing.T) replay.File {
	t.Helper()
	o := replay.Options{Seed: 42, Width: 24, Height: 24}
	g := colonyStart(o).(*game.Game)
	r := replay.NewRecorder(g, o, 100)
	ps := g.Colonists()
	if len(ps) < 2 {
		t.Fatal("colony has too few colonists")
	}
	r.Step(10)
	r.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ps[0].ID), X: 3, Y: 3})
	r.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ps[1].ID), X: 5, Y: 6})
	r.Step(140)
	r.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ps[0].ID), X: 8, Y: 9})
	r.Step(150)
	return r.File()
}

func TestRecordVerifyColony(t *testing.T) {
	f := recordColony(t)
	base := f.Checkpoints[0].Tick // a colony starts mid-day, not at tick 0
	if f.Ticks != base+300 || len(f.Commands) < 3 || base%100 != 0 {
		t.Fatalf("base %d ticks %d commands %d", base, f.Ticks, len(f.Commands))
	}
	if got := ticksOf(f); !eq(got, []uint64{base, base + 100, base + 200, base + 300}) {
		t.Fatalf("checkpoints %v", got)
	}
	if err := replay.Verify(f, colonyStart); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTamperedHash(t *testing.T) {
	f := recordColony(t)
	f.Checkpoints[2].Hash = "0000000000000000"
	var m *replay.Mismatch
	if err := replay.Verify(f, colonyStart); !errors.As(err, &m) || m.Tick != f.Checkpoints[0].Tick+200 {
		t.Fatalf("err %v", err)
	}
}

func TestVerifyTamperedCommand(t *testing.T) {
	f := recordColony(t)
	// First command is at tick 10; the first checkpoint after it is base+100.
	f.Commands[0].Cmd.X, f.Commands[0].Cmd.Y = 20, 20
	f.Commands[1].Cmd.X, f.Commands[1].Cmd.Y = 1, 1
	var m *replay.Mismatch
	err := replay.Verify(f, colonyStart)
	if !errors.As(err, &m) {
		t.Fatalf("err %v", err)
	}
	if m.Tick != f.Checkpoints[1].Tick {
		t.Fatalf("mismatch tick %d", m.Tick)
	}
	t.Logf("mismatch at %d", m.Tick)
}
