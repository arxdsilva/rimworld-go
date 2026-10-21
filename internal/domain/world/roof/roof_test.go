package roof

import "testing"

func TestSupportFieldChebyshevSix(t *testing.T) {
	const w, h = 30, 20
	sx, sy := 10, 8
	f := SupportField(w, h, func(x, y int) bool { return x == sx && y == sy })
	if len(f) != w*h {
		t.Fatalf("len %d, want %d", len(f), w*h)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := max(abs(x-sx), abs(y-sy))
			if want := d <= SupportRange; f[y*w+x] != want {
				t.Fatalf("(%d,%d) chebyshev %d: supported=%v, want %v", x, y, d, f[y*w+x], want)
			}
		}
	}
	// the diagonal corner of the range is supported, one past it is not
	if !f[(sy+6)*w+sx+6] || f[(sy+7)*w+sx+6] || f[(sy+6)*w+sx+7] {
		t.Fatal("range must be exactly 6 Chebyshev tiles")
	}
}

func TestSupportFieldMultiSourceAndEdges(t *testing.T) {
	const w, h = 40, 3
	// supports at both ends: the middle gap is 40-2*7 = 26 unsupported tiles
	f := SupportField(w, h, func(x, y int) bool { return y == 1 && (x == 0 || x == w-1) })
	n := 0
	for _, s := range f {
		if s {
			n++
		}
	}
	if want := 2 * 7 * h; n != want {
		t.Fatalf("supported tiles %d, want %d", n, want)
	}
	if f[1*w+20] {
		t.Fatal("the middle is beyond reach of both supports")
	}
	if none := SupportField(w, h, func(int, int) bool { return false }); len(none) != w*h || none[0] {
		t.Fatal("no supports: nothing supported")
	}
	if empty := SupportField(0, 0, func(int, int) bool { return true }); len(empty) != 0 {
		t.Fatal("empty grid")
	}
}

func TestCellFlags(t *testing.T) {
	var c Cell
	if c.Roofed() || c.Has(Built) {
		t.Fatal("zero cell is open")
	}
	c |= Natural | Supported
	if !c.Roofed() || !c.Has(Natural|Supported) || c.Has(Natural|Built) {
		t.Fatalf("flags %b", c)
	}
	if (Built|Supported).Roofed() != true || PlanBuild.Roofed() {
		t.Fatal("built roofs are roofs, plans are not")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
