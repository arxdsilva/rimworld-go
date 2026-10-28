package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-020 ZONEUI: hidden and renamed zones on the map, and the selected
// zone's highlight (zones_draw.go, zone_select.go).

// zoneFillArt is a catalog with just the zone fill sprites, so the fill
// batch has something to count.
func zoneFillArt() *artCatalog {
	c := &artCatalog{Sprites: map[string]*artSprite{}, Tinted: map[tintKey]*ebiten.Image{}}
	for _, k := range []string{"overlay/grow_zone", "overlay/stockpile", "overlay/pen_zone"} {
		c.Sprites[k] = fakeSprite([2]int{128, 128}, pt{}, nil)
	}
	return c
}

func TestHiddenZoneDrawsNoFill(t *testing.T) {
	t.Parallel()
	art := zoneFillArt()
	// fillQuads counts the last zone's batch: the hidden one goes last.
	shown := lineZone(game.ZoneGrow, "potato", 2, 5, 2) // four tiles
	hidden := lineZone(game.ZoneStockpile, "", 2, 4, 6) // three tiles
	hidden.ID = 2
	w := game.WorldView{Zones: []game.ZoneView{shown, hidden}}
	if n := fillQuads(t, art, w, 1); n != 3 {
		t.Fatalf("%d fill quads, want the stockpile's 3 when it is last and shown", n)
	}
	w.Zones[1].Hidden = true
	for _, zoom := range []float64{1, 0.2} {
		if n := fillQuads(t, art, w, zoom); n != 4 {
			t.Fatalf("zoom %g: %d fill quads with the stockpile hidden, want the grow zone's 4", zoom, n)
		}
	}
	w.Zones = w.Zones[1:] // only the hidden zone
	if n := fillQuads(t, art, w, 1); n != 0 {
		t.Fatalf("%d fill quads for a hidden zone, want none", n)
	}
}

func TestHiddenZoneDrawsNoLabelAndARenamedOneShowsItsName(t *testing.T) {
	wlSetScale(t, wlScale{"ui1", 1, 1})
	got := wlTrace(t)
	r := New(&stubSim{w: 64, h: 64})
	r.cam.Zoom = 1
	named := rectZone(1, game.ZoneGrow, 2, 2, 8, 6)
	named.Crop, named.Name = "potato", "North field"
	hidden := rectZone(2, game.ZoneStockpile, 12, 2, 18, 6)
	hidden.Hidden = true
	plain := rectZone(3, game.ZonePen, 2, 8, 8, 10)
	r.drawZoneLabels(ebiten.NewImage(64, 64), game.WorldView{Zones: []game.ZoneView{named, hidden, plain}}, 1280, 720, r.tilePx())
	var texts []string
	for _, e := range wlOfKind(*got, "text") {
		texts = append(texts, e.Text)
	}
	if len(texts) != 2 || texts[0] != "North field" || texts[1] != "Pen" {
		t.Fatalf("labels %q, want the custom name and the pen's, no hidden stockpile", texts)
	}
}

func TestZoneLabelTextPrefersTheName(t *testing.T) {
	t.Parallel()
	z := game.ZoneView{Kind: game.ZoneGrow, Crop: "wheat"}
	if got := zoneLabelText(z); got != "Wheat" {
		t.Fatalf("unnamed label %q", got)
	}
	z.Name = "Back forty"
	if got := zoneLabelText(z); got != "Back forty" {
		t.Fatalf("named label %q", got)
	}
}

// A selected hidden zone shows its outline (the highlight) and its name;
// a selected shown zone gets the highlight only (the map pass draws its
// name).
func TestSelectedHiddenZoneShowsItsNameAndAShownOneDoesNot(t *testing.T) {
	z := growZone(1, "potato", 8, 1, 15, 5)
	z.Name = "Hidden plot"
	z.Hidden = true
	a, _, f := zoneApp(t, z)
	a.selectCand(selCand{selZone, entity.EntityID(1)})
	f.Frame(t, a)
	got := wlTrace(t)
	a.drawZoneSelection(ebiten.NewImage(64, 64))
	if texts := wlOfKind(*got, "text"); len(texts) != 1 || texts[0].Text != "Hidden plot" {
		t.Fatalf("labels drawn for a selected hidden zone: %+v, want its name once", texts)
	}
	// Shown: the world pass names it; the highlight adds no label.
	s := a.renderer.game.(*zoneSim)
	s.world.Zones[0].Hidden = false
	*got = (*got)[:0]
	a.drawZoneSelection(ebiten.NewImage(64, 64))
	if texts := wlOfKind(*got, "text"); len(texts) != 0 {
		t.Fatalf("a shown zone's highlight drew text %+v", texts)
	}
	// Nothing selected, nothing drawn.
	a.clearZone()
	a.drawZoneSelection(ebiten.NewImage(64, 64))
}

func TestHiddenZoneLabelIsNotClickableUnlessSelected(t *testing.T) {
	t.Parallel()
	z := growZone(1, "potato", 8, 1, 15, 5)
	a, s, _ := zoneApp(t, z)
	r := a.renderer
	plan, _, _, _, _ := r.zoneLabelPlanFor(z, r.zoneGeomOf(z), r.tilePx())
	cx, cy := plan.plate.Center()
	px, py := int(cx*float32(uiScale())), int(cy*float32(uiScale()))
	if got, ok := r.zoneLabelAt(r.world(), px, py, r.tilePx(), 0); !ok || got.ID != 1 {
		t.Fatal("a shown zone's label plate is clickable")
	}
	s.world.Zones[0].Hidden = true
	if _, ok := r.zoneLabelAt(r.world(), px, py, r.tilePx(), 0); ok {
		t.Fatal("a hidden zone has no plate to click")
	}
	if got, ok := r.zoneLabelAt(r.world(), px, py, r.tilePx(), 1); !ok || got.ID != 1 {
		t.Fatal("the selected hidden zone shows its plate")
	}
	if _, ok := r.zoneLabelAt(r.world(), px-400, py, r.tilePx(), 0); ok {
		t.Fatal("a point off the plate hits nothing")
	}
}
