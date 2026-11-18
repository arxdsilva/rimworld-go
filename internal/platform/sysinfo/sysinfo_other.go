// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

//go:build !darwin && !linux && !windows

package sysinfo

func readMemory() (total, avail uint64) { return 0, 0 }
