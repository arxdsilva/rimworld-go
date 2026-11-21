// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

const statePath = "scripts/state.wasm"

func hours(h *Host, n int) []command.Command {
	var out []command.Command
	for i := 0; i < n; i++ {
		out = append(out, fire(h, event(scripting.OnHour)).Commands...)
	}
	return out
}

// C7: 3 hours, save, a fresh host, load, 3 more hours equal 6 hours.
func TestStateSaveLoadEqualsAContinuousRun(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		whole := hours(newHost(t, o, fixture(t, "p", "state")), 6)

		a := newHost(t, o, fixture(t, "p", "state"))
		first := hours(a, 3)
		blob, err := a.Save("p", statePath)
		if err != nil {
			t.Fatal(err)
		}
		want := append([]byte("RWMS"), 1, 0, 0, 0, 1, 0, 0, 0, 4, 0, 0, 0, 3, 0, 0, 0)
		if !reflect.DeepEqual(blob, want) {
			t.Errorf("blob = %x, want %x", blob, want)
		}
		b := newHost(t, o, fixture(t, "p", "state"))
		if err := b.Load("p", statePath, blob); err != nil {
			t.Fatal(err)
		}
		got := append(first, hours(b, 3)...)
		if !reflect.DeepEqual(got, whole) {
			t.Errorf("save/load run %v\ncontinuous   %v", messages(got), messages(whole))
		}
		if len(whole) != 6 || whole[5].Name != "count 6" {
			t.Errorf("continuous run = %v", messages(whole))
		}
	})
}

func TestStateRejectedBlobsMakeNoGuestCall(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "state"))
	hours(h, 2)
	good, err := h.Save("p", statePath)
	if err != nil {
		t.Fatal(err)
	}
	mut := func(f func(b []byte) []byte) []byte { return f(append([]byte(nil), good...)) }
	put := func(off int, v uint32) func([]byte) []byte {
		return func(b []byte) []byte { binary.LittleEndian.PutUint32(b[off:], v); return b }
	}
	cases := map[string][]byte{
		"empty":       nil,
		"short":       good[:10],
		"bad magic":   mut(func(b []byte) []byte { b[0] = 'X'; return b }),
		"bad ABI":     mut(put(4, 2)),
		"bad version": mut(put(8, 9)),
		"oversize":    mut(put(12, MaxState+1)),
		"over buffer": mut(func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[12:], 300) // buffer is 256
			return append(b[:16], make([]byte, 300)...)
		}),
		"len mismatch": mut(put(12, 3)),
		"trailing":     append(append([]byte(nil), good...), 0),
	}
	before := h.Stats()[0]
	for name, blob := range cases {
		if err := h.Load("p", statePath, blob); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	after := h.Stats()[0]
	if after != before || after.Off {
		t.Errorf("a rejected blob made a guest call or turned the file off: %+v -> %+v", before, after)
	}
	if got := hours(h, 1); got[0].Name != "count 3" {
		t.Errorf("memory touched by a rejected blob: %v", messages(got))
	}
}

func TestStateErrors(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "state"), fixture(t, "q", "dice"))
	for _, c := range []struct{ plugin, path, want string }{
		{"nope", statePath, "no file"},
		{"p", "scripts/nope.wasm", "no file"},
		{"q", "scripts/dice.wasm", "no save and load"},
	} {
		if _, err := h.Save(c.plugin, c.path); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Save(%s,%s) = %v", c.plugin, c.path, err)
		}
		if err := h.Load(c.plugin, c.path, nil); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Load(%s,%s) = %v", c.plugin, c.path, err)
		}
	}
	good, _ := h.Save("p", statePath)
	h.mods[0].off = true
	if _, err := h.Save("p", statePath); err == nil || !strings.Contains(err.Error(), "is off") {
		t.Errorf("Save on an off file = %v", err)
	}
	if err := h.Load("p", statePath, good); err == nil {
		t.Error("Load on an off file succeeded")
	}
}

// The state fixture's load returns 1 for a length other than 4: an error,
// the file stays on.
func TestStateLoadNonZeroIsAnErrorNotATrap(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "state"))
	good, _ := h.Save("p", statePath)
	short := append([]byte(nil), good[:12]...)
	short = append(short, 5, 0, 0, 0, 1, 2, 3, 4, 5)
	if err := h.Load("p", statePath, short); err == nil || !strings.Contains(err.Error(), "load returned 1") {
		t.Fatalf("err = %v", err)
	}
	if h.Stats()[0].Off {
		t.Error("a non-zero return must not turn the file off")
	}
}

func stateGuest(save, load []byte) []byte {
	g := newGuest()
	g.export("state_version", nil, []byte{0x7F}, i32c(1))
	g.export("save", nil, []byte{0x7F}, save)
	g.export("load", []byte{0x7F}, []byte{0x7F}, load)
	return g.bytes()
}

// A trapping load turns the file off: the error, then the next Fire's
// Disabled. (A trapping save leaves it on: TestSaveTrapLeavesTheFileOn.)
func TestLoadTrapTurnsTheFileOffAndIsReported(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		trap := []byte{0x00, 0x41, 0x00} // unreachable; i32.const 0
		h := newHost(t, o, plug("p", "scripts/x.wasm", stateGuest(i32c(0), trap)))
		blob := append([]byte("RWMS"), 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0)
		const want = "trap in load: "
		if err := h.Load("p", "scripts/x.wasm", blob); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v", err)
		}
		if !h.Stats()[0].Off {
			t.Error("file still on")
		}
		if res := fire(h, event(scripting.OnHour)); len(res.Disabled) != 1 || !strings.HasPrefix(res.Disabled[0].Msg, want) {
			t.Errorf("next Fire's Disabled = %+v", res.Disabled)
		}
	})
}

func TestStateSaveSizeLimits(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter},
		plug("big", "scripts/x.wasm", stateGuest(i32c(MaxState+1), i32c(0))),
		plug("neg", "scripts/y.wasm", stateGuest(i32c(-1), i32c(0))),
		plug("buf", "scripts/z.wasm", stateGuest(i32c(bufLen+1), i32c(0))))
	for _, c := range [][2]string{{"big", "x"}, {"neg", "y"}, {"buf", "z"}} {
		if _, err := h.Save(c[0], "scripts/"+c[1]+".wasm"); err == nil || !strings.Contains(err.Error(), "save returned") {
			t.Errorf("%s: err = %v", c[0], err)
		}
		if h.Stats()[0].Off {
			t.Error("an oversize save is an error, not a trap")
		}
	}
}
