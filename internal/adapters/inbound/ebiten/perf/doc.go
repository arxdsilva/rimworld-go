// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package perf is the frame-timing instruments: the always-on clock
// behind the performance overlay (PERF-002), the dev frame-rate probe and
// the F10 screenshot. Meter owns their state; the root feeds it each
// Update and Draw.
package perf
