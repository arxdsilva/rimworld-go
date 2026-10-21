// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestOklabLightnessMatchesReference(t *testing.T) {
	for _, c := range []struct {
		name    string
		r, g, b uint8
		want    float32
	}{
		{"black", 0, 0, 0, 0}, {"white", 255, 255, 255, 1},
		{"mid grey", 128, 128, 128, 0.5999}, {"red", 255, 0, 0, 0.6279},
	} {
		if got := lightness(c.r, c.g, c.b); math.Abs(float64(got-c.want)) > 1e-4 {
			t.Errorf("%s: L = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEDTMatchesBruteForce(t *testing.T) {
	state := uint32(75)
	for height := 1; height < 13; height++ {
		for width := 1; width < 17; width++ {
			mask := make([]bool, width*height)
			for i := range mask {
				state = state*1664525 + 1013904223
				mask[i] = state%4 != 0
			}
			got := edt(mask, width, height)
			for index, actual := range got.data {
				expected := math.Inf(1)
				for other, inside := range mask {
					if inside {
						continue
					}
					dx := float64(index%width - other%width)
					dy := float64(index/width - other/width)
					expected = math.Min(expected, math.Hypot(dx, dy))
				}
				if math.IsInf(expected, 1) {
					if !math.IsInf(float64(actual), 1) {
						t.Fatalf("%dx%d[%d] = %v, want +Inf", width, height, index, actual)
					}
				} else if math.Abs(float64(actual)-expected) >= 1e-5 {
					t.Fatalf("%dx%d[%d] = %v, want %v", width, height, index, actual, expected)
				}
			}
		}
	}
}

func TestGaussianNormalisedAndZeroExtended(t *testing.T) {
	in := newField(21, 21)
	in.data[10*21+10] = 1
	g := gaussian(in, 1.5)
	var sum float64
	for _, v := range g.data {
		sum += float64(v)
	}
	if math.Abs(sum-1) > 1e-5 {
		t.Errorf("interior impulse sums to %v, want 1", sum)
	}
	if g.data[10*21+10] <= g.data[10*21+11] || g.data[10*21+11] != g.data[10*21+9] {
		t.Error("impulse response is not a symmetric peak")
	}
	edge := newField(5, 5)
	edge.data[0] = 1
	var lost float64
	for _, v := range gaussian(edge, 1.5).data {
		lost += float64(v)
	}
	if lost >= 0.5 {
		t.Errorf("corner impulse keeps %v of its mass; zero extension should drop most", lost)
	}
	small := newField(3, 3)
	small.data[4] = 2
	if got := gaussian(small, 0.01); got.data[4] != 2 || &got.data[0] == &small.data[0] {
		t.Error("sigma < 0.05 must be an independent identity copy")
	}
}

func TestMaskedGaussianIgnoresHiddenRGB(t *testing.T) {
	alpha, a, b := newField(9, 9), newField(9, 9), newField(9, 9)
	for y := 2; y < 7; y++ {
		for x := 2; x < 5; x++ {
			alpha.data[y*9+x] = 1
			a.data[y*9+x], b.data[y*9+x] = 0.4, 0.4
		}
	}
	for i := range alpha.data { // hidden pixels differ wildly
		if alpha.data[i] == 0 {
			a.data[i], b.data[i] = 9, -7
		}
	}
	ga, gb := maskedGaussian(a, alpha, 1), maskedGaussian(b, alpha, 1)
	for i := range ga.data {
		if ga.data[i] != gb.data[i] {
			t.Fatalf("hidden RGB leaked at %d: %v vs %v", i, ga.data[i], gb.data[i])
		}
	}
	if v := ga.data[4*9+3]; math.Abs(float64(v)-0.4) > 1e-5 {
		t.Errorf("flat island interior = %v, want 0.4", v)
	}
}

func TestSobelCorrelationSigns(t *testing.T) {
	f := newField(3, 3)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			f.data[y*3+x] = float32(x) // rises to the right
		}
	}
	if dx, dy := sobel(f, 4); dx != 1 || dy != 0 {
		t.Errorf("x ramp: (%v, %v), want (1, 0)", dx, dy)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			f.data[y*3+x] = float32(y) * 2 // rises downward
		}
	}
	if dx, dy := sobel(f, 4); dx != 0 || dy != 2 {
		t.Errorf("y ramp: (%v, %v), want (0, 2)", dx, dy)
	}
}

func square(w, h, x0, y0, x1, y1 int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestNeutralForTransparent(t *testing.T) {
	src := square(8, 8, 2, 2, 6, 6, color.NRGBA{200, 50, 50, 255})
	src.SetNRGBA(0, 0, color.NRGBA{9, 9, 9, 2}) // below threshold: neutral, alpha kept
	out, err := Generate(src, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if got := out.NRGBAAt(7, 7); got != (color.NRGBA{128, 128, 255, 0}) {
		t.Errorf("transparent = %v", got)
	}
	if got := out.NRGBAAt(0, 0); got != (color.NRGBA{128, 128, 255, 2}) {
		t.Errorf("faint = %v", got)
	}
	if out.Bounds() != src.Bounds() {
		t.Error("size changed")
	}
}

func TestFlatOpaqueSquareDomesOutward(t *testing.T) {
	out, err := Generate(square(16, 16, 3, 3, 13, 13, color.NRGBA{120, 120, 120, 255}), DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if p := out.NRGBAAt(3, 8); p.R >= 128 {
		t.Errorf("left edge R = %d, want < 128", p.R)
	}
	if p := out.NRGBAAt(12, 8); p.R <= 128 {
		t.Errorf("right edge R = %d, want > 128", p.R)
	}
	if p := out.NRGBAAt(8, 3); p.G <= 128 {
		t.Errorf("top edge G = %d, want > 128 (OpenGL, y up)", p.G)
	}
	if p := out.NRGBAAt(8, 12); p.G >= 128 {
		t.Errorf("bottom edge G = %d, want < 128", p.G)
	}
	if p := out.NRGBAAt(7, 7); p.B < 200 {
		t.Errorf("centre B = %d, want near 255", p.B)
	}
}

func TestDirectXFlipsGreen(t *testing.T) {
	src := square(16, 16, 3, 3, 13, 13, color.NRGBA{120, 120, 120, 255})
	gl, _ := Generate(src, DefaultSettings())
	s := DefaultSettings()
	s.DirectX = true
	dx, _ := Generate(src, s)
	if p := dx.NRGBAAt(8, 3); p.G >= 128 {
		t.Errorf("DirectX top edge G = %d, want < 128", p.G)
	}
	for i := 0; i < len(gl.Pix); i += 4 {
		if gl.Pix[i] != dx.Pix[i] || gl.Pix[i+2] != dx.Pix[i+2] {
			t.Fatal("DirectX changed R or B")
		}
		if a, b := int(gl.Pix[i+1]), int(dx.Pix[i+1]); a+b < 254 || a+b > 256 {
			t.Fatalf("green not mirrored at %d: %d + %d", i/4, a, b)
		}
	}
}

func TestInvertXY(t *testing.T) {
	src := square(16, 16, 3, 3, 13, 13, color.NRGBA{120, 120, 120, 255})
	s := DefaultSettings()
	s.InvertX = true
	ix, _ := Generate(src, s)
	if p := ix.NRGBAAt(3, 8); p.R <= 128 {
		t.Errorf("InvertX left edge R = %d, want > 128", p.R)
	}
	if p := ix.NRGBAAt(8, 3); p.G <= 128 {
		t.Errorf("InvertX must not touch green, got %d", p.G)
	}
	s = DefaultSettings()
	s.InvertY = true
	iy, _ := Generate(src, s)
	if p := iy.NRGBAAt(8, 3); p.G >= 128 {
		t.Errorf("InvertY top edge G = %d, want < 128", p.G)
	}
}

func TestIslandsIndependent(t *testing.T) {
	a := color.NRGBA{200, 40, 40, 255}
	b := color.NRGBA{30, 60, 220, 255}
	both := image.NewNRGBA(image.Rect(0, 0, 30, 14))
	onlyA := image.NewNRGBA(both.Rect)
	onlyB := image.NewNRGBA(both.Rect)
	for y := 2; y < 12; y++ {
		for x := 2; x < 9; x++ {
			both.SetNRGBA(x, y, a)
			onlyA.SetNRGBA(x, y, a)
		}
		for x := 11; x < 25; x++ { // separated by a 2px gap
			if (x+y)%5 != 0 {
				both.SetNRGBA(x, y, b)
				onlyB.SetNRGBA(x, y, b)
			}
		}
	}
	s := DefaultSettings()
	got, _ := Generate(both, s)
	ga, _ := Generate(onlyA, s)
	gb, _ := Generate(onlyB, s)
	for y := 0; y < 14; y++ {
		for x := 0; x < 30; x++ {
			want := ga.NRGBAAt(x, y)
			if x >= 10 {
				want = gb.NRGBAAt(x, y)
			}
			if got.NRGBAAt(x, y) != want {
				t.Fatalf("(%d,%d) = %v, want %v", x, y, got.NRGBAAt(x, y), want)
			}
		}
	}
}

func TestDiagonalPixelsShareAnIsland(t *testing.T) {
	mask := []bool{true, false, false, false, true, false, false, false, true}
	if got := components(mask, 3, 3); len(got) != 1 || len(got[0].pixels) != 3 {
		t.Errorf("diagonal stroke split into %d islands", len(got))
	}
	mask[4] = false
	if got := components(mask, 3, 3); len(got) != 2 {
		t.Errorf("want 2 islands, got %d", len(got))
	}
}

func testSprite() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 14, 12))
	for y := 1; y < 11; y++ {
		for x := 1; x < 13; x++ {
			if (x-6)*(x-6)+(y-5)*(y-5) < 30 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(40 + 15*x), uint8(200 - 12*y), uint8(90 + 7*x*y%90), uint8(255 - 20*(x%3))})
			}
		}
	}
	return img
}

func TestDeterministicBytes(t *testing.T) {
	a, _ := Generate(testSprite(), DefaultSettings())
	b, _ := Generate(testSprite(), DefaultSettings())
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("two runs differ")
	}
}

func TestValidateRanges(t *testing.T) {
	if err := DefaultSettings().Validate(); err != nil {
		t.Fatal(err)
	}
	nan := float32(math.NaN())
	for name, mut := range map[string]func(*Settings){
		"alpha-threshold":  func(s *Settings) { s.AlphaThreshold = 1 },
		"shape-radius":     func(s *Settings) { s.ShapeRadius = 0.4 },
		"shape-strength":   func(s *Settings) { s.ShapeStrength = 8.5 },
		"shape-softness":   func(s *Settings) { s.ShapeSoftness = -1 },
		"detail-strength":  func(s *Settings) { s.DetailStrength = nan },
		"detail-scale":     func(s *Settings) { s.DetailScale = 0.5 },
		"normal-strength":  func(s *Settings) { s.NormalStrength = float32(math.Inf(1)) },
		"shape-radius ":    func(s *Settings) { s.ShapeRadius = 513 },
		"alpha-threshold ": func(s *Settings) { s.AlphaThreshold = -0.1 },
	} {
		s := DefaultSettings()
		mut(&s)
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), strings.TrimSpace(name)+" must be finite and between") {
			t.Errorf("%s: err = %v", name, err)
		}
		if _, gerr := Generate(image.NewNRGBA(image.Rect(0, 0, 2, 2)), s); gerr == nil {
			t.Errorf("%s: Generate accepted invalid settings", name)
		}
	}
	// Inclusive bounds are valid.
	s := DefaultSettings()
	s.AlphaThreshold, s.DetailScale, s.ShapeRadius = 0.99, 0.6, 512
	if err := s.Validate(); err != nil {
		t.Errorf("bounds rejected: %v", err)
	}
}

func TestGenerateRejectsBadSizes(t *testing.T) {
	if _, err := Generate(image.NewNRGBA(image.Rect(0, 0, 0, 5)), DefaultSettings()); err == nil {
		t.Error("empty image accepted")
	}
	if _, err := Generate(image.NewNRGBA(image.Rect(0, 0, 3000, 3000)), DefaultSettings()); err == nil {
		t.Error("oversized image accepted")
	}
}

func TestGenerateHonoursNonZeroBounds(t *testing.T) {
	base := testSprite()
	shifted := &image.NRGBA{Pix: base.Pix, Stride: base.Stride, Rect: base.Rect.Add(image.Pt(5, 7))}
	a, _ := Generate(base, DefaultSettings())
	b, err := Generate(shifted, DefaultSettings())
	if err != nil || b.Rect != shifted.Rect || !bytes.Equal(a.Pix, b.Pix) {
		t.Fatalf("shifted bounds differ: %v", err)
	}
}

func TestSymmetricSpriteHasNeutralCentre(t *testing.T) {
	// A single opaque pixel: no slope, so (128, 128, 255) exactly.
	one := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	one.SetNRGBA(0, 0, color.NRGBA{10, 200, 30, 255})
	out, _ := Generate(one, DefaultSettings())
	if got := out.NRGBAAt(0, 0); got != (color.NRGBA{128, 128, 255, 255}) {
		t.Errorf("1x1 = %v", got)
	}
}

// reference is an independent, naive float64 implementation of the same
// pipeline (brute-force distance, direct 2-D kernel sums), used to check
// the optimised float32 code to within one byte.
func reference(src *image.NRGBA, s Settings) *image.NRGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	pad := s.padding()
	W, H := w+2*pad, h+2*pad
	at := func(f []float64, x, y int) float64 {
		if x < 0 || y < 0 || x >= W || y >= H {
			return 0
		}
		return f[y*W+x]
	}
	gauss := func(f []float64, sigma float64) []float64 {
		if sigma < 0.05 {
			return append([]float64(nil), f...)
		}
		r := int(math.Ceil(3 * sigma))
		k := make([]float64, 2*r+1)
		var tot float64
		for i := range k {
			k[i] = math.Exp(-0.5 * math.Pow(float64(i-r)/sigma, 2))
			tot += k[i]
		}
		out := make([]float64, W*H)
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				var acc float64
				for j := -r; j <= r; j++ {
					for i := -r; i <= r; i++ {
						acc += at(f, x+i, y+j) * k[i+r] / tot * k[j+r] / tot
					}
				}
				out[y*W+x] = acc
			}
		}
		return out
	}
	out := neutralMap(src)
	// Islands: flood fill, brute force.
	mask := make([]bool, w*h)
	for i := range mask {
		mask[i] = float32(src.Pix[i*4+3])/255 > s.AlphaThreshold
	}
	for _, part := range components(mask, w, h) {
		inI := make([]bool, W*H)
		alpha, L := make([]float64, W*H), make([]float64, W*H)
		lo, hi := 1.0, 0.0
		for _, idx := range part.pixels {
			x, y := idx%w-part.minX+pad, idx/w-part.minY+pad
			p := src.Pix[idx*4:]
			i := y*W + x
			inI[i] = true
			alpha[i] = float64(p[3]) / 255
			L[i] = float64(lightness(p[0], p[1], p[2]))
			lo, hi = math.Min(lo, L[i]), math.Max(hi, L[i])
		}
		// Padding as the port: island image is its bbox plus pad.
		_ = W
		for i := range L {
			if inI[i] {
				L[i] -= (lo + hi) / 2
			}
		}
		shape := make([]float64, W*H)
		for i := range shape {
			d := math.Inf(1)
			for j := range inI {
				if !inI[j] {
					d = math.Min(d, math.Hypot(float64(i%W-j%W), float64(i/W-j/W)))
				}
			}
			R := float64(s.ShapeRadius)
			shape[i] = float64(s.ShapeStrength) * R * math.Log1p(math.Max(d-0.5, 0)/R)
		}
		hgt := gauss(shape, float64(s.ShapeSoftness))
		masked := func(sigma float64) []float64 {
			num := make([]float64, W*H)
			for i := range num {
				num[i] = L[i] * alpha[i]
			}
			n, d := gauss(num, sigma), gauss(alpha, sigma)
			for i := range n {
				n[i] /= math.Max(d[i], 1e-8)
			}
			return n
		}
		f1, f2 := masked(float64(s.DetailScale)), masked(2*float64(s.DetailScale))
		for i := range hgt {
			hgt[i] += float64(s.DetailStrength) * (0.7*f1[i] + 0.3*f2[i])
		}
		for _, idx := range part.pixels {
			x, y := idx%w-part.minX+pad, idx/w-part.minY+pad
			g := func(dx, dy int) float64 { return hgt[(y+dy)*W+x+dx] }
			dx := (g(1, -1) + 2*g(1, 0) + g(1, 1) - g(-1, -1) - 2*g(-1, 0) - g(-1, 1)) * 0.125
			dy := (g(-1, 1) + 2*g(0, 1) + g(1, 1) - g(-1, -1) - 2*g(0, -1) - g(1, -1)) * 0.125
			nx, ny := -dx*float64(s.NormalStrength), dy*float64(s.NormalStrength)
			l := math.Sqrt(nx*nx + ny*ny + 1)
			o := out.Pix[idx*4:]
			for c, v := range []float64{nx / l, ny / l, 1 / l} {
				o[c] = uint8(math.Round((v*0.5 + 0.5) * 255))
			}
		}
	}
	return out
}

func TestGenerateMatchesNaiveReference(t *testing.T) {
	s := DefaultSettings()
	s.ShapeRadius, s.ShapeSoftness, s.DetailScale = 2, 0.8, 0.7 // keep the naive O(n^2) cheap
	src := testSprite()
	got, err := Generate(src, s)
	if err != nil {
		t.Fatal(err)
	}
	want := reference(src, s)
	for i := range got.Pix {
		if d := int(got.Pix[i]) - int(want.Pix[i]); d < -1 || d > 1 {
			t.Fatalf("byte %d (px %d ch %d): got %d, naive reference %d", i, i/4, i%4, got.Pix[i], want.Pix[i])
		}
	}
}

// TestGenerateMatchesGolden compares against testdata/sprite_n.png. No
// Rust toolchain was available, so the golden is a self-consistency
// snapshot (regenerate with -update) guarded by the naive-reference and
// hand-checked-pixel tests above.
func TestGenerateMatchesGolden(t *testing.T) {
	got, err := Generate(testSprite(), DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	const path = "testdata/sprite_n.png"
	if *update {
		var buf bytes.Buffer
		if err := png.Encode(&buf, got); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewNRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			want.Set(x, y, img.At(x, y))
		}
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("output differs from golden; inspect, then run with -update if intended")
	}
}
