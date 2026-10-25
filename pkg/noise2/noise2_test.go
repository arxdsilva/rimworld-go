package noise2

import (
	"math"
	"testing"
)

func TestDeterministicAndOrderFree(t *testing.T) {
	a := FBm(7, 12.5, 40.25, 4, 30, 0.5)
	_ = FBm(7, 900, 3, 4, 30, 0.5)
	if b := FBm(7, 12.5, 40.25, 4, 30, 0.5); a != b {
		t.Fatalf("FBm changed between calls: %v %v", a, b)
	}
	if FBm(8, 12.5, 40.25, 4, 30, 0.5) == a {
		t.Fatal("different seeds gave the same value")
	}
}

func TestRangeAndMean(t *testing.T) {
	var sum float64
	n := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			fx, fy := float64(x)*0.73, float64(y)*0.61
			for _, v := range []float64{
				Gradient(3, fx, fy, 9), FBm(3, fx, fy, 5, 40, 0.5), Ridged(3, fx, fy, 3, 25, 0.5),
			} {
				if v < 0 || v > 1 || math.IsNaN(v) {
					t.Fatalf("value %v out of [0,1] at %d,%d", v, x, y)
				}
			}
			sum += FBm(3, fx, fy, 5, 40, 0.5)
			n++
		}
	}
	if m := sum / float64(n); m < 0.4 || m > 0.6 {
		t.Fatalf("FBm mean %.3f, want about 0.5", m)
	}
}

func TestSmoothAtCellScale(t *testing.T) {
	// neighbouring samples one unit apart in a 30-unit cell differ little
	worst := 0.0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			d := math.Abs(FBm(11, float64(x), float64(y), 1, 30, 0.5) - FBm(11, float64(x+1), float64(y), 1, 30, 0.5))
			worst = max(worst, d)
		}
	}
	if worst > 0.08 {
		t.Fatalf("largest step %.3f between neighbours: not smooth", worst)
	}
}

func TestCorrelationFallsWithDistance(t *testing.T) {
	corr := func(off float64) float64 {
		var sab, sa, sb, saa, sbb float64
		n := 0.0
		for y := 0; y < 120; y++ {
			for x := 0; x < 120; x++ {
				a := Gradient(5, float64(x)*3, float64(y)*3, 20)
				b := Gradient(5, float64(x)*3+off, float64(y)*3, 20)
				sab += a * b
				sa += a
				sb += b
				saa += a * a
				sbb += b * b
				n++
			}
		}
		cov := sab/n - sa/n*sb/n
		return cov / math.Sqrt((saa/n-sa*sa/n/n)*(sbb/n-sb*sb/n/n))
	}
	near, far := corr(2), corr(60)
	if near < 0.8 || math.Abs(far) > 0.3 {
		t.Fatalf("correlation near %.2f far %.2f: want high then low", near, far)
	}
}

func TestWarpBoundedAndSmooth(t *testing.T) {
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			wx, wy := Warp(9, float64(x), float64(y), 10, 40)
			if math.Abs(wx-float64(x)) > 10 || math.Abs(wy-float64(y)) > 10 {
				t.Fatalf("warp moved %d,%d to %.1f,%.1f: beyond amp", x, y, wx, wy)
			}
			nx, _ := Warp(9, float64(x+1), float64(y), 10, 40)
			if math.Abs(nx-wx) > 2 {
				t.Fatalf("warp tore at %d,%d", x, y)
			}
		}
	}
}

func TestSeedSeparatesNames(t *testing.T) {
	if Seed(1, "elevation") == Seed(1, "fertility") || Seed(1, "a") == Seed(2, "a") {
		t.Fatal("Seed collides")
	}
	if Seed(1, "x") != Seed(1, "x") {
		t.Fatal("Seed not deterministic")
	}
}

func TestDegenerateArgs(t *testing.T) {
	if v := FBm(1, 3, 4, 0, 10, 0.5); v != 0.5 {
		t.Fatalf("zero octaves = %v, want 0.5", v)
	}
	if v := Gradient(1, 3.5, 4.5, 0); v < 0 || v > 1 {
		t.Fatalf("cell 0: %v", v)
	}
	if v := FBm(1, 3.5, 4.5, 2, -1, 0.5); v < 0 || v > 1 {
		t.Fatalf("cell -1: %v", v)
	}
}

func BenchmarkFBm4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FBm(1, float64(i%500), float64(i/500%500), 4, 40, 0.5)
	}
}
