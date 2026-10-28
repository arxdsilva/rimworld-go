package ebiten

import (
	"image"
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/sim/entity"
)

// Realistic creature shadows (ART-012): the projection, the light plan and
// the cast-mode routing are pure, so these run without a GPU readback.

const castTile = 64.0

// headOf maps the top of a caster hRef tiles tall (calibrated so its
// opaque height is exactly hRef tiles on screen) through the projection.
func headOf(t *testing.T, l lightState, hRef, kappaK float64) (dx, dy float64) {
	t.Helper()
	const px, py = 300.0, 200.0
	opaque := hRef * artTilePx // source px: hRef tiles tall
	k := spriteKappa(hRef, opaque, artTilePx) * kappaK
	g := shadowProjection(px, py, k, l, hRef, 1)
	top := py - opaque*castTile/artTilePx // screen px above the pivot
	x, y := g.Apply(px, top)
	return (x - px) / castTile, (y - py) / castTile
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestProjectionAnchorsFeet(t *testing.T) {
	t.Parallel()
	for _, l := range []lightState{testLight(0.2, 0.3, 0.3), testLight(1.2, 1.1, 0.3), testLight(math.Pi-0.3, 0.25, 0.3)} {
		g := shadowProjection(123.5, 77.25, 1.3, l, 0.95, 1)
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
		opaque := hRef * artTilePx
		g := shadowProjection(px, py, spriteKappa(hRef, opaque, artTilePx), l, hRef, rho)
		x, y := g.Apply(px, py-opaque*castTile/artTilePx)
		wantX, wantY := px+l.ShadowDir[0]*n*castTile, py+l.ShadowDir[1]*n*rho*castTile
		if !near(x, wantX, 1e-6) || !near(y, wantY, 1e-6) {
			t.Fatalf("rho %v: head at (%.3f, %.3f), want the shadow tip (%.3f, %.3f)", rho, x, y, wantX, wantY)
		}
	}
}

func TestNoonShadowShortUnderBody(t *testing.T) {
	t.Parallel()
	l := lightAt(12, 15, "clear", 0) // summer noon
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
	dx, dy := headOf(t, lightAt(6.8, 15, "clear", 0), 0.948, 1)
	if dx > -2 || math.Abs(dy) > math.Abs(dx)/2 {
		t.Fatalf("dawn head shadow (%.2f, %.2f): want long and west", dx, dy)
	}
}

func TestDuskShadowLongEast(t *testing.T) {
	t.Parallel()
	dx, dy := headOf(t, lightAt(17.2, 15, "clear", 0), 0.948, 1)
	if dx < 2 || math.Abs(dy) > math.Abs(dx)/2 {
		t.Fatalf("dusk head shadow (%.2f, %.2f): want long and east", dx, dy)
	}
}

func TestWidthPreservedAcrossShadow(t *testing.T) {
	t.Parallel()
	// every row of the sprite keeps its width, laid across the shadow
	for _, l := range []lightState{lightAt(7, 15, "clear", 0), lightAt(12, 15, "clear", 0), lightAt(17, 15, "clear", 0),
		testLight(1, 0.6, 0.3), testLight(0, 0.3, 0.3)} {
		g := shadowProjection(300, 200, 1.1, l, 0.95, 1)
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
	g := shadowProjection(300, 200, 1.1, lightAt(12, 15, "clear", 0), 0.95, 1)
	if ax, _ := g.Apply(290, 150); !near(ax, 290, 1e-6) {
		t.Fatalf("noon: x moved to %v", ax)
	}
	// a level east-west light keeps the body's width as the shadow's
	// thickness instead of folding it into a hairline
	g = shadowProjection(300, 200, 1, testLight(0, 0.3, 0.3), 1, 1)
	_, y0 := g.Apply(290, 150)
	_, y1 := g.Apply(310, 150)
	if !near(math.Abs(y1-y0), 20, 1e-9) {
		t.Fatalf("an east-west shadow is %.2f px thick, want the 20 px width", math.Abs(y1-y0))
	}
}

func TestLyingPoseFlat(t *testing.T) {
	t.Parallel()
	l := testLight(1, 0.6, 0.3)
	ux, uy := headOf(t, l, 1, 1)
	lx, ly := headOf(t, l, 1, lyingKappa)
	if lyingKappa != 0.25 || !near(lx, ux*0.25, 1e-9) || !near(ly, uy*0.25, 1e-9) {
		t.Fatalf("lying tip (%.3f, %.3f), upright (%.3f, %.3f): want a quarter", lx, ly, ux, uy)
	}
	// a pawn lying on the ground uses the lying projection; one in bed casts nothing
	p := game.PawnView{ID: 1, Lying: true}
	if !figureLying(p) {
		t.Fatal("a pawn on the ground is not lying flat")
	}
}

// --- point lights --------------------------------------------------------

func fire(id uint64, x, y float64) pointLight {
	return pointLight{ID: id, X: x, Y: y, H: campfireLightH, Reach: campfireLightReach}
}

func lamp(id uint64, x, y float64) pointLight {
	return pointLight{ID: id, X: x, Y: y, H: lampLightH, Reach: lampLightReach}
}

func TestPointLightDirectionAwayFromFire(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]float64{{7, 5}, {3, 5}, {5, 8}, {6.5, 3.5}} {
		cl, ok := pointCast(c[0], c[1], 0.948, fire(1, 5, 5), 0)
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
	if _, ok := pointCast(10, 5, 0.948, fire(1, 5, 5), 0); ok {
		t.Fatal("a fire casts beyond its reach")
	}
	if _, ok := pointCast(6, 5, 0.948, fire(1, 5, 5), 1); ok {
		t.Fatal("a fire casts in full daylight")
	}
}

func TestPointLightLengthShrinksNearLight(t *testing.T) {
	t.Parallel()
	const h = 0.948
	tip := func(d float64) float64 {
		cl, ok := pointCast(5+d, 5, h, lamp(1, 5, 5), 0)
		if !ok {
			t.Fatalf("no shadow at %v", d)
		}
		return cl.M * h
	}
	t1, t15, t3 := tip(1), tip(1.5), tip(3)
	if !(t1 < t15 && t15 < t3) {
		t.Fatalf("tips %.2f %.2f %.2f: want shorter nearer the lamp", t1, t15, t3)
	}
	if !near(t1, 1*h/(lampLightH-h), 1e-9) {
		t.Fatalf("tip at 1 tile %.3f, want d·h/(H−h)", t1)
	}
	if t3 > shadowMaxLen+1e-9 || tip(0.1) < 0.3*h-1e-9 {
		t.Fatal("tip not clamped to [0.3h, 3]")
	}
	// a caster taller than the light: the longest shadow
	if cl, _ := pointCast(6, 5, 0.948, fire(1, 5, 5), 0); !near(cl.M*0.948, shadowMaxLen, 1e-9) {
		t.Fatalf("over a low fire the shadow is %.2f long", cl.M*0.948)
	}
}

func TestTwoNearestLightsOnly(t *testing.T) {
	t.Parallel()
	lights := []pointLight{fire(9, 10, 5), lamp(4, 7, 5), fire(2, 3, 5), lamp(3, 5, 9), fire(7, 7, 5)}
	var idx [2]int
	n := nearestLights(5, 5, lights, &idx)
	// three lights at distance 2 (ids 2, 4, 7): ties break by id; the
	// fire at distance 5 is out of reach
	if n != 2 || lights[idx[0]].ID != 2 || lights[idx[1]].ID != 4 {
		t.Fatalf("picked %d: %v %v", n, lights[idx[0]].ID, lights[idx[1]].ID)
	}
	pl := planCreature(5, 5, 0.948, true, lightState{}, 0, lights)
	if pl.Np != 2 {
		t.Fatalf("planned %d point-light shadows, want 2", pl.Np)
	}
	if n := nearestLights(5, 5, []pointLight{fire(1, 20, 20)}, &idx); n != 0 {
		t.Fatal("a light out of reach was picked")
	}
}

// --- the per-creature plan ------------------------------------------------

func TestIndoorNoSunShadowButContactAndLamp(t *testing.T) {
	t.Parallel()
	l := lightAt(21, 15, "clear", 0) // the moon is up
	pl := planCreature(5, 5, 0.948, false, l, 0, []pointLight{lamp(1, 3, 4)})
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
	l := lightAt(2, 15, "clear", 0)
	l.ShadowAlpha = 0 // moonless
	pl := planCreature(5, 5, 0.948, true, l, 0, nil)
	if pl.Sun || pl.Np != 0 {
		t.Fatalf("a shadow with no light: %+v", pl)
	}
	if !near(pl.Contact, contactMinAlpha, 1e-9) {
		t.Fatalf("night contact %.3f, want %.2f", pl.Contact, contactMinAlpha)
	}
}

func TestStormFadesSunNotContact(t *testing.T) {
	t.Parallel()
	clear := planCreature(5, 5, 0.948, true, lightAt(10, 15, "clear", 0), 1, nil)
	storm := planCreature(5, 5, 0.948, true, lightAt(10, 15, "storm", 1), 1, nil)
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

// --- encoding and calibration ---------------------------------------------

func TestHeightEncodedInRed(t *testing.T) {
	t.Parallel()
	// a 10x40 px sprite standing on the pivot line (y 200), 40 px tall
	var geo ebiten.GeoM
	geo.Translate(100, 160)
	c := &shadowCast{Alpha: 0.5, PivotY: 200, HeightPx: 40}
	l := testLight(1, 0.6, 0.3)
	c.Proj = shadowProjection(105, 200, 1, l, 1, 1)
	var vs [4]ebiten.Vertex
	castQuad(&vs, 0, 0, 10, 40, geo, c)
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
	sp := &artSprite{Meta: artMeta{Canvas: [2]int{20, 80}}, Pix: pix}
	if h := sp.OpaqueHeightPx(); h != 50 {
		t.Fatalf("opaque height %v, want 50", h)
	}
	if k := spriteKappa(0.948, 50, artTilePx); !near(k, 0.948*artTilePx/50, 1e-12) {
		t.Fatalf("kappa %v", k)
	}
	// a sprite without opaque pixels falls back to its canvas
	empty := &artSprite{Meta: artMeta{Canvas: [2]int{20, 80}}, Pix: image.NewNRGBA(image.Rect(0, 0, 5, 10))}
	if h := empty.OpaqueHeightPx(); h != 80 {
		t.Fatalf("empty sprite height %v", h)
	}
	// pixels smaller than the canvas (test art) scale to canvas units
	half := image.NewNRGBA(image.Rect(0, 0, 10, 40))
	for y := 5; y < 30; y++ {
		half.SetNRGBA(2, y, color.NRGBA{1, 1, 1, 255})
	}
	if h := (&artSprite{Meta: artMeta{Canvas: [2]int{20, 80}}, Pix: half}).OpaqueHeightPx(); h != 50 {
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
	r0, r1 := blurRadii(1, 1)
	_, r1low := blurRadii(1, 0)
	if !near(r0, 0.6, 1e-9) || !near(r1, 3.5, 1e-9) || !near(r1low, 3.5*1.6, 1e-9) {
		t.Fatalf("radii %v %v %v", r0, r1, r1low)
	}
	if lo, hi := shadowTint(0), shadowTint(1); lo != [3]float32{18, 20, 34} || hi != [3]float32{18, 20, 30} {
		t.Fatalf("tint %v %v", lo, hi)
	}
}

// --- cast-mode routing and purity ------------------------------------------

// figureArt is the combat test art plus the clothing and hair layers.
func figureArt() *artCatalog {
	c := fakeCombatArt()
	for f := faceSouth; f <= faceWest; f++ {
		for _, k := range []string{"pants_basic_", "shirt_basic_", "vest_utility_", "hair_short_", "hair_crop_", "hair_bun_"} {
			c.Sprites["pawn/"+k+f.String()] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
		}
	}
	return c
}

// keyOf maps a drawn image back to its sprite key.
func keyOf(c *artCatalog, img *ebiten.Image) string {
	for k, sp := range c.Sprites {
		if sp.Img == img {
			return k
		}
	}
	return "?"
}

// castAll draws p's figure in cast mode and returns the keys drawn.
func castAll(r *Renderer, p game.PawnView) map[string]int {
	got := map[string]int{}
	layer := ebiten.NewImage(512, 512)
	r.cast = &shadowCast{Layer: layer, Alpha: 0.3, OnDraw: func(img *ebiten.Image) { got[keyOf(r.art, img)]++ }}
	defer func() { r.cast = nil }()
	r.drawPawnFigure(ebiten.NewImage(512, 512), p, 2, 2, castTile, r.pawnFigurePoseOf(p))
	return got
}

func TestCastModeRoutesAllFigureDraws(t *testing.T) {
	t.Parallel()
	shooter := game.PawnView{ID: 1, X: 2, Y: 2, Weapon: "rifle", Drafted: true, Hostile: true}
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	r.view.Pawns = []game.PawnView{shooter}
	got := castAll(r, shooter)
	hair := "pawn/hair_" + look.HairStyles[shooter.Look.HairStyle] + "_south"
	for _, k := range []string{"pawn/body_south", "pawn/pants_basic_south", "pawn/shirt_basic_south", hair, "weapon/rifle_basic"} {
		if got[k] != 1 {
			t.Fatalf("cast %q %d times: %v", k, got[k], got)
		}
	}
	if got["?"] != 0 {
		t.Fatalf("a non-figure draw went through the cast: %v", got)
	}
	if got[staleHandKey] != 0 {
		t.Fatalf("no hands cast: %v", got)
	}
	// a hauler's load is part of its silhouette
	hauler := game.PawnView{ID: 2, X: 2, Y: 2, Carrying: "wood"}
	r.view.Pawns = []game.PawnView{hauler}
	if got := castAll(r, hauler); got["item/wood_pile"] != 1 || got["pawn/body_south"] != 1 {
		t.Fatalf("hauler cast %v", got)
	}
	// a rescuer's patient is too
	rescuer := game.PawnView{ID: 3, X: 2, Y: 2, Carrying: "pawn"}
	patient := game.PawnView{ID: 4, X: 2, Y: 2, CarriedBy: 3}
	r.view.Pawns = []game.PawnView{rescuer, patient}
	if got := castAll(r, rescuer); got["pawn/body_south"] != 2 {
		t.Fatalf("rescuer and patient cast %v", got)
	}
}

func TestFigureDrawIsPure(t *testing.T) {
	t.Parallel()
	rescuer := game.PawnView{ID: 3, X: 2, Y: 2, Carrying: "pawn", Drafted: true, Hostile: true, Weapon: "rifle", HitTick: 95, HitFromX: 3, HitFromY: 2}
	patient := game.PawnView{ID: 4, X: 2, Y: 2, CarriedBy: 3}
	downed := game.PawnView{ID: 5, X: 4, Y: 4, Lying: true, Status: "downed", HitTick: 99, HitFromX: 5, HitFromY: 4}
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	r.present = 100
	r.view.Pawns = []game.PawnView{rescuer, patient, downed}
	r.last[5] = pawnVisual{facing: faceEast, lyingSince: 90}
	st := r.animState()
	st.PrevStride[3] = 0.25
	type snap struct {
		carry, stride, work, recs, last, parts, floaters int
	}
	take := func() snap {
		return snap{len(st.CarryBorn), len(st.PrevStride), len(st.PrevWork), len(st.Recs), len(r.last), len(r.parts), len(r.floaters)}
	}
	before := take()
	for _, p := range []game.PawnView{rescuer, downed} {
		if got := castAll(r, p); len(got) == 0 {
			t.Fatalf("pawn %d cast nothing", p.ID)
		}
	}
	if after := take(); after != before {
		t.Fatalf("a cast draw wrote renderer state: %+v -> %+v", before, after)
	}
	if _, lifted := st.CarryBorn[entity.EntityID(4)]; lifted {
		t.Fatal("the cast pass started the patient's lift")
	}
	// the normal draw still starts it
	r.drawPawnArt(ebiten.NewImage(512, 512), rescuer, 2, 2, castTile, 512, 512)
	if _, lifted := st.CarryBorn[entity.EntityID(4)]; !lifted {
		t.Fatal("the object pass no longer lifts the patient")
	}
	// pawns in bed cast nothing; the figure draws no extras (markers, icons)
	sleeper := game.PawnView{ID: 6, X: 8, Y: 8, Lying: true, InBed: true, Status: "alive"}
	r.view.W.Buildings = []game.BuildingView{{ID: 1, Kind: "bed", X: 8, Y: 8, W: 1, H: 2}}
	r.view.Pawns = []game.PawnView{sleeper}
	if got := castAll(r, sleeper); len(got) != 0 {
		t.Fatalf("a pawn in bed cast %v", got)
	}
	if figureLying(sleeper) {
		t.Fatal("a pawn in bed counts as lying on the ground")
	}
}

func TestPointLightsFromCampfiresAndLitLamps(t *testing.T) {
	t.Parallel()
	w := shadowWorld(nil)
	for i := range w.RoomGrid[:20*6] { // rows 0-5: room 1 (lit), rows 6-11: room 2 (dark)
		w.RoomGrid[i] = 1
	}
	for i := 20 * 6; i < len(w.RoomGrid); i++ {
		w.RoomGrid[i] = 2
	}
	w.Rooms = []game.RoomView{{ID: 1, Lit: true}, {ID: 2}}
	w.Buildings = []game.BuildingView{
		{ID: 1, Kind: "campfire", X: 2, Y: 8, W: 1, H: 1, Burning: true, Glow: true, GlowRadius: 5},
		{ID: 2, Kind: "campfire", X: 5, Y: 8, W: 1, H: 1},                         // out
		{ID: 3, Kind: "lamp", Glow: true, GlowRadius: 6, X: 10, Y: 2, W: 1, H: 1}, // room 1
		{ID: 4, Kind: "lamp", Glow: true, GlowRadius: 6, X: 10, Y: 9, W: 1, H: 1}, // room 2
	}
	r := shadowRenderer(t, w)
	r.view.W = w
	// PLAY-020 §3b, PLAY-021: every lamp that glows (powered) is a light,
	// not only those in a room the sim calls lit; the banked campfire is out.
	got := r.pointLights()
	if len(got) != 3 || got[0].ID != 1 || got[0].H != campfireLightH || got[1].ID != 3 || got[1].H != lampLightH ||
		got[2].ID != 4 || got[1].Reach != lampLightReach {
		t.Fatalf("lights %+v", got)
	}
	if got[0].X != 2.5 || got[0].Y != 8.5 {
		t.Fatalf("a light sits at its tile centre: %+v", got[0])
	}
	// the night pass: a pawn by the fire and one in the lamp-lit room
	// (roofed) each cast a point-light silhouette and a contact pool, and
	// no moon shadow under the roof
	r.art = figureArt()
	for i := 0; i < 20*6; i++ {
		if w.RoofCells == nil {
			w.RoofCells = make([]uint8, len(w.RoomGrid))
		}
		w.RoofCells[i] = roofBuilt
	}
	r.view.W = w
	r.view.Pawns = []game.PawnView{{ID: 1, X: 3, Y: 8}, {ID: 2, X: 9, Y: 3}}
	r.lit.Light = lightAt(23, 15, "clear", 0)
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	// pawn 1: the moon and the fire; pawn 2: the lamp only
	if r.env.shadow.Casts != 3 || r.env.shadow.Composites != 1 {
		t.Fatalf("drew %d silhouettes, %d composites", r.env.shadow.Casts, r.env.shadow.Composites)
	}
	if n := len(r.env.shadow.Contacts.Verts); n != 0 {
		t.Fatalf("contact batch not flushed: %d", n)
	}
	// Full daylight no longer empties the light list (the old "no point
	// lights at daylight 1"): it washes out only open-sky creatures. At
	// noon pawn 1 casts the sun only (its fire shadow is gone) and pawn 2,
	// under the roof, keeps its lamp shadow.
	r.lit.Light = lightAt(12, 15, "clear", 0)
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if r.env.shadow.Casts != 3+2 {
		t.Fatalf("noon drew %d silhouettes, want the sun and the indoor lamp", r.env.shadow.Casts-3)
	}
}

// castsAt draws one shadow frame of pawns in w at hour and returns how
// many silhouettes it cast.
func castsAt(t *testing.T, w game.WorldView, hour float64, pawns ...game.PawnView) int {
	t.Helper()
	r := shadowRenderer(t, w)
	r.art = figureArt()
	r.view.W = w
	r.view.Pawns = pawns
	r.lit.Light = lightAt(hour, 15, "clear", 0)
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	return r.env.shadow.Casts
}

func TestOutdoorLampCastsCreatureShadowAtNight(t *testing.T) {
	t.Parallel()
	w := shadowWorld(nil)
	for y := 2; y <= 8; y++ {
		w.RoomGrid[y*20+12] = -2 // a wall east of the lamp
	}
	w.Buildings = []game.BuildingView{lampBuilding(1, 9, 5)}
	if n := castsAt(t, w, 23, game.PawnView{ID: 1, X: 11, Y: 5}); n != 2 {
		t.Fatalf("by an outdoor lamp at night: %d silhouettes, want the moon and the lamp", n)
	}
	if n := castsAt(t, w, 23, game.PawnView{ID: 1, X: 13, Y: 5}); n != 1 {
		t.Fatalf("behind the wall at night: %d silhouettes, want the moon only", n)
	}
	if n := castsAt(t, w, 12, game.PawnView{ID: 1, X: 11, Y: 5}); n != 1 {
		t.Fatalf("by an outdoor lamp at noon: %d silhouettes, want the sun only", n)
	}
}

func TestRoofedRoomAtNoonKeepsLampShadows(t *testing.T) {
	t.Parallel()
	w := shadowWorld(map[int]uint8{})
	lampRoom(&w, 0, 1, 1, 9, 8)  // inside x 2..8
	lampRoom(&w, 1, 9, 1, 16, 8) // shares the wall at x 9; inside x 10..15
	w.Buildings = []game.BuildingView{lampBuilding(1, 7, 4)}
	if n := castsAt(t, w, 12, game.PawnView{ID: 1, X: 5, Y: 4}); n != 1 {
		t.Fatalf("a roofed room with a lamp at noon: %d silhouettes, want the lamp only", n)
	}
	if n := castsAt(t, w, 23, game.PawnView{ID: 1, X: 10, Y: 4}); n != 0 {
		t.Fatalf("the next room, 3 tiles from the lamp through a wall: %d silhouettes, want none", n)
	}
}

func TestSunElevationFromLight(t *testing.T) {
	t.Parallel()
	for _, el := range []float64{0.1, 0.5, 1.1} {
		l := lightState{SunL: f32v(lightDir(el, 0.7))}
		if got := sunElevation(l); !near(got, el, 1e-5) {
			t.Fatalf("elevation %v, want %v", got, el)
		}
	}
	if sunElevation(lightAt(2, 15, "clear", 0)) != 0 || daylightOf(0) != 0 || daylightOf(math.Pi/4) != 1 {
		t.Fatal("night is daylight")
	}
	if f := fireFlicker(123); f < -1 || f > 1 {
		t.Fatalf("flicker %v", f)
	}
}
