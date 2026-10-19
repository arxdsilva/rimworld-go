package mentalbreak

import "testing"

func TestRisk(t *testing.T) {
	if RiskPerHour(Threshold) != 0 || RiskPerHour(50) != 0 {
		t.Error("content colonists don't break")
	}
	if RiskPerHour(10) <= RiskPerHour(15) || RiskPerHour(0) != 400 {
		t.Error("risk should rise as mood falls")
	}
	for _, k := range Kinds() {
		if d := DefOf(k); d.Label == "" || d.Duration == 0 {
			t.Errorf("break %d incomplete", k)
		}
	}
}
