// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"context"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	"rimworld-go/internal/application/scripting"
)

// Plugin is one loaded plugin's Wasm files, in load order.
type Plugin struct {
	ID    string
	Files []File // sorted by path
}

// File is one module: Path is its slash path inside the plugin
// ("scripts/main.wasm").
type File struct {
	Path string
	Wasm []byte
}

// module is one live file: its compiled module, its instance, the reserved
// and saved globals and the exports the host calls.
type module struct {
	plugin, path string
	key          uint64                // fnv64a(plugin + "\x00" + path), the rand stage
	cm           wazero.CompiledModule // the rewritten module, kept for Reset and the save copy
	name         string                // the instance's wazero name, the same after Reset
	gnames       []string              // the saved globals' export names (wasmrewrite.SavedGlobals)
	globals      []api.MutableGlobal   // the live instance's saved globals, by gnames
	inst         api.Module            // nil after a failed Reset
	mem          api.Memory
	fuel, depth  api.MutableGlobal
	hooks        [hookSlots]api.Function // by scripting.Hook; nil when not exported
	buf, bufSize uint32                  // the event buffer inside mem
	save, load   api.Function            // nil when the module has no state
	version      uint32                  // state_version(), read at load
	off          bool
	calls, spent uint64 // for Stats
}

const hookSlots = int(scripting.OnPawn) + 1

// Host runs the modules. Fire is called by the simulation owner only;
// Problems may be read from any goroutine. Methods after Close are not
// supported. The host starts no goroutine.
type Host struct {
	ctx  context.Context // the one context.Background()
	rt   wazero.Runtime
	opts Options // defaults filled
	rw   wasmrewrite.Config

	mods    []*module // load order
	nameSeq int       // unique wazero module names
	cur     *callState
	scratch []byte // event encoding, reused

	compiles int // CompileModule calls, read by tests: Reset never compiles

	mu     sync.Mutex
	probs  []scripting.Problem // load-time first, then this game's
	unsent []scripting.Problem // problems not yet in a Result, load failures first
	// loadProbs is how many of probs are load failures; loadUnsent how
	// many of unsent are (0 once a Result carried them).
	loadProbs, loadUnsent int
}

var _ scripting.Host = (*Host)(nil)

// runtimeMu serialises wazero.NewRuntimeWithConfig: v1.9.0 races on
// internal/version when runtimes are made concurrently.
var runtimeMu sync.Mutex

func newRuntime(ctx context.Context, c wazero.RuntimeConfig) wazero.Runtime {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	return wazero.NewRuntimeWithConfig(ctx, c)
}

// New compiles, rewrites and instantiates every plugin's files. A file that
// fails is a problem and stays off; the rest run.
func New(ps []Plugin, o Options) *Host {
	h := &Host{ctx: context.Background(), opts: o.withDefaults(), rw: rewriteConfig()}
	h.rt = newRuntime(h.ctx, runtimeConfig(h.opts))
	err := h.registerImports()
	for _, p := range ps {
		for _, f := range p.Files {
			if err != nil {
				h.unsent = append(h.unsent, h.fail(p.ID, f.Path, "internal: imports: "+err.Error()))
				continue
			}
			m, msg := h.loadFile(p.ID, f)
			if msg != "" {
				h.unsent = append(h.unsent, h.fail(p.ID, f.Path, msg))
				continue
			}
			h.mods = append(h.mods, m)
		}
	}
	h.loadProbs, h.loadUnsent = len(h.probs), len(h.unsent)
	return h
}

// Uses reports that a live module exports hook hk. It does not allocate.
func (h *Host) Uses(hk scripting.Hook) bool { return uses(h.mods, hk) }

// uses reports that a live module of mods exports hook hk.
func uses(mods []*module, hk scripting.Hook) bool {
	if int(hk) >= hookSlots {
		return false
	}
	for _, m := range mods {
		if !m.off && m.hooks[hk] != nil {
			return true
		}
	}
	return false
}

// Problems are the modules disabled so far, in order.
func (h *Host) Problems() []scripting.Problem {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]scripting.Problem(nil), h.probs...)
}

// fail records a problem for a file that never loaded.
func (h *Host) fail(plugin, path, msg string) scripting.Problem {
	p := scripting.Problem{Plugin: plugin, File: path, Msg: msg}
	h.mu.Lock()
	h.probs = append(h.probs, p)
	h.mu.Unlock()
	return p
}

// disable turns m off with msg as its problem.
func (h *Host) disable(m *module, msg string) scripting.Problem {
	m.off = true
	return h.fail(m.plugin, m.path, msg)
}

// Close releases the runtime and every module.
func (h *Host) Close() error {
	return h.rt.Close(h.ctx)
}
