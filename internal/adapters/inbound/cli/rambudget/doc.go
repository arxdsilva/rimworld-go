// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package rambudget prints how many parallel agents this machine can run
// without swapping, from its free memory and measured per-agent peaks
// (PLAY-019; .agents/specs/PARALLEL_WORK.md §1). The lead runs it, through
// `make agent-budget`, before launching agents and before each phase.
package rambudget
