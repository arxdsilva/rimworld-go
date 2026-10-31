package animal

import "testing"

// TestEveryLivingSpeciesHasAComfortRange (TEMP-3): every species but the
// machines has a range with min under max, and the wiki's published
// numbers hold.
func TestEveryLivingSpeciesHasAComfortRange(t *testing.T) {
	for _, s := range All() {
		lo, hi, ok := Comfort(s)
		if ok == IsMech(s) || ok && (lo >= hi || hi == 0 && lo == 0) {
			t.Errorf("%s: %d..%d %v", s, lo, hi, ok)
		}
	}
	for s, want := range map[Species][2]int{Alpaca: {-200, 450}, Dromedary: {-220, 600}, ArcticWolf: {-550, 400},
		Capybara: {-100, 500}, Cobra: {0, 600}, Cat: {-250, 400}, Chinchilla: {-150, 550}, Iguana: {0, 600}, HiveBeetle: {0, 600}} {
		if lo, hi, _ := Comfort(s); lo != want[0] || hi != want[1] {
			t.Errorf("%s: %d..%d, want %v", s, lo, hi, want)
		}
	}
	if !Slows(HiveBrute) || Slows(Cow) || Slows(Harrow) {
		t.Error("only the hive insects slow")
	}
}
