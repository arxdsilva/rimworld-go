// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/adapters/outbound/wasmhost"
)

// The rand stream is keyed on "plugin NUL path", so the Starlark twin and the
// Wasm dice fixture must share the plugin ID and the file Path string.
const (
	diceID   = "dice"
	dicePath = "scripts/dice"
)

// diceStar is the Starlark twin of testdata/rust/dice: on_hour from
// randSpawner and on_pawn from pawnSpawner.
const diceStar = randSpawner + pawnSpawner

func diceWasm(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "adapters", "outbound", "wasmhost", "testdata", "dice.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// wasmDiceColony runs the colony under a fresh Wasm host and returns the hash,
// wood, and the host's total fuel; the host has no Problems.
func wasmDiceColony(t *testing.T, seed uint64, hours int) (uint64, int, uint64) {
	t.Helper()
	h := wasmhost.New([]wasmhost.Plugin{{ID: diceID, Files: []wasmhost.File{{Path: dicePath, Wasm: diceWasm(t)}}}},
		wasmhost.Options{Engine: wasmhost.Interpreter})
	defer func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	}()
	if p := h.Problems(); len(p) > 0 {
		t.Fatalf("wasm problems: %+v", p)
	}
	hash, wood := runColonyHost(t, seed, hours, h)
	if p := h.Problems(); len(p) > 0 {
		t.Fatalf("wasm problems after the run: %+v", p)
	}
	var fuel uint64
	for _, s := range h.Stats() {
		if s.Off {
			t.Fatalf("module off: %+v", s)
		}
		fuel += s.Fuel
	}
	return hash, wood, fuel
}

// The same mod, written once in Starlark and once in Wasm, gives the same
// colony through the real game (PLG-063 AC-5).
func TestWasmAndStarlarkGiveTheSameColony(t *testing.T) {
	t.Parallel()
	sh, sw := runColony(t, 11, 3, starlarkhost.File{Path: dicePath, Src: []byte(diceStar)})
	wh, ww, fuel := wasmDiceColony(t, 11, 3)
	if sh != wh || sw != ww {
		t.Fatalf("hosts differ: hash %x/%x, wood %d/%d (starlark/wasm)", sh, wh, sw, ww)
	}
	plain, woodPlain := runColony(t, 11, 3)
	if plain == wh || ww <= woodPlain {
		t.Fatalf("the mod did not change the colony: wood %d scripted, %d plain", ww, woodPlain)
	}
	if fuel == 0 {
		t.Fatal("the wasm host spent no fuel")
	}
}

// Two Wasm runs of one seed are equal, and run in parallel with others.
func TestWasmColonyIsDeterministic(t *testing.T) {
	t.Parallel()
	h1, w1, _ := wasmDiceColony(t, 11, 2)
	h2, w2, _ := wasmDiceColony(t, 11, 2)
	if h1 != h2 || w1 != w2 {
		t.Fatalf("two wasm runs differ: hash %x/%x, wood %d/%d", h1, h2, w1, w2)
	}
	if o, _, _ := wasmDiceColony(t, 12, 2); o == h1 {
		t.Fatal("another seed gave the same colony")
	}
}
