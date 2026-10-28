package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestSteamVentResolvesToItsSprite(t *testing.T) {
	t.Parallel()
	if got := buildSprite("steam vent", "", 0, false, false); got != "structure/steam_vent" {
		t.Fatalf("sprite = %q", got)
	}
	if got := buildSpriteStuff("steam vent", "", "", 0, false, false); got != "structure/steam_vent" {
		t.Fatalf("stuff sprite = %q", got)
	}
}

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
		if got := buildingTitle(c.b); got != c.want {
			t.Errorf("%+v: %q, want %q", c.b, got, c.want)
		}
	}
	if ventLine(game.BuildingView{Kind: "steam vent"}) == "" || ventLine(game.BuildingView{Kind: "wall"}) != "" {
		t.Error("vent line only for vents")
	}
}
