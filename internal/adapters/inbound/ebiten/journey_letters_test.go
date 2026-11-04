package ebiten

import (
	"fmt"
	"testing"
)

// TestJourneyLetterOlderOpens: with more letters than rows, the "+N older"
// row is a button that opens the newest letter folded into it (RHUD-474).
func TestJourneyLetterOlderOpens(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	keys := []string{"flu", "plague", "malaria", "sleeping_sickness", "gut_worms", "muscle_parasites", "heat_wave", "cold_snap", "volcanic_winter", "eclipse", "toxic_fallout", "solar_flare", "party", "crop_blight", "short_circuit"}
	for _, k := range keys {
		j.g.FireIncident(k)
	}
	if n := len(j.world().Letters); n <= 8 {
		t.Fatalf("only %d letters posted", n)
	}
	j.update(2)
	rows := j.a.letterRows(j.a.hud())
	if len(rows) == 0 || rows[0].Older == 0 {
		t.Fatalf("rows %+v: no +N older row", rows)
	}
	want := j.world().Letters[len(j.world().Letters)-(len(rows)-1)-1].ID
	if !j.clickID(fmt.Sprintf("letter.%d", want)) {
		t.Fatal("the older row is not a button")
	}
	if j.a.uiModalTop() != fmt.Sprintf("letter.%d", want) {
		t.Fatal("clicking the older row opened no letter")
	}
}
