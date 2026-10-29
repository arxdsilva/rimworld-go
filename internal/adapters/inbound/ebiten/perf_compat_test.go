package ebiten

import (
	perfpkg "rimworld-go/internal/adapters/inbound/ebiten/perf"

	"rimworld-go/internal/adapters/inbound/ebiten/perf"
)

func budgetLine(st *perfpkg.Probe, d tickDebt) string {
	return perf.BudgetLine(st, d.held, d.maxOwed)
}
