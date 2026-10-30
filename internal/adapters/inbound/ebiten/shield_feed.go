package ebiten

import (
	"reflect"

	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/application/game"
)

// shieldStates appends each pawn's shield belt charge to out for the cue
// bus (Bus.ListenShields: a pop when a charge falls to 0). PawnView.ShieldPct
// is CBT-3's; until the field exists nothing is appended. It reads the
// field by index, so a frame allocates nothing once out has grown.
func shieldStates(out []cue.ShieldState, pawns []game.PawnView) []cue.ShieldState {
	t := reflect.TypeOf(game.PawnView{})
	idx := -1
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "ShieldPct" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return out
	}
	for i := range pawns {
		f := reflect.ValueOf(&pawns[i]).Elem().Field(idx)
		if f.CanInt() {
			out = append(out, cue.ShieldState{ID: pawns[i].ID, Pct: int(f.Int()), X: pawns[i].X, Y: pawns[i].Y})
		}
	}
	return out
}
