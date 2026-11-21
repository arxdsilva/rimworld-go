// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	"rimworld-go/internal/application/scripting"
)

// fnv64a is the rand key's hash, as starlarkhost's key().
func fnv64a(s string) uint64 {
	f := fnv.New64a()
	_, _ = f.Write([]byte(s))
	return f.Sum64()
}

// firstLine cuts a wazero error to its first line, so both engines give
// the same text.
func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// compile is wazero's CompileModule with a panic turned into an error:
// v1.9.0's validator panics on some malformed function bodies, and a bad
// file must only turn itself off.
func (h *Host) compile(wasm []byte) (cm wazero.CompiledModule, err error) {
	defer func() {
		if r := recover(); r != nil {
			cm, err = nil, fmt.Errorf("wazero panicked while compiling: %v", r)
		}
	}()
	h.compiles++
	return h.rt.CompileModule(h.ctx, wasm)
}

// memoryOver is the rejection of a declared memory whose min or max is over
// Options.MaxPages ("" when fine). A memory with no declared max stays
// allowed: the runtime's limit caps it.
func (h *Host) memoryOver(m *wasmrewrite.Module) string {
	limit := uint64(h.opts.MaxPages)
	for _, l := range m.Memories {
		if l.Min > limit || (l.HasMax && l.Max > limit) {
			if !l.HasMax {
				return fmt.Sprintf("rejected: memory declares min %d pages, over the host's %d", l.Min, limit)
			}
			return fmt.Sprintf("rejected: memory declares min %d, max %d pages, over the host's %d", l.Min, l.Max, limit)
		}
	}
	return ""
}

// savedGlobalNames are the export names Prepare gave orig's saved globals.
func savedGlobalNames(orig *wasmrewrite.Module) []string {
	idx := wasmrewrite.SavedGlobals(orig)
	names := make([]string, len(idx))
	for i, g := range idx {
		names[i] = wasmrewrite.GlobalName(g)
	}
	return names
}

// loadFile turns f into a live module, or returns the problem's message.
// Prepare runs first so Check's typed rejection names a rule; the original
// is still compiled by wazero (validity) before the output is trusted. The
// rewritten CompiledModule stays on the module for Reset and the save copy.
func (h *Host) loadFile(plugin string, f File) (*module, string) {
	if len(f.Wasm) > MaxWasmBytes {
		return nil, fmt.Sprintf("rejected: module is %d bytes, at most %d", len(f.Wasm), MaxWasmBytes)
	}
	out, err := wasmrewrite.Prepare(f.Wasm, h.rw)
	if err != nil {
		return nil, "rejected: " + firstLine(err)
	}
	dec, err := wasmrewrite.Decode(f.Wasm) // Prepare already decoded these bytes
	if err != nil {
		return nil, "rejected: " + firstLine(err)
	}
	if msg := h.memoryOver(dec); msg != "" {
		return nil, msg
	}
	orig, err := h.compile(f.Wasm)
	if err != nil {
		return nil, "invalid module: " + firstLine(err)
	}
	_ = orig.Close(h.ctx)
	cm, err := h.compile(out)
	if err != nil {
		return nil, "internal: rewritten module does not compile: " + firstLine(err)
	}
	h.nameSeq++
	m := &module{plugin: plugin, path: f.Path, key: fnv64a(plugin + "\x00" + f.Path),
		cm: cm, name: fmt.Sprintf("m%d", h.nameSeq), gnames: savedGlobalNames(dec)}
	if msg := h.instantiate(m); msg != "" {
		_ = cm.Close(h.ctx)
		return nil, msg
	}
	return m, ""
}

// instantiate makes m's live instance from m.cm under m.name and binds it.
// On failure m has no instance. The load-time calls are not the hooks'
// counters, so calls and spent restart at 0.
func (h *Host) instantiate(m *module) string {
	cfg := wazero.NewModuleConfig().WithName(m.name).WithStartFunctions()
	inst, err := h.rt.InstantiateModule(h.ctx, m.cm, cfg)
	if err != nil {
		return "instantiate: " + firstLine(err)
	}
	m.inst = inst
	if msg := h.bind(m); msg != "" {
		_ = inst.Close(h.ctx)
		m.inst, m.mem = nil, nil
		return msg
	}
	m.calls, m.spent = 0, 0
	return ""
}

// bindGlobals reads the memory, the reserved globals and the saved globals
// from m's instance (a live one, or a save copy).
func (m *module) bindGlobals() string {
	if m.mem = m.inst.ExportedMemory("memory"); m.mem == nil {
		return "missing export memory"
	}
	var ok bool
	if m.fuel, ok = m.inst.ExportedGlobal(fuelExport).(api.MutableGlobal); !ok {
		return "internal: no fuel global"
	}
	if m.depth, ok = m.inst.ExportedGlobal(depthExport).(api.MutableGlobal); !ok {
		return "internal: no depth global"
	}
	if m.globals, ok = savedGlobals(m.inst, m.gnames, m.globals[:0]); !ok {
		return "internal: a saved global is not exported"
	}
	return ""
}

// savedGlobals appends inst's saved globals, by names, to dst.
func savedGlobals(inst api.Module, names []string, dst []api.MutableGlobal) ([]api.MutableGlobal, bool) {
	for _, n := range names {
		g, ok := inst.ExportedGlobal(n).(api.MutableGlobal)
		if !ok {
			return dst, false
		}
		dst = append(dst, g)
	}
	return dst, true
}

var i32 = []api.ValueType{api.ValueTypeI32}

func sigEq(def api.FunctionDefinition, params, results []api.ValueType) bool {
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

// bind checks the exports against ABI v1 and reads the buffer and state
// version, each guest call with its own fuel.
func (h *Host) bind(m *module) string {
	if msg := m.bindGlobals(); msg != "" {
		return msg
	}
	defs := m.inst.ExportedFunctionDefinitions()
	for _, name := range []string{"rw_buf", "rw_buf_size"} {
		if d, has := defs[name]; !has || !sigEq(d, nil, i32) {
			return "export " + name + " is missing or not () -> i32"
		}
	}
	if msg := m.bindHooks(defs); msg != "" {
		return msg
	}
	if msg := h.bindState(m, defs); msg != "" {
		return msg
	}
	return h.readBuffer(m)
}

// bindHooks binds the exported hooks, each () -> ().
func (m *module) bindHooks(defs map[string]api.FunctionDefinition) string {
	for _, hk := range scripting.Hooks() {
		d, has := defs[hk.String()]
		if !has {
			continue
		}
		if !sigEq(d, nil, nil) {
			return "export " + hk.String() + " is not () -> ()"
		}
		m.hooks[hk] = m.inst.ExportedFunction(hk.String())
	}
	return ""
}

// bindState checks the all-or-none save/load exports.
func (h *Host) bindState(m *module, defs map[string]api.FunctionDefinition) string {
	_, v := defs["state_version"]
	_, s := defs["save"]
	_, l := defs["load"]
	if !v && !s && !l {
		return ""
	}
	if !(v && s && l) {
		return "state_version, save and load must be exported together"
	}
	if !sigEq(defs["state_version"], nil, i32) || !sigEq(defs["save"], nil, i32) || !sigEq(defs["load"], i32, i32) {
		return "state_version, save or load has the wrong signature"
	}
	m.save, m.load = m.inst.ExportedFunction("save"), m.inst.ExportedFunction("load")
	r, msg := h.once(m, "state_version", m.inst.ExportedFunction("state_version"))
	if msg != "" {
		return msg
	}
	m.version = uint32(api.DecodeI32(r[0]))
	return ""
}

// readBuffer calls rw_buf and rw_buf_size and checks the range.
func (h *Host) readBuffer(m *module) string {
	p, msg := h.once(m, "rw_buf", m.inst.ExportedFunction("rw_buf"))
	if msg != "" {
		return msg
	}
	n, msg := h.once(m, "rw_buf_size", m.inst.ExportedFunction("rw_buf_size"))
	if msg != "" {
		return msg
	}
	m.buf, m.bufSize = uint32(api.DecodeI32(p[0])), uint32(api.DecodeI32(n[0]))
	if m.bufSize < minBufSize || uint64(m.buf)+uint64(m.bufSize) > uint64(m.mem.Size()) {
		return fmt.Sprintf("rw_buf (%d, %d bytes) is not %d or more bytes inside memory", m.buf, m.bufSize, minBufSize)
	}
	return ""
}
