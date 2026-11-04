package inspector

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// The merged log (RHUD-027) is newest first across both sources, keeps every
// line, and stamps a social line with the day and hour of its tick.
func TestMergeLogNewestFirst(t *testing.T) {
	own := []game.LogLineView{{Tick: 300, Text: "own late"}, {Tick: 100, Text: "own early"}}
	social := []game.SocialLogLineView{{Tick: 200, Text: "chat"}, {Tick: clock.TicksPerDay + clock.TicksPerHour*3, Text: "next day"}}
	got := mergeLog(own, social)
	want := []string{"next day", "own late", "chat", "own early"}
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Text != w {
			t.Fatalf("line %d is %q, want %q", i, got[i].Text, w)
		}
	}
	// the same calendar as the pawn's own lines
	if d := clock.DateAt(clock.Tick(got[0].Tick)); got[0].Day != d.Day+1 || got[0].Hour != d.Hour || d.Hour != 3 {
		t.Fatalf("social line stamped day %d hour %d, date %+v", got[0].Day, got[0].Hour, d)
	}
	if &own[0] == &got[0] || own[0].Text != "own late" {
		t.Fatal("the merge touched the view's own slice")
	}
}

func TestMergeLogCap(t *testing.T) {
	own := make([]game.LogLineView, mergedLogCap+10)
	if got := mergeLog(own, nil); len(got) != mergedLogCap {
		t.Fatalf("%d lines kept", len(got))
	}
}

// Fixed tabs sit against the strip's right edge, in order, one width each;
// more than fit share the strip (RHUD-005, 007).
func TestFixedTabRects(t *testing.T) {
	strip := gfx.Rect{X: 0, Y: 10, W: 432, H: 24}
	rs := fixedTabRects(strip, 3, 72)
	if len(rs) != 3 {
		t.Fatal(len(rs))
	}
	if got := rs[2].Right(); got != strip.Right() {
		t.Fatalf("the last tab ends at %v, not the strip's %v", got, strip.Right())
	}
	for i := 1; i < 3; i++ {
		if rs[i].X <= rs[i-1].Right()-1 {
			t.Fatalf("tab %d at %v overlaps %v", i, rs[i], rs[i-1])
		}
	}
	many := fixedTabRects(strip, 10, 72)
	if many[0].X < strip.X-0.01 || many[9].Right() > strip.Right()+0.01 {
		t.Fatalf("ten tabs overflow: %v .. %v", many[0], many[9])
	}
	if fixedTabRects(strip, 0, 72) != nil {
		t.Fatal("rects for no tabs")
	}
}
