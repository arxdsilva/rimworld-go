package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// TestJourneyCustomCursor (OPT-028): the option hides the OS cursor and
// draws the pointer sprite last, at the mouse; off, the OS cursor returns
// and nothing is drawn.
func TestJourneyCustomCursor(t *testing.T) {
	j := newJourney(t)
	j.a.art = testArt(t)
	visible := true
	j.a.cfg.Cursor = func(v bool) { visible = v }
	j.f.Cur.Mx, j.f.Cur.My = 300, 200
	j.update(1)

	var last *gfx.TraceEvent
	var n int
	gfx.SetDrawTrace(func(e gfx.TraceEvent) {
		if e.Kind == "image" {
			e := e
			last = &e
			n++
		}
	})
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })

	j.a.settings.Camera.CustomCursor = false
	j.a.applySettings()
	j.draw(1)
	if !visible {
		t.Fatal("OS cursor hidden with the option off")
	}
	offDraws := n

	j.a.settings.Camera.CustomCursor = true
	j.a.applySettings()
	n = 0
	j.draw(1)
	if visible {
		t.Fatal("OS cursor still visible with the option on")
	}
	if n != offDraws+1 { // the pointer, and nothing else new
		t.Fatalf("images %d, want %d (off) + 1", n, offDraws)
	}
	s := float32(j.a.scale.S)
	wantW := 32 * s
	if last.W < wantW-1 || last.W > wantW+1 {
		t.Fatalf("pointer %v px wide, want ~%v", last.W, wantW)
	}
	if want := float32(300) - 3.5*s; last.X < want-1 || last.X > want+1 {
		t.Fatalf("pointer x = %v, want ~%v (hotspot on the mouse)", last.X, want)
	}

	j.a.settings.Camera.CustomCursor = false
	j.a.applySettings()
	n = 0
	j.draw(1)
	if !visible || n != offDraws {
		t.Fatalf("option off: visible %v, images %d", visible, n)
	}
}
