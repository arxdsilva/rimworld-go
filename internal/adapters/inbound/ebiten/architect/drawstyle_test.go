package architect

import "testing"

func TestShapeCellsPerStyle(t *testing.T) {
	t.Parallel()
	a, b := [2]int{0, 0}, [2]int{4, 4}
	for _, c := range []struct {
		style DrawStyle
		want  int
	}{
		{StyleDefault, 1},   // single: the end cell
		{StyleLine, 5},      // along the dominant axis
		{StyleRect, 25},     // filled 5x5
		{StyleRectEdge, 16}, // its outline
		{StyleOval, 21},     // a disc without its corners
	} {
		if got := len(ShapeCells(c.style, a, b, false)); got != c.want {
			t.Errorf("style %s: %d cells, want %d", StyleNames[c.style], got, c.want)
		}
	}
	if got := len(ShapeCells(StyleRect, a, [2]int{4, 1}, true)); got != 25 {
		t.Errorf("Ctrl squares the box: %d cells", got)
	}
	if edge, full := len(ShapeCells(StyleOvalEdge, a, b, false)), len(ShapeCells(StyleOval, a, b, false)); edge >= full || edge == 0 {
		t.Errorf("empty oval %d of %d", edge, full)
	}
}

func TestRowRunsSplitAnOval(t *testing.T) {
	t.Parallel()
	runs := RowRuns(ShapeCells(StyleOval, [2]int{0, 0}, [2]int{4, 4}, false))
	if len(runs) != 5 || runs[0] != (RowRun{Y: 0, X0: 1, X1: 3}) || runs[2] != (RowRun{Y: 2, X0: 0, X1: 4}) {
		t.Fatalf("runs %+v", runs)
	}
	if got := RowRuns([][2]int{{3, 1}, {0, 1}, {1, 1}}); len(got) != 2 || got[0] != (RowRun{Y: 1, X0: 0, X1: 1}) {
		t.Fatalf("a gap splits a row: %+v", got)
	}
}

func TestStylesCycleAndRotate(t *testing.T) {
	t.Parallel()
	tl, _ := newFake(nil)
	wall, _ := ToolByID("wall")
	if tl.CurStyle(wall) != StyleLine {
		t.Fatalf("wall default %v", tl.CurStyle(wall))
	}
	tl.ToolID = "wall"
	tl.rotateOrStyle(1)
	if tl.Style != StyleRect || tl.CurStyle(wall) != StyleRect {
		t.Fatalf("E steps the style: %v", tl.Style)
	}
	tl.rotateOrStyle(-1)
	tl.rotateOrStyle(-1)
	if tl.Style != StyleOvalEdge {
		t.Fatalf("Q wraps to the last style: %v", tl.Style)
	}
	stock, _ := ToolByID("stockpile")
	if tl.CurStyle(stock) != StyleRect {
		t.Fatalf("a style the zone tool lacks falls back: %v", tl.CurStyle(stock))
	}
	tl.ToolID = "bed"
	tl.rotateOrStyle(1)
	if tl.Facing != 1 {
		t.Fatalf("a single placement rotates instead: facing %d", tl.Facing)
	}
	tl.ToolID = "nothing"
	tl.rotateOrStyle(1)
	if tl.Facing != 1 {
		t.Fatal("no tool: nothing turns")
	}
	if x, y := constrainEnd(2, 2, 0, 5); x != -1 || y != 5 {
		t.Fatalf("constrainEnd = %d,%d", x, y)
	}
}
