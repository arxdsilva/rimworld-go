package uikit

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

func TestDialogFrameLayout(t *testing.T) {
	t.Parallel()
	frame, title, content, footer := DialogFrame(1280, 720)
	if frame != (gfx.Rect{X: 40, Y: 40, W: 1200, H: 640}) {
		t.Fatalf("frame at 1280x720 = %+v", frame)
	}
	if title != (gfx.Rect{X: 40, Y: 40, W: 1200, H: 44}) || footer != (gfx.Rect{X: 40, Y: 624, W: 1200, H: 56}) {
		t.Fatalf("title %+v footer %+v", title, footer)
	}
	if content != (gfx.Rect{X: 56, Y: 100, W: 1168, H: 508}) {
		t.Fatalf("content = %+v (16 px padding between the bars)", content)
	}
	frame, _, _, _ = DialogFrame(1920, 1080)
	if frame != (gfx.Rect{X: 340, Y: 160, W: 1240, H: 760}) {
		t.Fatalf("frame at 1920x1080 is capped and centred: %+v", frame)
	}
	back, next := FooterBack(footer), FooterNext(footer)
	if back != (gfx.Rect{X: 56, Y: 634, W: 140, H: 36}) || next != (gfx.Rect{X: 1064, Y: 634, W: 160, H: 36}) {
		t.Fatalf("back %+v next %+v", back, next)
	}

	r := gfx.Rect{X: 10, Y: 20, W: 100, H: 60}
	l, rt := r.SplitH(30)
	if l != (gfx.Rect{X: 10, Y: 20, W: 30, H: 60}) || rt != (gfx.Rect{X: 40, Y: 20, W: 70, H: 60}) {
		t.Fatalf("splitH %+v %+v", l, rt)
	}
	l, rt = r.SplitH(-30)
	if l.W != 70 || rt != (gfx.Rect{X: 80, Y: 20, W: 30, H: 60}) {
		t.Fatalf("splitH(-30) cuts from the right: %+v %+v", l, rt)
	}
	tp, bt := r.SplitV(-20)
	if tp.H != 40 || bt != (gfx.Rect{X: 10, Y: 60, W: 100, H: 20}) {
		t.Fatalf("splitV(-20) %+v %+v", tp, bt)
	}
	cols := gfx.Rect{X: 0, Y: 0, W: 100, H: 10}.Columns(3, 5)
	if len(cols) != 3 || cols[0].W != 30 || cols[1].X != 35 || cols[2].Right() != 100 {
		t.Fatalf("columns = %+v", cols)
	}
	rows := gfx.Rect{X: 0, Y: 0, W: 50, H: 100}.Rows(20, 5)
	if len(rows) != 4 || rows[3].Y != 75 {
		t.Fatalf("rows fit the height: %+v", rows)
	}
	if got := r.RowsN(5, 20, 0); len(got) != 5 || got[4].Y != 100 {
		t.Fatalf("rowsN overflows on purpose: %+v", got)
	}
	if !r.Contains(10, 20) || r.Contains(110, 20) || !r.Hit(50, 50) || r.Hit(9, 50) {
		t.Fatal("contains / hit")
	}
	if x, y := r.Center(); x != 60 || y != 50 {
		t.Fatalf("center %v,%v", x, y)
	}
	if in := r.Inset(5); in != (gfx.Rect{X: 15, Y: 25, W: 90, H: 50}) {
		t.Fatalf("inset %+v", in)
	}
	if !r.Intersect(gfx.Rect{X: 200, Y: 200, W: 5, H: 5}).Empty() || r.Intersect(gfx.Rect{X: 50, Y: 50, W: 100, H: 100}) != (gfx.Rect{X: 50, Y: 50, W: 60, H: 30}) {
		t.Fatal("intersect")
	}
	if c := r.Centered(20, 20); c != (gfx.Rect{X: 50, Y: 40, W: 20, H: 20}) {
		t.Fatalf("centered %+v", c)
	}
}

func TestProgressAndSparkline(t *testing.T) {
	t.Parallel()
	r := gfx.Rect{X: 0, Y: 0, W: 100, H: 40}
	pts := SparkPoints([]int{1, 5, 3, 9}, r)
	if len(pts) != 4 || pts[0][0] != SparkPad || pts[3][0] != 100-SparkPad {
		t.Fatalf("x spans the box: %v", pts)
	}
	if !(pts[3][1] < pts[1][1] && pts[1][1] < pts[2][1] && pts[2][1] < pts[0][1]) {
		t.Fatalf("larger values sit higher: %v", pts)
	}
	flat := SparkPoints([]int{4, 4, 4}, r)
	if flat[0][1] != 20 || SparkPoints(nil, r) != nil {
		t.Fatalf("flat series sits mid-height: %v", flat)
	}
	one := SparkPoints([]int{7}, r)
	if len(one) != 1 || one[0][0] != 50 {
		t.Fatalf("single point is centred: %v", one)
	}
	img := ebiten.NewImage(200, 100)
	DrawSparkline(img, r, []int{1, 5, 3, 9}, Accent)
	DrawSparkline(img, r, nil, Accent)
	DrawProgress(img, gfx.Rect{X: 0, Y: 50, W: 160, H: 40}, "World", 0.4, "Rivers")
	DrawProgress(img, gfx.Rect{X: 0, Y: 50, W: 160, H: 40}, "World", 1.7, "")
}
