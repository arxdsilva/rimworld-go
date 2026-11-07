package uikit

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// TestTipCardSize (UIS-015): the card is at most TipMaxW wide, its text
// TipMaxW-8 wide; a title and a body are separated by one blank line, and
// the height is a Body line per line plus 14.
func TestTipCardSize(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 80)
	c := TipLayout("Title\n"+long, 1280, 720, 100, 100)
	if c.Box.W > TipMaxW {
		t.Fatalf("card %v wide, cap %d", c.Box.W, TipMaxW)
	}
	lines := len(c.Title) + 1 + len(c.Body)
	if want := float32(float64(lines)*gfx.LineH(gfx.Body)) + 14; c.Box.H != want {
		t.Fatalf("height %v for %d lines, want %v", c.Box.H, lines, want)
	}
	one := TipLayout("Only a title", 1280, 720, 100, 100)
	if want := float32(gfx.LineH(gfx.Body)) + 14; one.Box.H != want || len(one.Body) != 0 {
		t.Fatalf("a title alone: height %v, want %v", one.Box.H, want)
	}
}
