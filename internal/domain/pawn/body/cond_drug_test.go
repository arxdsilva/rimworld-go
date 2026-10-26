package body

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestOverdoseStages (MED-07): doses add up through the wiki stages:
// major takes 500 consciousness, extreme caps it at 10 %; the sum is
// capped at the lethal 1000.
func TestOverdoseStages(t *testing.T) {
	var b Body
	if b.AddOverdose(0, 0) != 0 || b.Conds != nil {
		t.Fatal("no dose, no overdose")
	}
	if got := b.AddOverdose(300, 5); got != 300 || b.OverdoseStage() != 0 || b.Capacity(Consciousness) != 1000 {
		t.Fatalf("300: sev %d stage %d consciousness %d", got, b.OverdoseStage(), b.Capacity(Consciousness))
	}
	if got := b.AddOverdose(200, 6); got != 500 || b.OverdoseStage() != 1 || b.Capacity(Consciousness) != 500 {
		t.Fatalf("500: sev %d stage %d consciousness %d", got, b.OverdoseStage(), b.Capacity(Consciousness))
	}
	if got := b.AddOverdose(250, 7); got != 750 || b.OverdoseStage() != 2 || b.Capacity(Consciousness) != 100 {
		t.Fatalf("750: sev %d stage %d consciousness %d", got, b.OverdoseStage(), b.Capacity(Consciousness))
	}
	if got := b.AddOverdose(400, 8); got != OverdoseDeath {
		t.Fatalf("capped at death: %d", got)
	}
	if c, _ := b.Cond(CondOverdose); c.Since != 5 || len(b.Conds) != 1 {
		t.Fatalf("one overdose, from the first dose: %+v", b.Conds)
	}
}

// TestOverdoseDecaysPerDay: 1000 a day, exactly, stage only moving down,
// then gone.
func TestOverdoseDecaysPerDay(t *testing.T) {
	var b Body
	b.AddOverdose(800, 0)
	changes := 0
	for h := uint64(0); h < 24; h++ {
		if b.StepOverdose(h) {
			changes++
		}
		if h == 11 && b.Overdose() != 300 {
			t.Fatalf("after 12 h: %d, want 300", b.Overdose())
		}
	}
	if b.Overdose() != 0 || b.Conds != nil {
		t.Fatalf("after a day: %d %+v", b.Overdose(), b.Conds)
	}
	if changes != 3 { // extreme→major, major→none, gone
		t.Fatalf("stage changes %d, want 3", changes)
	}
	if b.StepOverdose(0) {
		t.Fatal("nothing to step")
	}
}

// TestPainkillerHalvesPainBody: an acting painkiller halves the pain for
// 12 hours, its severity the share left; a new dose starts over.
func TestPainkillerHalvesPainBody(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: Torso, Dmg: 40, Kind: Bruise}}} // pain 500
	if b.Pain() != 500 || b.Numbed() {
		t.Fatalf("pain %d", b.Pain())
	}
	b.Numb(100)
	if b.Pain() != 250 || !b.Numbed() {
		t.Fatalf("numbed pain %d", b.Pain())
	}
	h := uint64(clock.TicksPerHour)
	if b.StepPainkiller(100 + 6*h) {
		t.Fatal("still acting at 6 h")
	}
	if c, _ := b.Cond(CondPainkiller); c.Sev != 500 {
		t.Fatalf("half left: %d", c.Sev)
	}
	b.Numb(100 + 6*h) // a fresh dose
	if b.StepPainkiller(100 + 12*h) {
		t.Fatal("the fresh dose acts 12 h from its own start")
	}
	if !b.StepPainkiller(100+18*h) || b.Numbed() || b.Pain() != 500 || b.Conds != nil {
		t.Fatalf("worn off: pain %d conds %+v", b.Pain(), b.Conds)
	}
	if b.StepPainkiller(100 + 30*h) {
		t.Fatal("nothing to step")
	}
}
