package ebiten

import (
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
)

func TestTargetFacingTurnsToAttackTarget(t *testing.T) {
	shooter := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionAim, ActTarget: 9}
	pawns := []game.PawnView{shooter, {ID: 9, X: 5, Y: 1}}
	if f, ok := targetFacing(shooter, pawns, nil, faceSouth); !ok || f != faceNorth {
		t.Fatalf("aiming at a pawn to the north: got %v, %v", f, ok)
	}
	animals := []game.AnimalView{{ID: 9, X: 9, Y: 5}}
	if f, ok := targetFacing(shooter, nil, animals, faceSouth); !ok || f != faceEast {
		t.Fatalf("aiming at an animal to the east: got %v, %v", f, ok)
	}
	idle := game.PawnView{ID: 1, X: 5, Y: 5, ActTarget: 9}
	if _, ok := targetFacing(idle, pawns, nil, faceSouth); ok {
		t.Fatal("no attack action: no target facing")
	}
	gone := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionMelee, ActTarget: 42}
	if _, ok := targetFacing(gone, pawns, animals, faceSouth); ok {
		t.Fatal("target not in view: no target facing")
	}
}

// Pawns are rendered south, east and north: facing west draws the east
// sprites (every layer, the held item and the hands) flipped about the
// figure's pivot.
func TestWestMirrorsEast(t *testing.T) {
	t.Parallel()
	for f, want := range map[facing]struct {
		key    string
		mirror bool
	}{faceSouth: {"south", false}, faceEast: {"east", false}, faceNorth: {"north", false}, faceWest: {"east", true}} {
		if k, m := facingKey(f); k != want.key || m != want.mirror {
			t.Fatalf("facingKey(%v) = %s %v", f, k, m)
		}
	}
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	p := game.PawnView{ID: 1, X: 2, Y: 2, Weapon: "rifle", Drafted: true}
	west, east := drawnKeys(r, p, faceWest), drawnKeys(r, p, faceEast)
	if !slices.Equal(west, east) || len(west) == 0 {
		t.Fatalf("west draws %v, east %v", west, east)
	}
	for _, k := range west {
		if strings.Contains(k, "west") {
			t.Fatalf("west art drawn: %s", k)
		}
	}
	// the geometry is the east figure flipped about the pivot's screen x
	sx, _ := r.worldToScreenF(2+pawnFeetX, 2)
	er, eps := rigFor(r, faceEast)
	wr, wps := rigFor(r, faceWest)
	h := readyHeld(p)
	for _, name := range []string{"head", "hold", "back"} {
		e, w := bodyPoint(r, er, eps, name, 2, 2, 64), bodyPoint(r, wr, wps, name, 2, 2, 64)
		if math.Abs((e[0]-float64(sx))+(w[0]-float64(sx))) > 1e-6 || math.Abs(e[1]-w[1]) > 1e-6 {
			t.Fatalf("%s: west %v is not east %v mirrored about x=%v", name, w, e, sx)
		}
	}
	if slices.Contains(west, staleHandKey) {
		t.Fatalf("no hands drawn: %v", west)
	}
	em, _ := r.heldPoint(er, h, 2, 2, eps, "muzzle")
	wm, _ := r.heldPoint(wr, h, 2, 2, wps, "muzzle")
	if math.Abs((em.X-2-pawnFeetX)+(wm.X-2-pawnFeetX)) > 1e-9 || wm.X >= 2+pawnFeetX {
		t.Fatalf("the muzzle points west: east %v west %v", em, wm)
	}
	// animals too: a west-facing animal draws its east art, mirrored
	r.art.Sprites["animal/deer_east"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	r.last[7] = pawnVisual{facing: faceWest}
	an := game.AnimalView{ID: 7, Species: "deer", X: 2, Y: 2}
	if k := r.animalBaseKey(an); k != "animal/deer_east" || !r.animalMirrored(an) {
		t.Fatalf("west animal: %s mirrored=%v", k, r.animalMirrored(an))
	}
	var got []string
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { got = append(got, keyOf(r.art, img)) }
	r.drawAnimalFigure(ebiten.NewImage(256, 256), an, 2, 2, 64)
	r.lit.OnDraw = nil
	if !slices.Equal(got, []string{"animal/deer_east"}) {
		t.Fatalf("west animal drew %v", got)
	}
}

// drawnKeys draws p's standing figure facing f and lists the sprite keys
// in draw order.
func drawnKeys(r *Renderer, p game.PawnView, f facing) []string {
	var keys []string
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
	defer func() { r.lit.OnDraw = nil }()
	r.drawPawnFigure(ebiten.NewImage(512, 512), p, 2, 2, 64, pawnFigurePose{Face: f, Ps: standPose(), Held: r.idleHeld(p, moveState{})})
	return keys
}

// The colony figure is the pawn's own look (PawnView.Look): its build's
// body and clothes, its beard and hair, so the colonist built on the pawns
// screen is the one walking in the colony; a raider's clothes are maroon.
func TestColonyUsesLookBuildAndBeard(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	for _, fs := range []string{"south", "east", "north"} {
		for _, k := range []string{"body_f_heavy_", "pants_basic_f_heavy_", "shirt_basic_f_heavy_", "beard_full_", "hair_long_"} {
			r.art.Sprites["pawn/"+k+fs] = fakeSprite([2]int{160, 297}, pt{16, 153}, map[string]pt{"head": {80, 122}, "hold": {104, 190}})
		}
	}
	r.art.Tints = map[string]map[string]color.NRGBA{
		"cloth": {"blue": {0, 0, 200, 255}, "raider": {120, 20, 30, 255}, "slate": {60, 60, 70, 255}},
	}
	l := look.Look{Sex: look.Female, Build: look.Heavy, Beard: 2, HairStyle: 3}
	p := game.PawnView{ID: 1, X: 2, Y: 2, Look: l}
	for _, f := range []facing{faceSouth, faceEast, faceNorth, faceWest} {
		keys := drawnKeys(r, game.PawnView{ID: 1, X: 2, Y: 2, Look: l, Weapon: "rifle", Drafted: true}, f)
		fs, _ := facingKey(f)
		for _, want := range []string{"pawn/body_f_heavy_", "pawn/shirt_basic_f_heavy_", "pawn/beard_full_", "pawn/hair_long_"} {
			if !slices.Contains(keys, want+fs) {
				t.Fatalf("%v: %s missing from the colony figure: %v", f, want+fs, keys)
			}
		}
	}
	// the held item anchors on the build's own hold point
	rig := r.rigOf(p, faceEast)
	if rig.Body != r.art.Sprite("pawn/body_f_heavy_east") {
		t.Fatal("the rig is the build's body")
	}
	// raiders keep their look's build and hair in maroon and slate
	raider := r.pawnFigureLayers(game.PawnView{Look: l, Hostile: true}, "south")
	byKey := map[string]color.NRGBA{}
	for _, ly := range raider {
		byKey[ly.Key] = ly.Tint
	}
	if byKey["pawn/shirt_basic_f_heavy_south"] != (color.NRGBA{120, 20, 30, 255}) || byKey["pawn/pants_basic_f_heavy_south"] != (color.NRGBA{60, 60, 70, 255}) {
		t.Fatalf("raider clothes: %v", byKey)
	}
	if _, ok := byKey["pawn/hair_long_south"]; !ok {
		t.Fatalf("raider keeps its hair: %v", byKey)
	}
	// the colonist bar portrait shows the same layers
	if got, want := r.pawnPortraitLayers(p), r.pawnFigureLayers(p, "south"); !slices.Equal(got, want) {
		t.Fatalf("portrait %v, figure %v", got, want)
	}
}

// The portrait and bust crops follow the body metadata: re-rendered art
// with a new canvas, origin or head moves them with it.
func TestPawnCropsFromMetadata(t *testing.T) {
	t.Parallel()
	body := fakeSprite([2]int{160, 297}, pt{16, 153}, map[string]pt{"head": {80, 122}})
	port, bust := pawnCrops(body)
	feet := 153 + pawnFeetY*artTilePx
	if port.Min.Y >= 122-30 || port.Max.Y <= int(feet) || bust.Max.Y >= int(feet) || bust.Min.Y != port.Min.Y {
		t.Fatalf("portrait %v covers the head to the feet (%v), the bust the head and shoulders %v", port, feet, bust)
	}
	if c := (port.Min.X + port.Max.X) / 2; c < 79 || c > 81 {
		t.Fatalf("centred on the head: %v", port)
	}
	moved := fakeSprite([2]int{160, 288}, pt{16, 144}, map[string]pt{"head": {80, 118.8}})
	p2, b2 := pawnCrops(moved)
	if p2 == port || b2 == bust || p2.Max.Y >= port.Max.Y {
		t.Fatalf("crops follow the metadata: %v %v vs %v %v", p2, b2, port, bust)
	}
	if full, _ := pawnCrops(fakeSprite([2]int{160, 297}, pt{16, 153}, nil)); full != image.Rect(0, 0, 160, 297) {
		t.Fatalf("no head: the whole canvas, got %v", full)
	}
}

// A mirrored draw (west = the east art flipped) inverts its normal map's
// x, so the sprite stays lit from the light's side: drawLit marks the
// draw with a negative normal offset (Custom2), which lit.kage reads as
// "flip x" (Go twin decodeNormalMirrored). Pawns, held items and animals
// all mirror through their GeoM, so all of them are covered.
func TestMirroredDrawInvertsNormalX(t *testing.T) {
	flip := ebiten.GeoM{}
	flip.Scale(-1, 1)
	flip.Translate(8, 0)
	if mirrorNormalOff(9, flip) != -9 || mirrorNormalOff(9, ebiten.GeoM{}) != 9 || mirrorNormalOff(0, flip) != 0 {
		t.Fatal("the offset's sign marks a mirrored draw")
	}
	east := [4]float32{1, 0.5, 0.5, 1}
	if n, w := decodeNormalMirrored(east, true, true), decodeNormal([4]float32{0, 0.5, 0.5, 1}, true); math.Abs(float64(n[0]-w[0])) > 1e-6 || n[0] >= 0 {
		t.Fatalf("mirrored east normal %v reads as the west normal %v", n, w)
	}
	if n := decodeNormalMirrored(east, true, false); n != decodeNormal(east, true) {
		t.Fatal("unmirrored: unchanged")
	}
	sprite, normal := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8)
	r := &Renderer{art: &artCatalog{Normals: map[*ebiten.Image]*ebiten.Image{sprite: normal}}}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(8, summerDay, "clear", 0)
	dst := ebiten.NewImage(32, 32)
	for _, c := range []struct {
		g    ebiten.GeoM
		want float32
	}{{ebiten.GeoM{}, 8 + litGap}, {flip, -(8 + litGap)}} {
		r.drawLit(dst, sprite, c.g, noCS, 0, 0)
		if r.lit.Err != nil {
			t.Fatal(r.lit.Err)
		}
		for i, v := range r.lit.Verts {
			if v.Custom2 != c.want {
				t.Fatalf("vertex %d Custom2 %v, want %v", i, v.Custom2, c.want)
			}
		}
	}
	// a mirrored pose (west) gives drawSpritePose a mirroring GeoM
	ps := mirrored(standPose())
	if g := ps.Geo(0, 0, 64); g.Element(0, 0)*g.Element(1, 1)-g.Element(0, 1)*g.Element(1, 0) >= 0 {
		t.Fatal("a west pose mirrors")
	}
}

// Status markers (hostile chevron, draft icon, fight bolt) float a fixed
// height above the head centre read from the body metadata, so a
// re-rendered, shorter figure brings them down with it.
func TestMarkersFloatAboveHead(t *testing.T) {
	t.Parallel()
	c := &artCatalog{Sprites: map[string]*artSprite{
		"pawn/body_south": fakeSprite([2]int{160, 288}, pt{16, 144}, map[string]pt{"head": {80, 118.8}}),
	}}
	_, hy := pawnHead(c)
	if lift := pawnMarkerLift(c); math.Abs(lift-(markerAboveHead-hy)) > 1e-9 || lift <= -hy {
		t.Fatalf("marker lift %v over a head at %v", lift, hy)
	}
	taller := &artCatalog{Sprites: map[string]*artSprite{
		"pawn/body_south": fakeSprite([2]int{160, 297}, pt{16, 153}, map[string]pt{"head": {80, 90}}),
	}}
	if pawnMarkerLift(taller) <= pawnMarkerLift(c) {
		t.Fatal("a higher head lifts the markers")
	}
	// the rescuer's head stays in front of a slung patient (drawn first)
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	res := game.PawnView{ID: 1, X: 2, Y: 2}
	pat := game.PawnView{ID: 2, X: 2, Y: 2, CarriedBy: 1, Look: look.Look{HairStyle: 3}}
	r.art.Sprites["pawn/hair_long_south"] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
	r.view.Pawns = []game.PawnView{res, pat}
	for _, c := range []struct {
		f      facing
		before bool
	}{{faceEast, true}, {faceSouth, true}, {faceNorth, false}} {
		keys := drawnKeys(r, res, c.f)
		patient := slices.Index(keys, "pawn/hair_long_south") // only the patient has long hair
		hair := slices.IndexFunc(keys, func(k string) bool { return strings.HasPrefix(k, "pawn/hair_short_") })
		if patient < 0 || hair < 0 || (patient < hair) != c.before {
			t.Fatalf("%v: patient at %d, rescuer's hair at %d: %v", c.f, patient, hair, keys)
		}
	}
}
