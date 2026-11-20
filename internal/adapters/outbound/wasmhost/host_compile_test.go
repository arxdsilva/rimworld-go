// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
)

// crasher reads the fuzz corpus file whose bytes make wazero v1.9.0's
// CompileModule panic.
func crasher(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("wasmrewrite", "testdata", "fuzz", "FuzzPrepare", "wazero_validator_panic"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	lit := strings.TrimSuffix(strings.TrimPrefix(lines[1], "[]byte("), ")")
	s, err := strconv.Unquote(lit)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(s)
}

// A compile that panics is an error, so the file turns itself off.
func TestHostCompilePanicIsAnError(t *testing.T) {
	bad := crasher(t)
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "good", "dice"))
		if _, err := h.compile(bad); err == nil || !strings.Contains(err.Error(), "panicked") {
			t.Fatalf("compile err = %v, want a recovered panic", err)
		}
		if _, err := h.compile(readFixture(t, "dice")); err != nil {
			t.Fatalf("the host still compiles after a panic: %v", err)
		}
	})
}

// The crasher through New: a Problem for that file, the others unaffected.
func TestHostCrasherThroughNewIsAProblem(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, plug("bad", "scripts/bad.wasm", crasher(t)), fixture(t, "good", "dice"))
	res := fire(h, event(scripting.OnHour))
	if len(res.Disabled) != 1 || res.Disabled[0].Plugin != "bad" || len(res.Commands) != 2 {
		t.Fatalf("res = %+v", res)
	}
}

func TestHostModuleOverMaxWasmBytesIsRejectedBeforePrepare(t *testing.T) {
	big := append(readFixture(t, "dice"), make([]byte, MaxWasmBytes)...) // trailing junk: would also fail Prepare
	h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/big.wasm", big), fixture(t, "q", "dice"))
	p := h.Problems()
	if len(p) != 1 || !strings.HasPrefix(p[0].Msg, "rejected: module is ") || !strings.Contains(p[0].Msg, "at most 1048576") {
		t.Fatalf("problems = %+v", p)
	}
	if len(h.Stats()) != 1 {
		t.Error("the other file must load")
	}
}
