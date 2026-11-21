// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"errors"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// saveCopy makes the throwaway instance Save runs on: an anonymous
// instance of m's kept CompiledModule, its memory grown to m's page count
// and filled with m's bytes, and every saved global set to m's value. The
// returned record has the copy's handles and m's buffer and version; the
// caller closes its instance. m is only read.
func (h *Host) saveCopy(m *module) (*module, error) {
	cfg := wazero.NewModuleConfig().WithName("").WithStartFunctions()
	inst, err := h.rt.InstantiateModule(h.ctx, m.cm, cfg)
	if err != nil {
		return nil, errors.New("wasmhost: save copy: " + firstLine(err))
	}
	c := &module{plugin: m.plugin, path: m.path, key: m.key, cm: m.cm, gnames: m.gnames, inst: inst,
		buf: m.buf, bufSize: m.bufSize, version: m.version}
	if msg := c.copyFrom(m); msg != "" {
		_ = inst.Close(h.ctx)
		return nil, errors.New("wasmhost: save copy: " + msg)
	}
	return c, nil
}

// copyFrom binds c's handles on its instance and copies live's memory and
// saved globals into it.
func (c *module) copyFrom(live *module) string {
	var ok bool
	c.mem = c.inst.ExportedMemory("memory")
	c.fuel, ok = c.inst.ExportedGlobal(fuelExport).(api.MutableGlobal)
	if !ok || c.mem == nil {
		return "no memory or fuel global"
	}
	if c.depth, ok = c.inst.ExportedGlobal(depthExport).(api.MutableGlobal); !ok {
		return "no depth global"
	}
	if c.save = c.inst.ExportedFunction("save"); c.save == nil {
		return "no save export"
	}
	if c.globals, ok = savedGlobals(c.inst, c.gnames, nil); !ok {
		return "a saved global is not exported"
	}
	size := live.mem.Size()
	if have := c.mem.Size(); have < size {
		if _, ok := c.mem.Grow((size - have) / 65536); !ok {
			return "memory does not grow to the live size"
		}
	}
	data, ok := live.mem.Read(0, size)
	if !ok || !c.mem.Write(0, data) {
		return "memory does not copy"
	}
	for i, g := range live.globals {
		c.globals[i].Set(g.Get())
	}
	return ""
}
