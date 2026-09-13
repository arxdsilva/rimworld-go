package ebiten

import "testing"

func TestNavEscapeIntent(t *testing.T) {
	t.Parallel()

	n := newNav()
	if got := n.escapeIntent(); got.kind != intentFinishBoot {
		t.Fatalf("boot escape = %v, want finish boot", got.kind)
	}

	n.reset(frame{screen: screenMainMenu})
	if got := n.escapeIntent(); got.kind != intentNone {
		t.Fatalf("menu escape = %v, want none", got.kind)
	}

	n.push(frame{screen: screenScenario})
	if got := n.escapeIntent(); got.kind != intentBack {
		t.Fatalf("wizard escape = %v, want back", got.kind)
	}

	n.reset(frame{screen: screenColony})
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
	order := []screenID{screenScenario, screenStoryteller, screenWorldGen, screenLanding, screenPawns}
	for i := 0; i < len(order)-1; i++ {
		if got := wizardNext(order[i]); got != order[i+1] {
			t.Fatalf("wizardNext(%s) = %s, want %s", order[i], got, order[i+1])
		}
	}
}

func TestHitButton(t *testing.T) {
	t.Parallel()
	buttons := []button{
		{id: "a", x: 10, y: 10, w: 40, h: 20},
		{id: "b", x: 10, y: 40, w: 40, h: 20, disabled: true, reason: "nope"},
	}
	got, ok := hitButton(buttons, 15, 15)
	if !ok || got.id != "a" {
		t.Fatalf("expected button a, got %+v ok=%v", got, ok)
	}
	got, ok = hitButton(buttons, 15, 45)
	if !ok || got.id != "b" || !got.disabled {
		t.Fatalf("expected disabled b, got %+v ok=%v", got, ok)
	}
	if _, ok := hitButton(buttons, 0, 0); ok {
		t.Fatal("miss should not hit")
	}
}
