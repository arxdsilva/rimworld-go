package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// rectZone builds a rectangular ZoneView for tests.
func rectZone(id uint16, kind string, minX, minY, maxX, maxY int) game.ZoneView {
	z := game.ZoneView{ID: id, Kind: kind, MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			z.Cells = append(z.Cells, [2]int{x, y})
		}
	}
	return z
}

func zonesOfKind(w game.WorldView, kind string) []game.ZoneView {
	var out []game.ZoneView
	for _, z := range w.Zones {
		if z.Kind == kind {
			out = append(out, z)
		}
	}
	return out
}

func cellSet(cells ...[2]int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for _, c := range cells {
		m[c] = true
	}
	return m
}

func TestZoneEdgeMask(t *testing.T) {
	t.Parallel()
	z := rectZone(1, game.ZoneStockpile, 0, 0, 2, 2)
	set := cellSet(z.Cells...)
	cases := []struct {
		x, y, want int
	}{
		{1, 1, 0},
		{0, 0, zoneN | zoneW},
		{2, 0, zoneN | zoneE},
		{0, 2, zoneS | zoneW},
		{2, 2, zoneS | zoneE},
		{1, 0, zoneN},
	}
	for _, c := range cases {
		if got := zoneEdgeMask(set, c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) mask = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if got := zoneEdgeMask(cellSet([2]int{5, 5}), 5, 5); got != 15 {
		t.Fatalf("single tile mask = %d, want 15", got)
	}
}

func TestZoneEdgeRunsMergeConsecutiveCells(t *testing.T) {
	t.Parallel()
	z := rectZone(1, game.ZoneGrow, 2, 3, 4, 3) // a 3x1 strip
	runs := zoneEdgeRuns(cellSet(z.Cells...), z.Cells)
	if len(runs) != 4 {
		t.Fatalf("a strip has 4 outline runs, got %d: %+v", len(runs), runs)
	}
	long := 0
	for _, r := range runs {
		if r.y0 == r.y1 && r.x1-r.x0 == 3 {
			long++
		}
	}
	if long != 2 {
		t.Fatalf("north and south edges must each be one run of 3: %+v", runs)
	}
	// two touching zones' union has no seam between them
	a := rectZone(1, game.ZoneStockpile, 0, 0, 1, 1)
	b := rectZone(2, game.ZoneStockpile, 2, 0, 3, 1)
	union := append(append([][2]int{}, a.Cells...), b.Cells...)
	if got := len(zoneEdgeRuns(cellSet(union...), union)); got != 4 {
		t.Fatalf("merged 4x2 block must outline as 4 runs, got %d", got)
	}
}

func TestZoneEdgeRunsLShape(t *testing.T) {
	t.Parallel()
	cells := [][2]int{{0, 0}, {1, 0}, {0, 1}}
	runs := zoneEdgeRuns(cellSet(cells...), cells)
	if len(runs) != 6 {
		t.Fatalf("an L outlines as 6 runs, got %d: %+v", len(runs), runs)
	}
}

func TestZoneGhostCellsColourPerCell(t *testing.T) {
	t.Parallel()
	can := func(x, y int) bool { return x != 1 }
	got := zoneGhostCells(2, 1, 0, 0, [4]int{-5, -5, 50, 50}, can) // dragged backwards
	if len(got) != 6 {
		t.Fatalf("3x2 drag = %d cells", len(got))
	}
	for _, c := range got {
		if c.ok != (c.x != 1) {
			t.Errorf("cell %+v: ok must follow CanZoneAt", c)
		}
	}
	if clipped := zoneGhostCells(0, 0, 100, 100, [4]int{10, 10, 12, 11}, can); len(clipped) != 6 {
		t.Fatalf("the ghost clips to the viewport, got %d", len(clipped))
	}
	if anyZoneCell(0, 0, 2, 2, func(x, y int) bool { return false }) {
		t.Fatal("no free cell expected")
	}
}

func TestZoneToolsMapToKinds(t *testing.T) {
	t.Parallel()
	want := map[string]string{"stockpile": game.ZoneStockpile, "grow": game.ZoneGrow, "mine": ""}
	for id, kind := range want {
		tool, ok := toolByID(id)
		if !ok || zoneKindOf(tool) != kind {
			t.Errorf("tool %s kind = %q, want %q", id, zoneKindOf(tool), kind)
		}
	}
}

func TestSecondZoneOverClaimedTilesHints(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.stockpile")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 8, 7
	a.submitArchitect()
	g.Step(1)
	a.hint = ""
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 8, 7
	a.submitArchitect()
	if a.hint != "No free tiles there" {
		t.Fatalf("hint = %q", a.hint)
	}
}

func TestZoneLabelOncePerZone(t *testing.T) {
	t.Parallel()
	z := rectZone(3, game.ZoneGrow, 4, 4, 6, 5)
	z.Crop = "potato"
	if zoneLabelText(z) != "Potato" || zoneLabelText(game.ZoneView{Kind: game.ZonePen}) != "Pen" {
		t.Fatal("label text")
	}
	g := (&Renderer{}).zoneGeomOf(z)
	if len(g.cells) != 6 {
		t.Fatalf("geom cells = %d", len(g.cells))
	}
}

// TestZoneDrawsOverFloor: floors are opaque, so they must be drawn before
// the zones laid on them. Pixels can't be read outside the game loop, so
// the test checks the layer order of a frame with a floor and a zone.
func TestZoneDrawsOverFloor(t *testing.T) {
	art := testArt(t)
	g := game.NewColony(game.Config{Seed: 7, Width: 40, Height: 30})
	r := New(g)
	r.art = art
	var order []string
	r.layerTrace = func(n string) { order = append(order, n) }
	screen := ebiten.NewImage(640, 400)
	tile := r.tilePx()
	r.setView(640, 400, tile)
	r.drawArt(screen, 640, 400, tile)
	// other traced layers (tree trunks and canopies, PLAY-010) may come
	// and go; floors and zones each appear once, floors first
	var fz []string
	for _, n := range order {
		if n == "floors" || n == "zones" {
			fz = append(fz, n)
		}
	}
	if len(fz) != 2 || fz[0] != "floors" || fz[1] != "zones" {
		t.Fatalf("layer order = %v, want floors then zones", order)
	}
}

// --- zone labels (PLAY-017 sections 2-3) ---------------------------------
//
// The layout tests run at several UI scales and change curUIScale, so none
// of them runs in parallel.

func TestZoneLabelAnchor(t *testing.T) {
	t.Parallel()
	lShape := [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}}
	for _, c := range []struct {
		name           string
		cells          [][2]int
		ax, ay         float64
		minX, maxX, my int
	}{
		{"odd rectangle: its centre cell's centre", rectZone(1, game.ZoneGrow, 4, 4, 6, 6).Cells, 5.5, 5.5, 4, 6, 6},
		{"even rectangle: its exact centre", rectZone(1, game.ZoneGrow, 4, 4, 7, 5).Cells, 6, 5, 4, 7, 5},
		{"a single cell", [][2]int{{9, 3}}, 9.5, 3.5, 9, 9, 3},
		// the bounding box centre (2.5, 2.5) is outside the L; the label
		// goes on the nearest cell, the first of the tied ones
		{"L shape: the cell nearest the bounding box centre", lShape, 2.5, 0.5, 0, 4, 4},
	} {
		minX, _, maxX, maxY, ax, ay := zoneLabelAnchor(c.cells)
		if ax != c.ax || ay != c.ay || minX != c.minX || maxX != c.maxX || maxY != c.my {
			t.Errorf("%s: anchor (%g,%g) box x %d..%d maxY %d, want (%g,%g) x %d..%d maxY %d",
				c.name, ax, ay, minX, maxX, maxY, c.ax, c.ay, c.minX, c.maxX, c.my)
		}
	}
	// the anchor of a non-rectangular zone is always inside the zone
	set := cellSet(lShape...)
	_, _, _, _, ax, ay := zoneLabelAnchor(lShape)
	if !set[[2]int{int(math.Floor(ax)), int(math.Floor(ay))}] {
		t.Errorf("L shape anchor (%g,%g) is outside the zone", ax, ay)
	}
}

func TestZoneLabelSizeGrowsWithZone(t *testing.T) {
	for _, ui := range []float64{1, 1.5, 2} {
		withUIScale(t, ui)
		prev, first := 0.0, 0.0
		for side := 40.0; side <= 500; side += 10 {
			p := zoneLabelLayout("Pen", side, side, side/2, side/2, zoneLabelMax)
			if !p.ok {
				continue
			}
			sz := p.style.Size
			if sz < 11 || sz > 40 || math.Mod(sz*2, 1) != 0 {
				t.Fatalf("ui %g side %g: size %g outside 11..40 or not a half step", ui, side, sz)
			}
			if sz < prev {
				t.Fatalf("ui %g side %g: size %g shrank from %g as the zone grew", ui, side, sz, prev)
			}
			if first == 0 {
				first = sz
			}
			prev = sz
		}
		if first == 0 || first >= 20 || prev != 40 {
			t.Errorf("ui %g: sizes ran from %g to %g, want a small label growing to the 40 px cap", ui, first, prev)
		}
	}
}

func TestZoneLabelFitsTheZoneWidth(t *testing.T) {
	for _, ui := range []float64{1, 1.5, 2} {
		withUIScale(t, ui)
		for _, text := range []string{"Pen", "Stockpile", "Potato", "Strawberries"} {
			for w := 24.0; w <= 600; w += 6 {
				for _, h := range []float64{30, 80, 400} {
					p := zoneLabelLayout(text, w, h, w/2, h/2, zoneLabelMax)
					if !p.ok {
						continue
					}
					if p.plate.W > float32(0.9*w)+0.01 {
						t.Fatalf("ui %g %q in %gx%g: plate %g wide, over 90%% of the zone", ui, text, w, h, p.plate.W)
					}
					if p.plate.X < 0 || p.plate.Y < 0 || p.plate.X+p.plate.W > float32(w) || p.plate.Y+p.plate.H > float32(h) {
						t.Fatalf("ui %g %q in %gx%g: plate %+v sticks out of a zone it is centred in", ui, text, w, h, p.plate)
					}
					if tw := textW(p.text, p.style); tw > p.plate.W {
						t.Fatalf("ui %g %q in %gx%g: text %g wide on a plate %g wide", ui, text, w, h, tw, p.plate.W)
					}
					// ellipsized only at the 11 px floor
					if p.text != text && p.style.Size != 11 {
						t.Fatalf("ui %g %q in %gx%g: %q ellipsized at %g px, only 11 px may be", ui, text, w, h, p.text, p.style.Size)
					}
					// shrunk only as far as needed: half a step more would not fit
					want := math.Round(min(40, max(11, 0.45*min(w, h)))*2) / 2
					if p.style.Size < want {
						px, _ := zoneLabelPad(p.style.Size + 0.5)
						up := p.style
						up.Size += 0.5
						if tw := float64(textW(text, up)); tw+2*px <= 0.9*w {
							t.Fatalf("ui %g %q in %gx%g: shrunk to %g although %g fits", ui, text, w, h, p.style.Size, up.Size)
						}
					}
				}
			}
		}
	}
}

func TestZoneLabelEllipsizesThenHides(t *testing.T) {
	withUIScale(t, 1.5)
	// wide enough for the whole word (shrunk to fit)
	wide := zoneLabelLayout("Stockpile", 110, 200, 55, 100, zoneLabelMax)
	if !wide.ok || wide.text != "Stockpile" {
		t.Fatalf("110 px wide: %+v", wide)
	}
	// too narrow for the word at 11 px: ellipsized at 11 px, still shown
	narrow := zoneLabelLayout("Stockpile", 56, 200, 28, 100, zoneLabelMax)
	if !narrow.ok || narrow.style.Size != 11 || narrow.text == "Stockpile" || narrow.text[len(narrow.text)-len("…"):] != "…" {
		t.Fatalf("56 px wide: %+v", narrow)
	}
	// narrower than about two letters: hidden
	if p := zoneLabelLayout("Stockpile", 16, 200, 8, 100, zoneLabelMax); p.ok {
		t.Fatalf("16 px wide must hide the label, got %+v", p)
	}
	// 11 px text cannot fit a zone shorter than its line
	if p := zoneLabelLayout("Pen", 300, 10, 150, 5, zoneLabelMax); p.ok {
		t.Fatalf("a 10 px tall zone must hide the label, got %+v", p)
	}
	if p := zoneLabelLayout("", 300, 100, 150, 50, zoneLabelMax); p.ok {
		t.Fatal("no text, no label")
	}
}

// zlFrame draws z's label once on a renderer whose camera shows the zone
// centre at the screen centre plus pan, at the given scales and zoom. It
// returns the renderer and the events drawn.
func zlFrame(t *testing.T, z game.ZoneView, sc wlScale, zoom float64, pan [2]float64) (*Renderer, []traceEvent) {
	t.Helper()
	wlSetScale(t, sc)
	got := wlTrace(t)
	r := New(&stubSim{w: 64, h: 64})
	r.cam.zoom, r.cam.dev = zoom, sc.dev
	tile := r.tilePx()
	sw, sh := int(1280*sc.dev), int(720*sc.dev)
	cx := float64(z.MinX+z.MaxX+1) / 2
	cy := float64(z.MinY+z.MaxY+1) / 2
	r.cam.offsetX = cx*tile - float64(sw)/2 + pan[0]
	r.cam.offsetY = cy*tile - float64(sh)/2 + pan[1]
	r.drawZoneLabel(ebiten.NewImage(64, 64), z, r.zoneGeomOf(z), sw, sh, tile)
	return r, *got
}

// zlPhysRect is the zone's bounding box on screen, in physical pixels.
func zlPhysRect(r *Renderer, z game.ZoneView) (x0, y0, x1, y1 float64) {
	ax, ay := r.worldToScreenF(float64(z.MinX), float64(z.MinY))
	bx, by := r.worldToScreenF(float64(z.MaxX+1), float64(z.MaxY+1))
	return float64(ax), float64(ay), float64(bx), float64(by)
}

var zlScales = []wlScale{{"ui1", 1, 1}, {"ui1.5", 1, 1.5}, {"ui2", 1, 2}, {"retina ui1.5", 2, 1.5}}

func TestZoneLabelStaysInsideTheZoneAtScale(t *testing.T) {
	for _, kind := range []string{game.ZoneStockpile, game.ZoneGrow, game.ZonePen} {
		z := rectZone(4, kind, 10, 8, 14, 12) // 5x5 tiles
		for _, sc := range zlScales {
			for _, zoom := range []float64{0.5, 1, 2} {
				var first *traceEvent
				var firstPan [2]float64
				for _, pan := range [][2]float64{{0, 0}, {137.5, -61}, {-90, 40.25}} {
					r, evs := zlFrame(t, z, sc, zoom, pan)
					fills, texts := wlOfKind(evs, "fill"), wlOfKind(evs, "text")
					if len(fills) != 1 || len(texts) != 1 {
						t.Fatalf("%s %s zoom %g pan %v: %d plates and %d texts, want one each", kind, sc.name, zoom, pan, len(fills), len(texts))
					}
					plate, txt := fills[0], texts[0]
					if txt.Text != zoneLabelText(z) {
						t.Errorf("%s %s zoom %g: text %q, want %q", kind, sc.name, zoom, txt.Text, zoneLabelText(z))
					}
					if txt.Via != "worldText" {
						t.Errorf("%s %s zoom %g: text drawn via %q, want worldText", kind, sc.name, zoom, txt.Via)
					}
					x0, y0, x1, y1 := zlPhysRect(r, z)
					in := func(what string, e traceEvent, tol float64) {
						if float64(e.X) < x0-tol || float64(e.Y) < y0-tol || float64(e.X+e.W) > x1+tol || float64(e.Y+e.H) > y1+tol {
							t.Errorf("%s %s zoom %g pan %v: %s %+v outside the zone (%g,%g)-(%g,%g)", kind, sc.name, zoom, pan, what, e, x0, y0, x1, y1)
						}
					}
					in("plate", plate, 0.01)
					// the text sits on its plate (a pixel of rounding in the glyph box)
					if float64(txt.X) < float64(plate.X)-1.5 || float64(txt.X+txt.W) > float64(plate.X+plate.W)+1.5 ||
						float64(txt.Y) < float64(plate.Y)-1.5 || float64(txt.Y+txt.H) > float64(plate.Y+plate.H)+1.5 {
						t.Errorf("%s %s zoom %g pan %v: text %+v off its plate %+v", kind, sc.name, zoom, pan, txt, plate)
					}
					// centred on the zone's centre
					wlNear(t, "plate centre x", float64(plate.X+plate.W/2), (x0+x1)/2, 1)
					wlNear(t, "plate centre y", float64(plate.Y+plate.H/2), (y0+y1)/2, 1)
					// panning the camera moves the label with the zone, no more
					// and no less: the plate shifts by exactly the pan
					if first == nil {
						first, firstPan = &plate, pan
						continue
					}
					wlNear(t, "plate x shift", float64(plate.X-first.X), -(pan[0] - firstPan[0]), 0.5)
					wlNear(t, "plate y shift", float64(plate.Y-first.Y), -(pan[1] - firstPan[1]), 0.5)
				}
			}
		}
	}
}

func TestZoneLabelGrowsWithZoom(t *testing.T) {
	z := rectZone(4, game.ZoneGrow, 10, 8, 17, 9) // 8x2 tiles
	z.Crop = "rice"
	for _, sc := range zlScales {
		prev := 0.0
		// zoomed out, zoom in so the zone's short side is 26, 32, 38
		// logical px: label sizes 11.5, 14.5, 17, clear of the 11 px floor
		// and of the zoom cap (zoneLabelCap; tiles 13..19 px allow 33+)
		for _, side := range []float64{26, 32, 38} {
			zoom := side * sc.ui / (2 * TileSize)
			_, evs := zlFrame(t, z, sc, zoom, [2]float64{})
			texts := wlOfKind(evs, "text")
			if len(texts) != 1 {
				t.Fatalf("%s zoom %g: %d texts", sc.name, zoom, len(texts))
			}
			if h := float64(texts[0].H); h <= prev {
				t.Errorf("%s zoom %g: text %g tall, not bigger than %g at the zoom before", sc.name, zoom, h, prev)
			} else {
				prev = h
			}
		}
	}
}

func TestZoneLabelHiddenWhenTheZoneIsTooSmall(t *testing.T) {
	for _, sc := range zlScales {
		// a single tile far zoomed out is narrower than a letter pair
		_, evs := zlFrame(t, rectZone(4, game.ZoneStockpile, 10, 8, 10, 8), sc, 0.25, [2]float64{})
		if len(evs) != 0 {
			t.Errorf("%s: a tiny zone drew %+v, want no label", sc.name, evs)
		}
		// far off screen: nothing either
		_, evs = zlFrame(t, rectZone(4, game.ZoneStockpile, 10, 8, 14, 12), sc, 1, [2]float64{5000, 0})
		if len(evs) != 0 {
			t.Errorf("%s: an off-screen zone drew %+v", sc.name, evs)
		}
	}
}

func TestZoneLabelOfAnLShapeIsInsideIt(t *testing.T) {
	var cells [][2]int
	for x := 10; x <= 16; x++ { // the top arm
		cells = append(cells, [2]int{x, 8}, [2]int{x, 9})
	}
	for y := 10; y <= 16; y++ { // the left arm, two tiles wide
		cells = append(cells, [2]int{10, y}, [2]int{11, y})
	}
	z := game.ZoneView{ID: 5, Kind: game.ZoneGrow, Crop: "rice", Cells: cells, MinX: 10, MinY: 8, MaxX: 16, MaxY: 16}
	set := cellSet(cells...)
	for _, sc := range zlScales {
		for _, zoom := range []float64{1, 2} {
			r, evs := zlFrame(t, z, sc, zoom, [2]float64{})
			fills := wlOfKind(evs, "fill")
			if len(fills) != 1 {
				t.Fatalf("%s zoom %g: %d plates", sc.name, zoom, len(fills))
			}
			p := fills[0]
			// the plate centre, back in tiles, is on a zone cell
			tile := r.tilePx()
			tx := (float64(p.X+p.W/2) + r.cam.offsetX) / tile
			ty := (float64(p.Y+p.H/2) + r.cam.offsetY) / tile
			if !set[[2]int{int(math.Floor(tx)), int(math.Floor(ty))}] {
				t.Errorf("%s zoom %g: label centre (%.2f,%.2f) tiles is outside the L", sc.name, zoom, tx, ty)
			}
		}
	}
}

// TestZoneLabelDrawnOncePerZone: drawZoneLabels labels each zone once,
// however many tiles it has.
func TestZoneLabelDrawnOncePerZone(t *testing.T) {
	wlSetScale(t, wlScale{"ui1.5", 1, 1.5})
	got := wlTrace(t)
	r := New(&stubSim{w: 64, h: 64})
	r.cam.zoom = 1
	w := game.WorldView{Zones: []game.ZoneView{
		rectZone(1, game.ZoneStockpile, 2, 2, 6, 5),
		rectZone(2, game.ZoneGrow, 9, 2, 13, 6),
		rectZone(3, game.ZonePen, 2, 9, 6, 13),
	}}
	r.drawZoneLabels(ebiten.NewImage(64, 64), w, 1280, 720, r.tilePx())
	if texts := wlOfKind(*got, "text"); len(texts) != 3 {
		t.Fatalf("%d labels for 3 zones: %+v", len(texts), texts)
	}
	if plates := wlOfKind(*got, "fill"); len(plates) != 3 {
		t.Fatalf("%d plates for 3 zones", len(plates))
	}
}

// Zoomed in, a zone's label stays modest (it would bury the stacks and
// crops it names); zoomed out it may grow to the full 40 px, so a field's
// name reads from afar (PLAY-017).
func TestZoneLabelCapFollowsZoom(t *testing.T) {
	if c := zoneLabelCap(43); c != zoneLabelNear {
		t.Fatalf("close zoom (43 px tiles): cap %g, want %g", c, zoneLabelNear)
	}
	if c := zoneLabelCap(12); c != zoneLabelMax {
		t.Fatalf("far zoom (12 px tiles): cap %g, want %g", c, zoneLabelMax)
	}
	prev := zoneLabelMax + 1
	for tl := 8.0; tl <= 64; tl++ {
		c := zoneLabelCap(tl)
		if c > prev {
			t.Fatalf("cap grew from %g to %g as tiles grew to %g px", prev, c, tl)
		}
		prev = c
	}
	withUIScale(t, 1.5)
	big := zoneLabelLayout("Stockpile", 300, 200, 150, 100, zoneLabelCap(43))
	if !big.ok || big.style.Size > zoneLabelNear {
		t.Fatalf("a big zone up close: size %g, want at most %g", big.style.Size, zoneLabelNear)
	}
}

// Zone labels are drawn in their own pass after the objects, so stacks and
// crops never cover them; drawZones only fills and outlines.
func TestZoneLabelsDrawnAfterObjects(t *testing.T) {
	z := game.ZoneView{Kind: game.ZoneStockpile, MinX: 2, MinY: 2, MaxX: 9, MaxY: 7}
	for y := z.MinY; y <= z.MaxY; y++ {
		for x := z.MinX; x <= z.MaxX; x++ {
			z.Cells = append(z.Cells, [2]int{x, y})
		}
	}
	w := game.WorldView{Zones: []game.ZoneView{z}}
	sc := wlScale{name: "ui1", dev: 1, ui: 1}
	wlSetScale(t, sc)
	got := wlTrace(t)
	r := New(&stubSim{w: 64, h: 64})
	tile := r.tilePx()
	r.drawZones(ebiten.NewImage(64, 64), w, 1280, 720, tile)
	for _, e := range *got {
		if e.Kind == "text" {
			t.Fatalf("drawZones drew label text %q; labels belong to drawZoneLabels", e.Text)
		}
	}
	r.drawZoneLabels(ebiten.NewImage(64, 64), w, 1280, 720, tile)
	n := 0
	for _, e := range *got {
		if e.Kind == "text" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("drawZoneLabels drew %d labels, want 1", n)
	}
}

// --- PLAY-020: a growing zone's fill leaves its own plants uncovered -------

// fillQuads draws w's zones at zoom and counts the fill batch's quads.
func fillQuads(t *testing.T, art *artCatalog, w game.WorldView, zoom float64) int {
	t.Helper()
	r := New(&stubSim{w: 32, h: 32})
	r.art = art
	r.cam.zoom = zoom
	tile := r.tilePx()
	r.setView(640, 400, tile)
	r.drawZones(ebiten.NewImage(640, 400), w, 640, 400, tile)
	return len(r.lod.idx) / 6
}

// lineZone is a one-row zone of kind and crop over x0..x1 at y.
func lineZone(kind, crop string, x0, x1, y int) game.ZoneView {
	z := game.ZoneView{ID: 1, Kind: kind, Crop: crop, MinX: x0, MinY: y, MaxX: x1, MaxY: y}
	for x := x0; x <= x1; x++ {
		z.Cells = append(z.Cells, [2]int{x, y})
	}
	return z
}

// TestGrowZoneFillSkipsItsPlants: the potato zone's fill is not drawn on
// the cell holding a potato it grows (the furrows and the plant show
// there), at a live zoom and at the far zoom where plants are baked under
// the zones; wild grass in the zone and empty cells keep the fill, and a
// stockpile over a plant keeps every cell.
func TestGrowZoneFillSkipsItsPlants(t *testing.T) {
	art := testArt(t)
	plants := []game.PlantView{
		{X: 2, Y: 2, Species: "potato", Zone: "potato"},
		{X: 3, Y: 2, Species: "grass", Zone: "potato"},
	}
	for _, zoom := range []float64{1, 0.2} {
		w := game.WorldView{Zones: []game.ZoneView{lineZone(game.ZoneGrow, "potato", 2, 5, 2)}, Plants: plants}
		if n := fillQuads(t, art, w, zoom); n != 3 {
			t.Errorf("zoom %g: %d fill quads, want 3 (not under the potato)", zoom, n)
		}
		w.Zones = []game.ZoneView{lineZone(game.ZoneStockpile, "", 2, 5, 2)}
		if n := fillQuads(t, art, w, zoom); n != 4 {
			t.Errorf("zoom %g: stockpile %d fill quads, want all 4", zoom, n)
		}
	}
}

// TestTreeZoneFillSkipsItsTrees: a tree zone's fill is not drawn under the
// trees it grows.
func TestTreeZoneFillSkipsItsTrees(t *testing.T) {
	art := testArt(t)
	w := game.WorldView{
		Zones:  []game.ZoneView{lineZone(game.ZoneGrow, "tree", 2, 4, 2)},
		Plants: []game.PlantView{{X: 4, Y: 2, Species: "pine", Zone: "tree"}},
	}
	if n := fillQuads(t, art, w, 1); n != 2 {
		t.Fatalf("%d fill quads, want 2 (not under the pine)", n)
	}
}
