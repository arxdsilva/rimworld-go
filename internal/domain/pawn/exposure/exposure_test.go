package exposure

import "testing"

func TestHypothermiaBuildsAndFades(t *testing.T) {
	hypo, ticks := 0, 0
	for hypo < Serious {
		hypo, _ = Step(hypo, 0, -100, 250)
		ticks += 250
	}
	if hours := ticks / 2500; hours < 2 || hours > 4 {
		t.Errorf("collapse at -10 C after %d hours, want 2-4", hours)
	}
	for i := 0; i < 20; i++ {
		hypo, _ = Step(hypo, 0, 200, 250)
	}
	if hypo != 0 {
		t.Errorf("hypothermia should fade in a warm room, got %d", hypo)
	}
	if h, _ := Step(0, 0, 50, 250); h != 0 {
		t.Error("cool but above freezing is not dangerous")
	}
	if _, heat := Step(0, 0, 450, 250); heat <= 0 {
		t.Error("45 C should build heatstroke")
	}
}
