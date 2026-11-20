// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// fixtureHooks are the optional ABI hook exports, each () -> ().
var fixtureHooks = []string{"on_hour", "on_day", "on_incident", "on_colonist_died", "on_pawns", "on_pawn"}

// fixtureWant lists, per committed fixture, the hooks it must export.
var fixtureWant = map[string][]string{
	"bench_batched": {"on_hour"},
	"bench_percall": {"on_hour"},
	"bench_noop":    {"on_hour"},
	"dice":          {"on_hour", "on_pawn"},
	"hooks":         fixtureHooks,
	"state":         {"on_hour"},
	"golden":        fixtureHooks,
}

// fixtureRuntime is a wazero runtime with the contract's features (doc.go).
func fixtureRuntime(ctx context.Context) wazero.Runtime {
	cfg := wazero.NewRuntimeConfig().
		WithCoreFeatures(api.CoreFeaturesV2 &^ api.CoreFeatureSIMD).
		WithMemoryLimitPages(DefaultMaxPages).
		WithCloseOnContextDone(false)
	return wazero.NewRuntimeWithConfig(ctx, cfg)
}

// readFixture returns the committed testdata/<name>.wasm (name without the
// extension, e.g. "dice"); the other wasmhost tests load fixtures with it.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".wasm"))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

func fixtureFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "*.wasm"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no committed fixtures (%v)", err)
	}
	sort.Strings(files)
	return files
}

func sigIs(def api.FunctionDefinition, params, results []api.ValueType) bool {
	eq := func(a, b []api.ValueType) bool {
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
	return eq(def.ParamTypes(), params) && eq(def.ResultTypes(), results)
}

// Every fixture compiles under the contract features, exports the ABI
// names with the right signatures, imports only rimworld functions, stays
// under 64 KiB and caps its memory at DefaultMaxPages.
//
// Serial on purpose: wazero v1.9.0's compiler engine has a data race in
// internal/version.GetWazeroVersion when runtimes are made concurrently.
func TestFixturesValid(t *testing.T) {
	ctx := context.Background()
	files := fixtureFiles(t)
	if len(files) != len(fixtureWant) {
		t.Errorf("%d fixtures, want %d", len(files), len(fixtureWant))
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".wasm")
		t.Run(name, func(t *testing.T) {
			bin := readFixture(t, name)
			if len(bin) >= 64<<10 {
				t.Errorf("%d bytes, want under 64 KiB", len(bin))
			}
			rt := fixtureRuntime(ctx)
			defer rt.Close(ctx)
			cm, err := rt.CompileModule(ctx, bin)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			fx := cm.ExportedFunctions()
			for fn, want := range map[string][2][]api.ValueType{
				"rw_buf":      {nil, {api.ValueTypeI32}},
				"rw_buf_size": {nil, {api.ValueTypeI32}},
			} {
				d, ok := fx[fn]
				if !ok || !sigIs(d, want[0], want[1]) {
					t.Errorf("export %s missing or wrong signature", fn)
				}
			}
			mem, ok := cm.ExportedMemories()["memory"]
			if !ok {
				t.Fatal("no memory export")
			}
			if max, has := mem.Max(); !has || max > DefaultMaxPages {
				t.Errorf("memory max = %d (declared %v), want at most %d pages", max, has, DefaultMaxPages)
			}
			wantHooks := fixtureWant[name]
			for _, h := range fixtureHooks {
				d, ok := fx[h]
				want := fixtureContains(wantHooks, h)
				if ok != want {
					t.Errorf("hook %s exported = %v, want %v", h, ok, want)
				}
				if ok && !sigIs(d, nil, nil) {
					t.Errorf("hook %s is not () -> ()", h)
				}
			}
			_, hasVer := fx["state_version"]
			_, hasSave := fx["save"]
			_, hasLoad := fx["load"]
			if hasVer != hasSave || hasSave != hasLoad {
				t.Errorf("save/load exports are not all-or-none: %v %v %v", hasVer, hasSave, hasLoad)
			}
			if name == "state" {
				if !hasVer || !sigIs(fx["state_version"], nil, []api.ValueType{api.ValueTypeI32}) ||
					!sigIs(fx["save"], nil, []api.ValueType{api.ValueTypeI32}) ||
					!sigIs(fx["load"], []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}) {
					t.Error("state: state_version/save/load missing or wrong signature")
				}
			} else if hasVer {
				t.Errorf("%s exports save/load", name)
			}
			for _, imp := range cm.ImportedFunctions() {
				mod, _, isImp := imp.Import()
				if !isImp || mod != Namespace {
					t.Errorf("import from %q, want only %q functions", mod, Namespace)
				}
			}
			if len(cm.ImportedMemories()) != 0 {
				t.Error("memory import")
			}
		})
	}
}

func fixtureContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// With the pinned toolchain the script rebuilds every fixture byte for
// byte. Without Rust (CI) the committed bytes are what the other tests use.
func TestFixturesRebuild(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not installed; the committed fixtures are tested as they are")
	}
	pin, err := os.ReadFile(filepath.Join("testdata", "rust", "TOOLCHAIN"))
	if err != nil {
		t.Fatal(err)
	}
	have, err := exec.Command("rustc", "-V").Output()
	if err != nil {
		t.Skipf("rustc not usable: %v", err)
	}
	if got, want := strings.TrimSpace(string(have)), strings.TrimSpace(string(pin)); got != want {
		t.Skipf("rustc is %q, the pin is %q", got, want)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "scripts", "wasm_fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "OUT_DIR="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", script, err, b)
	}
	for _, f := range fixtureFiles(t) {
		want, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(out, filepath.Base(f)))
		if err != nil {
			t.Fatalf("%s not rebuilt: %v", filepath.Base(f), err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: rebuilt %d bytes differ from committed %d", filepath.Base(f), len(got), len(want))
		}
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != len(fixtureFiles(t)) {
		t.Errorf("rebuilt %d files, committed %d", len(entries), len(fixtureFiles(t)))
	}
}
