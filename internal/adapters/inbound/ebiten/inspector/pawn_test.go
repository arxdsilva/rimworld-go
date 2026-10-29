package inspector

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestPawnStatusSaysBleeding(t *testing.T) {
	t.Parallel()
	name := func(entity.EntityID) string { return "" }
	got := PawnStatusLines(game.PawnView{Status: "alive", Bleeding: true, BleedDeathHours: 7}, name)
	if !slices.Contains(got, "Bleeding (death in 7h)") {
		t.Fatalf("lines %v", got)
	}
	if got := PawnStatusLines(game.PawnView{Status: "dead", Bleeding: true}, name); slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "Bleeding") }) {
		t.Fatalf("the dead do not bleed: %v", got)
	}
}

func TestInspectorShowsWaitingFor(t *testing.T) {
	names := map[entity.EntityID]string{7: "Dustbuck"}
	lookup := func(id entity.EntityID) string { return names[id] }
	if got := WaitingForLine(7, lookup); got != "waiting for Dustbuck" {
		t.Fatalf("got %q", got)
	}
	if got := WaitingForLine(0, lookup); got != "" {
		t.Fatalf("unblocked should show nothing, got %q", got)
	}
	if got := WaitingForLine(9, lookup); got != "waiting for something" {
		t.Fatalf("unknown blocker got %q", got)
	}
}

func TestPostponedLine(t *testing.T) {
	if PostponedLine(nil) != "" {
		t.Fatal("nothing put off: no line")
	}
	got := PostponedLine([]string{"build wall: blocked, retry in 12 s", "haul wood: can't reach"})
	if got != "put off: build wall: blocked, retry in 12 s (+1)" {
		t.Fatalf("line %q", got)
	}
}

// PLAY-017: the Gear tab marks the garments the player chose and names the
// one the colonist is walking to put on; the layout below follows.
func TestGearRows(t *testing.T) {
	p := game.PawnView{Apparel: []string{"parka", "tuque"}, ApparelForced: []bool{true, false}, ChangingInto: "duster"}
	rows := GearRows(p)
	want := []string{"everyday clothes", "leather parka  (you chose this)", "knitted tuque  (warmer)", "Changing into: duster"}
	if !slices.Equal(rows, want) {
		t.Fatalf("rows %q\nwant %q", rows, want)
	}
	if got := GearRows(game.PawnView{}); !slices.Equal(got, []string{"everyday clothes"}) {
		t.Fatalf("bare colonist rows %q", got)
	}
	// the comfort line (and the hint after it) sit below every Wearing row
	g := GearLayoutFor(len(rows) - 1)
	if last := g.Clothes + float32(len(rows))*gfx.LineStep(gfx.Small); g.Comfort < last || g.Hint <= g.Comfort {
		t.Fatalf("comfort at %v, hint at %v, want both below the last row ending at %v", g.Comfort, g.Hint, last)
	}
}

func TestLogLineTextAndWrap(t *testing.T) {
	t.Parallel()
	if got := LogLineText(game.LogLineView{Day: 2, Hour: 7, Text: "Ate a meal"}); got != "Day 2, 07h  Ate a meal" {
		t.Fatalf("text = %q", got)
	}
	rows := WrapLogRows([]game.LogLineView{{Day: 1, Hour: 1, Text: strings.Repeat("x", 60)}}, 200)
	if w, _ := gfx.Measure(rows[0], LogStyle); len(rows) < 2 || !strings.HasPrefix(rows[1], "  ") || w > 200 {
		t.Fatalf("rows = %q", rows)
	}
	if got := WrapLogRows(logs(3), 300); len(got) != 3 || !strings.Contains(got[0], "event A") {
		t.Fatalf("newest first: %q", got)
	}
	if ClampLogScroll(99, 10, 4) != 6 || ClampLogScroll(-1, 10, 4) != 0 {
		t.Fatal("scroll clamp")
	}
}
func logs(n int) []game.LogLineView {
	var out []game.LogLineView
	for i := 0; i < n; i++ { // newest first
		out = append(out, game.LogLineView{Day: 3, Hour: 20 - i, Text: "event " + string(rune('A'+i))})
	}
	return out
}

func TestPawnStatusShowsCarryLines(t *testing.T) {
	t.Parallel()
	names := map[entity.EntityID]string{3: "Ann", 4: "Bo"}
	name := func(id entity.EntityID) string { return names[id] }
	got := PawnStatusLines(game.PawnView{Status: "alive", CarriedBy: 3, CarryingPawn: 4}, name)
	if !slices.Contains(got, "Carried by Ann") || !slices.Contains(got, "Carrying Bo") {
		t.Fatalf("lines %v", got)
	}
	got = PawnStatusLines(game.PawnView{Status: "alive"}, name)
	for _, l := range got {
		if strings.HasPrefix(l, "Carr") {
			t.Fatalf("no carry line for zero ids: %v", got)
		}
	}
}
