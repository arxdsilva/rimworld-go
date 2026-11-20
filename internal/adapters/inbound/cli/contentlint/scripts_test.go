// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package contentlint

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/application/modding"
)

type memSource []modding.Plugin

func (m memSource) Plugins() ([]modding.Plugin, error) { return m, nil }

func scripted(scripts ...modding.Blob) Open {
	return func(...string) modding.Source {
		return memSource{{Manifest: modding.Manifest{ID: "scripty", Name: "Scripty", Version: "1.0.0", Game: []string{"0.1"}},
			Scripts: scripts}}
	}
}

func TestScriptSyntaxErrorsHaveFileAndLine(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	code := Main([]string{"-q", "-game", "0.1.0"}, &out, &errOut, scripted(
		modding.Blob{Path: "scripts/good.star", Data: []byte("def on_day(ctx):\n    ctx.message(\"hi\")\n")},
		modding.Blob{Path: "scripts/bad.star", Data: []byte("def on_day(ctx):\n    ctx.message(\"hi\"\n\nx = 1\n")},
		modding.Blob{Path: "scripts/names.star", Data: []byte("def on_day(ctx):\n    pass\n\ndef on_hour(ctx):\n    undefined_thing()\n")},
	), nil, "dev")
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s%s", code, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{"scripty: scripts/bad.star:4: script: ", "scripty: scripts/names.star:5: script: undefined: undefined_thing"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "good.star") {
		t.Errorf("a good script is reported:\n%s", got)
	}
}

func TestShippedExampleScriptsLintClean(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	open := func(roots ...string) modding.Source { return modloader.New(roots...) }
	mods := filepath.Join("..", "..", "..", "..", "..", "mods")
	if code := Main([]string{"-q", "-mods", mods, "-enable", "example_frontier", "-game", "0.1.35"}, &out, &errOut, open, nil, "0.1.35"); code != 0 || strings.Contains(out.String(), ".star") {
		t.Fatalf("exit %d:\n%s%s", code, out.String(), errOut.String())
	}
}

func TestScriptOptionsMatchTheHost(t *testing.T) {
	t.Parallel()
	if !reflect.DeepEqual(scriptOptions(), starlarkhost.Options()) {
		t.Fatalf("contentlint compiles scripts with %+v, the game with %+v", scriptOptions(), starlarkhost.Options())
	}
}

func TestScriptPawnHooksLintClean(t *testing.T) {
	t.Parallel()
	src := "def on_pawns(ctx, pawns):\n    ctx.message(str(len(pawns)))\n\n" +
		"def on_pawn(ctx, pawn):\n    first = ctx.pawns()\n    other = ctx.pawn(1)\n    if other != None and len(first) > 0:\n        ctx.message(pawn.name)\n"
	var out, errOut bytes.Buffer
	code := Main([]string{"-q", "-game", "0.1.0"}, &out, &errOut, scripted(
		modding.Blob{Path: "scripts/pawns.star", Data: []byte(src)},
	), nil, "dev")
	if code != 0 || strings.Contains(out.String(), ".star") {
		t.Fatalf("exit %d, want 0 and no problems:\n%s%s", code, out.String(), errOut.String())
	}
}
