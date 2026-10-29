package ebiten

import (
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
)

// scarArt is figureArt plus the face scar sprites.
func scarArt() *artpkg.Catalog {
	c := figureArt()
	for f := artpkg.FaceSouth; f <= artpkg.FaceWest; f++ {
		c.Sprites["pawn/face_scar_"+f.String()] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
	}
	return c
}

// figureKeys draws p's standing figure and lists the sprite keys in draw order.
func figureKeys(r *Renderer, p game.PawnView) []string {
	var keys []string
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
	defer func() { r.lit.OnDraw = nil }()
	r.view.Pawns = []game.PawnView{p}
	r.pipe().Creatures.DrawPawnFigure(r.liveView(), r.pipe().Pass(), ebiten.NewImage(512, 512), p, 2, 2, 64, r.pipe().Creatures.FigurePoseOf(r.liveView(), p))
	return keys
}

func TestFaceScarDrawnOverHairWhenDisfigured(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.art = scarArt()
	p := game.PawnView{ID: 1, X: 2, Y: 2, Health: 100, Status: "alive"}
	for _, k := range figureKeys(r, p) {
		if strings.HasPrefix(k, "pawn/face_scar_") {
			t.Fatalf("an unscarred pawn drew %s", k)
		}
	}
	p.Disfigured = true
	keys := figureKeys(r, p)
	scar := slices.IndexFunc(keys, func(k string) bool { return strings.HasPrefix(k, "pawn/face_scar_") })
	hair := slices.IndexFunc(keys, func(k string) bool { return strings.HasPrefix(k, "pawn/hair_") })
	if scar < 0 || hair < 0 || scar < hair {
		t.Fatalf("scar at %d, hair at %d: %v", scar, hair, keys)
	}
	// no scar art: nothing breaks, nothing drawn for it
	r.art = figureArt()
	for _, k := range figureKeys(r, p) {
		if strings.HasPrefix(k, "pawn/face_scar_") {
			t.Fatalf("drew %s without the art", k)
		}
	}
}

func TestPortraitFaceScar(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	l := look.Look{Sex: look.Male, Skin: 1, Hair: 1, HairStyle: 2}
	has := func(ls []portraitLayer) int {
		return slices.IndexFunc(ls, func(ly portraitLayer) bool { return ly.Key == "pawn/face_scar_south" })
	}
	if has(portraitLayers(art, l)) >= 0 {
		t.Fatal("an unscarred portrait has the scar")
	}
	ls := scarredLayers(art, l, true)
	i := has(ls)
	hair := slices.IndexFunc(ls, func(ly portraitLayer) bool { return strings.HasPrefix(ly.Key, "pawn/hair_") })
	if i < 0 || i < hair {
		t.Fatalf("scar at %d, hair at %d", i, hair)
	}
	if !art.Has("pawn/face_scar_south") {
		t.Fatal("no generated face scar art")
	}
	dst := ebiten.NewImage(200, 240)
	if !drawPortrait(dst, art, l, true, 20, 20, 160, 200) {
		t.Fatal("nothing drawn")
	}
}

// The colonist bar / HUD portrait shows a disfigured pawn's scar over the
// hair; an unscarred one, or missing art, shows none.
func TestPawnPortraitArtFaceScar(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.art = scarArt()
	keys := func(p game.PawnView) []string {
		var out []string
		for _, ly := range pawnFigureLayers(r.art, p, "south") {
			out = append(out, ly.Key)
		}
		return out
	}
	p := game.PawnView{ID: 1, Health: 100, Status: "alive"}
	if slices.Contains(keys(p), faceScarSouth) {
		t.Fatal("an unscarred portrait has the scar")
	}
	p.Disfigured = true
	ks := keys(p)
	scar := slices.Index(ks, faceScarSouth)
	hair := slices.IndexFunc(ks, func(k string) bool { return strings.HasPrefix(k, "pawn/hair_") })
	if scar < 0 || hair < 0 || scar != hair+1 {
		t.Fatalf("scar at %d, hair at %d: %v", scar, hair, ks)
	}
	if !r.drawPawnPortraitArt(ebiten.NewImage(80, 100), p, 0, 0, 80, 100) {
		t.Fatal("portrait not drawn")
	}
	r.art = figureArt()
	if slices.Contains(keys(p), faceScarSouth) {
		t.Fatal("scar layer without the art")
	}
}

// A figure drawn in any facing keeps its scar over the hair; facing west
// it is the east scar, mirrored.
func TestFallingFigureFaceScar(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.art = scarArt()
	p := game.PawnView{ID: 1, X: 2, Y: 2, Health: 0, Status: "alive", Lying: true, Disfigured: true}
	var keys []string
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
	r.pipe().Creatures.DrawFigureIn(r.liveView(), r.pipe().Pass(), ebiten.NewImage(512, 512), p, artpkg.FaceWest, 2, 2, 64, creature.StandPose())
	r.lit.OnDraw = nil
	scar := slices.IndexFunc(keys, func(k string) bool { return k == "pawn/face_scar_east" })
	hair := slices.IndexFunc(keys, func(k string) bool { return strings.HasPrefix(k, "pawn/hair_") })
	if scar < 0 || hair < 0 || scar < hair {
		t.Fatalf("scar at %d, hair at %d: %v", scar, hair, keys)
	}
}

// drawPawnPortraitArt draws the layered pawn (facing south, the portrait
// crop of pawnCrops) fitted into the logical w x h box at (x, y), for HUD
// chrome (the inspector); it draws through the UI canvas, so the art
// scales with the UI like the names beside it (PLAY-017).
func (r *Renderer) drawPawnPortraitArt(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) bool {
	body := r.art.Sprite("pawn/body_south")
	if body == nil {
		return false
	}
	crop, _ := pawnCrops(body)
	drawPawnCropped(screen, r.art, p, body, crop, x, y, w, h)
	return true
}
