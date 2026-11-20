// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scriptbench is a benchmark only (PLG-060): it measures the
// scripting port's Starlark host against a gopher-lua (Lua 5.1) runtime on
// the same per-colonist work. It holds no code; its tests import
// gopher-lua, so the module is a test-only dependency that never reaches
// the game binary. Like game/scalegolden, it runs in its own test binary.
package scriptbench
