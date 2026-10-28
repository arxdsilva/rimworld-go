package light

import (
	"image"
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// Realistic creature shadows (ART-012): the projection, the light plan and
// the silhouette encoding are pure, so these run without a GPU readback.

const castTile = 64.0

// headOf maps the top of a caster hRef tiles tall (calibrated so its
// opaque height is exactly hRef tiles on screen) through the projection.
func headOf(t *testing.T, l State, hRef, kappaK float64) (dx, dy float64) {
	t.Helper()
	const px, py = 300.0, 200.0
	opaque := hRef * art.TilePx // source px: hRef tiles tall
	k := SpriteKappa(hRef, opaque, art.TilePx) * kappaK
	g := ShadowProjection(px, py, k, l, hRef, 1)
	top := py - opaque*castTile/art.TilePx // screen px above the pivot
	x, y := g.Apply(px, top)
	return (x - px) / castTile, (y - py) / castTile
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestProjectionAnchorsFeet(t *testing.T) {
	t.Parallel()
	for _, l := range []State{testLight(0.2, 0.3, 0.3), testLight(1.2, 1.1, 0.3), testLight(math.Pi-0.3, 0.25, 0.3)} {
		g := ShadowProjection(123.5, 77.25, 1.3, l, 0.95, 1)
		if x, y := g.Apply(123.5, 77.25); !near(x, 123.5, 1e-9) || !near(y, 77.25, 1e-9) {
			t.Fatalf("pivot moved to (%v, %v)", x, y)
		}
		// the feet's own row lies across the shadow through the pivot, its
		// width kept
		x, y := g.Apply(140, 77.25)
		if along := (x-123.5)*l.ShadowDir[0] + (y-77.25)*l.ShadowDir[1]; !near(along, 0, 1e-9) || !near(math.Hypot(x-123.5, y-77.25), 16.5, 1e-9) {
			t.Fatalf("ground point moved to (%v, %v)", x, y)
		}
	}
}

func TestHeadMapsToShadowTip(t *testing.T) {
	t.Parallel()
	l := testLight(1, 0.6, 0.3) // far enough from east-west that the depth floor is not in play
	const hRef = 1.0
	n := l.ShadowLen(hRef)
	for _, rho := range []float64{1, 0.5} {
		const px, py = 300.0, 200.0
		opaque := hRef * art.TilePx
		g := ShadowProjection(px, py, SpriteKappa(hRef, opaque, art.TilePx), l, hRef, rho)
		x, y := g.Apply(px, py-opaque*castTile/art.TilePx)
		wantX, wantY := px+l.ShadowDir[0]*n*castTile, py+l.ShadowDir[1]*n*rho*castTile
		if !near(x, wantX, 1e-6) || !near(y, wantY, 1e-6) {
			t.Fatalf("rho %v: head at (%.3f, %.3f), want the shadow tip (%.3f, %.3f)", rho, x, y, wantX, wantY)
		}
	}
}

func TestNoonShadowShortUnderBody(t *testing.T) {
	t.Parallel()
	l := LightAt(12, 15, "clear", 0) // summer noon
	dx, dy := headOf(t, l, 0.948, 1)
	if dy >= 0 || math.Hypot(dx, dy) > 0.6 {
		t.Fatalf("noon head shadow (%.2f, %.2f) tiles: want short and north, under the body", dx, dy)
	}
	if math.Abs(dx) > 0.05 {
		t.Fatalf("noon shadow drifts sideways %.3f", dx)
	}
}

func TestDawnShadowLongWest(t *testing.T) {
	t.Parallel()
	dx, dy := headOf(t, LightAt(6.8, 15, "clear", 0), 0.948, 1)
	if dx > -2 || math.Abs(dy) > math.Abs(dx)/2 {
		t.Fatalf("dawn head shadow (%.2f, %.2f): want long and west", dx, dy)
	}
}

func TestDuskShadowLongEast(t *testing.T) {
	t.Parallel()
	dx, dy := headOf(t, LightAt(17.2, 15, "clear", 0), 0.948, 1)
	if dx < 2 || math.Abs(dy) > math.Abs(dx)/2 {
		t.Fatalf("dusk head shadow (%.2f, %.2f): want long and east", dx, dy)
	}
}

func TestWidthPreservedAcrossShadow(t *testing.T) {
	t.Parallel()
	// every row of the sprite keeps its width, laid across the shadow
	for _, l := range []State{LightAt(7, 15, "clear", 0), LightAt(12, 15, "clear", 0), LightAt(17, 15, "clear", 0),
		testLight(1, 0.6, 0.3), testLight(0, 0.3, 0.3)} {
		g := ShadowProjection(300, 200, 1.1, l, 0.95, 1)
		for _, y := range []float64{200, 180, 120} {
			ax, ay := g.Apply(290, y)
			bx, by := g.Apply(310, y)
			along := (bx-ax)*l.ShadowDir[0] + (by-ay)*l.ShadowDir[1]
			if !near(math.Hypot(bx-ax, by-ay), 20, 1e-9) || !near(along, 0, 1e-9) {
				t.Fatalf("row %v: width %.3f (want 20), along the shadow %.3f", y, math.Hypot(bx-ax, by-ay), along)
			}
		}
	}
	// under the noon sun (shadows due north) x is untouched
	g := ShadowProjection(300, 200, 1.1, LightAt(12, 15, "clear", 0), 0.95, 1)
	if ax, _ := g.Apply(290, 150); !near(ax, 290, 1e-6) {
		t.Fatalf("noon: x moved to %v", ax)
	}
	// a level east-west light keeps the body's width as the shadow's
	// thickness instead of folding it into a hairline
	g = ShadowProjection(300, 200, 1, testLight(0, 0.3, 0.3), 1, 1)
	_, y0 := g.Apply(290, 150)
	_, y1 := g.Apply(310, 150)
	if !near(math.Abs(y1-y0), 20, 1e-9) {
		t.Fatalf("an east-west shadow is %.2f px thick, want the 20 px width", math.Abs(y1-y0))
	}
}

func fire(id uint64, x, y float64) PointLight {
	return PointLight{ID: id, X: x, Y: y, H: CampfireLightH, Reach: CampfireLightReach}
}

func lamp(id uint64, x, y float64) PointLight {
	return PointLight{ID: id, X: x, Y: y, H: LampLightH, Reach: LampLightReach}
}

func TestPointLightDirectionAwayFromFire(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]float64{{7, 5}, {3, 5}, {5, 8}, {6.5, 3.5}} {
		cl, ok := PointCast(c[0], c[1], 0.948, fire(1, 5, 5), 0)
		if !ok {
			t.Fatalf("no shadow at %v", c)
		}
		ux, uy := unit(c[0]-5, c[1]-5)
		if !near(cl.DirX, ux, 1e-9) || !near(cl.DirY, uy, 1e-9) {
			t.Fatalf("at %v the shadow points (%.2f, %.2f), want away from the fire (%.2f, %.2f)", c, cl.DirX, cl.DirY, ux, uy)
		}
		if cl.Alpha <= 0 || cl.Alpha > pointShadowAlpha {
			t.Fatalf("alpha %v", cl.Alpha)
		}
	}
	if _, ok := PointCast(10, 5, 0.948, fire(1, 5, 5), 0); ok {
		t.Fatal("a fire casts beyond its reach")
	}
	if _, ok := PointCast(6, 5, 0.948, fire(1, 5, 5), 1); ok {
		t.Fatal("a fire casts in full daylight")
	}
}

func TestPointLightLengthShrinksNearLight(t *testing.T) {
	t.Parallel()
	const h = 0.948
	tip := func(d float64) float64 {
		cl, ok := PointCast(5+d, 5, h, lamp(1, 5, 5), 0)
		if !ok {
			t.Fatalf("no shadow at %v", d)
		}
		return cl.M * h
	}
	t1, t15, t3 := tip(1), tip(1.5), tip(3)
	if !(t1 < t15 && t15 < t3) {
		t.Fatalf("tips %.2f %.2f %.2f: want shorter nearer the lamp", t1, t15, t3)
	}
	if !near(t1, 1*h/(LampLightH-h), 1e-9) {
		t.Fatalf("tip at 1 tile %.3f, want d·h/(H−h)", t1)
	}
	if t3 > ShadowMaxLen+1e-9 || tip(0.1) < 0.3*h-1e-9 {
		t.Fatal("tip not clamped to [0.3h, 3]")
	}
	// a caster taller than the light: the longest shadow
	if cl, _ := PointCast(6, 5, 0.948, fire(1, 5, 5), 0); !near(cl.M*0.948, ShadowMaxLen, 1e-9) {
		t.Fatalf("over a low fire the shadow is %.2f long", cl.M*0.948)
	}
}

func TestTwoNearestLightsOnly(t *testing.T) {
	t.Parallel()
	lights := []PointLight{fire(9, 10, 5), lamp(4, 7, 5), fire(2, 3, 5), lamp(3, 5, 9), fire(7, 7, 5)}
	var idx [2]int
	n := NearestLights(5, 5, lights, &idx)
	// three lights at distance 2 (ids 2, 4, 7): ties break by id; the
	// fire at distance 5 is out of reach
	if n != 2 || lights[idx[0]].ID != 2 || lights[idx[1]].ID != 4 {
		t.Fatalf("picked %d: %v %v", n, lights[idx[0]].ID, lights[idx[1]].ID)
	}
	pl := PlanCreature(5, 5, 0.948, true, State{}, 0, lights)
	if pl.Np != 2 {
		t.Fatalf("planned %d point-light shadows, want 2", pl.Np)
	}
	if n := NearestLights(5, 5, []PointLight{fire(1, 20, 20)}, &idx); n != 0 {
		t.Fatal("a light out of reach was picked")
	}
}

func TestIndoorNoSunShadowButContactAndLamp(t *testing.T) {
	t.Parallel()
	l := LightAt(21, 15, "clear", 0) // the moon is up
	pl := PlanCreature(5, 5, 0.948, false, l, 0, []PointLight{lamp(1, 3, 4)})
	if pl.Sun {
		t.Fatal("a roofed creature casts a moon shadow")
	}
	if pl.Np != 1 || pl.Points[0].DirX <= 0 {
		t.Fatalf("lamp shadow %+v", pl)
	}
	if !near(pl.Contact, contactRoofAlpha, 1e-9) {
		t.Fatalf("contact under a roof %.3f", pl.Contact)
	}
}

func TestContactShadowAtNight(t *testing.T) {
	t.Parallel()
	l := LightAt(2, 15, "clear", 0)
	l.ShadowAlpha = 0 // moonless
	pl := PlanCreature(5, 5, 0.948, true, l, 0, nil)
	if pl.Sun || pl.Np != 0 {
		t.Fatalf("a shadow with no light: %+v", pl)
	}
	if !near(pl.Contact, contactMinAlpha, 1e-9) {
		t.Fatalf("night contact %.3f, want %.2f", pl.Contact, contactMinAlpha)
	}
}

func TestStormFadesSunNotContact(t *testing.T) {
	t.Parallel()
	clear := PlanCreature(5, 5, 0.948, true, LightAt(10, 15, "clear", 0), 1, nil)
	storm := PlanCreature(5, 5, 0.948, true, LightAt(10, 15, "storm", 1), 1, nil)
	if !clear.Sun || !storm.Sun || !(storm.SunCast.Alpha < clear.SunCast.Alpha) {
		t.Fatalf("sun alpha clear %.3f storm %.3f", clear.SunCast.Alpha, storm.SunCast.Alpha)
	}
	if storm.Contact < contactMinAlpha || !near(storm.Contact, math.Max(contactSunK*storm.SunCast.Alpha, contactMinAlpha), 1e-9) {
		t.Fatalf("storm contact %.3f", storm.Contact)
	}
	if clear.Contact < storm.Contact {
		t.Fatal("contact darker in the storm")
	}
}

func TestHeightEncodedInRed(t *testing.T) {
	t.Parallel()
	// a 10x40 px sprite standing on the pivot line (y 200), 40 px tall
	var geo ebiten.GeoM
	geo.Translate(100, 160)
	c := &Cast{Alpha: 0.5, PivotY: 200, HeightPx: 40}
	l := testLight(1, 0.6, 0.3)
	c.Proj = ShadowProjection(105, 200, 1, l, 1, 1)
	var vs [4]ebiten.Vertex
	CastQuad(&vs, 0, 0, 10, 40, geo, c)
	for i, v := range vs {
		top := i < 2
		wantR := float32(0)
		if top {
			wantR = 0.5
		}
		if !near(float64(v.ColorR), float64(wantR), 1e-6) || v.ColorG != 0.5 || v.ColorB != 0.5 || v.ColorA != 0.5 {
			t.Fatalf("vertex %d colour (%v %v %v %v)", i, v.ColorR, v.ColorG, v.ColorB, v.ColorA)
		}
		along := (float64(v.DstX)-105)*l.ShadowDir[0] + (float64(v.DstY)-200)*l.ShadowDir[1]
		if !top && !near(along, 0, 1e-4) {
			t.Fatalf("bottom vertex left the feet: %.3f along the shadow", along)
		}
		if top && along < 20 {
			t.Fatalf("top vertex not projected along the shadow: %.3f", along)
		}
	}
	if vs[1].SrcX != 10 || vs[2].SrcY != 40 {
		t.Fatalf("texels %+v", vs)
	}
}

func TestCalibrationFromAlphaBounds(t *testing.T) {
	t.Parallel()
	pix := image.NewNRGBA(image.Rect(0, 0, 20, 80))
	for y := 10; y < 60; y++ { // 50 opaque rows, with faint glow above
		pix.SetNRGBA(7, y, color.NRGBA{200, 100, 50, 255})
	}
	pix.SetNRGBA(3, 4, color.NRGBA{0, 0, 0, 8})
	sp := &art.Sprite{Meta: art.Meta{Canvas: [2]int{20, 80}}, Pix: pix}
	if h := sp.OpaqueHeightPx(); h != 50 {
		t.Fatalf("opaque height %v, want 50", h)
	}
	if k := SpriteKappa(0.948, 50, art.TilePx); !near(k, 0.948*art.TilePx/50, 1e-12) {
		t.Fatalf("kappa %v", k)
	}
	// a sprite without opaque pixels falls back to its canvas
	empty := &art.Sprite{Meta: art.Meta{Canvas: [2]int{20, 80}}, Pix: image.NewNRGBA(image.Rect(0, 0, 5, 10))}
	if h := empty.OpaqueHeightPx(); h != 80 {
		t.Fatalf("empty sprite height %v", h)
	}
	// pixels smaller than the canvas (test art) scale to canvas units
	half := image.NewNRGBA(image.Rect(0, 0, 10, 40))
	for y := 5; y < 30; y++ {
		half.SetNRGBA(2, y, color.NRGBA{1, 1, 1, 255})
	}
	if h := (&art.Sprite{Meta: art.Meta{Canvas: [2]int{20, 80}}, Pix: half}).OpaqueHeightPx(); h != 50 {
		t.Fatalf("scaled opaque height %v", h)
	}
}

func TestBlurShaderCompiles(t *testing.T) {
	t.Parallel()
	if _, err := ebiten.NewShader(shadowBlurKage); err != nil {
		t.Fatalf("shadow_blur.kage: %v", err)
	}
	if _, err := ebiten.NewShader([]byte(castKage)); err != nil {
		t.Fatalf("cast shader: %v", err)
	}
	r0, r1 := BlurRadii(1, 1)
	_, r1low := BlurRadii(1, 0)
	if !near(r0, 0.6, 1e-9) || !near(r1, 3.5, 1e-9) || !near(r1low, 3.5*1.6, 1e-9) {
		t.Fatalf("radii %v %v %v", r0, r1, r1low)
	}
	if lo, hi := ShadowTint(0), ShadowTint(1); lo != [3]float32{18, 20, 34} || hi != [3]float32{18, 20, 30} {
		t.Fatalf("tint %v %v", lo, hi)
	}
}

func TestSunElevationFromLight(t *testing.T) {
	t.Parallel()
	for _, el := range []float64{0.1, 0.5, 1.1} {
		l := State{SunL: F32v(LightDir(el, 0.7))}
		if got := SunElevation(l); !near(got, el, 1e-5) {
			t.Fatalf("elevation %v, want %v", got, el)
		}
	}
	if SunElevation(LightAt(2, 15, "clear", 0)) != 0 || DaylightOf(0) != 0 || DaylightOf(math.Pi/4) != 1 {
		t.Fatal("night is daylight")
	}
	if f := FireFlicker(123); f < -1 || f > 1 {
		t.Fatalf("flicker %v", f)
	}
}
