// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
)

// Raw opcodes the adversarial guests use.
var (
	gGet0   = []byte{0x23, 0}
	gSet0   = []byte{0x24, 0}
	i32Add  = []byte{0x6A}
	i32Ld   = []byte{0x28, 2, 0}
	i32St   = []byte{0x36, 2, 0}
	memGrow = []byte{0x40, 0, 0x1A} // memory.grow 0; drop
	spinner = []byte{0x03, 0x40, 0x0C, 0x00, 0x0B}
)

// counterGuest has a mutable i32 global (index 0, init 1) and a memory
// counter at outAt. on_hour adds 1 to both and spawns core.wood by each,
// so the Results show both. save is the given body.
func counterGuest(save []byte) []byte {
	g := newGuest()
	g.b.Global(0x7F, true, cat(i32c(1), []byte{0x0B})...)
	wood := g.str("core.wood")
	g.hook("on_hour",
		gGet0, i32c(1), i32Add, gSet0,
		i32c(outAt), i32c(outAt), i32Ld, i32c(1), i32Add, i32St,
		call("spawn", false, wood, gGet0),
		call("spawn", false, wood, i32c(outAt), i32Ld))
	g.export("state_version", nil, []byte{0x7F}, i32c(1))
	g.export("save", nil, []byte{0x7F}, save)
	g.export("load", []byte{0x7F}, []byte{0x7F}, i32c(0))
	return g.bytes()
}

// liveState is everything Save must leave alone.
type liveState struct {
	mem      []byte
	pages    uint32
	globals  []uint64
	stats    []Stat
	problems []scripting.Problem
}

func snapshot(h *Host) liveState {
	m := h.mods[0]
	b, _ := m.mem.Read(0, m.mem.Size())
	s := liveState{mem: bytes.Clone(b), pages: m.mem.Size() / 65536, stats: h.Stats(), problems: h.Problems()}
	for _, n := range []string{"__rimworld_g0", fuelExport, depthExport} {
		s.globals = append(s.globals, m.inst.ExportedGlobal(n).Get())
	}
	return s
}

// C2: whatever save does on the copy (write memory, grow it, set a global,
// burn its fuel, trap), five Saves leave the live file as a host that never
// saved: memory, globals, Stats, Problems and the next three hooks.
func TestSaveCopyNeverChangesTheRun(t *testing.T) {
	cases := map[string][]byte{
		"writes memory": cat(i32c(outAt), i32c(999), i32St, i32c(0)),
		"grows memory":  cat(i32c(1), memGrow, i32c(0)),
		"sets a global": cat(i32c(77), gSet0, i32c(0)),
		"burns fuel":    cat(spinner, i32c(0)),
		"traps":         cat([]byte{0x00}, i32c(0)),
		"all":           cat(i32c(outAt), i32c(999), i32St, i32c(1), memGrow, i32c(77), gSet0, i32c(4)),
	}
	eachEngine(t, func(t *testing.T, o Options) {
		for name, save := range cases {
			wasm := counterGuest(save)
			live := newHost(t, o, plug("p", "scripts/x.wasm", wasm))
			never := newHost(t, o, plug("p", "scripts/x.wasm", wasm))
			hours(live, 2)
			hours(never, 2)
			for i := 0; i < 5; i++ {
				_, err := live.Save("p", "scripts/x.wasm")
				if (err != nil) != (name == "burns fuel" || name == "traps") {
					t.Errorf("%s: Save %d = %v", name, i, err)
				}
			}
			if a, b := snapshot(live), snapshot(never); !reflect.DeepEqual(a, b) {
				t.Errorf("%s: live state changed by Save:\n%+v\n%+v", name, a.stats, b.stats)
			}
			for i := 0; i < 3; i++ {
				a, b := fire(live, event(scripting.OnHour)), fire(never, event(scripting.OnHour))
				if !reflect.DeepEqual(a, b) || len(a.Commands) != 2 {
					t.Errorf("%s: hour %d after Save = %v %+v, never saved %v %+v", name, i, counts(a), a.Disabled, counts(b), b.Disabled)
				}
			}
		}
	})
}

func counts(r scripting.Result) []int {
	var out []int
	for _, c := range r.Commands {
		out = append(out, c.Count)
	}
	return out
}

// liveSave is the old Save: save on the live instance (test only).
func liveSave(t *testing.T, h *Host, plugin, path string) []byte {
	t.Helper()
	m, err := h.stateModule(plugin, path)
	if err != nil {
		t.Fatal(err)
	}
	r, msg := h.once(m, "save", m.save)
	if msg != "" {
		t.Fatal(msg)
	}
	b, err := m.blob(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// grownGuest grows its memory to 2 pages in on_hour, adds 1 to its global
// and stores it in the last word of memory; save returns that word plus the
// global, so a copy without the grown page or the global differs.
func grownGuest() []byte {
	lastWord := []byte{0x3F, 0, 0x41, 16, 0x74, 0x41, 4, 0x6B} // memory.size << 16 - 4
	g := newGuest()
	g.b.Global(0x7F, true, cat(i32c(10), []byte{0x0B})...)
	g.hook("on_hour",
		[]byte{0x3F, 0}, i32c(2), []byte{0x49, 0x04, 0x40}, i32c(1), memGrow, []byte{0x0B},
		gGet0, i32c(1), i32Add, gSet0,
		lastWord, gGet0, i32St)
	g.export("state_version", nil, []byte{0x7F}, i32c(1))
	g.export("save", nil, []byte{0x7F}, i32c(bufAt), lastWord, i32Ld, gGet0, i32Add, i32St, i32c(4))
	g.export("load", []byte{0x7F}, []byte{0x7F}, i32c(0))
	return g.bytes()
}

// C3: Save on the copy equals a save on the live instance, after k hooks
// and after a load.
func TestSaveCopyMatchesLiveSave(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		for _, p := range []Plugin{fixture(t, "p", "state"), fixture(t, "p", "tally"), plug("p", "scripts/grown.wasm", grownGuest())} {
			path := p.Files[0].Path
			h := newHost(t, o, p)
			other := newHost(t, o, p)
			hours(other, 7)
			loaded, err := other.Save("p", path)
			if err != nil {
				t.Fatal(err)
			}
			check := func(when string) {
				got, err := h.Save("p", path)
				if want := liveSave(t, h, "p", path); err != nil || !bytes.Equal(got, want) {
					t.Errorf("%s %s: copy %x (%v), live %x", path, when, got, err, want)
				}
			}
			for k := 0; k < 4; k++ {
				check("after " + string(rune('0'+k)) + " hooks")
				hours(h, 1)
			}
			if err := h.Load("p", path, loaded); err != nil {
				t.Fatal(err)
			}
			check("after a load")
			hours(h, 1)
			check("after a load and a hook")
		}
	})
}

// C11: a trapping save is the caller's error; the live file stays on with
// its Stats and Problems, and the next Fire runs it with no Disabled.
func TestSaveTrapLeavesTheFileOn(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, plug("p", "scripts/x.wasm", counterGuest(cat([]byte{0x00}, i32c(0)))))
		hours(h, 1)
		st, ps := h.Stats(), h.Problems()
		_, err := h.Save("p", "scripts/x.wasm")
		if err == nil || !strings.Contains(err.Error(), "trap in save: ") {
			t.Fatalf("err = %v", err)
		}
		if got := h.Stats(); !reflect.DeepEqual(got, st) || got[0].Off {
			t.Errorf("Stats %+v -> %+v", st, got)
		}
		if got := h.Problems(); !reflect.DeepEqual(got, ps) {
			t.Errorf("Problems %+v -> %+v", ps, got)
		}
		res := fire(h, event(scripting.OnHour))
		if res.Disabled != nil || len(res.Commands) != 2 {
			t.Errorf("next Fire = %+v", res)
		}
	})
}

// A copy that cannot be made (the kept CompiledModule is gone) is Save's
// error and leaves the file on.
func TestSaveCopyFailures(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "state"))
	_ = h.mods[0].cm.Close(h.ctx)
	if _, err := h.Save("p", statePath); err == nil || !strings.Contains(err.Error(), "save copy") {
		t.Errorf("closed CompiledModule: %v", err)
	}
	if h.Stats()[0].Off {
		t.Error("a failed copy turned the file off")
	}
}

// C12: a file with a mutable reference global never loads: the save copy
// could not carry it.
func TestMutableReferenceGlobalIsRejectedAtLoad(t *testing.T) {
	g := newGuest()
	g.b.Global(0x70, true, 0xD0, 0x70, 0x0B)
	h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/x.wasm", g.bytes()))
	ps := h.Problems()
	if len(ps) != 1 || !strings.HasPrefix(ps[0].Msg, "rejected: ") || !strings.Contains(ps[0].Msg, "mutable reference global") || len(h.Stats()) != 0 {
		t.Errorf("Problems = %+v, Stats = %+v", ps, h.Stats())
	}
}

// C7: one Save, copy included, at the state fixture's size and at 256
// pages.
func BenchmarkSaveCopy(b *testing.B) {
	big := newGuestPages(DefaultMaxPages, DefaultMaxPages)
	big.export("state_version", nil, []byte{0x7F}, i32c(1))
	big.export("save", nil, []byte{0x7F}, i32c(4))
	big.export("load", []byte{0x7F}, []byte{0x7F}, i32c(0))
	for _, c := range []struct {
		name string
		wasm []byte
	}{{"state", nil}, {"256pages", big.bytes()}} {
		b.Run(c.name, func(b *testing.B) {
			wasm := c.wasm
			if wasm == nil {
				wasm = readFixture(b, "state")
			}
			h := New([]Plugin{plug("p", "scripts/x.wasm", wasm)}, Options{})
			defer h.Close()
			if len(h.mods) != 1 {
				b.Fatalf("not loaded: %+v", h.Problems())
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := h.Save("p", "scripts/x.wasm"); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(h.mods[0].mem.Size()/65536), "pages")
		})
	}
}

// rawInstance instantiates wasm as it is (no Prepare) in h's runtime, or
// the Prepare output when prepare is set.
func rawInstance(t *testing.T, h *Host, wasm []byte, prepare bool) *module {
	t.Helper()
	if prepare {
		out, err := wasmrewrite.Prepare(wasm, h.rw)
		if err != nil {
			t.Fatal(err)
		}
		wasm = out
	}
	cm, err := h.compile(wasm)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := h.rt.InstantiateModule(h.ctx, cm, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.Close(h.ctx) })
	return &module{inst: inst}
}

// C15: bindGlobals names each missing handle; a module the rewriter never
// saw has none of the reserved globals.
func TestBindGlobalsFailures(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter})
	var noMem, fuelOnly wasmtest.Builder
	fuelOnly.Memory(1, 1)
	fuelOnly.Export("memory", wasmtest.KindMemory, 0)
	fuelOnly.Export(fuelExport, wasmtest.KindGlobal, fuelOnly.Global(wasmtest.I64, true, cat(i64c(0), []byte{0x0B})...))
	missing := rawInstance(t, h, newGuest().bytes(), true)
	missing.gnames = []string{wasmrewrite.GlobalName(9)}
	for want, m := range map[string]*module{
		"missing export memory":                    rawInstance(t, h, noMem.Bytes(), false),
		"internal: no fuel global":                 rawInstance(t, h, newGuest().bytes(), false),
		"internal: no depth global":                rawInstance(t, h, fuelOnly.Bytes(), false),
		"internal: a saved global is not exported": missing,
	} {
		if got := m.bindGlobals(); got != want {
			t.Errorf("bindGlobals = %q, want %q", got, want)
		}
	}
}

// C14: copyFrom fails, never panics, on a copy without save or one whose
// memory cannot grow to the live size.
func TestCopyFromFailures(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter})
	stateful := func(min, max uint32) []byte {
		g := newGuestPages(min, max)
		g.export("state_version", nil, []byte{0x7F}, i32c(1))
		g.export("save", nil, []byte{0x7F}, i32c(0))
		g.export("load", []byte{0x7F}, []byte{0x7F}, i32c(0))
		return g.bytes()
	}
	live := rawInstance(t, h, stateful(2, 2), true)
	if msg := live.bindGlobals(); msg != "" {
		t.Fatal(msg)
	}
	for want, c := range map[string]*module{
		"no save export":                        rawInstance(t, h, newGuest().bytes(), true),
		"memory does not grow to the live size": rawInstance(t, h, stateful(1, 1), true),
	} {
		if got := c.copyFrom(live); got != want {
			t.Errorf("copyFrom = %q, want %q", got, want)
		}
	}
	ok := rawInstance(t, h, stateful(1, 2), true)
	if got := ok.copyFrom(live); got != "" || ok.mem.Size() != live.mem.Size() {
		t.Errorf("copyFrom = %q, pages %d", got, ok.mem.Size()/65536)
	}
}
