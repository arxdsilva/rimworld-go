package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/application/game"
)

func assignCells(pawns []game.PawnView, cells [][2]int) []formationMove {
	var out []formationMove
	for _, m := range orders.AssignCells(pawns, cells) {
		out = append(out, formationMove{pawn: m.Pawn, x: m.X, y: m.Y})
	}
	return out
}

func (a *App) searchHits() []searchHit {
	var out []searchHit
	for _, h := range a.ord.SearchHits() {
		out = append(out, searchHit{label: h.Label, kind: h.Kind, c: h.C, x: h.X, y: h.Y})
	}
	return out
}
