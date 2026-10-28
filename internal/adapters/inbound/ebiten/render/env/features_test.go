package env

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestSteamPuffFrameAndAlpha(t *testing.T) {
	t.Parallel()
	phase := SteamPhase(10, 20)
	period := SteamPeriod(10, 20)
	if period < SteamPeriodMin || period > SteamPeriodMin+SteamPeriodSpread {
		t.Fatalf("period %d out of range", period)
	}
	start := uint64(period) - phase // tick where this vent's burst begins
	if _, _, ok := SteamPuff(start-1, 10, 20); ok {
		t.Fatal("puff before the burst")
	}
	f, a, ok := SteamPuff(start, 10, 20)
	if !ok || f != 0 || a <= 0.5 {
		t.Fatalf("burst start: frame %d alpha %v ok %v", f, a, ok)
	}
	f2, a2, ok := SteamPuff(start+SteamFrameTicks*3, 10, 20)
	if !ok || f2 != 3 || a2 >= a {
		t.Fatalf("late burst: frame %d alpha %v (start %v)", f2, a2, a)
	}
	if _, _, ok := SteamPuff(start+SteamFrameTicks*4, 10, 20); ok {
		t.Fatal("puff after the burst")
	}
	// the burst repeats every period
	if f3, a3, ok := SteamPuff(start+uint64(period), 10, 20); !ok || f3 != 0 || a3 != a {
		t.Fatal("burst does not repeat")
	}
}

func TestSteamPhaseVariesByPosition(t *testing.T) {
	t.Parallel()
	seen := map[uint64]bool{}
	for x := 0; x < 8; x++ {
		seen[SteamPhase(x*7, x*3)] = true
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
			k := RuinOverlayKey(x, y)
			if k != RuinOverlayKey(x, y) {
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
		if got := WantsRuinOverlay(c.b); got != c.want {
			t.Errorf("%+v: got %v", c.b, got)
		}
	}
}

func TestDuskGlowStrength(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    game.PlantView
		want float64
	}{
		{"other species", game.PlantView{Species: "moss", Growth: 100}, 0},
		{"seedling", game.PlantView{Species: "duskmoss", Growth: 0}, 0},
		{"half grown", game.PlantView{Species: "duskmoss", Growth: 50}, 0.5},
		{"mature", game.PlantView{Species: "duskmoss", Growth: 100}, 1},
		{"over range", game.PlantView{Species: "duskmoss", Growth: 400}, 1},
		{"wilted", game.PlantView{Species: "duskmoss", Growth: 100, Wilted: true}, 0},
	}
	for _, c := range cases {
		if got := DuskGlowStrength(c.p); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// the glow is faint: it may tint a cave but never light it
	if DuskGlowMax > 0.12 {
		t.Errorf("duskGlowMax = %v is too bright", DuskGlowMax)
	}
}
