package pixeldiff

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetRGBA(x, y, c)
		}
	}
	return m
}

func TestIdenticalIsExact(t *testing.T) {
	a := solid(8, 8, color.RGBA{10, 20, 30, 255})
	r := Compare(a, solid(8, 8, color.RGBA{10, 20, 30, 255}), nil)
	if ok, why := r.Pass(Exact); !ok || r.Differing != 0 || !r.Bounds.Empty() {
		t.Fatalf("identical: %v %s %+v", ok, why, r)
	}
}

func TestOnePixelBoundsAndMax(t *testing.T) {
	a := solid(8, 8, color.RGBA{0, 0, 0, 255})
	b := solid(8, 8, color.RGBA{0, 0, 0, 255})
	b.SetRGBA(3, 5, color.RGBA{0, 7, 0, 255})
	r := Compare(a, b, nil)
	if r.Differing != 1 || r.Max != 7 || r.MaxChannel[1] != 7 || r.Bounds != image.Rect(3, 5, 4, 6) {
		t.Fatalf("%+v", r)
	}
	if ok, _ := r.Pass(Exact); ok {
		t.Fatal("exact must fail")
	}
	if ok, _ := r.Pass(AA); ok {
		t.Fatal("a non-edge delta of 7 must fail AA")
	}
}

func TestAAToleratesEdgeNoise(t *testing.T) {
	a := solid(8, 8, color.RGBA{0, 0, 0, 255})
	for y := 0; y < 8; y++ {
		for x := 4; x < 8; x++ {
			a.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	b := solid(8, 8, color.RGBA{0, 0, 0, 255})
	copy(b.Pix, a.Pix)
	b.SetRGBA(3, 2, color.RGBA{20, 20, 20, 255}) // next to the step
	edge := EdgeMask(a, 48)
	if !edge(3, 2) || edge(0, 0) {
		t.Fatal("edge mask wrong")
	}
	r := Compare(a, b, edge)
	if ok, why := r.Pass(AA); !ok {
		t.Fatal(why)
	}
	b.SetRGBA(0, 0, color.RGBA{9, 9, 9, 255})
	if ok, _ := Compare(a, b, edge).Pass(AA); ok {
		t.Fatal("off-edge delta 9 must fail")
	}
	b.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	b.SetRGBA(3, 2, color.RGBA{80, 80, 80, 255})
	if ok, _ := Compare(a, b, edge).Pass(AA); ok {
		t.Fatal("edge delta 80 must fail")
	}
}

func TestSizeMismatch(t *testing.T) {
	r := Compare(solid(2, 2, color.RGBA{}), solid(3, 2, color.RGBA{}), nil)
	if ok, _ := r.Pass(AA); ok {
		t.Fatal("size mismatch must fail")
	}
}

func TestHeatAndSideBySide(t *testing.T) {
	a, b := solid(4, 4, color.RGBA{}), solid(4, 4, color.RGBA{})
	b.SetRGBA(1, 1, color.RGBA{1, 0, 0, 255})
	h := HeatMap(a, b)
	if h.RGBAAt(1, 1).R < 64 || h.RGBAAt(0, 0).R != 0 {
		t.Fatalf("heat %v %v", h.RGBAAt(1, 1), h.RGBAAt(0, 0))
	}
	s := SideBySide(a, b)
	if s.Bounds().Dx() != 10 || s.RGBAAt(7, 1).R != 1 {
		t.Fatalf("side by side %v", s.Bounds())
	}
}

func TestRunDeterminismAndPolicy(t *testing.T) {
	dir := t.TempDir()
	base := solid(4, 4, color.RGBA{1, 2, 3, 255})
	same := func(int) *image.RGBA { return base }
	if res, err := Run(dir, Exact, same); err != nil || !res.Deterministic {
		t.Fatal(err)
	}
	for _, f := range []string{"A.png", "B.png", "heat.png", "report.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	flaky := func(int) *image.RGBA { n++; m := solid(4, 4, color.RGBA{}); m.Pix[0] = uint8(n); return m }
	if _, err := Run(dir, Exact, flaky); err == nil {
		t.Fatal("non-deterministic scene must fail")
	}
	changed := func(v int) *image.RGBA {
		m := solid(4, 4, color.RGBA{})
		if v == 1 {
			m.Pix[0] = 200
		}
		return m
	}
	if _, err := Run(dir, Exact, changed); err == nil {
		t.Fatal("changed scene must fail exact")
	}
}
