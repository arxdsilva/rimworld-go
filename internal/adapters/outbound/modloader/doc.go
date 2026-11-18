// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package modloader finds plugins on disk and reads them into the plain
// trees application/modding works on (PLG-002, PLG-004): one folder per
// plugin under each root, plugin.json, defs/, patches/ and lang/ JSON
// files. Every read goes through an os.Root per plugin, so nothing
// outside a plugin's folder can be reached; size and count caps bound
// what a plugin can make the game read.
package modloader
