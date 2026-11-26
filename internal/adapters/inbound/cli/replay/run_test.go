// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package replay

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	appreplay "rimworld-go/internal/application/replay"
	"rimworld-go/internal/domain/sim/command"
)

func record(t *testing.T) string {
	t.Helper()
	o := appreplay.Options{Seed: 7, Width: 24, Height: 24}
	g := game.NewColony(game.Config{Seed: o.Seed, Width: o.Width, Height: o.Height})
	r := appreplay.NewRecorder(g, o, 50)
	r.Step(20)
	r.Apply(command.Command{Kind: command.DesignateStockpile, X: 3, Y: 3, X2: 5, Y2: 5})
	r.Step(60)
	r.Apply(command.Command{Kind: command.DesignateWall, X: 8, Y: 8, X2: 12, Y2: 8})
	r.Step(120)
	path := filepath.Join(t.TempDir(), "run.replay")
	var buf bytes.Buffer
	if err := appreplay.Encode(&buf, r.File()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Main(append([]string{"replay"}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestMainRecordThenVerify(t *testing.T) {
	code, out, errOut := run("verify", record(t))
	if code != 0 || !strings.HasPrefix(out, "ok: ") || !strings.Contains(out, "2 commands") {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestMainTamperedExits1(t *testing.T) {
	path := record(t)
	b, _ := os.ReadFile(path)
	f, err := appreplay.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	tick := f.Checkpoints[2].Tick
	f.Checkpoints[2].Hash = "0000000000000000"
	var buf bytes.Buffer
	if err := appreplay.Encode(&buf, f); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("verify", path)
	want := "replay: checkpoint tick " + strconv.FormatUint(tick, 10) + ": hash "
	if code != 1 || !strings.HasPrefix(out, want) {
		t.Fatalf("exit %d out %q, want prefix %q", code, out, want)
	}
}

func TestMainBadInput(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := record(t)
	b, _ := os.ReadFile(good)
	cases := map[string][]string{
		"no args":     {},
		"no file":     {"verify"},
		"extra":       {"verify", good, "x"},
		"unknown sub": {"frobnicate", good},
		"bad flag":    {"-nope"},
		"missing":     {"verify", filepath.Join(dir, "absent")},
		"bad version": {"verify", write("v.json", versionBump(string(b)))},
		"malformed":   {"verify", write("m.json", "{not json")},
	}
	for name, args := range cases {
		if code, _, errOut := run(args...); code != 2 || errOut == "" {
			t.Errorf("%s: exit %d err %q, want 2 and a message", name, code, errOut)
		}
	}
	if code, _, _ := run("-h"); code != 0 {
		t.Errorf("-h exit %d, want 0", code)
	}
}

func TestVerifyFileSizeCap(t *testing.T) {
	path := record(t)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := verify(path, st.Size()-1, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "replay: file exceeds ") {
		t.Fatalf("over cap: exit %d err %q", code, errOut.String())
	}
	errOut.Reset()
	if code := verify(path, st.Size(), &out, &errOut); code != 0 {
		t.Fatalf("at cap: exit %d err %q", code, errOut.String())
	}
}

func TestMainPreStartCommandExits2(t *testing.T) {
	path := record(t)
	b, _ := os.ReadFile(path)
	f, _ := appreplay.Decode(bytes.NewReader(b))
	f.Commands[0].At = 1
	f.Checkpoints = f.Checkpoints[:0]
	var buf bytes.Buffer
	_ = appreplay.Encode(&buf, f)
	_ = os.WriteFile(path, buf.Bytes(), 0o600)
	if code, _, errOut := run("verify", path); code != 2 {
		t.Fatalf("exit %d err %q, want 2", code, errOut)
	}
}

func versionBump(s string) string {
	return strings.Replace(s, `"version": 1`, `"version": 99`, 1)
}
