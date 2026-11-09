package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// TestSavingRowHoldsTheAnchor: while a background save is written the
// Saving row sits at the toast anchor and the toasts stack above it; once
// shown it stays SavingMinFrames Updates, so a quick save does not flicker.
func TestSavingRowHoldsTheAnchor(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	f.env.Colony = true
	f.lay = hudlayout.Layout{HintX: 20, HintBottom: 600}
	f.env.Updates = 100
	c.Toast("Screenshot saved")
	free := c.ToastRects(1280, 720)[0]
	if _, ok := c.SavingRect(1280, 720); ok {
		t.Fatal("a Saving row with no save")
	}
	f.env.Saving = true
	row, ok := c.SavingRect(1280, 720)
	if !ok || row.Y != free.Y {
		t.Fatalf("saving row %+v (%v), want it where the toast was (%+v)", row, ok, free)
	}
	if up := c.ToastRects(1280, 720)[0]; up.Y >= row.Y {
		t.Fatalf("the toast %+v is not above the saving row %+v", up, row)
	}
	f.env.Saving = false // written at once: the row stays for the minimum
	for u := uint64(1); u < SavingMinFrames; u++ {
		f.env.Updates = 100 + u
		if !c.SavingShown() {
			t.Fatalf("the Saving row went away after %d Updates (min %d)", u, SavingMinFrames)
		}
	}
	f.env.Updates = 100 + SavingMinFrames
	if c.SavingShown() {
		t.Fatal("the Saving row outstays its minimum")
	}
	if got := c.ToastRects(1280, 720)[0]; got.Y != free.Y {
		t.Fatalf("the toast %+v did not return to the anchor %+v", got, free)
	}
	c.DrawToasts(ebiten.NewImage(1280, 720), 1280, 720)
}

// TestAlarmIsADangerToastThatStaysLonger: a failure is a danger toast and
// outlives an ordinary one.
func TestAlarmIsADangerToastThatStaysLonger(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	c.Alarm("save failed: disk full")
	c.Toast("Screenshot saved")
	f.env.Updates = ToastFrames
	ts := c.LiveToasts()
	if len(ts) != 1 || !ts[0].Danger || ts[0].Text != "save failed: disk full" {
		t.Fatalf("live toasts %+v, want only the alarm", ts)
	}
	f.env.Updates = AlarmFrames
	if len(c.LiveToasts()) != 0 {
		t.Fatal("the alarm never goes")
	}
	f.env.Updates = 0
	c.DrawToasts(ebiten.NewImage(640, 480), 640, 480)
}
