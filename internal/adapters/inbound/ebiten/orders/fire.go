package orders

// The Extinguish order of a drafted pawn on a burning cell (the fire's
// drawing is render/overlay, fire_world_draw.go).

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// fireOn is the burning cell at (x, y) in w, if any.
func fireOn(w game.WorldView, x, y int) (game.FireView, bool) {
	for _, f := range w.Fires {
		if f.X == x && f.Y == y {
			return f, true
		}
	}
	return game.FireView{}, false
}

// FireOrders is the right-click menu of a drafted pawn on a burning cell:
// walk there, or beat the fire out (a forced order, anywhere). Nil when
// the pawn is not drafted or the cell is not burning.
func FireOrders(p game.PawnView, w game.WorldView, tx, ty int) []game.OrderOption {
	if !p.Drafted {
		return nil
	}
	if _, ok := fireOn(w, tx, ty); !ok {
		return nil
	}
	return []game.OrderOption{
		{Label: "Go here", Cmd: command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty}},
		{Label: "Extinguish fire", Cmd: command.Command{Kind: command.Prioritize, Pawn: uint64(p.ID), X: tx, Y: ty, Order: command.OrderExtinguish}},
	}
}

// FloatIcon is the icon of an order-menu option ("" for none).
func FloatIcon(o game.OrderOption) string {
	if o.Cmd.Kind == command.Prioritize && o.Cmd.Order == command.OrderExtinguish {
		return "extinguish"
	}
	return ""
}
