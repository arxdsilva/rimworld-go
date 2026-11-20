// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

const (
	blobMagic  = "RWMS"
	blobHeader = 16 // magic, ABI, state_version, len
)

// find is the live-or-off module for plugin and path.
func (h *Host) find(plugin, path string) *module {
	for _, m := range h.mods {
		if m.plugin == plugin && m.path == path {
			return m
		}
	}
	return nil
}

// stateModule finds the module and checks that it keeps state and is on.
func (h *Host) stateModule(plugin, path string) (*module, error) {
	m := h.find(plugin, path)
	switch {
	case m == nil:
		return nil, fmt.Errorf("wasmhost: no file %s in plugin %s", path, plugin)
	case m.save == nil:
		return nil, fmt.Errorf("wasmhost: %s/%s has no save and load exports", plugin, path)
	case m.off:
		return nil, fmt.Errorf("wasmhost: %s/%s is off", plugin, path)
	}
	return m, nil
}

// Save returns the save blob of the plugin's file at path:
// "RWMS" | u32 ABIVersion | u32 state_version | u32 len | data.
// A trap turns the file off and is the error.
func (h *Host) Save(plugin, path string) ([]byte, error) {
	m, err := h.stateModule(plugin, path)
	if err != nil {
		return nil, err
	}
	r, msg := h.once(m, "save", m.save)
	if msg != "" {
		return nil, h.stateFailed(m, msg)
	}
	n := uint32(api.DecodeI32(r[0]))
	if n > MaxState || n > m.bufSize {
		return nil, fmt.Errorf("wasmhost: save returned %d bytes (limit %d, buffer %d)", int32(n), MaxState, m.bufSize)
	}
	data, ok := m.mem.Read(m.buf, n)
	if !ok {
		return nil, errors.New("wasmhost: save data is outside memory")
	}
	le := binary.LittleEndian
	blob := make([]byte, 0, blobHeader+int(n))
	blob = append(blob, blobMagic...)
	blob = le.AppendUint32(blob, ABIVersion)
	blob = le.AppendUint32(blob, m.version)
	blob = le.AppendUint32(blob, n)
	return append(blob, data...), nil
}

// Load checks blob's magic, ABI, version and size, then hands its data to
// the module's load export. A rejected blob leaves the file on and its
// memory untouched; a trap turns the file off.
func (h *Host) Load(plugin, path string, blob []byte) error {
	m, err := h.stateModule(plugin, path)
	if err != nil {
		return err
	}
	data, err := m.checkBlob(blob)
	if err != nil {
		return err
	}
	if !m.mem.Write(m.buf, data) {
		return errors.New("wasmhost: state does not fit in memory")
	}
	r, msg := h.once(m, "load", m.load, uint64(uint32(len(data))))
	if msg != "" {
		return h.stateFailed(m, msg)
	}
	if rc := api.DecodeI32(r[0]); rc != 0 {
		return fmt.Errorf("wasmhost: load returned %d", rc)
	}
	return nil
}

// checkBlob returns blob's data, or why the blob is rejected.
func (m *module) checkBlob(blob []byte) ([]byte, error) {
	le := binary.LittleEndian
	switch {
	case len(blob) < blobHeader || string(blob[:4]) != blobMagic:
		return nil, errors.New("wasmhost: state blob: bad magic")
	case le.Uint32(blob[4:]) != ABIVersion:
		return nil, fmt.Errorf("wasmhost: state blob: ABI %d, want %d", le.Uint32(blob[4:]), ABIVersion)
	case le.Uint32(blob[8:]) != m.version:
		return nil, fmt.Errorf("wasmhost: state blob: version %d, want %d", le.Uint32(blob[8:]), m.version)
	}
	n := le.Uint32(blob[12:])
	switch {
	case n > MaxState || n > m.bufSize:
		return nil, fmt.Errorf("wasmhost: state blob: %d bytes over the limit (%d, buffer %d)", n, MaxState, m.bufSize)
	case uint64(len(blob)-blobHeader) != uint64(n):
		return nil, fmt.Errorf("wasmhost: state blob: %d data bytes, header says %d", len(blob)-blobHeader, n)
	}
	return blob[blobHeader:], nil
}

// stateFailed turns m off; the problem also goes to the next Fire's
// Disabled, and the error is returned to the caller.
func (h *Host) stateFailed(m *module, msg string) error {
	h.unsent = append(h.unsent, h.disable(m, msg))
	return errors.New("wasmhost: " + msg)
}
