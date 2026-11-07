package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/domain/sim/command"
)

// TestScheduleAreaStripMatchesDropdownCommand (UIS-062 journey): in the real
// loop the Schedule's area column is a strip of segments (Unrestricted,
// Home, then each area); a segment click sends AssignArea with the ref the
// dropdown's picker row sends. With more areas than fit at SchedSegMin the
// column is the dropdown again.
func TestScheduleAreaStripMatchesDropdownCommand(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	for i := 0; i < 2; i++ {
		j.g.Enqueue(command.Command{Kind: command.CreateArea})
		j.g.ApplyPending()
	}
	areas := j.g.Areas()
	if len(areas) != 2 {
		t.Fatalf("%d areas, want 2", len(areas))
	}
	pawn := strconv.FormatUint(uint64(j.g.Snapshot()[0].ID), 10)
	j.a.win.Show("schedule")
	j.update(2)
	home := "sched.area." + pawn + "." + strconv.Itoa(command.AreaHome)
	if !hasWidget(j.a, home) || hasWidget(j.a, "sched.area."+pawn) {
		t.Fatalf("two areas should show the strip: home segment %v, dropdown %v", hasWidget(j.a, home), hasWidget(j.a, "sched.area."+pawn))
	}
	// the architect's mirror of the window's geometry: the strip starts after
	// the hours plus the gap and spans SchedAreaW
	first, _ := j.a.uiSt().Find("sched.area." + pawn + "." + strconv.Itoa(command.AreaUnrestricted))
	last, _ := j.a.uiSt().Find("sched.area." + pawn + "." + strconv.Itoa(int(areas[1].ID)))
	wantX := j.a.win.Rect().X + 12 + windows.SchedNameW + 24*windows.SchedCellW + windows.SchedAreaGap
	if first.R.X != wantX || last.R.Right()-first.R.X != windows.SchedAreaW {
		t.Fatalf("strip at x %v spanning %v, want x %v spanning %v", first.R.X, last.R.Right()-first.R.X, wantX, windows.SchedAreaW)
	}
	if !j.clickID(home) {
		t.Fatal("Home segment not clickable")
	}
	strip, ok := j.lastCmd(command.AssignArea)
	if !ok || strip.Index != command.AreaHome {
		t.Fatalf("Home segment sent %+v (%v)", strip, ok)
	}
	area := int(areas[1].ID)
	if !j.clickID("sched.area." + pawn + "." + strconv.Itoa(area)) {
		t.Fatal("area segment not clickable")
	}
	if c, ok := j.lastCmd(command.AssignArea); !ok || c.Index != area || c.Pawn != strip.Pawn {
		t.Fatalf("area segment sent %+v (%v)", c, ok)
	}

	// Seven more areas: the segments would be under 40 px, the dropdown returns
	// and its picker row sends the same command shape.
	for i := 0; i < 7; i++ {
		j.g.Enqueue(command.Command{Kind: command.CreateArea})
		j.g.ApplyPending()
	}
	j.update(2)
	if hasWidget(j.a, home) || !hasWidget(j.a, "sched.area."+pawn) {
		t.Fatal("nine areas should fall back to the dropdown")
	}
	j.clickID("sched.area." + pawn)
	j.update(1)
	if !j.clickID("areapick.row." + strconv.Itoa(command.AreaHome)) {
		t.Fatal("no Home row in the picker")
	}
	if c, ok := j.lastCmd(command.AssignArea); !ok || c.Index != strip.Index || c.Pawn != strip.Pawn {
		t.Fatalf("dropdown sent %+v, strip sent %+v", c, strip)
	}
}
