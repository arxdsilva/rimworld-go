package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"
)

func TestFontMeasureAndWrap(t *testing.T) {
	w, h := gfx.Measure("Broken Hull", gfx.Body)
	if w <= 0 || h <= 0 || gfx.LineH(gfx.Body) <= 0 {
		t.Fatalf("measure %v x %v, line %v", w, h, gfx.LineH(gfx.Body))
	}
	if wb, _ := gfx.Measure("Broken Hull", gfx.Title); wb <= w {
		t.Fatal("a title is wider than body text")
	}
	lines := gfx.WrapPx("three crew members survive a hull breach and ride the lifeboats down", gfx.Body, 120)
	if len(lines) < 3 {
		t.Fatalf("wrap = %q", lines)
	}
	for _, l := range lines {
		if lw, _ := gfx.Measure(l, gfx.Body); lw > 120 {
			t.Fatalf("line %q is %v wide", l, lw)
		}
	}
	if long := gfx.WrapPx("Supercalifragilisticexpialidocious", gfx.Body, 40); len(long) < 2 {
		t.Fatalf("a long word breaks: %q", long)
	}
	if e := gfx.Ellipsize("Gilded Expedition", gfx.Body, 50); e == "Gilded Expedition" || len(e) == 0 {
		t.Fatalf("ellipsize = %q", e)
	}
}
