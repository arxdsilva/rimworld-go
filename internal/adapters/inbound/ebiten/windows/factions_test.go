package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// facShell is a fakeShell with n factions.
type facShell struct {
	*fakeShell
	n int
}

func (s facShell) Factions() []Faction {
	out := make([]Faction, s.n)
	for i := range out {
		out[i] = Faction{Name: "Faction", TypeLabel: "Outlander", Tech: "Industrial", Standing: "Neutral", Leader: "Leader"}
	}
	return out
}

// TestFactionRowsAre80AndScroll (UIS-068): the window wants header + n x 80 +
// foot, rows sit 80 apart, and past the room the list scrolls to its end.
func TestFactionRowsAre80AndScroll(t *testing.T) {
	t.Parallel()
	for _, n := range []int{6, 30} {
		f, _ := newFake()
		sh := facShell{f, n}
		w := New(sh)
		f.env.Game = fakeReader{}
		w.Factions().Launch()
		ww, wh := w.Want()
		if want := float32(hudlayout.FactionHeadH + n*hudlayout.FactionRowH + hudlayout.FactionFootH); ww != colFacW || wh != want {
			t.Fatalf("n=%d wants %vx%v, want %vx%v", n, ww, wh, colFacW, want)
		}
		f.kit.Begin()
		w.Declare()
		r0, ok0 := f.kit.Find(ColFacListID + ".0")
		r1, ok1 := f.kit.Find(ColFacListID + ".1")
		if !ok0 || !ok1 || r1.R.Y-r0.R.Y != hudlayout.FactionRowH || r0.R.H != hudlayout.FactionRowH {
			t.Fatalf("n=%d rows: %+v %+v", n, r0.R, r1.R)
		}
		list, _ := f.kit.Find(ColFacListID)
		over := float32(n*hudlayout.FactionRowH) > list.R.H
		f.kit.Scroll[ColFacListID] = 1e6
		f.kit.Begin()
		w.Declare()
		got := f.kit.Scroll[ColFacListID]
		if over && got != float32(n*hudlayout.FactionRowH)-list.R.H {
			t.Fatalf("n=%d scrolls to %v, want the end of the list", n, got)
		}
		if !over && got != 0 {
			t.Fatalf("n=%d fits (%v rows in %v) but scrolls to %v", n, n*hudlayout.FactionRowH, list.R.H, got)
		}
		if n == 30 && !over {
			t.Fatal("30 factions fit in the window")
		}
	}
}

func TestFactionInitialSkipsTheArticle(t *testing.T) {
	for in, want := range map[string]string{"the Lone Drift": "L", "Oakwood": "O", "people": "P", "": "?", "the ": "T"} {
		if got := facInitial(in); got != want {
			t.Errorf("facInitial(%q) = %q, want %q", in, got, want)
		}
	}
}
