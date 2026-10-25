package ebiten

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// PLAY-017 §12: frame invariance. Two bugs shipped because nothing looked
// at where things were drawn: zone labels drifted off their zone as the
// camera moved (a world pass passed physical positions to drawText, which
// takes logical ones) and the colonist-bar portraits floated mid-screen (a
// helper drew logical positions raw). These tests draw full App frames of
// a real colony at every UI scale x device scale x zoom and check, from
// the canvas's drawTrace (ui_canvas.go), that:
//
//   - Pan: moving the camera by whole tiles leaves every HUD draw where it
//     was and moves every world draw by exactly -delta (physical pixels);
//   - Scale: HUD draws at scale s are the scale-1 draws x s;
//   - Anchors: labels sit where they belong (zone names in their zone, the
//     animal's name under it, portraits in their bar slot, stack counts on
//     their stack, the draft/chat/fight markers over their colonist);
//   - Panels: every inspector, tab, the options screen and a letter stay
//     inside the screen and their own panel, and scale like the rest.
//
// Raw draws (vector, DrawImage) are invisible to the trace; the AST guard
// (ui_canvas_guard_test.go) covers them.

// Matrix of the frame tests: device density x UI scale x apparent zoom.
// 0.8 is the smallest second zoom every scale allows (the 32x32 map's
// zoom-out limit is 0.75 at UI scale 2) whose tile (51.2 px) still shows
// zone names (they need 40).
var (
	visDevs  = []float64{1, 2}
	visUIs   = []float64{1, 1.5, 2}
	visZooms = []float64{1, 0.8}
)

const (
	draftIconSize = 18 // hud.go drawColonistPortraits: logical size of the draft icon

	visPanX = 2 // the pan test moves the camera right 2 tiles ...
	visPanY = 1 // ... and down 1: the world moves left 2, up 1
	posTol  = 1 // pixels: pan tolerance
)

func visName(dev, ui, zoom float64) string {
	return fmt.Sprintf("dev%g_ui%g_zoom%g", dev, ui, zoom)
}

// visCheckPan compares a frame before (a) and after (b) the camera moved by
// (dx, dy) tiles of tile physical pixels.
func visCheckPan(a, b []traceEvent, dx, dy int, tile float64) []string {
	var bad []string
	ha, hb := visPass(a, "hud"), visPass(b, "hud")
	if len(ha) != len(hb) {
		bad = append(bad, fmt.Sprintf("hud: %d draws before the pan, %d after", len(ha), len(hb)))
	} else {
		for i := range ha {
			if !visSame(ha[i], hb[i], 0.01) {
				bad = append(bad, fmt.Sprintf("hud draw moved with the camera:\n\t\tbefore %s\n\t\tafter  %s", ha[i], hb[i]))
			}
		}
	}
	wa, wb := visPass(a, "world"), visPass(b, "world")
	if len(wa) != len(wb) {
		bad = append(bad, fmt.Sprintf("world: %d draws before the pan, %d after", len(wa), len(wb)))
		return bad
	}
	for i := range wa {
		want := wa[i]
		want.X -= float32(float64(dx) * tile)
		want.Y -= float32(float64(dy) * tile)
		if !visSame(want, wb[i], posTol) {
			bad = append(bad, fmt.Sprintf("world draw did not move by exactly (-%d, -%d) tiles = (-%.1f, -%.1f) px:\n\t\tbefore %s\n\t\twant   %s\n\t\tafter  %s",
				dx, dy, float64(dx)*tile, float64(dy)*tile, wa[i], want, wb[i]))
		}
	}
	return bad
}

// visSame reports a and b are the same draw within tol pixels.
func visSame(a, b traceEvent, tol float64) bool {
	return a.Kind == b.Kind && a.Text == b.Text &&
		visNear(a.X, b.X, tol) && visNear(a.Y, b.Y, tol) && visNear(a.W, b.W, tol) && visNear(a.H, b.H, tol)
}

func visNear(a, b float32, tol float64) bool { return math.Abs(float64(a-b)) <= tol }

// visCheckScale compares the HUD draws of a frame at scale s with the same
// frame at scale 1 (ref). The logical layout is the same, so each draw is
// the reference x s: kind, order and text match. Positions agree to one
// logical pixel plus half a physical one, s + 0.5 (a measured width rounds
// to a whole logical pixel, and the layout carries that on; today the
// layouts agree to 0.02 px). Text sizes agree to 5 % plus the same: the
// font is rasterised at a whole pixel size (font.go faceSize floors
// size x s), so glyph runs come out up to one ppem narrower than s x the
// reference (3 % measured at s = 1.5).
func visCheckScale(ref, got []traceEvent, s float64) []string {
	var bad []string
	if len(ref) != len(got) {
		bad = append(bad, fmt.Sprintf("hud: %d draws at scale 1, %d at scale %g%s", len(ref), len(got), s, visFirstDiff(ref, got, s)))
		return bad
	}
	tol := s + 0.5
	for i := range ref {
		r, g := ref[i], got[i]
		if r.Kind != g.Kind || visScaleText(r.Text) != visScaleText(g.Text) {
			bad = append(bad, fmt.Sprintf("hud draw %d differs in kind/text:\n\t\tscale 1 %s\n\t\tscale %g %s", i, r, s, g))
			continue
		}
		wTol, hTol := tol, tol
		if r.Kind == "text" {
			wTol += 0.05 * float64(r.W) * s
			hTol += 0.05 * float64(r.H) * s
		}
		// centred/right text anchors at a point: its left edge moves with its width
		xTol := tol + math.Abs(float64(g.W)-float64(r.W)*s)
		if math.Abs(float64(g.X)-float64(r.X)*s) > xTol || math.Abs(float64(g.Y)-float64(r.Y)*s) > tol ||
			math.Abs(float64(g.W)-float64(r.W)*s) > wTol || math.Abs(float64(g.H)-float64(r.H)*s) > hTol {
			bad = append(bad, fmt.Sprintf("hud draw %d is not the scale-1 draw x %g:\n\t\tscale 1 %s\n\t\tscale %g %s", i, s, r, s, g))
		}
	}
	return bad
}

// autoScaleRE is the Options "UI scale" stepper's Auto label, which names
// the current scale by design ("Auto (150%)").
var autoScaleRE = regexp.MustCompile(`^Auto \(\d+%\)$`)

// visScaleText is a text as the scale check compares it: the one label
// that legitimately shows the scale itself is reduced to a constant.
func visScaleText(s string) string {
	if autoScaleRE.MatchString(s) {
		return "Auto (scale%)"
	}
	return s
}

// visFirstDiff names the first draw where two HUD streams stop agreeing.
func visFirstDiff(ref, got []traceEvent, s float64) string {
	for i := 0; i < len(ref) && i < len(got); i++ {
		if ref[i].Kind != got[i].Kind || visScaleText(ref[i].Text) != visScaleText(got[i].Text) {
			return fmt.Sprintf("; first difference at %d:\n\t\tscale 1 %s\n\t\tscale %g %s", i, ref[i], s, got[i])
		}
	}
	if len(ref) > len(got) {
		return "; scale 1 also draws " + ref[len(got)].String()
	}
	return "; scale " + strconv.FormatFloat(s, 'g', -1, 64) + " also draws " + got[len(ref)].String()
}

// visInside reports the event's rectangle lies in [x0,x1] x [y0,y1].
func visInside(e traceEvent, x0, y0, x1, y1, tol float64) bool {
	return float64(e.X) >= x0-tol && float64(e.Y) >= y0-tol && float64(e.X+e.W) <= x1+tol && float64(e.Y+e.H) <= y1+tol
}

// checkAnchors verifies the labels of the world frame ev sit where they
// belong. tile is the tile size in physical pixels.
func (s *visualScene) checkAnchors(ev []traceEvent, sc, tile float64) []string {
	var bad []string
	r := s.a.renderer
	w := r.world()
	world := visPass(ev, "world")
	textsOf := func(txt string) []traceEvent {
		var out []traceEvent
		for _, e := range world {
			if e.Kind == "text" && e.Text == txt {
				out = append(out, e)
			}
		}
		return out
	}

	// Zone names sit inside their zone.
	if tile >= 40 {
		for _, z := range w.Zones {
			lbl := zoneLabelText(z)
			x0f, y0f := r.worldToScreenF(float64(z.MinX), float64(z.MinY))
			x1f, y1f := r.worldToScreenF(float64(z.MaxX+1), float64(z.MaxY+1))
			x0, y0, x1, y1 := float64(x0f), float64(y0f), float64(x1f), float64(y1f)
			found := textsOf(lbl)
			if len(found) == 0 {
				bad = append(bad, fmt.Sprintf("zone label %q is not drawn (tile %.1f px)", lbl, tile))
			}
			for _, e := range found {
				if !visInside(e, x0, y0, x1, y1, 1) {
					bad = append(bad, fmt.Sprintf("zone label %q at %s is outside its zone (%.0f,%.0f)-(%.0f,%.0f)", lbl, e, x0, y0, x1, y1))
				}
			}
		}
	}

	// The tame animal's name sits under its animal.
	if tile >= stackCountMinTile {
		for _, an := range w.Animals {
			if !an.Tame || an.Name == "" {
				continue
			}
			wx, wy := r.interpAnimal(an)
			sx32, sy32 := r.worldToScreenF(wx, wy)
			sx, sy := float64(sx32), float64(sy32)
			found := textsOf(an.Name)
			if len(found) == 0 {
				bad = append(bad, fmt.Sprintf("animal name %q is not drawn (tile %.1f px)", an.Name, tile))
			}
			for _, e := range found {
				cx := float64(e.X + e.W/2)
				if cx < sx-1 || cx > sx+tile+1 || float64(e.Y) < sy+0.5*tile-1 || float64(e.Y) > sy+1.6*tile {
					bad = append(bad, fmt.Sprintf("animal name %s is not under its animal (tile at %.0f,%.0f, %.1f px)", e, sx, sy, tile))
				}
			}
		}
	}

	// Stack counts sit on their stack's tile.
	if tile >= stackCountMinTile {
		for _, st := range visStacks {
			if st.qty <= 1 {
				continue
			}
			sx32, sy32 := r.worldToScreen(st.x, st.y)
			sx, sy := float64(sx32), float64(sy32)
			found := textsOf(strconv.Itoa(st.qty))
			if len(found) == 0 {
				bad = append(bad, fmt.Sprintf("stack count %d is not drawn (tile %.1f px)", st.qty, tile))
			}
			for _, e := range found {
				if !visInside(e, sx, sy, sx+tile, sy+tile, 1) {
					bad = append(bad, fmt.Sprintf("stack count %s is off its tile (%.0f,%.0f, %.1f px)", e, sx, sy, tile))
				}
			}
		}
	}

	// The draft, chat and fight markers float over their colonist.
	for i, name := range []string{"draft icon", "chat bubble icon", "fight marker"} {
		px, py, found := 0.0, 0.0, false
		for _, pv := range r.snapshot() {
			if pv.ID == s.pawns[i] {
				wx, wy := r.interpWorld(pv)
				x, y := r.worldToScreenF(wx, wy)
				px, py, found = float64(x), float64(y), true
			}
		}
		if !found {
			bad = append(bad, name+": colonist missing from the snapshot")
			continue
		}
		hit := false
		for _, e := range world {
			if e.Kind != "image" {
				continue
			}
			cx, cy := float64(e.X+e.W/2), float64(e.Y+e.H/2)
			if cx >= px+0.2*tile && cx <= px+1.3*tile && cy >= py-1.3*tile && cy <= py-0.2*tile {
				hit = true
			}
		}
		if !hit {
			bad = append(bad, fmt.Sprintf("%s is not drawn over its colonist (tile at %.0f,%.0f, %.1f px)", name, px, py, tile))
		}
	}
	return bad
}

// checkPortraits verifies every colonist-bar portrait image sits inside
// its bar slot, and that every colonist has one.
func (s *visualScene) checkPortraits(ev []traceEvent, sc float64) []string {
	var bad []string
	a := s.a
	n := len(a.barPawns())
	top, bottom := float64(hudBarY(a.sh))*sc, float64(hudBarY(a.sh)+hudBarH)*sc
	have := map[int]bool{}
	for _, e := range visPass(ev, "hud") {
		if e.Kind != "image" || float64(e.Y) >= bottom || float64(e.Y+e.H) <= top {
			continue
		}
		slot := int(math.Floor((float64(e.X)/sc - hudPad) / barSlot))
		left, right := float64(hudPad+slot*barSlot)*sc, float64(hudPad+(slot+1)*barSlot)*sc
		if slot < 0 || slot >= n || !visInside(e, left, top, right, bottom, 0.5) {
			bad = append(bad, fmt.Sprintf("bar image %s is outside its colonist-bar slot (slot %d of %d, x %.0f..%.0f, bar y %.0f..%.0f)", e, slot, n, left, right, top, bottom))
			continue
		}
		draftIcon := math.Abs(float64(e.W)-draftIconSize*sc) < 0.5 && math.Abs(float64(e.H)-draftIconSize*sc) < 0.5
		if !draftIcon { // the drafted colonist's icon is not a portrait layer
			have[slot] = true
		}
	}
	for i := 0; i < n; i++ {
		if !have[i] {
			bad = append(bad, fmt.Sprintf("colonist-bar slot %d draws no portrait image", i))
		}
	}
	return bad
}

// visCheckOnScreen verifies the HUD stays on the physical screen: text,
// images and outlines lie inside it, and a fill starts inside it (panels
// and backdrops may bleed past an edge by design: a footer band a pixel
// too tall, a backdrop disc bigger than the window).
func visCheckOnScreen(ev []traceEvent, pw, ph int) []string {
	var bad []string
	for _, e := range visPass(ev, "hud") {
		switch e.Kind {
		case "text", "image", "stroke":
			if !visInside(e, 0, 0, float64(pw), float64(ph), 0.5) {
				bad = append(bad, fmt.Sprintf("hud draw %s leaves the %dx%d screen", e, pw, ph))
			}
		case "fill":
			if float64(e.X) < -0.5 || float64(e.Y) < -0.5 || float64(e.X) > float64(pw) || float64(e.Y) > float64(ph) {
				bad = append(bad, fmt.Sprintf("hud fill %s starts off the %dx%d screen", e, pw, ph))
			}
		}
	}
	return bad
}

// visReport fails t with the problems, capped so a systematic bug stays readable.
func visReport(t *testing.T, what string, bad []string) {
	t.Helper()
	if len(bad) == 0 {
		return
	}
	const cap = 8
	more := ""
	if len(bad) > cap {
		more = fmt.Sprintf("\n\t... and %d more", len(bad)-cap)
		bad = bad[:cap]
	}
	t.Errorf("%s: %d problem(s):\n\t%s%s", what, len(bad), strings.Join(bad, "\n\t"), more)
}

// TestFrameInvariance draws the scene at every scale and zoom and checks
// the pan, scale and anchor invariants (see the file comment).
func TestFrameInvariance(t *testing.T) {
	art := visArt(t)
	s := newVisualScene(t, art, false)

	// The scale-1 reference: the HUD does not depend on the zoom.
	ref := visPass(s.frame(s.configure(t, 1, 1, 1)), "hud")
	if len(ref) < 50 {
		t.Fatalf("the reference HUD drew only %d things", len(ref))
	}

	for _, dev := range visDevs {
		for _, ui := range visUIs {
			for _, zoom := range visZooms {
				sc := dev * ui
				t.Run(visName(dev, ui, zoom), func(t *testing.T) {
					screen := s.configure(t, dev, ui, zoom)
					tile := s.a.renderer.tilePx()
					a := s.frame(screen)

					visReport(t, "anchors", s.checkAnchors(a, sc, tile))
					visReport(t, "portraits", s.checkPortraits(a, sc))
					visReport(t, "on screen", visCheckOnScreen(a, s.a.pw, s.a.ph))
					if zoom == visZooms[0] && sc != 1 {
						visReport(t, "scale", visCheckScale(ref, visPass(a, "hud"), sc))
					}

					s.pan(visPanX, visPanY)
					b := s.frame(screen)
					visReport(t, "pan", visCheckPan(a, b, visPanX, visPanY, tile))
				})
			}
		}
	}
}

// visPanel is one screen state of the panel test: how to open it and,
// when the state has a panel of known size, where (logical pixels).
type visPanel struct {
	name string
	open func(t *testing.T, s *visualScene)
	rect func(s *visualScene) (button, bool)
}

func visPanels() []visPanel {
	var ps []visPanel
	// the colonist inspector, every tab
	for _, tab := range []string{"character", "needs", "bio", "health", "social", "gear", "log"} {
		ps = append(ps, visPanel{
			name: "colonist_" + tab,
			open: func(t *testing.T, s *visualScene) { s.a.renderer.Select(s.pawns[3]); s.a.inspectTab = tab },
			rect: func(*visualScene) (button, bool) { return inspectorRect(), true },
		})
	}
	ps = append(ps,
		visPanel{"animal", func(t *testing.T, s *visualScene) {
			if !s.a.selectAnimalAt(visAnimal[0], visAnimal[1]) {
				t.Fatal("animal not selectable")
			}
		}, func(s *visualScene) (button, bool) { return s.a.animalPanel(), true }},
		visPanel{"item", func(t *testing.T, s *visualScene) {
			if !s.a.selectItemAt(visStacks[0].x, visStacks[0].y) {
				t.Fatal("item not selectable")
			}
		}, func(*visualScene) (button, bool) { return itemRect(), true }},
		visPanel{"plant", func(t *testing.T, s *visualScene) {
			if !s.a.selectPlantAt(visPlant[0], visPlant[1]) {
				t.Fatal("plant not selectable")
			}
		}, func(*visualScene) (button, bool) { return plantRect(), true }},
		visPanel{"building", func(t *testing.T, s *visualScene) {
			if !s.a.selectBuildingAt(visBuilding[0], visBuilding[1]) {
				t.Fatal("building not selectable")
			}
		}, func(s *visualScene) (button, bool) { return s.a.buildingPanelRect(), true }},
	)
	// the main tabs (the first four open the work panel, which fills the map)
	for _, tab := range []string{"work", "schedule", "research", "animals"} {
		ps = append(ps, visPanel{
			name: "tab_" + tab,
			open: func(t *testing.T, s *visualScene) { s.a.applyClick("tab." + tab) },
			rect: func(s *visualScene) (button, bool) { return workRect(s.a.sw, s.a.sh), true },
		})
	}
	ps = append(ps,
		visPanel{"tab_architect", func(t *testing.T, s *visualScene) { s.a.applyClick("tab.architect") },
			func(s *visualScene) (button, bool) { return s.a.architectRect(), true }},
		// modal windows and screens: no single panel rect, the screen bounds them
		visPanel{"tab_assign", func(t *testing.T, s *visualScene) { s.a.applyClick("tab.assign") }, nil},
		visPanel{"tab_world", func(t *testing.T, s *visualScene) { s.a.applyClick("tab.world") }, nil},
		visPanel{"tab_history", func(t *testing.T, s *visualScene) { s.a.applyClick("tab.history") }, nil},
		visPanel{"pause_menu", func(t *testing.T, s *visualScene) { s.a.applyClick("tab.menu") }, nil},
		visPanel{"letter", func(t *testing.T, s *visualScene) {
			s.a.openLetter(game.MessageView{Tick: 1, Title: "Heat wave", Letter: true,
				Text: "A heat wave has settled over the land. Keep colonists indoors and watered."})
		}, nil},
	)
	for _, tab := range []string{"audio", "graphics", "gameplay", "controls", "access"} {
		ps = append(ps, visPanel{
			name: "options_" + tab,
			open: func(t *testing.T, s *visualScene) {
				s.a.optionsTab = tab
				s.a.apply(intent{kind: intentPush, screen: screenOptions})
			},
		})
	}
	return ps
}

// visAdded are the events of got that base does not already hold (a
// multiset difference): what opening a panel drew.
func visAdded(base, got []traceEvent) []traceEvent {
	have := map[string]int{}
	for _, e := range base {
		have[e.String()]++
	}
	var out []traceEvent
	for _, e := range got {
		if k := e.String(); have[k] > 0 {
			have[k]--
		} else {
			out = append(out, e)
		}
	}
	return out
}

// TestPanelsStayInTheirPanel opens every inspector, main tab, the options
// screens, the pause menu and a letter, and checks at scale 1 and at
// larger scales that everything the state draws stays on the screen and
// (where the panel has a known rectangle) inside it, and that the HUD
// scales like the scale-1 HUD does.
func TestPanelsStayInTheirPanel(t *testing.T) {
	art := visArt(t)
	type cfg struct{ dev, ui float64 }
	cfgs := []cfg{{1, 1}, {1, 1.5}, {2, 2}}
	scenes := map[cfg]*visualScene{}
	base := map[cfg][]traceEvent{} // the HUD with nothing open
	for _, c := range cfgs {
		s := newVisualScene(t, art, true)
		base[c] = visPass(s.frame(s.configure(t, c.dev, c.ui, 1)), "hud")
		scenes[c] = s
	}
	for _, p := range visPanels() {
		t.Run(p.name, func(t *testing.T) {
			var ref []traceEvent
			for _, c := range cfgs {
				sc := c.dev * c.ui
				s := scenes[c]
				screen := s.configure(t, c.dev, c.ui, 1)
				p.open(t, s)
				s.update(t)
				hud := visPass(s.frame(screen), "hud")
				added := visAdded(base[c], hud)
				if len(added) == 0 {
					t.Fatalf("opening %s drew nothing new at scale %g", p.name, sc)
				}
				visReport(t, fmt.Sprintf("scale %g on screen", sc), visCheckOnScreen(hud, s.a.pw, s.a.ph))
				if p.rect != nil {
					r, _ := p.rect(s)
					x0, y0, x1, y1 := float64(r.x)*sc, float64(r.y)*sc, float64(r.x+r.w)*sc, float64(r.y+r.h)*sc
					var bad []string
					for _, e := range added {
						if !visInside(e, x0, y0, x1, y1, 1) {
							bad = append(bad, fmt.Sprintf("%s is outside its panel (%.0f,%.0f)-(%.0f,%.0f)", e, x0, y0, x1, y1))
						}
					}
					visReport(t, fmt.Sprintf("scale %g panel", sc), bad)
				}
				if sc == 1 {
					ref = hud
				} else {
					visReport(t, fmt.Sprintf("scale %g", sc), visCheckScale(ref, hud, sc))
				}
				// the next panel starts from the bare colony: prove it is
				s.closeAll(t)
				s.update(t)
				if left := visAdded(base[c], visPass(s.frame(screen), "hud")); len(left) > 0 {
					t.Fatalf("closing %s left %d draws behind, e.g. %s", p.name, len(left), left[0])
				}
			}
		})
	}
}

// The checkers must see the bugs they exist for: these feed them frames
// shaped like the shipped ones (a label drawn at physical x instead of
// logical x drifts at twice the camera's speed; a HUD draw left unscaled
// stays at its scale-1 size).
func TestVisualCheckersCatchTheBugs(t *testing.T) {
	hudBox := traceEvent{Kind: "fill", X: 0, Y: 0, W: 1280, H: 32, Pass: "hud"}
	label := func(x, y float32) traceEvent {
		return traceEvent{Kind: "text", Text: "Stockpile", X: x, Y: y, W: 50, H: 15, Pass: "world"}
	}
	const tile = 64.0

	// pan: the camera moves right 2 tiles, down 1: the world moves (-128, -64)
	before := []traceEvent{hudBox, label(500, 400)}
	good := []traceEvent{hudBox, label(372, 336)}
	if bad := visCheckPan(before, good, 2, 1, tile); len(bad) != 0 {
		t.Errorf("a label that moved with the world was flagged: %q", bad)
	}
	drift := []traceEvent{hudBox, label(500-2*128, 400-2*64)} // the zone-label bug at scale 2
	if bad := visCheckPan(before, drift, 2, 1, tile); len(bad) != 1 || !strings.Contains(bad[0], "did not move by exactly") {
		t.Errorf("a label drifting at twice the camera speed = %q, want one problem", bad)
	}
	still := []traceEvent{hudBox, label(500, 400)}
	if bad := visCheckPan(before, still, 2, 1, tile); len(bad) != 1 {
		t.Errorf("a world label that stayed on screen = %q, want one problem", bad)
	}
	moved := hudBox
	moved.X -= 128
	if bad := visCheckPan(before, []traceEvent{moved, label(372, 336)}, 2, 1, tile); len(bad) != 1 || !strings.Contains(bad[0], "hud draw moved") {
		t.Errorf("a HUD draw that moved with the camera = %q, want one problem", bad)
	}
	if bad := visCheckPan(before, good[:1], 2, 1, tile); len(bad) != 1 || !strings.Contains(bad[0], "world: 1 draws before") {
		t.Errorf("a draw that vanished = %q, want one problem", bad)
	}

	// scale: HUD draws at 2 are the scale-1 draws x 2
	ref := []traceEvent{hudBox, {Kind: "text", Text: "Food 7", X: 8, Y: 8, W: 41, H: 17.7, Pass: "hud"}}
	ok := []traceEvent{{Kind: "fill", X: 0, Y: 0, W: 2560, H: 64, Pass: "hud"}, {Kind: "text", Text: "Food 7", X: 16, Y: 16, W: 82.6, H: 35.4, Pass: "hud"}}
	if bad := visCheckScale(ref, ok, 2); len(bad) != 0 {
		t.Errorf("a correctly scaled HUD was flagged: %q", bad)
	}
	unscaled := []traceEvent{hudBox, ref[1]} // drawn raw: still at scale-1 size and place
	if bad := visCheckScale(ref, unscaled, 2); len(bad) != 2 {
		t.Errorf("an unscaled HUD = %q, want both draws flagged", bad)
	}
	if bad := visCheckScale(ref, ok[:1], 2); len(bad) != 1 || !strings.Contains(bad[0], "2 draws at scale 1, 1 at scale 2") {
		t.Errorf("a HUD draw that vanished at scale 2 = %q", bad)
	}
	renamed := []traceEvent{ok[0], ok[1]}
	renamed[1].Text = "Food 8"
	if bad := visCheckScale(ref, renamed, 2); len(bad) != 1 || !strings.Contains(bad[0], "kind/text") {
		t.Errorf("a changed text = %q, want one problem", bad)
	}
	auto1 := []traceEvent{{Kind: "text", Text: "Auto (100%)", X: 270, Y: 171, W: 73, H: 17.7, Pass: "hud"}}
	auto2 := []traceEvent{{Kind: "text", Text: "Auto (200%)", X: 540, Y: 342, W: 146, H: 35.4, Pass: "hud"}}
	if bad := visCheckScale(auto1, auto2, 2); len(bad) != 0 {
		t.Errorf("the Options \"Auto (n%%)\" label names the scale by design: %q", bad)
	}

	// panel diff and screen bounds
	panel := []traceEvent{hudBox, {Kind: "fill", X: 8, Y: 40, W: 400, H: 500, Pass: "hud"}}
	if added := visAdded([]traceEvent{hudBox}, panel); len(added) != 1 || added[0].W != 400 {
		t.Errorf("visAdded = %v", added)
	}
	off := []traceEvent{{Kind: "text", Text: "x", X: 1270, Y: 10, W: 30, H: 15, Pass: "hud"}, {Kind: "fill", X: 0, Y: 660, W: 1280, H: 61, Pass: "hud"}}
	if bad := visCheckOnScreen(off, 1280, 720); len(bad) != 1 {
		t.Errorf("a text past the right edge = %q, want one problem (a fill may bleed)", bad)
	}
}
