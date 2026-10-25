package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The draw trace (ui_canvas.go drawTrace, font.go traceText) is the eye of
// the visual tests (visual_invariance_test.go): it must see every kind of
// draw, in physical pixels, tagged with its pass. These tests change
// package globals and must not run in parallel.

func collectTrace(t *testing.T) *[]traceEvent {
	t.Helper()
	var got []traceEvent
	drawTrace = func(e traceEvent) { got = append(got, e) }
	t.Cleanup(func() { drawTrace = nil; tracePass = "hud"; traceVia = "" })
	return &got
}

func wantEvent(t *testing.T, got []traceEvent, i int, kind string, x, y, w, h float64) traceEvent {
	t.Helper()
	if i >= len(got) {
		t.Fatalf("event %d missing (%d traced)", i, len(got))
	}
	e := got[i]
	if e.Kind != kind || math.Abs(float64(e.X)-x) > 0.01 || math.Abs(float64(e.Y)-y) > 0.01 ||
		math.Abs(float64(e.W)-w) > 0.01 || math.Abs(float64(e.H)-h) > 0.01 {
		t.Errorf("event %d = %s, want %s %g,%g %gx%g", i, e, kind, x, y, w, h)
	}
	return e
}

func TestDrawTraceSeesEveryCanvasDraw(t *testing.T) {
	withUIScale(t, 2)
	got := collectTrace(t)
	dst := ebiten.NewImage(400, 300)
	img := ebiten.NewImage(10, 20)
	black := uiText

	uiFillRect(dst, 10, 20, 30, 40, black, false)
	uiStrokeRect(dst, 10, 20, 30, 40, 1, black, false)
	uiLine(dst, 1, 2, 5, 10, 1, black, false)
	uiFillCircle(dst, 10, 10, 3, black, false)
	uiStrokeCircle(dst, 10, 10, 3, 1, black, false)
	uiFillPoly(dst, [][2]float32{{1, 1}, {9, 2}, {4, 7}}, black, false)
	uiFillPoly(dst, [][2]float32{{1, 1}, {2, 2}}, black, false) // not a polygon: nothing drawn
	uiImage(dst, img, 5, 6, 7, 8, nil)
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(3, 4)
	uiDrawImage(dst, img, &op) // a 10x20 image at (3,4), x2
	vs := []ebiten.Vertex{{DstX: 1, DstY: 2}, {DstX: 9, DstY: 3}, {DstX: 4, DstY: 8}}
	uiTriangles(dst, vs, []uint16{0, 1, 2}, img, nil)
	uiTriangles32(dst, vs, []uint32{0, 1, 2}, img, nil)
	uiTriangles(dst, nil, nil, img, nil) // empty: nothing to report

	g := *got
	wantEvent(t, g, 0, "fill", 20, 40, 60, 80)
	wantEvent(t, g, 1, "stroke", 20, 40, 60, 80)
	wantEvent(t, g, 2, "line", 2, 4, 8, 16)
	wantEvent(t, g, 3, "circle", 14, 14, 12, 12)
	wantEvent(t, g, 4, "circle", 14, 14, 12, 12)
	wantEvent(t, g, 5, "poly", 2, 2, 16, 12)
	wantEvent(t, g, 6, "image", 10, 12, 14, 16)
	wantEvent(t, g, 7, "image", 6, 8, 20, 40)
	wantEvent(t, g, 8, "tris", 2, 4, 16, 12)
	wantEvent(t, g, 9, "tris", 2, 4, 16, 12)
	if len(g) != 10 {
		t.Errorf("%d events, want 10:%s", len(g), visSummary(g, 20))
	}
	for i, e := range g {
		if e.Pass != "hud" || e.Via != "" || e.Text != "" {
			t.Errorf("event %d = %s, want a plain hud draw", i, e)
		}
	}
}

func TestDrawTraceTextAndWorldHelpers(t *testing.T) {
	withUIScale(t, 2)
	got := collectTrace(t)
	dst := ebiten.NewImage(400, 300)

	drawText(dst, "Hello", 10, 20, tsBody)
	drawText(dst, "Hello", 100, 20, textStyle{Size: 13, Align: alignCenter})
	drawText(dst, "Hello", 200, 20, textStyle{Size: 13, Align: alignRight})
	drawText(dst, "", 10, 20, tsBody) // nothing drawn, nothing reported
	left, centre, right := (*got)[0], (*got)[1], (*got)[2]
	if len(*got) != 3 {
		t.Fatalf("%d text events, want 3", len(*got))
	}
	if left.Kind != "text" || left.Text != "Hello" || left.X != 20 || left.Y != 40 || left.W < 20 || left.H < 20 {
		t.Errorf("left text = %s, want \"Hello\" at 20,40 with a real box", left)
	}
	if math.Abs(float64(centre.X+centre.W/2)-200) > 0.01 || math.Abs(float64(right.X+right.W)-400) > 0.01 {
		t.Errorf("centre %s should straddle x=200 and right %s end at x=400", centre, right)
	}

	// worldText takes physical coordinates and converts them once.
	*got = nil
	tracePass = "world"
	worldText(dst, "Zone", 100, 60, tsSmall)
	if len(*got) != 1 || (*got)[0].X != 100 || (*got)[0].Y != 60 || (*got)[0].Pass != "world" || (*got)[0].Via != "worldText" {
		t.Errorf("worldText traced %s, want text at the physical 100,60 via worldText in the world pass", visSummary(*got, 5))
	}
	if traceVia != "" {
		t.Errorf("traceVia = %q after worldText returned", traceVia)
	}
	*got = nil
	img := ebiten.NewImage(10, 10)
	worldIcon(dst, img, 100, 60, 8) // 8 logical = 16 physical, at the physical 100,60
	wantEvent(t, *got, 0, "image", 100, 60, 16, 16)
	if (*got)[0].Via != "worldIcon" || (*got)[0].Pass != "world" || traceVia != "" {
		t.Errorf("worldIcon traced %s (traceVia %q)", (*got)[0], traceVia)
	}
	// the legacy callers: drawIcon / drawText with logical coordinates stay "hud"
	*got = nil
	tracePass = "hud"
	drawIcon(dst, img, 5, 5, 8)
	wantEvent(t, *got, 0, "image", 10, 10, 16, 16)
}

// canvasTrace (counted exactly by older tests) must keep seeing only
// fills, strokes and images, whatever drawTrace sees.
func TestCanvasTraceStaysTheOldStream(t *testing.T) {
	withUIScale(t, 1.5)
	old := traceCanvas(t)
	all := collectTrace(t)
	dst := ebiten.NewImage(300, 200)
	img := ebiten.NewImage(10, 10)
	drawPanel(dst, 10, 10, 50, 20)
	drawText(dst, "x", 5, 5, tsBody)
	uiLine(dst, 0, 0, 5, 5, 1, uiText, false)
	uiFillCircle(dst, 5, 5, 2, uiText, false)
	uiImage(dst, img, 1, 1, 4, 4, nil)
	if len(*old) != 3 || (*old)[0] != "fill 15,15 75x30" || (*old)[1] != "stroke 15,15 75x30" || (*old)[2] != "image 1.5,1.5 6x6" {
		t.Errorf("canvasTrace = %q, want the fill, stroke and image only", *old)
	}
	if len(*all) != 6 {
		t.Errorf("drawTrace saw %d draws, want 6 (fill, stroke, text, line, circle, image):%s", len(*all), visSummary(*all, 10))
	}
}

func TestTraceEventString(t *testing.T) {
	e := traceEvent{Kind: "text", Text: "Hi", X: 1, Y: 2, W: 3, H: 4, Pass: "world", Via: "worldText"}
	if got, want := e.String(), `text "Hi" 1.0,2.0 3.0x4.0 [world via worldText]`; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if got := visSummary([]traceEvent{e, e, e}, 2); got == "" {
		t.Error("empty summary")
	}
}
