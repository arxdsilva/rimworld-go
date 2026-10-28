package ebiten

import (
	"strings"
	"testing"
)

func TestMainMenuButtonsRightRail(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	mid := float32(a.sw) / 2
	found := 0
	for _, b := range a.mainMenuButtons(a.sw, a.sh) {
		if !strings.HasPrefix(b.ID, "menu.") {
			continue
		}
		found++
		if b.X <= mid {
			t.Fatalf("%s x=%v, want > %v", b.ID, b.X, mid)
		}
	}
	if found < 8 {
		t.Fatalf("menu buttons = %d, want at least 8", found)
	}
}
