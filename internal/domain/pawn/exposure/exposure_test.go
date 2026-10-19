package exposure

import "testing"

func TestHypothermiaBuildsAndFades(t *testing.T) {
	hypo, ticks := 0, 0
	for hypo < Serious {
		hypo, _ = Step(hypo, 0, -100, 250, Everyday)
		ticks += 250
	}
	if hours := ticks / 2500; hours < 2 || hours > 4 {
		t.Errorf("collapse at -10 C after %d hours, want 2-4", hours)
	}
	for i := 0; i < 20; i++ {
		hypo, _ = Step(hypo, 0, 200, 250, Everyday)
	}
	if hypo != 0 {
		t.Errorf("hypothermia should fade in a warm room, got %d", hypo)
	}
	if h, _ := Step(0, 0, 50, 250, Everyday); h != 0 {
		t.Error("cool but above freezing is not dangerous")
	}
	if _, heat := Step(0, 0, 450, 250, Everyday); heat <= 0 {
		t.Error("45 C should build heatstroke")
	}
}

func TestParkaBandsSurviveWinter(t *testing.T) {
	parka := Everyday.Wearing(200+60, -30) // parka + tuque
	if parka.Min != -180 || parka.ColdDanger() != -260 {
		t.Fatalf("bands = %+v", parka)
	}
	if h, _ := Step(0, 0, -120, 250, parka); h != 0 {
		t.Error("a -12 C winter night is fine in a parka and tuque")
	}
	if h, _ := Step(0, 0, -120, 250, Everyday); h == 0 {
		t.Error("but deadly in everyday clothes")
	}
}
