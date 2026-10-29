package inspector

import (
	"sort"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// colonistAction is one colonist's way to act on a tile, from the same
// option list as the right-click order menu (game.OrdersAt).
type ColonistAction struct {
	Pawn     entity.EntityID
	Name     string
	Label    string // the order menu's label, e.g. "Equip rifle"
	Cmd      command.Command
	Disabled bool
	Reason   string
	Dist     int // tiles (Chebyshev) from the colonist to the target
}

// maxColonistActions caps the colonist chips a panel shows.
const maxColonistActions = 6

// colonistActions lists, per living and able colonist, the first order at
// (tx, ty) that match accepts: enabled ones first, then nearest, then by
// id; at most maxColonistActions.
func (a Pawn) ColonistActions(tx, ty int, match func(game.OrderOption) bool) []ColonistAction {
	if a.sim() == nil {
		return nil
	}
	var out []ColonistAction
	for _, p := range a.shell.Pawns() {
		if p.Hostile || p.Prisoner || p.Embodied || p.Status != "alive" {
			continue
		}
		for _, o := range a.sim().OrdersAt(p.ID, tx, ty) {
			if !match(o) {
				continue
			}
			out = append(out, ColonistAction{Pawn: p.ID, Name: p.Name, Label: o.Label, Cmd: o.Cmd,
				Disabled: o.Disabled, Reason: o.Reason, Dist: max(abs(p.X-tx), abs(p.Y-ty))})
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Disabled != out[j].Disabled {
			return !out[i].Disabled
		}
		if out[i].Dist != out[j].Dist {
			return out[i].Dist < out[j].Dist
		}
		return out[i].Pawn < out[j].Pawn
	})
	if len(out) > maxColonistActions {
		out = out[:maxColonistActions]
	}
	return out
}

// orderIs matches order options by their command's Order.
func OrderIs(orders ...command.Order) func(game.OrderOption) bool {
	return func(o game.OrderOption) bool {
		for _, want := range orders {
			if o.Cmd.Kind == command.Prioritize && o.Cmd.Order == want {
				return true
			}
		}
		return false
	}
}
