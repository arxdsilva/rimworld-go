// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import "rimworld-go/internal/application/pluginapi"

// CapsOf is the capability set m's plugin is granted, parsed once here so
// no adapter reads the names; legacy is true when m declares none (nil),
// which grants pluginapi.Legacy. An unknown name grants nothing (the
// manifest check refuses it before this is used).
func CapsOf(m Manifest) (s pluginapi.Set, legacy bool) {
	if m.Capabilities == nil {
		return pluginapi.Legacy(), true
	}
	s, _, _ = pluginapi.Parse(m.Capabilities)
	return s, false
}
