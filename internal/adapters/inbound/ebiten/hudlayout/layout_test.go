package hudlayout

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// layoutScreens are the logical screens the invariants run on.
var layoutScreens = [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}}

// Six colonists fit one full-size row; thirty wrap to more rows at a
// smaller scale (18 + 12 at 0.75 on the default screen), and the bar never
// reaches the resource list or the right column.
func TestLayoutColonistBarWraps(t *testing.T) {
	t.Parallel()
	st := Inputs{GlobalH: 196, ResourcesH: 600, Alerts: 8, Letters: 4, LearnH: LearnHeaderH}
	for _, sz := range layoutScreens {
		st.Colonists = 6
		l := Compute(sz[0], sz[1], st)
		if l.BarScale != 1 || len(l.BarSlots) != 6 || l.BarMore != 0 || l.Bar.H != BarSlotH {
			t.Errorf("%v: 6 colonists = scale %v, %d slots (+%d), bar %+v; want one full-size row", sz, l.BarScale, len(l.BarSlots), l.BarMore, l.Bar)
		}
		st.Colonists = 30
		l = Compute(sz[0], sz[1], st)
		rows := map[float32]bool{}
		for _, s := range l.BarSlots {
			rows[s.Y] = true
		}
		if l.BarScale >= 1 || len(l.BarSlots) != 30 || l.BarMore != 0 {
			t.Errorf("%v: 30 colonists = scale %v, %d slots (+%d); want them all, smaller", sz, l.BarScale, len(l.BarSlots), l.BarMore)
		}
		if sz[0] < 1920 && len(rows) < 2 { // the big screen holds them in one smaller row
			t.Errorf("%v: 30 colonists sit in %d row(s), want them wrapped", sz, len(rows))
		}
		if sz == [2]int{1280, 720} {
			first := 0
			for _, s := range l.BarSlots {
				if s.Y == l.BarSlots[0].Y {
					first++
				}
			}
			if l.BarScale != 0.75 || first != 18 || len(rows) != 2 {
				t.Errorf("1280x720: 30 colonists = scale %v, %d in the first row, %d rows; want 18 + 12 at 0.75", l.BarScale, first, len(rows))
			}
		}
		for _, n := range []int{6, 30, 90} {
			st.Colonists = n
			l = Compute(sz[0], sz[1], st)
			if l.Resources.Empty() {
				t.Fatalf("%v: no resource list", sz)
			}
			if l.Bar.X < l.Resources.Right()+HudMargin || l.Bar.Right() > l.Right.X-HudMargin {
				t.Errorf("%v %d colonists: bar %+v reaches the resources (right %v) or the right column (%v)", sz, n, l.Bar, l.Resources.Right(), l.Right.X)
			}
		}
	}
}

func TestBoundsOf(t *testing.T) {
	t.Parallel()
	if got := BoundsOf(nil); got.W != 0 || got.H != 0 {
		t.Fatalf("BoundsOf(nil) = %+v, want zero", got)
	}
	rs := []gfx.Rect{{X: 10, Y: 20, W: 5, H: 5}, {X: 0, Y: 30, W: 4, H: 10}}
	if got := BoundsOf(rs); got != (gfx.Rect{X: 0, Y: 20, W: 15, H: 20}) {
		t.Fatalf("BoundsOf = %+v", got)
	}
}

func TestPerfRectEmptyWhenOff(t *testing.T) {
	t.Parallel()
	l := Compute(1280, 720, Inputs{GlobalH: 196, ResourcesH: 100})
	if !PerfRect(l, 0, 10).Empty() {
		t.Fatal("zero-width perf box must be empty")
	}
	if r := PerfRect(l, 100, 60); r.Empty() {
		t.Fatal("a small box fits on a 1280x720 screen")
	}
}
