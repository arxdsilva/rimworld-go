package inspector

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
)

// AIL-004: the inspector says why a downed colonist's exposure is not
// healing, from the view's numbers.
func TestExposureNoteLine(t *testing.T) {
	hot := game.PawnView{Heatstroke: 800, Temp: 334, ComfortMin: 50, ComfortMax: 292, ExposureNote: "Not recovering: air 33 °C, needs 29 °C or cooler"}
	if got := ExposureNoteLine(hot); got != "Not recovering: air 33.4 °C, needs 29.2 °C or cooler" {
		t.Fatalf("hot: %q", got)
	}
	if !slices.Contains(ExposureLines(hot), ExposureNoteLine(hot)) {
		t.Fatal("the note is on the status lines")
	}
	cold := game.PawnView{Hypothermia: 700, Temp: -50, ComfortMin: 160, ComfortMax: 260, ExposureNote: "x"}
	if got := ExposureNoteLine(cold); got != "Not recovering: air -5 °C, needs 16 °C or warmer" {
		t.Fatalf("cold: %q", got)
	}
	hot.ExposureNote = ""
	if ExposureNoteLine(hot) != "" {
		t.Fatal("no note while it heals")
	}
}
