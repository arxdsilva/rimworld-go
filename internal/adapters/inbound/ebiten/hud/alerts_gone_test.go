package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/axview"
)

// TestGoneAlert: "Nobody left" counts down to the wanderers option and has an
// explanation; it is silent while anyone is left.
func TestGoneAlert(t *testing.T) {
	t.Parallel()
	_, w := rightWorld()
	if _, ok := GoneAlert(w); ok {
		t.Fatal("the alert is up with colonists about")
	}
	w.AX = axview.World{EveryoneGone: true, WanderersMax: 3, WanderersLeft: 7, WanderersIn: 6 * 2500}
	al, ok := GoneAlert(w)
	if !ok || al.Label != "Nobody left: wanderers in 6h" {
		t.Fatalf("alert %+v ok %v", al, ok)
	}
	w.AX.WanderersIn, w.AX.WanderersOK = 0, true
	if al, _ := GoneAlert(w); !strings.Contains(al.Label, "can come") {
		t.Fatalf("open: %q", al.Label)
	}
	w.AX.WanderersLeft = 0
	if al, _ := GoneAlert(w); al.Label != "Nobody left" {
		t.Fatalf("spent: %q", al.Label)
	}
	if len(AlertExplain(GoneAlertKey)) < 24 || AlertExplain(GoneAlertKey) == AlertExplain("nope") {
		t.Fatal("no explanation for the gone alert")
	}
}
