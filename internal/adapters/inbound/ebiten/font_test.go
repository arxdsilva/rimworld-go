package ebiten

import "testing"

func TestFontMeasureAndWrap(t *testing.T) {
	w, h := measure("Broken Hull", tsBody)
	if w <= 0 || h <= 0 || lineH(tsBody) <= 0 {
		t.Fatalf("measure %v x %v, line %v", w, h, lineH(tsBody))
	}
	if wb, _ := measure("Broken Hull", tsTitle); wb <= w {
		t.Fatal("a title is wider than body text")
	}
	lines := wrapPx("three crew members survive a hull breach and ride the lifeboats down", tsBody, 120)
	if len(lines) < 3 {
		t.Fatalf("wrap = %q", lines)
	}
	for _, l := range lines {
		if lw, _ := measure(l, tsBody); lw > 120 {
			t.Fatalf("line %q is %v wide", l, lw)
		}
	}
	if long := wrapPx("Supercalifragilisticexpialidocious", tsBody, 40); len(long) < 2 {
		t.Fatalf("a long word breaks: %q", long)
	}
	if e := ellipsize("Gilded Expedition", tsBody, 50); e == "Gilded Expedition" || len(e) == 0 {
		t.Fatalf("ellipsize = %q", e)
	}
}
