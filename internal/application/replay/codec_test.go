// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay_test

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/replay"
	"rimworld-go/internal/domain/sim/command"
)

const h = "00000000000000ab"

func sample() replay.File {
	return replay.File{
		Version: 1, Options: replay.Options{Seed: 9, Width: 4, Height: 5}, Ticks: 10, Every: 5,
		Commands: []replay.Entry{
			{At: 0, Seq: 1, Cmd: command.Command{Kind: command.MovePawn, X: 1, Name: "x"}},
			{At: 3, Seq: 2, Cmd: command.Command{Kind: command.MovePawn, Pawn: 7}},
		},
		Checkpoints: []replay.Checkpoint{{Tick: 0, Hash: h}, {Tick: 5, Hash: h}, {Tick: 10, Hash: h}},
	}
}

func rawFile(body string) string {
	return `{"version":1,"options":{"seed":1,"width":4,"height":4},"ticks":10,"every":0,` + body + `}`
}

func enc(t *testing.T, f replay.File) string {
	t.Helper()
	var b bytes.Buffer
	if err := replay.Encode(&b, f); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	f := sample()
	s := enc(t, f)
	if !strings.HasSuffix(s, "}\n") || s != enc(t, f) {
		t.Fatal("encoding not stable or no trailing newline")
	}
	got, err := replay.Decode(strings.NewReader(s))
	if err != nil || !reflect.DeepEqual(got, f) {
		t.Fatalf("round trip: %v %+v", err, got)
	}
	empty := replay.File{Version: 1, Options: replay.Options{Width: 1, Height: 1}}
	got, err = replay.Decode(strings.NewReader(enc(t, empty)))
	if err != nil || !reflect.DeepEqual(got, empty) {
		t.Fatalf("empty round trip: %v %+v", err, got)
	}
}

func TestDecodeRejects(t *testing.T) {
	mut := func(fn func(*replay.File)) string { f := sample(); fn(&f); return enc(t, f) }
	tests := []struct {
		name, in, want string
	}{
		{"version", mut(func(f *replay.File) { f.Version = 2 }), "version 2"},
		{"unknown field", strings.Replace(enc(t, sample()), `"every"`, `"bogus": 1, "every"`, 1), "bogus"},
		{"not json", "{", "decode"},
		{"width zero", mut(func(f *replay.File) { f.Options.Width = 0 }), "options.width"},
		{"width negative", mut(func(f *replay.File) { f.Options.Width = -3 }), "options.width"},
		{"width huge", mut(func(f *replay.File) { f.Options.Width = replay.MaxSide + 1 }), "options.width"},
		{"height zero", mut(func(f *replay.File) { f.Options.Height = 0 }), "options.height"},
		{"height huge", mut(func(f *replay.File) { f.Options.Height = replay.MaxSide + 1 }), "options.height"},
		{"ticks max uint64", mut(func(f *replay.File) { f.Ticks = math.MaxUint64 }), "ticks"},
		{"ticks past span", mut(func(f *replay.File) { f.Ticks = 5 + replay.MaxTicks + 1 }), "ticks"},
		{"ticks past span no checkpoint", mut(func(f *replay.File) {
			f.Commands, f.Checkpoints, f.Ticks = nil, nil, 20000+replay.MaxTicks+1
		}), "ticks"},
		{"too many commands", rawFile(`"commands": [` + strings.Repeat(`{"at":0,"seq":1,"cmd":{}},`, replay.MaxCommands) + `{"at":0,"seq":1,"cmd":{}}], "checkpoints": []`), "commands"},
		{"too many checkpoints", rawFile(`"commands": [], "checkpoints": [` + strings.Repeat(`{"tick":0,"hash":"`+h+`"},`, replay.MaxCheckpoints) + `{"tick":0,"hash":"` + h + `"}]`), "checkpoints"},
		{"trailing data", enc(t, sample()) + "{}", "trailing"},
		{"cp not increasing", mut(func(f *replay.File) { f.Checkpoints[1].Tick = 0 }), "strictly"},
		{"cp past ticks", mut(func(f *replay.File) { f.Checkpoints[2].Tick = 11 }), "exceeds"},
		{"entry unsorted at", mut(func(f *replay.File) { f.Commands[1].At = 0; f.Commands[0].At = 1 }), "sorted"},
		{"entry unsorted seq", mut(func(f *replay.File) { f.Commands[1].At = 0; f.Commands[1].Seq = 1 }), "sorted"},
		{"entry past ticks", mut(func(f *replay.File) { f.Commands[1].At = 11 }), "exceeds"},
		{"hash short", mut(func(f *replay.File) { f.Checkpoints[0].Hash = "ab" }), "malformed"},
		{"hash upper", mut(func(f *replay.File) { f.Checkpoints[0].Hash = "00000000000000AB" }), "malformed"},
		{"hash non-hex", mut(func(f *replay.File) { f.Checkpoints[0].Hash = "zzzzzzzzzzzzzzzz" }), "malformed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := replay.Decode(strings.NewReader(tc.in))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want containing %q", err, tc.want)
			}
		})
	}
	if _, err := replay.Decode(strings.NewReader(mut(func(f *replay.File) { f.Version = 2 }))); !strings.Contains(err.Error(), "1") {
		t.Fatal("error must name both versions")
	}
}

type failW struct{}

func (failW) Write([]byte) (int, error) { return 0, bytes.ErrTooLarge }

func TestEncodeWriteError(t *testing.T) {
	if replay.Encode(failW{}, sample()) == nil {
		t.Fatal("want error")
	}
}
