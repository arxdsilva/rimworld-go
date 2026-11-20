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
	return h.rt.CompileModule(h.ctx, wasm)
}

// memoryOver is the rejection of a declared memory whose min or max is over
// Options.MaxPages ("" when fine). A memory with no declared max stays
// allowed: the runtime's limit caps it.
func (h *Host) memoryOver(wasm []byte) string {
	m, err := wasmrewrite.Decode(wasm)
	if err != nil {
		return "" // Prepare already accepted these bytes
	}
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

// loadFile turns f into a live module, or returns the problem's message.
// Prepare runs first so Check's typed rejection names a rule; the original
// is still compiled by wazero (validity) before the output is trusted.
func (h *Host) loadFile(plugin string, f File) (*module, string) {
	out, err := wasmrewrite.Prepare(f.Wasm, h.rw)
	if err != nil {
		return nil, "rejected: " + firstLine(err)
	}
	if msg := h.memoryOver(f.Wasm); msg != "" {
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
	cfg := wazero.NewModuleConfig().WithName(fmt.Sprintf("m%d", h.nameSeq)).WithStartFunctions()
	inst, err := h.rt.InstantiateModule(h.ctx, cm, cfg)
	if err != nil {
		return nil, "instantiate: " + firstLine(err)
	}
	m := &module{plugin: plugin, path: f.Path, key: fnv64a(plugin + "\x00" + f.Path), inst: inst}
	if msg := h.bind(m); msg != "" {
		_ = inst.Close(h.ctx)
		return nil, msg
	}
	m.calls, m.spent = 0, 0 // the load-time calls are not the hooks' counters
	return m, ""
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
	defs := m.inst.ExportedFunctionDefinitions()
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
	for _, name := range []string{"rw_buf", "rw_buf_size"} {
		if d, has := defs[name]; !has || !sigEq(d, nil, i32) {
			return "export " + name + " is missing or not () -> i32"
		}
	}
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
	if msg := h.bindState(m, defs); msg != "" {
		return msg
	}
	return h.readBuffer(m)
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
