// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"context"

	"github.com/tetratelabs/wazero"

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

// Host runs the modules. Fire is called by the simulation owner only;
// Problems may be read from any goroutine.
type Host struct {
	ctx  context.Context // the one context.Background()
	rt   wazero.Runtime
	opts Options // defaults filled
	rw   wasmrewrite.Config
}

var _ scripting.Host = (*Host)(nil)

// New compiles, rewrites and instantiates every plugin's files. A file
// that fails is a problem and stays off; the rest run.
func New(ps []Plugin, o Options) *Host {
	h := &Host{ctx: context.Background(), opts: o.withDefaults(), rw: rewriteConfig()}
	h.rt = wazero.NewRuntimeWithConfig(h.ctx, runtimeConfig(h.opts))
	return h // W0 stub: ps is not loaded yet
}

// Fire runs every live module's hook for ev, plugins in load order.
func (h *Host) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	return scripting.Result{} // W0 stub
}

// Uses reports that a live module exports hook hk. It does not allocate.
func (h *Host) Uses(hk scripting.Hook) bool {
	return false // W0 stub
}

// Problems are the modules disabled so far, in order.
func (h *Host) Problems() []scripting.Problem {
	return nil // W0 stub
}

// Close releases the runtime and every module.
func (h *Host) Close() error {
	return h.rt.Close(h.ctx)
}
