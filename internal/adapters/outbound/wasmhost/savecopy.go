// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"errors"

	"github.com/tetratelabs/wazero"
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
	if msg := c.bindGlobals(); msg != "" {
		return msg
	}
	if c.save = c.inst.ExportedFunction("save"); c.save == nil {
		return "no save export"
	}
	size := live.mem.Size()
	if have := c.mem.Size(); have < size {
		if _, ok := c.mem.Grow((size - have) / 65536); !ok {
			return "memory does not grow to the live size"
		}
	}
	// [0, size) is inside both memories now, so Read and Write succeed.
	data, _ := live.mem.Read(0, size)
	_ = c.mem.Write(0, data)
	for i, g := range live.globals {
		c.globals[i].Set(g.Get())
	}
	return ""
}
