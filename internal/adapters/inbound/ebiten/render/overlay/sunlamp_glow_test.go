package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestSunLampGlowFollowsGlowNotPower: the lamp shines by the sim's Glow
// (powered and by day), a basin by Powered.
func TestSunLampGlowFollowsGlowNotPower(t *testing.T) {
	t.Parallel()
	night := &game.BuildingView{Kind: SunLampKind, PowerRole: "consumer", Powered: true}
	if poweredLook(night) {
		t.Fatal("a powered lamp at night is dark")
	}
	day := &game.BuildingView{Kind: SunLampKind, PowerRole: "consumer", Powered: true, Glow: true}
	if !poweredLook(day) {
		t.Fatal("the lamp glows when the sim says so")
	}
	if !poweredLook(&game.BuildingView{Kind: "hydroponics basin", PowerRole: "consumer", Powered: true}) {
		t.Fatal("a powered basin glows")
	}
}
