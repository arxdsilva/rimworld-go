package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
)

func TestRuinAndVentTitles(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: "wall", Stuff: "granite blocks", Ruin: true}, "Ruined granite wall"},
		{game.BuildingView{Kind: "wall", Stuff: "wood", Ruin: true}, "Ruined wooden wall"},
		{game.BuildingView{Kind: "floor", Stuff: "stone", Ruin: true}, "Ruined stone floor"},
		{game.BuildingView{Kind: "wall", Stuff: "granite blocks"}, "Granite wall"},
		{game.BuildingView{Kind: "steam vent"}, "Steam vent"},
	} {
		if got := inspector.BuildingTitle(c.b); got != c.want {
			t.Errorf("%+v: %q, want %q", c.b, got, c.want)
		}
	}
	if env.VentLine(game.BuildingView{Kind: "steam vent"}) == "" || env.VentLine(game.BuildingView{Kind: "wall"}) != "" {
		t.Error("vent line only for vents")
	}
}
