package architect

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestCropTipCarriesTheNumbers(t *testing.T) {
	t.Parallel()
	tip := CropTip(game.CropView{Name: "rice", Label: "rice", Days: 5.5, GravelDay: 7.8, RichDays: 4.6, Yield: 6, MinSkill: 5, Hydro: true, FrostKill: -50})
	for _, want := range []string{"Rice\n", "Ripe in 5.5 days", "Yields 6 rice", "Plants 5", "hydroponics", "below -5 C"} {
		if !strings.Contains(tip, want) {
			t.Errorf("%q missing from %q", want, tip)
		}
	}
}
