// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

// Save returns the save blob of the plugin's file at path:
// "RWMS" | u32 ABIVersion | u32 state_version | u32 len | data.
func (h *Host) Save(plugin, path string) ([]byte, error) {
	return nil, nil // W0 stub
}

// Load checks blob's magic, ABI, version and size, then hands its data to
// the module's load export.
func (h *Host) Load(plugin, path string, blob []byte) error {
	return nil // W0 stub
}
