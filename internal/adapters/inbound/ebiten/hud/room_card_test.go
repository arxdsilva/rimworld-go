package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestRoomCardStats(t *testing.T) {
	t.Parallel()
	rm := game.RoomView{Role: "hospital", Score: 41, Impressiveness: 412, Label: "impressive", Space: 305, SpaceLabel: "spacious",
		Beauty: 15, BeautyLabel: "pleasant", Wealth: 480, WealthLabel: "rich", CleanStat: -40, Cleanliness: "dirty", Cells: 20}
	got := strings.Join(RoomCardLines(rm), "|")
	for _, w := range []string{"Hospital|Impressiveness 41.2 (impressive)", "Space 30.5 (spacious)", "Beauty +1.5 (pleasant)",
		"Wealth 480 silver (rich)", "Cleanliness -0.40 (dirty)", "20 cells"} {
		if !strings.Contains(got, w) {
			t.Fatalf("card %q lacks %q", got, w)
		}
	}
	if !strings.Contains(strings.Join(RoomCardLines(game.RoomView{}), "|"), "(clean)") {
		t.Fatal("empty cleanliness falls back to clean")
	}
}

func TestRoomTintBands(t *testing.T) {
	t.Parallel()
	seen := map[[3]uint8]bool{}
	for _, s := range []int{0, 20, 30, 40, 50, 65, 85, 120, 170, 240} {
		c := RoomTint(s)
		seen[[3]uint8{c.R, c.G, c.B}] = true
	}
	if len(seen) != 10 {
		t.Fatalf("each band has its own tint, got %d", len(seen))
	}
	if RoomTint(19) != RoomTint(0) || RoomTint(20) == RoomTint(19) || RoomTint(500) != RoomTint(240) {
		t.Fatal("band edges")
	}
}
