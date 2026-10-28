package ebiten

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Setup for the frame-invariance tests (PLAY-017 §12): one real colony
// with everything that carries a world-anchored label or marker, drawn as
// full frames by App.Draw at any UI scale x device scale x zoom, with the
// canvas's drawTrace recording what was drawn (ui_canvas.go). Unit tests
// cannot read GPU pixels back; the trace is the next best thing, and it
// sees text, icons and rectangles in physical pixels, tagged "world" or
// "hud". Everything here changes package globals (gfx.CurrentScale(), drawTrace,
// gfx.Pass()): never in parallel.

// The logical screen every frame is drawn on; the physical screen is this
// x the UI scale s = dev x ui.
const (
	visLogicalW = 1280
	visLogicalH = 720
)

// The scene, in tiles. Row 13-14 holds the three zones, row 16 the item
// stacks and the colonists, and the tame animal stands at the end of the
// zone row; the whole scene is 14 x 5 tiles, so it fits the map viewport
// at every scale and zoom with room for a two-tile pan.
var (
	visGrow    = [4]int{12, 13, 14, 14}
	visStock   = [4]int{16, 13, 18, 14}
	visPen     = [4]int{20, 13, 22, 14}
	visAnimal  = [2]int{24, 14}
	visFocus   = [2]float64{19, 15} // the tile the camera centres on
	visPawnXY  = [4][2]int{{19, 16}, {21, 16}, {23, 16}, {25, 16}}
	visStacks  = []visStack{{item.Wood, 25, 13, 16}, {item.Meal, 7, 15, 16}}
	visAnimalN = "Clover"

	visPlant    = [2]int{28, 20} // inspected by the panel tests
	visBuilding = [2]int{28, 22}
)

type visStack struct {
	kind item.Kind
	qty  int
	x, y int
}

// visSim is the game with a few presentation-only fields forced into its
// pawn snapshot: a social fight and a chat bubble need ticks of social
// simulation to arise, and the renderer only reads these two fields.
type visSim struct {
	*game.Game
	chat, fight entity.EntityID
}

func (s *visSim) Snapshot() []game.PawnView {
	ps := s.Game.Snapshot()
	out := make([]game.PawnView, len(ps))
	copy(out, ps)
	for i := range out {
		switch out[i].ID {
		case s.fight:
			out[i].SocialFight = true
		case s.chat:
			out[i].Interaction, out[i].InteractTick = "chitchat", uint64(s.Game.Tick())
		}
	}
	return out
}

// visualScene is the colony App the visual tests draw.
type visualScene struct {
	a      *App
	g      *game.Game
	pawns  []entity.EntityID // drafted, chatting, fighting, plain
	animal entity.EntityID
	items  map[int]entity.EntityID // qty -> stack
	shots  map[[2]int]*ebiten.Image
	in     *fakeInput
}

// visArtCat is the art index every visual test shares (testArt decodes
// lazily, so one index serves them all).
var visArtCat *artCatalog

// visArt returns the shared art index, or skips the test without art.
func visArt(t testing.TB) *artCatalog {
	t.Helper()
	if visArtCat == nil {
		visArtCat = testArt(t) // skips when the art is missing
	}
	return visArtCat
}

// newVisualScene builds the colony (extras: with a plant and a building to
// inspect) drawn with art, which the labels need.
func newVisualScene(t *testing.T, art *artCatalog, extras bool) *visualScene {
	t.Helper()
	g := game.New(game.Config{Seed: 1, Width: 32, Height: 32})
	s := &visualScene{g: g, items: map[int]entity.EntityID{}, shots: map[[2]int]*ebiten.Image{}}
	for _, xy := range visPawnXY {
		id, err := g.SpawnPawn(xy[0], xy[1])
		if err != nil {
			t.Fatal(err)
		}
		s.pawns = append(s.pawns, id)
	}
	zone := func(k command.Kind, r [4]int) {
		g.Enqueue(command.Command{Kind: k, X: r[0], Y: r[1], X2: r[2], Y2: r[3]})
	}
	zone(command.DesignateGrowZone, visGrow)
	zone(command.DesignateStockpile, visStock)
	zone(command.DesignatePen, visPen)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(s.pawns[0]), Enabled: true})
	g.ApplyPending()
	for _, st := range visStacks {
		id, err := g.SpawnStack(st.kind, st.qty, st.x, st.y)
		if err != nil {
			t.Fatal(err)
		}
		s.items[st.qty] = id
	}
	an, err := g.SpawnAnimal("dustbuck", visAnimal[0], visAnimal[1])
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(an, visAnimalN)
	s.animal = an
	if extras { // something to inspect, away from the scene
		if _, err := g.SpawnCrop("potato", visPlant[0], visPlant[1], 50); err != nil {
			t.Fatal(err)
		}
		if !g.BuildNow("campfire", "wood", "south", visBuilding[0], visBuilding[1]) {
			t.Fatal("campfire not built")
		}
	}

	sim := &visSim{Game: g, chat: s.pawns[1], fight: s.pawns[2]}
	a := testApp(sim)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony || a.renderer == nil {
		t.Fatalf("startGame = %s renderer=%v", a.nav.screen(), a.renderer != nil)
	}
	a.art, a.renderer.art = art, art
	a.renderer.speed = speedPaused
	// The learning helper reacts to what a panel test selects (a lesson done
	// flashes, the next one slides in), so it would change between the
	// scales a test compares: it is off here. TestFrameInvariance, which
	// selects nothing, turns it back on so the frame checks cover it.
	a.settings.Gameplay.ShowLearningHelper = false
	s.a = a
	t.Cleanup(func() {
		gfx.SetScale(unitUIScale)
		gfx.SetDrawTrace(nil)
		gfx.ForcePass("hud")
		for _, img := range s.shots {
			img.Deallocate()
		}
	})
	return s
}

// configure draws at device scale dev, UI scale ui and the given apparent
// zoom: the physical screen is 1280x720 logical x s, the camera centres on
// the scene. It returns the (cached) screen image to draw on.
func (s *visualScene) configure(t testing.TB, dev, ui, zoom float64) *ebiten.Image {
	t.Helper()
	a := s.a
	sc := dev * ui
	a.scale = uiScaleState{Dev: dev, UI: ui, S: sc}
	a.syncUIScale()
	pw, ph := int(math.Round(visLogicalW*sc)), int(math.Round(visLogicalH*sc))
	a.setPhysical(pw, ph)
	r := a.renderer
	r.SyncDev()
	s.update(t) // may clamp the camera: place it after
	r.cam.Zoom = zoom
	v := a.mapViewport()
	r.centerOnView(visFocus[0], visFocus[1], v, v)
	if r.cam.Zoom != zoom {
		t.Fatalf("zoom %v is below the map's zoom-out limit %v at dev %v ui %v", zoom, r.cam.Zoom, dev, ui)
	}
	key := [2]int{pw, ph}
	img := s.shots[key]
	if img == nil {
		img = ebiten.NewImage(pw, ph)
		s.shots[key] = img
	}
	return img
}

// update runs one App.Update with the cursor off the window: the widget kit
// declares its windows and widgets there (modals, the options screen), and
// a still colony does not change. Call it after changing what is open.
func (s *visualScene) update(t testing.TB) {
	t.Helper()
	if s.in == nil {
		s.in = fakeOn(s.a)
		s.in.Cur.Mx, s.in.Cur.My = -50, -50
	}
	if err := s.a.Update(); err != nil {
		t.Fatal(err)
	}
}

// closeAll puts the colony back to nothing open and nothing selected: the
// inspectors, the work and architect panels, modal windows, the pause menu
// and any screen pushed over the colony.
func (s *visualScene) closeAll(t testing.TB) {
	t.Helper()
	a := s.a
	a.closePaneTab()
	a.clearSelections(selNone)
	a.win.Open = false
	a.closeArchitect()
	a.closeFloat()
	for i := 0; i < 8 && len(a.uiSt().Modals) > 0; i++ {
		a.uiCloseTop()
	}
	for i := 0; i < 8 && a.nav.screen() != screenColony; i++ {
		a.apply(intent{kind: intentBack})
	}
	if a.nav.overlay() != overlayNone {
		a.apply(intent{kind: intentResume})
	}
	a.inspectTab, a.opt.Tab = "character", ""
	if a.nav.screen() != screenColony || a.nav.overlay() != overlayNone || len(a.uiSt().Modals) > 0 {
		t.Fatalf("closeAll left %s/%s with %d modal(s)", a.nav.screen(), a.nav.overlay(), len(a.uiSt().Modals))
	}
}

// pan moves the camera by whole tiles (the world moves the other way).
func (s *visualScene) pan(dx, dy int) {
	r := s.a.renderer
	t := r.tilePx()
	r.cam.OffsetX += float64(dx) * t
	r.cam.OffsetY += float64(dy) * t
}

// frame draws one full App frame and returns everything the canvas drew.
func (s *visualScene) frame(screen *ebiten.Image) []traceEvent {
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	defer func() { gfx.SetDrawTrace(nil) }()
	screen.Clear()
	s.a.Draw(screen)
	return ev
}

// passOf returns the events drawn in one pass.
func visPass(ev []traceEvent, pass string) []traceEvent {
	var out []traceEvent
	for _, e := range ev {
		if e.Pass == pass {
			out = append(out, e)
		}
	}
	return out
}

// summary is a compact listing of events for failure messages.
func visSummary(ev []traceEvent, max int) string {
	out := ""
	for i, e := range ev {
		if i == max {
			out += fmt.Sprintf("\n\t... %d more", len(ev)-max)
			break
		}
		out += "\n\t" + e.String()
	}
	return out
}
