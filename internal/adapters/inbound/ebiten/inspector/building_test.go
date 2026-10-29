package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestBillsBodyNotesAnEmptyBench(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 77, Kind: "stove", Bench: true}
	if body := BillsBody(b, 400); len(body.Buttons) != 0 || len(body.Labels) != 1 || !strings.Contains(body.Labels[0].Text, "No bills yet") {
		t.Fatalf("empty bench body = %+v", body)
	}
	b.Bills = threeBills()
	body := BillsBody(b, 400)
	if len(body.Buttons) != 9*len(b.Bills) {
		t.Fatalf("buttons = %d, want %d", len(body.Buttons), 9*len(b.Bills))
	}
	if body.H != float32(len(b.Bills))*BillRowH {
		t.Fatalf("content height = %v", body.H)
	}
	// the paused bill's pause button reads as pressed
	for _, bt := range body.Buttons {
		if bt.ID == "bill.pause.2" && !bt.Selected {
			t.Error("a suspended bill's pause button is selected")
		}
		if bt.ID == "bill.pause.0" && bt.Selected {
			t.Error("a running bill's pause button is not selected")
		}
	}
}

func TestPowerWarningLinesAreMarked(t *testing.T) {
	t.Parallel()
	for _, warn := range []string{"No power: brownout", "Not producing: no fuel", "Not connected: it makes power for nobody", "Brownout: some devices are shed"} {
		if !PaneLineWarns(warn) {
			t.Errorf("%q should draw in the warning colour", warn)
		}
	}
	for _, calm := range []string{"Uses 180 W", "Produces 0 W", "Grid: +350 W (800 W made, 450 W used)", "Closed (carries power)", "Out of fuel", ""} {
		if PaneLineWarns(calm) {
			t.Errorf("%q should draw in the body colour", calm)
		}
	}
}

func TestFmtTarget(t *testing.T) {
	t.Parallel()
	for tenths, want := range map[int]string{210: "21 °C", 215: "21.5 °C", 0: "0 °C", -50: "-5 °C", -55: "-5.5 °C", 5: "0.5 °C"} {
		if got := FmtTarget(tenths); got != want {
			t.Errorf("fmtTarget(%d) = %q, want %q", tenths, got, want)
		}
	}
}

func TestPenFoodVerdict(t *testing.T) {
	t.Parallel()
	pv := game.PenView{Enclosed: true, GrazePerDay: 30, DemandPerDay: 45, TroughStock: 12}
	got := strings.Join(PenFoodLines(pv), "|")
	for _, want := range []string{"30", "45", "12", "Short by 15 per day: add hay or expand"} {
		if !strings.Contains(got, want) {
			t.Errorf("food lines %q lack %q", got, want)
		}
	}
	pv.GrazePerDay = 45
	if got := strings.Join(PenFoodLines(pv), "|"); !strings.Contains(got, "Enough grazing") || strings.Contains(got, "Short") {
		t.Errorf("enough grazing: %q", got)
	}
	pv.GrazePerDay, pv.DemandPerDay = 0, 0
	if got := strings.Join(PenFoodLines(pv), "|"); !strings.Contains(got, "Enough grazing") {
		t.Errorf("an empty herd needs nothing: %q", got)
	}
}
func threeBills() []game.BillView {
	return []game.BillView{
		{Label: "Cook meal", ModeID: BillModeDo, Target: 5, Done: 2},
		{Label: "Make bandage", ModeID: BillModeForever, Target: 3, Status: "no cloth"},
		{Label: "Make pemmican", ModeID: BillModeUntil, Target: 10, Suspended: true},
	}
}
