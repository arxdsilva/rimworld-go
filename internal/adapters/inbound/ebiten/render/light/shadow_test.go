package light

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// testLight is a hand-built light: shadows fall away from a light at
// azimuth az (screen space, 0 = east) and elevation el (radians), with
// the ART §6.1 length clamp and the given alpha.
func testLight(az, el, alpha float64) State {
	lx, ly := math.Cos(az)*math.Cos(el), 0.5*math.Sin(az)*math.Cos(el)
	n := math.Hypot(lx, ly)
	return State{
		SunL:      [3]float32{float32(lx), float32(ly), float32(math.Sin(el))},
		ShadowDir: [2]float64{-lx / n, -ly / n},
		ShadowLen: func(h float64) float64 {
			return math.Min(math.Max(h/math.Tan(el), 0.25*h), ShadowMaxLen)
		},
		ShadowAlpha: alpha,
	}
}

func quadCentre(q [4][2]float64) (float64, float64) {
	return (q[0][0] + q[1][0] + q[2][0] + q[3][0]) / 4, (q[0][1] + q[1][1] + q[2][1] + q[3][1]) / 4
}

// quadExtent is the quad's length along its texture-x axis (the shadow).
func quadExtent(q [4][2]float64) float64 {
	return math.Hypot(q[1][0]-q[0][0], q[1][1]-q[0][1])
}

func hullCentroid(pts [8][2]float64, n int) (float64, float64) {
	var x, y float64
	for i := 0; i < n; i++ {
		x += pts[i][0]
		y += pts[i][1]
	}
	return x / float64(n), y / float64(n)
}

var pawnBlob = ShadowSpec{Kind: ShadowBlob, W: 0.55, H: 1.7, D: 0.35}

var wallBox = ShadowSpec{Kind: ShadowBox, W: 1, H: 1, D: 1}

func TestShadowPointsAwayFromSun(t *testing.T) {
	t.Parallel()
	morning := testLight(0.3, 0.4, 0.3)         // sun in the east, a little south
	evening := testLight(math.Pi-0.3, 0.4, 0.3) // sun in the west
	cx, _ := quadCentre(BlobQuad(5, 5, pawnBlob, morning))
	ex, _ := quadCentre(BlobQuad(5, 5, pawnBlob, evening))
	if !(cx < 5 && ex > 5) {
		t.Fatalf("blob centres: morning %.2f (want west of 5), evening %.2f (want east)", cx, ex)
	}
	_, my := quadCentre(BlobQuad(5, 5, pawnBlob, morning))
	if my >= 5 {
		t.Fatalf("a southern sun casts north (up the screen); centre y %.2f", my)
	}
	pts, n := BoxHull(5, 5, 1, 1, wallBox, morning)
	hx, _ := hullCentroid(pts, n)
	if !(hx < 5.5) {
		t.Fatalf("box shadow centroid x %.2f, want west of the footprint centre", hx)
	}
	// the footprint stays inside the shadow
	for _, c := range [][2]float64{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		if !inConvex(pts, n, c[0], c[1]) {
			t.Fatalf("footprint corner %v outside the hull %v", c, pts[:n])
		}
	}
}

// inConvex reports p inside (or on) the counter-clockwise hull.
func inConvex(pts [8][2]float64, n int, x, y float64) bool {
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%n]
		if (b[0]-a[0])*(y-a[1])-(b[1]-a[1])*(x-a[0]) < -1e-9 {
			return false
		}
	}
	return true
}

func TestShadowLongAtDawnShortAtNoon(t *testing.T) {
	t.Parallel()
	dawn := testLight(0.1, 0.12, 0.1)
	noon := testLight(math.Pi/2, 65*math.Pi/180, 0.32)
	ld, ln := quadExtent(BlobQuad(0, 0, pawnBlob, dawn)), quadExtent(BlobQuad(0, 0, pawnBlob, noon))
	if !(ld > 2*ln) {
		t.Fatalf("blob length dawn %.2f vs noon %.2f", ld, ln)
	}
	if ld > ShadowMaxLen+1.2 {
		t.Fatalf("dawn shadow %.2f exceeds the clamp", ld)
	}
	area := func(l State) float64 {
		pts, n := BoxHull(0, 0, 1, 1, wallBox, l)
		a := 0.0
		for i := 0; i < n; i++ {
			p, q := pts[i], pts[(i+1)%n]
			a += p[0]*q[1] - q[0]*p[1]
		}
		return a / 2
	}
	if ad, an := area(dawn), area(noon); !(ad > an && an > 1) {
		t.Fatalf("box shadow area dawn %.2f noon %.2f", ad, an)
	}
}

func TestMoonShadowsFaint(t *testing.T) {
	t.Parallel()
	day := ShadowCompositeScale(testLight(1, 1, 0.32))
	moon := ShadowCompositeScale(testLight(1, 0.5, 0.12))
	if !(moon[3] > 0 && moon[3] < day[3]/2) {
		t.Fatalf("moon alpha %.3f vs day %.3f", moon[3], day[3])
	}
	// the composite is the dark cool shadow colour, premultiplied
	if !(day[2] > day[0] && math.Abs(float64(day[0])-18.0/255*0.32) < 1e-4) {
		t.Fatalf("composite colour %v", day)
	}
}

func TestBoxVsBlobByMetadata(t *testing.T) {
	t.Parallel()
	meta := func(kind string, fw, fd, ch int) *art.Meta {
		m := &art.Meta{Footprint: [2]int{fw, fd}, Canvas: [2]int{160, ch}}
		m.Shadow.Kind = kind
		if kind != "" {
			m.Shadow.W, m.Shadow.H, m.Shadow.D = 0.8, 2, 0.6
		}
		return m
	}
	cases := []struct {
		key  string
		m    *art.Meta
		want ShadowKind
		h    float64
	}{
		{"structure/wall_wood_05", meta("box", 1, 1, 224), ShadowBox, 2},
		{"plant/tree_oak_00", meta("blob", 1, 1, 400), ShadowBlob, 2},
		{"item/wood_pile", meta("none", 1, 1, 200), ShadowNone, 0},
		// fallbacks by family while the art has no shadow block
		{"structure/wall_wood_05", meta("", 1, 1, 224), ShadowBox, 1},
		{"structure/floor_wood", meta("", 1, 1, 128), ShadowNone, 0},
		{"furniture/table_2x1", meta("", 2, 1, 198), ShadowBox, 1},
		{"rock/rock_wall_00", nil, ShadowBox, 1},
		{"plant/tree_oak_00", meta("", 1, 1, 512), ShadowBlob, 512.0 / art.TilePx * 0.8},
		{"pawn/body_south", meta("", 1, 1, 250), ShadowBlob, 1.7},
		{"animal/dustbuck_east", meta("", 1, 1, 256), ShadowBlob, 256.0 / art.TilePx * 0.7},
		{"item/wood_pile", meta("", 1, 1, 128), ShadowBlob, 0.7},
		{"overlay/stockpile", meta("", 1, 1, 128), ShadowNone, 0},
		{"weather/raindrop", nil, ShadowNone, 0},
		{"terrain/soil_00", nil, ShadowNone, 0},
	}
	for _, c := range cases {
		s := ShadowSpecFor(c.key, c.m)
		if s.Kind != c.want || math.Abs(s.H-c.h) > 1e-9 {
			t.Errorf("%s (kind %q): got %+v, want kind %d h %.3f", c.key, kindOf(c.m), s, c.want, c.h)
		}
	}
	if s := ShadowSpecFor("furniture/table_2x1", meta("", 2, 1, 198)); s.W != 2 || s.D != 1 {
		t.Errorf("box fallback footprint %+v, want 2x1", s)
	}
	if s := ShadowSpecFor("plant/bush_mature_v0", meta("", 1, 1, 200)); math.Abs(s.W-0.7) > 1e-9 || s.D != 0.4 {
		t.Errorf("plant fallback %+v", s)
	}
}

func kindOf(m *art.Meta) string {
	if m == nil {
		return "<nil>"
	}
	return m.Shadow.Kind
}

func TestBoxSkirtPointsOutward(t *testing.T) {
	t.Parallel()
	for _, az := range []float64{0.2, 1.4, 2.0, 2.9, 3.5, 4.8} {
		l := testLight(az, 0.35, 0.3)
		pts, n := BoxHull(0, 0, 1, 1, wallBox, l)
		if n < 4 || n > 6 {
			t.Fatalf("az %.1f: hull of %d points", az, n)
		}
		cx, cy := hullCentroid(pts, n)
		for i := 0; i < n; i++ {
			p, q := pts[i], pts[(i+1)%n]
			dx, dy := q[0]-p[0], q[1]-p[1]
			nx, ny := dy, -dx
			mx, my := (p[0]+q[0])/2-cx, (p[1]+q[1])/2-cy
			if nx*mx+ny*my <= 0 {
				t.Fatalf("az %.1f: edge %d normal points inward", az, i)
			}
		}
	}
}
