// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/perf"
)

func budgetLine(st *perf.Probe, d tickDebt) string {
	return perf.BudgetLine(st, d.held, d.maxOwed)
}
