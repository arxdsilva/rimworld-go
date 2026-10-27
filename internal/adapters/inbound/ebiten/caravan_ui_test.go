package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// TestCaravanShowsSickMembers: the caravan row notes its sick and hurt
// members, and its hover card gives every member's health (WS-M).
func TestCaravanShowsSickMembers(t *testing.T) {
	t.Parallel()
	c := game.CaravanView{Members: []string{"Ada", "Bo", "Cy"}, Health: []string{"flu", "", "bleeding"}}
	if n := caravanHealthNote(c); n != " · Ada: flu · Cy: bleeding" {
		t.Fatalf("note %q", n)
	}
	tip := caravanTip(c)
	for _, want := range []string{"Caravan: Ada, Bo, Cy", "Ada: flu", "Bo: well", "Cy: bleeding"} {
		if !strings.Contains(tip, want) {
			t.Fatalf("tip %q lacks %q", tip, want)
		}
	}
	if n := caravanHealthNote(game.CaravanView{Members: []string{"Ada"}}); n != "" {
		t.Fatalf("a well caravan notes %q", n)
	}
}
