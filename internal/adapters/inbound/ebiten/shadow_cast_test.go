package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/sim/entity"
)

// Realistic creature shadows (ART-012): the cast-mode routing of the
// figure code and the shadow pass over real casters (the pure projection
// and plan tests are in render/light).

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
	if r.env.Shadow.Casts != 3 || r.env.Shadow.Composites != 1 {
		t.Fatalf("drew %d silhouettes, %d composites", r.env.Shadow.Casts, r.env.Shadow.Composites)
	}
	if n := len(r.env.Shadow.Contacts.Verts); n != 0 {
		t.Fatalf("contact batch not flushed: %d", n)
	}
	// Full daylight no longer empties the light list (the old "no point
	// lights at daylight 1"): it washes out only open-sky creatures. At
	// noon pawn 1 casts the sun only (its fire shadow is gone) and pawn 2,
	// under the roof, keeps its lamp shadow.
	r.lit.Light = lightAt(12, 15, "clear", 0)
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if r.env.Shadow.Casts != 3+2 {
		t.Fatalf("noon drew %d silhouettes, want the sun and the indoor lamp", r.env.Shadow.Casts-3)
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
	return r.env.Shadow.Casts
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
