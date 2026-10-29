package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"testing"
)

func TestNavEscapeIntent(t *testing.T) {
	t.Parallel()

	n := newNav()
	if got := n.escapeIntent(); got.kind != intentFinishBoot {
		t.Fatalf("boot escape = %v, want finish boot", got.kind)
	}

	n.reset(frame{screen: ports.ScreenMainMenu})
	if got := n.escapeIntent(); got.kind != intentNone {
		t.Fatalf("menu escape = %v, want none", got.kind)
	}

	n.push(frame{screen: ports.ScreenScenario})
	if got := n.escapeIntent(); got.kind != intentBack {
		t.Fatalf("wizard escape = %v, want back", got.kind)
	}

	n.reset(frame{screen: ports.ScreenColony})
	if got := n.escapeIntent(); got.kind != intentPause {
		t.Fatalf("colony escape = %v, want pause", got.kind)
	}

	n.setOverlay(overlayPause)
	if got := n.escapeIntent(); got.kind != intentResume {
		t.Fatalf("pause escape = %v, want resume", got.kind)
	}
}

func TestWizardNextOrder(t *testing.T) {
	t.Parallel()
	order := []ports.Screen{ports.ScreenScenario, ports.ScreenStoryteller, ports.ScreenWorldGen, ports.ScreenLanding, ports.ScreenPawns}
	for i := 0; i < len(order)-1; i++ {
		if got := wizardNext(order[i]); got != order[i+1] {
			t.Fatalf("wizardNext(%s) = %s, want %s", order[i], got, order[i+1])
		}
	}
}

func TestHitButton(t *testing.T) {
	t.Parallel()
	buttons := []uikit.Button{
		{ID: "a", X: 10, Y: 10, W: 40, H: 20},
		{ID: "b", X: 10, Y: 40, W: 40, H: 20, Disabled: true, Reason: "nope"},
	}
	got, ok := uikit.HitButton(buttons, 15, 15)
	if !ok || got.ID != "a" {
		t.Fatalf("expected button a, got %+v ok=%v", got, ok)
	}
	got, ok = uikit.HitButton(buttons, 15, 45)
	if !ok || got.ID != "b" || !got.Disabled {
		t.Fatalf("expected disabled b, got %+v ok=%v", got, ok)
	}
	if _, ok := uikit.HitButton(buttons, 0, 0); ok {
		t.Fatal("miss should not hit")
	}
}
