package gfx

import "testing"

func TestLargerSmallText(t *testing.T) {
	SetLargerSmall(false)
	t.Cleanup(func() { SetLargerSmall(false) })
	w0, h0 := Measure("Hello world", Small)
	l0 := LineH(Small)
	body0, _ := Measure("Hello world", Body)
	SetLargerSmall(true)
	w1, h1 := Measure("Hello world", Small)
	if !(w1 > w0 && h1 > h0 && LineH(Small) > l0) {
		t.Fatalf("larger small text measures bigger: %v,%v -> %v,%v", w0, h0, w1, h1)
	}
	if body, _ := Measure("Hello world", Body); body != body0 {
		t.Fatal("other sizes do not change")
	}
	// small text at the larger size measures as body text (14 px).
	if wb, _ := Measure("Hello world", Body); w1 != wb {
		t.Fatalf("small at the larger size is %v wide, body %v", w1, wb)
	}
	SetLargerSmall(false)
	if w, _ := Measure("Hello world", Small); w != w0 {
		t.Fatal("off restores 11 px")
	}
}
