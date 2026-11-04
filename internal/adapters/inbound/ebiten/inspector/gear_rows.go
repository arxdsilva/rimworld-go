package inspector

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// PackRows are the Gear tab's pack lines (CSE-036): its weight against the
// pawn's limit, then what is in it. No rows for an empty pack.
func PackRows(p game.PawnView) []string {
	if len(p.Pack) == 0 {
		return nil
	}
	rows := make([]string, 0, len(p.Pack)+1)
	rows = append(rows, fmt.Sprintf("Pack  %.1f / %.1f kg", float64(p.PackGrams)/1000, float64(p.PackLimitGrams)/1000))
	for _, it := range p.Pack {
		rows = append(rows, fmt.Sprintf("  %s x%d", it.Name, it.Qty))
	}
	return rows
}
