package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-017: world-anchored icons (chat bubble, fight marker, drafted flag)
// are placed with physical camera coordinates. drawIcon takes logical
// pixels and scales them again, so at UI scale > 1 they drifted off the
// pawn. These tests draw them at several UI and device scales and check
// the icon lands on the pawn, at the same physical place for every scale.
// They change package globals (curUIScale, drawTrace) and must not run in
// parallel.

// wlScale is one UI/device scale combination.
type wlScale struct {
	name    string
	dev, ui float64
}

var wlScales = []wlScale{
	{"ui1", 1, 1},
	{"ui1.5", 1, 1.5},
	{"ui2", 1, 2},
	{"retina ui1.5", 2, 1.5},
}

// wlSetScale applies sc for the rest of the test.
func wlSetScale(t *testing.T, sc wlScale) {
	t.Helper()
	old := gfx.CurrentScale()
	gfx.SetScale(uiScaleState{Dev: sc.dev, UI: sc.ui, S: sc.dev * sc.ui})
	t.Cleanup(func() { gfx.SetScale(old) })
}

// wlTrace records every canvas draw (including text) for the test.
func wlTrace(t *testing.T) *[]traceEvent {
	t.Helper()
	var got []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { got = append(got, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	return &got
}

// wlOfKind filters events by kind.
func wlOfKind(evs []traceEvent, kind string) []traceEvent {
	var out []traceEvent
	for _, e := range evs {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// wlArt is a fake catalog holding just the icons these labels use.
func wlArt() *artCatalog {
	c := figureArt()
	for _, k := range []string{"ui_icon/draft", "overlay/fight_lightning", "ui_icon/social_chitchat"} {
		c.Sprites[k] = fakeSprite([2]int{128, 128}, pt{0, 0}, nil)
	}
	return c
}

// wlRenderer is a renderer with the camera panned to (offX, offY) at zoom
// z, drawing pawns.
func wlRenderer(sc wlScale, zoom, offX, offY float64, pawns ...game.PawnView) *Renderer {
	r := New(&stubSim{w: 32, h: 32, tick: 100})
	r.art = wlArt()
	r.cam.Zoom, r.cam.OffsetX, r.cam.OffsetY, r.cam.Dev = zoom, offX, offY, sc.dev
	r.view.Pawns = pawns
	return r
}

// wlPans are camera positions in physical pixels (the zoom 1 tile is 64).
var wlPans = [][2]float64{{0, 0}, {137.5, 61}, {-90, 210.25}}

func wlNear(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.2f, want %.2f (+-%.2f)", what, got, want, tol)
	}
}

func TestChatIconSitsOnPawnAtScale(t *testing.T) {
	for _, sc := range wlScales {
		for _, zoom := range []float64{0.75, 1.5} {
			for _, pan := range wlPans {
				wlSetScale(t, sc)
				got := wlTrace(t)
				p := game.PawnView{ID: 7, X: 6, Y: 5, Interaction: "chitchat", InteractTick: 90}
				r := wlRenderer(sc, zoom, pan[0], pan[1], p)
				tile := r.tilePx()
				r.drawChatBubbles(ebiten.NewImage(64, 64), tile)

				imgs := wlOfKind(*got, "image")
				if len(imgs) != 1 {
					t.Fatalf("%s zoom %g: %d images drawn, want the one chat icon: %+v", sc.name, zoom, len(imgs), imgs)
				}
				icon := imgs[0]
				if icon.Via != "worldIcon" {
					t.Errorf("%s: chat icon drawn via %q, want worldIcon", sc.name, icon.Via)
				}
				// the bubble sits up and right of the pawn, beside its head
				// (body metadata); the icon fills it
				_, hy := pawnHead(r.art)
				bx, by := r.worldToScreenF(6+0.75, 5+hy-chatAboveHead)
				wlNear(t, sc.name+" icon centre x", float64(icon.X+icon.W/2), float64(bx), 1)
				wlNear(t, sc.name+" icon centre y", float64(icon.Y+icon.H/2), float64(by), 1)
				wlNear(t, sc.name+" icon size", float64(icon.W), tile*0.42-4, 1)
			}
		}
	}
}

func TestFightMarkerSitsOnPawnAtScale(t *testing.T) {
	for _, sc := range wlScales {
		for _, zoom := range []float64{0.75, 1.5} {
			for _, pan := range wlPans {
				wlSetScale(t, sc)
				got := wlTrace(t)
				p := game.PawnView{ID: 7, X: 6, Y: 5, SocialFight: true}
				r := wlRenderer(sc, zoom, pan[0], pan[1], p)
				tile := r.tilePx()
				r.drawChatBubbles(ebiten.NewImage(64, 64), tile)

				imgs := wlOfKind(*got, "image")
				if len(imgs) != 1 {
					t.Fatalf("%s zoom %g: %d images drawn, want the one fight marker: %+v", sc.name, zoom, len(imgs), imgs)
				}
				icon := imgs[0]
				if icon.Via != "worldIcon" {
					t.Errorf("%s: fight marker drawn via %q, want worldIcon", sc.name, icon.Via)
				}
				mx, my := r.worldToScreenF(6+0.5, 5-pawnMarkerLift(r.art))
				wlNear(t, sc.name+" marker centre x", float64(icon.X+icon.W/2), float64(mx), 1)
				wlNear(t, sc.name+" marker centre y", float64(icon.Y+icon.H/2), float64(my), 1)
				wlNear(t, sc.name+" marker size", float64(icon.W), tile*0.32, 1)
			}
		}
	}
}

func TestDraftedIconSitsOnPawnAtScale(t *testing.T) {
	for _, sc := range wlScales {
		for _, zoom := range []float64{0.75, 1.5} {
			for _, pan := range wlPans {
				wlSetScale(t, sc)
				got := wlTrace(t)
				p := game.PawnView{ID: 3, X: 6, Y: 5, Drafted: true}
				r := wlRenderer(sc, zoom, pan[0], pan[1], p)
				tile := r.tilePx()
				r.drawPawnArt(ebiten.NewImage(64, 64), p, 6, 5, tile, 64, 64)

				imgs := wlOfKind(*got, "image")
				if len(imgs) != 1 {
					t.Fatalf("%s zoom %g: %d images drawn, want the one draft icon: %+v", sc.name, zoom, len(imgs), imgs)
				}
				icon := imgs[0]
				if icon.Via != "worldIcon" {
					t.Errorf("%s: draft icon drawn via %q, want worldIcon", sc.name, icon.Via)
				}
				// above the pawn's head (body metadata), right of centre
				// (pose offsets are a few hundredths of a tile at rest)
				ix, iy := r.worldToScreenF(6+0.62, 5-pawnMarkerLift(r.art))
				wlNear(t, sc.name+" icon x", float64(icon.X), float64(ix), 0.1*tile+1)
				wlNear(t, sc.name+" icon y", float64(icon.Y), float64(iy), 0.1*tile+1)
				wlNear(t, sc.name+" icon size", float64(icon.W), tile*0.3, 1)
			}
		}
	}
}

// TestWorldIconsAreScaleInvariant: a world-anchored icon covers the same
// physical pixels at every UI scale (the UI scale never zooms the map).
func TestWorldIconsAreScaleInvariant(t *testing.T) {
	var ref traceEvent
	for i, sc := range wlScales[:3] { // same device density, three UI scales
		wlSetScale(t, sc)
		got := wlTrace(t)
		p := game.PawnView{ID: 7, X: 6, Y: 5, SocialFight: true}
		r := wlRenderer(sc, 1, 40, 30, p)
		r.drawChatBubbles(ebiten.NewImage(64, 64), r.tilePx())
		imgs := wlOfKind(*got, "image")
		if len(imgs) != 1 {
			t.Fatalf("%s: %d images", sc.name, len(imgs))
		}
		if i == 0 {
			ref = imgs[0]
			continue
		}
		e := imgs[0]
		for _, d := range [][3]float64{{float64(e.X), float64(ref.X), 0.5}, {float64(e.Y), float64(ref.Y), 0.5}, {float64(e.W), float64(ref.W), 0.5}} {
			wlNear(t, sc.name+" vs ui1", d[0], d[1], d[2])
		}
	}
}
