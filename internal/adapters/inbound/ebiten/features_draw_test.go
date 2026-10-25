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

func TestSteamPuffFrameAndAlpha(t *testing.T) {
	t.Parallel()
	phase := steamPhase(10, 20)
	period := steamPeriod(10, 20)
	if period < steamPeriodMin || period > steamPeriodMin+steamPeriodSpread {
		t.Fatalf("period %d out of range", period)
	}
	start := uint64(period) - phase // tick where this vent's burst begins
	if _, _, ok := steamPuff(start-1, 10, 20); ok {
		t.Fatal("puff before the burst")
	}
	f, a, ok := steamPuff(start, 10, 20)
	if !ok || f != 0 || a <= 0.5 {
		t.Fatalf("burst start: frame %d alpha %v ok %v", f, a, ok)
	}
	f2, a2, ok := steamPuff(start+steamFrameTicks*3, 10, 20)
	if !ok || f2 != 3 || a2 >= a {
		t.Fatalf("late burst: frame %d alpha %v (start %v)", f2, a2, a)
	}
	if _, _, ok := steamPuff(start+steamFrameTicks*4, 10, 20); ok {
		t.Fatal("puff after the burst")
	}
	// the burst repeats every period
	if f3, a3, ok := steamPuff(start+uint64(period), 10, 20); !ok || f3 != 0 || a3 != a {
		t.Fatal("burst does not repeat")
	}
}

func TestSteamPhaseVariesByPosition(t *testing.T) {
	t.Parallel()
	seen := map[uint64]bool{}
	for x := 0; x < 8; x++ {
		seen[steamPhase(x*7, x*3)] = true
	}
	if len(seen) < 4 {
		t.Fatalf("only %d distinct phases", len(seen))
	}
}

func TestRuinOverlayKeyStablePerCell(t *testing.T) {
	t.Parallel()
	counts := map[string]int{}
	for x := 0; x < 30; x++ {
		for y := 0; y < 30; y++ {
			k := ruinOverlayKey(x, y)
			if k != ruinOverlayKey(x, y) {
				t.Fatal("unstable")
			}
			counts[k]++
		}
	}
	if len(counts) != 3 {
		t.Fatalf("variants used: %v", counts)
	}
}

func TestRuinOverlayOnlyForRuinWalls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		b    game.BuildingView
		want bool
	}{
		{game.BuildingView{Kind: "wall", Ruin: true}, true},
		{game.BuildingView{Kind: "wall"}, false},
		{game.BuildingView{Kind: "floor", Ruin: true}, false},
		{game.BuildingView{Kind: "door", Ruin: true}, false},
	}
	for _, c := range cases {
		if got := wantsRuinOverlay(c.b); got != c.want {
			t.Errorf("%+v: got %v", c.b, got)
		}
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
