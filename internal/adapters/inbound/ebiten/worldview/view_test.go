package worldview

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// testPlanet is one small charted world the tests share (read-only).
var testPlanet = sync.OnceValue(func() *planet.Planet {
	b := planet.NewBuilder(testParams)
	for {
		if _, _, done := b.Step(50000); done {
			return b.Planet()
		}
	}
})

var testParams = planet.Params{SeedText: "world-view", Coverage: 5, Rain: 2, Temp: 2, Pop: 1}

// landTile is a land tile of the test planet with no settlement within a
// hop (ValidLanding passes); skip tiles to get another one.
func landTile(t *testing.T, skip int) int {
	t.Helper()
	p := testPlanet()
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok {
			if skip == 0 {
				return i
			}
			skip--
		}
	}
	t.Fatal("no land tile")
	return -1
}

// fakeGame is a Reader with fixed read models (WorldSource too).
type fakeGame struct {
	maps     []game.MapView
	world    game.WorldMapView
	sites    []game.WorldSiteView
	factions []game.FactionView
	view     game.MapID
	params   planet.Params
	hasMap   bool
}

func (g *fakeGame) WorldMap() game.WorldMapView  { return g.world }
func (g *fakeGame) Maps() []game.MapView         { return g.maps }
func (g *fakeGame) ViewedMap() game.MapID        { return g.view }
func (g *fakeGame) Factions() []game.FactionView { return g.factions }
func (g *fakeGame) WorldSites() []game.WorldSiteView {
	return g.sites
}
func (g *fakeGame) WorldParams() (planet.Params, bool) { return g.params, g.hasMap }

// estGame is a fakeGame with the simulation's estimates and refusals.
type estGame struct {
	*fakeGame
	ticks  int
	why    string
	attack string
	launch string
}

func (g *estGame) TravelEstimate(uint64, int) (int, string)               { return g.ticks, g.why }
func (g *estGame) AttackReason(uint64) string                             { return g.attack }
func (g *estGame) LaunchReason(entity.EntityID, int, uint8) (string, int) { return g.launch, 3 }

// fakeShell is a headless shell: a real kit over a fixed 1280x720 host; a
// test that reaches any other method panics on the nil embedded Shell.
type fakeShell struct {
	Shell
	kit      *uikit.Kit
	f        inputframe.Frame
	env      Env
	world    game.WorldView
	cmds     []command.Command
	intents  []ports.Intent
	formed   int
	hint     string
	mx, my   int
	chrome   []gfx.Rect
	settings config.Settings
	tile     [2]int
	tick     clock.Tick
	arch     bool
	settle   string
	bar      gfx.Rect
	window   bool
}

func newFake(g Reader) (*fakeShell, *View) {
	sh := &fakeShell{mx: -1, my: -1}
	sh.kit = uikit.New(fakeHost{sh})
	sh.env = Env{Game: g, Planet: testPlanet()}
	return sh, New(sh)
}

func (s *fakeShell) Kit() *uikit.Kit            { return s.kit }
func (s *fakeShell) Env() Env                   { return s.env }
func (s *fakeShell) Size() (int, int)           { return 1280, 720 }
func (s *fakeShell) Cursor() (int, int)         { return s.mx, s.my }
func (s *fakeShell) MapCursor() (int, int)      { return s.mx, s.my }
func (s *fakeShell) World() game.WorldView      { return s.world }
func (s *fakeShell) Tick() clock.Tick           { return s.tick }
func (s *fakeShell) Date() clock.Date           { return clock.Date{} }
func (s *fakeShell) MapSize() (int, int)        { return 50, 50 }
func (s *fakeShell) TileAt(int, int) (int, int) { return s.tile[0], s.tile[1] }
func (s *fakeShell) Enqueue(c command.Command)  { s.cmds = append(s.cmds, c) }
func (s *fakeShell) Do(it ports.Intent)         { s.intents = append(s.intents, it) }
func (s *fakeShell) UIHidden() bool             { return false }
func (s *fakeShell) Settings() config.Settings  { return s.settings }
func (s *fakeShell) FormCaravan(func() int)     { s.formed++ }
func (s *fakeShell) Layout() hudlayout.Layout {
	return hudlayout.Layout{
		Tabs: gfx.Rect{X: 0, Y: 692, W: 1280, H: 28},
		Pane: gfx.Rect{X: 0, Y: 480, W: 400, H: 212},
		Bar:  s.bar,
	}
}
func (s *fakeShell) ChromeHit(mx, my int) bool {
	for _, r := range s.chrome {
		if r.Hit(mx, my) {
			return true
		}
	}
	return false
}

// fakeHost is the kit's host.
type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return h.s.Cursor() }
func (h fakeHost) Size() (int, int)         { return h.s.Size() }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(s string)            { h.s.hint = s }
func (h fakeHost) ReducedMotion() bool      { return true }

// frame runs one Update's declaration with the pointer at (mx, my).
func frame(sh *fakeShell, v *View, mx, my int, raw inputframe.Raw) {
	sh.mx, sh.my = mx, my
	raw.Mx, raw.My = mx, my
	sh.f = inputframe.Frame{Raw: raw}
	sh.kit.Begin()
	v.Declare(1280, 720)
}

// press clicks at (mx, my): down on one frame.
func press(sh *fakeShell, v *View, mx, my int) {
	var raw inputframe.Raw
	raw.Press[inputframe.Left], raw.Down[inputframe.Left] = true, true
	frame(sh, v, mx, my, raw)
}

// clickWidget clicks widget id the way the App does (its centre; a marker
// widget through the kit's click path).
func clickWidget(t *testing.T, sh *fakeShell, v *View, id string) {
	t.Helper()
	frame(sh, v, -1, -1, inputframe.Raw{})
	w, ok := sh.kit.Find(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	x, y := w.R.Center()
	press(sh, v, int(x), int(y))
	if sh.f.Clicked(inputframe.Left) { // no button took it: the App's hit test
		if w, ok := sh.kit.Find(id); ok {
			sh.f.Take(inputframe.Left)
			w.Click()
		}
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
}

// colonyGame is one colony at home with a caravan beside it, a hostile
// settlement and a friendly one.
func colonyGame(t *testing.T) *fakeGame {
	t.Helper()
	home := landTile(t, 0)
	return &fakeGame{
		params: testParams, hasMap: true,
		maps:  []game.MapView{{ID: 0, Label: "Colony", Kind: "colony", Tile: home, Colonists: 3}},
		world: game.WorldMapView{Tile: home, Caravans: []game.CaravanView{{ID: 7, Members: []string{"Ada", "Bo"}, Health: []string{"", "flu"}, Tile: landTile(t, 40), Dest: landTile(t, 40), State: caravan.Arrived.Label()}}},
		sites: settlements(t),
	}
}

// settlements are a hostile and a friendly settlement on open land (the
// test world has no faction of its own).
func settlements(t *testing.T) []game.WorldSiteView {
	t.Helper()
	return []game.WorldSiteView{
		{Tile: landTile(t, 60), Name: "Ironhold", Faction: "FA", Relation: "hostile"},
		{Tile: landTile(t, 70), Name: "Oakhold", Faction: "FB", Relation: "friendly", Visitable: true},
	}
}

func TestObjectsListEveryKindInPickOrder(t *testing.T) {
	t.Parallel()
	r := readModels{
		tiles: 100,
		maps:  []game.MapView{{ID: 2, Label: "Camp", Kind: "site", Tile: 5}, {ID: 9, Tile: -1}},
		world: game.WorldMapView{Caravans: []game.CaravanView{{ID: 7, Members: []string{"Ada"}, Tile: 5}, {ID: 8, Tile: 500}}},
		sites: []game.WorldSiteView{{Tile: 5, Name: "Oakhold"}},
		ax: axview.World{Sites: []axview.Site{{Tile: 5, Mineral: "steel"}},
			Pods: []axview.PodFlight{{From: 1, To: 5, Count: 2}, {From: 1, To: 900}}},
	}
	got := buildObjects(nil, r)
	var keys, ids []string
	for _, o := range got {
		keys, ids = append(keys, o.Key), append(ids, o.ID)
	}
	want := []string{"map:2", "caravan:7", "settlement:5", "site:5", "pod:0"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
	if ids[0] != "world.obj.map.2" || ids[4] != "world.obj.pod.0" {
		t.Fatalf("ids %v", ids)
	}
	if !got[0].Destroyed() {
		t.Fatal("a site with nobody on it is the destroyed marker")
	}
	// pods are picked where they fly, not on their target
	if n := onTile(nil, got, 5); len(n) != 4 {
		t.Fatalf("on tile 5: %v", n)
	}
}

func TestNearestTakesTheClosestMarkerInItsRadius(t *testing.T) {
	t.Parallel()
	marks := []mark{{obj: 0, x: 100, y: 100, r: 10}, {obj: 1, x: 112, y: 100, r: 10}}
	if i := nearest(marks, 110, 100); i != 1 {
		t.Fatalf("nearest = %d", i)
	}
	if i := nearest(marks, 100+10+markSlack+1, 150); i != -1 {
		t.Fatalf("far away = %d", i)
	}
}

// The ring matches the planet's own hop count and only lies at the range.
func TestRangeRingIsTheHopsLimit(t *testing.T) {
	t.Parallel()
	p := testPlanet()
	home, rng := landTile(t, 0), 5
	dist, ring := rangeRing(p, home, rng)
	if dist[home] != 0 || len(ring) == 0 || slices.Contains(ring, home) {
		t.Fatalf("home %d ring %d", dist[home], len(ring))
	}
	var s planet.HopScratch
	for ti, d := range dist {
		if h := p.HopsWith(&s, home, ti, rng+1); (d >= 0) != (h <= rng) || (d >= 0 && int(d) != h) {
			t.Fatalf("tile %d: bfs %d, hops %d", ti, d, h)
		}
	}
	for _, ti := range ring {
		if int(dist[ti]) != rng {
			edge := false
			for _, nb := range p.Neighbours(ti) {
				edge = edge || dist[nb] < 0
			}
			if !edge {
				t.Fatalf("ring tile %d at %d inside the range", ti, dist[ti])
			}
		}
	}
}

// The view charts the colony's planet a slice per Update, once: closing
// and reopening finds it, and the founded planet is used when it matches.
func TestWorldViewChartsThePlanetOnce(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	g.params = planet.Params{SeedText: "world-tab", Coverage: 5, Rain: 2, Temp: 2, Pop: 1}
	sh, v := newFake(g)
	sh.env.Planet = nil
	v.Show("")
	var p *planet.Planet
	for i := 0; i < 2000 && p == nil; i++ {
		p = v.planetFor(sh.Env())
	}
	if p == nil {
		t.Fatal("the world was never charted")
	}
	v.Close()
	v.Show("")
	if q := v.planetFor(sh.Env()); q != p {
		t.Fatal("a reopened view charts the world again")
	}

	g2 := colonyGame(t)
	sh2, v2 := newFake(g2)
	v2.Show("")
	if v2.planetFor(sh2.Env()) != testPlanet() {
		t.Fatal("the founded planet was not reused")
	}
}

func TestNoMapShowsTheNote(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	g.hasMap = false
	sh, v := newFake(g)
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.g != nil {
		t.Fatal("a colony with no world charted a globe")
	}
	if w, ok := sh.kit.Find(GridID); !ok || !w.Disabled {
		t.Fatalf("grid = %+v", w)
	}
}

// F8 toggles, opening closes the Architect and the windows, and Esc goes
// targeting, selection, close.
func TestOpenCloseAndEscapeOrder(t *testing.T) {
	t.Parallel()
	sh, v := newFake(colonyGame(t))
	v.Toggle()
	if !v.Open() || len(sh.intents) != 2 || sh.intents[0] != (ports.CloseArchitect{}) || sh.intents[1] != (ports.CloseWindows{}) {
		t.Fatalf("open %v intents %v", v.Open(), sh.intents)
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
	v.Toggle()
	if v.Open() {
		t.Fatal("the toggle did not close")
	}
	if v.Escape() {
		t.Fatal("Esc on a closed view took the key")
	}
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.Selected() != "caravan:7" {
		t.Fatalf("selected %q", v.Selected())
	}
	(router{v}).startRoute(7, 5)
	v.pods = podState{on: true, menu: true}
	for _, want := range []string{"menu", "pods", "route", "sel", "close"} {
		if !v.Escape() {
			t.Fatalf("%s: Esc not taken", want)
		}
		switch want {
		case "menu":
			if v.pods.menu || !v.pods.on {
				t.Fatal("the menu goes first")
			}
		case "pods":
			if v.pods.on || !v.route.on {
				t.Fatal("targeting goes next")
			}
		case "route":
			if v.route.on || v.Selected() == "" {
				t.Fatal("the route goes before the selection")
			}
		case "sel":
			if v.Selected() != "" || !v.Open() {
				t.Fatal("the selection goes before the view")
			}
		case "close":
			if v.Open() {
				t.Fatal("the view closes last")
			}
		}
	}
}

// Another tab takes the screen: the view closes.
func TestAnotherTabClosesTheView(t *testing.T) {
	t.Parallel()
	sh, v := newFake(colonyGame(t))
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.window = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.Open() {
		t.Fatal("a main-tab window left the world view open")
	}
	sh.window, sh.arch = false, true
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.Open() {
		t.Fatal("the Architect left the world view open")
	}
}

// A click on a tile with objects cycles them; a double click on our map
// switches to it and closes; an empty tile selects the tile.
func TestPickingCyclesAndDoubleClickEnters(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	home := g.maps[0].Tile
	g.world.Caravans[0].Tile = home
	sh, v := newFake(g)
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.Selected() != "map:0" {
		t.Fatalf("opening selects the map on screen, got %q", v.Selected())
	}
	v.selectKey("")
	// slow clicks cycle: map, caravan, map
	(picker{v}).clickTile(home, 0, 0)
	if v.Selected() != "map:0" {
		t.Fatalf("first click %q", v.Selected())
	}
	v.frame += globe2x
	(picker{v}).clickTile(home, 0, 0)
	if v.Selected() != "caravan:7" {
		t.Fatalf("second click %q", v.Selected())
	}
	v.frame += globe2x
	(picker{v}).clickTile(home, 0, 0)
	if v.Selected() != "map:0" {
		t.Fatalf("third click %q", v.Selected())
	}
	// the double click
	v.frame += globe2x
	v.selectKey("")
	(picker{v}).clickTile(home, 0, 0)
	v.frame++
	(picker{v}).clickTile(home, 0, 0)
	if v.Open() || len(sh.intents) == 0 || sh.intents[len(sh.intents)-1] != (ports.SwitchMap{Map: 0}) {
		t.Fatalf("open %v intents %v", v.Open(), sh.intents)
	}
	// an empty tile
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	empty := landTile(t, 80)
	(picker{v}).clickTile(empty, 0, 0)
	if v.Selected() != "tile:"+strconv.Itoa(empty) || v.selTile() != empty {
		t.Fatalf("empty tile %q", v.Selected())
	}
	title, lines, _ := (inspector{v}).inspect()
	if title == "" || len(lines) < 5 {
		t.Fatalf("tile card %q %v", title, lines)
	}
}

// globe2x is well past the double-click window.
const globe2x = 1000

// gizmoOf is the named gizmo of the selection.
func gizmoOf(t *testing.T, v *View, name string) gizmo {
	t.Helper()
	_, _, gs := (inspector{v}).inspect()
	for _, g := range gs {
		if g.name == name {
			return g
		}
	}
	t.Fatalf("no gizmo %q in %v", name, gs)
	return gizmo{}
}

func TestCaravanGizmosCarryTheirReasons(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	hostile, friendly := g.sites[0], g.sites[1]
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})

	if why := gizmoOf(t, v, "split").why; why != notYet {
		t.Fatalf("split: %q", why)
	}
	if why := gizmoOf(t, v, "attack").why; why == "" {
		t.Fatal("no settlement here: attack must be refused")
	}
	if gz := gizmoOf(t, v, "settle"); gz.why == "" || !strings.Contains(gz.why, "colonies") {
		t.Fatalf("the colony cap (default 1): %q", gz.why)
	}
	sh.settings.Gameplay.MaxColonies = 3
	if why := gizmoOf(t, v, "settle").why; why != "" {
		t.Fatalf("settle on open land with room: %q", why)
	}
	gizmoOf(t, v, "settle").run()
	if c := sh.cmds[len(sh.cmds)-1]; c.Kind != command.SettleHere || c.Target != 7 {
		t.Fatalf("settle sent %+v", c)
	}

	// standing on the hostile settlement
	g.world.Caravans[0].Tile = hostile.Tile
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "attack").why; why != "" {
		t.Fatalf("attack a hostile base: %q", why)
	}
	if why := gizmoOf(t, v, "enter").why; why == "" {
		t.Fatal("entering a hostile base must point at Attack")
	}
	gizmoOf(t, v, "attack").run()
	if c := sh.cmds[len(sh.cmds)-1]; c.Kind != command.AttackSettlement || c.Target != 7 {
		t.Fatalf("attack sent %+v", c)
	}

	// standing on the friendly one
	g.world.Caravans[0].Tile = friendly.Tile
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "attack").why; why != "" { // MAP-15: it asks first instead (TestAttackConfirmsAndAbandonWarns)
		t.Fatalf("attack a friend: %q", why)
	}
	if why := gizmoOf(t, v, "enter").why; why != "" {
		t.Fatalf("enter a friend: %q", why)
	}

	// on water: the planet refuses
	p := testPlanet()
	for i := 0; i < p.TileCount(); i++ {
		if !p.Land(i) {
			g.world.Caravans[0].Tile = i
			break
		}
	}
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "settle").why; why != planet.ReasonWater {
		t.Fatalf("settle on water: %q", why)
	}

	// forming: only Return (Call off) works
	g.world.Caravans[0].State = caravan.Forming.Label()
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	for _, name := range []string{"travel", "settle", "enter", "attack"} {
		if why := gizmoOf(t, v, name).why; why != stillForms {
			t.Fatalf("%s while forming: %q", name, why)
		}
	}
	if gz := gizmoOf(t, v, "return"); gz.label != "Call off" || gz.why != "" {
		t.Fatalf("return while forming: %+v", gz)
	}
}

// The simulation's refusal wins on Attack when the game offers it.
func TestAttackReasonComesFromTheGame(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	g.world.Caravans[0].Tile = g.sites[0].Tile
	sh, v := newFake(&estGame{fakeGame: g, attack: "Too few fighters."})
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "attack").why; why != "Too few fighters." {
		t.Fatalf("attack: %q", why)
	}
}

func TestMapGizmos(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	other := landTile(t, 10)
	g.maps = append(g.maps, game.MapView{ID: 3, Label: "Oak camp", Kind: "site", Tile: other, Colonists: 2, Countdown: 5 * clock.TicksPerHour})
	sh, v := newFake(g)
	v.Show("map:0")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "abandon").why; why != "" {
		t.Fatalf("Abandon is refused: %q", why)
	}
	gizmoOf(t, v, "form").run()
	if sh.formed != 1 {
		t.Fatal("Form caravan did not open the dialog")
	}
	v.selectKey("map:3")
	_, lines, _ := (inspector{v}).inspect()
	if !slices.ContainsFunc(lines, func(l line) bool { return l.label == "Packs up in" && strings.Contains(l.value, "5 hours") }) {
		t.Fatalf("no pack-up countdown in %v", lines)
	}
	gizmoOf(t, v, "reform").run()
	if c := sh.cmds[len(sh.cmds)-1]; c.Kind != command.CaravanReform || c.Target != 3 {
		t.Fatalf("reform sent %+v", c)
	}
	gizmoOf(t, v, "abandon").run()
	if !sh.kit.ModalOpen() {
		t.Fatal("Abandon must ask first")
	}
	gizmoOf(t, v, "jump").run()
	if v.Open() || sh.intents[len(sh.intents)-1] != (ports.SwitchMap{Map: 3}) {
		t.Fatalf("jump: open %v intents %v", v.Open(), sh.intents)
	}
}

func TestSettlementShowsDistanceFromTheCaravan(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	g.factions = []game.FactionView{{Name: g.sites[0].Faction, Goodwill: -40}}
	sh, v := newFake(&estGame{fakeGame: g, ticks: 3 * clock.TicksPerDay})
	v.Show("settlement:" + strconv.Itoa(g.sites[0].Tile))
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, lines, _ := (inspector{v}).inspect()
	if !slices.ContainsFunc(lines, func(l line) bool { return l.label == "Goodwill" && l.value == "-40" }) {
		t.Fatalf("no goodwill in %v", lines)
	}
	if !slices.ContainsFunc(lines, func(l line) bool { return strings.HasPrefix(l.value, "Select a caravan") }) {
		t.Fatalf("no caravan hint in %v", lines)
	}
	v.selectKey("caravan:7")
	v.selectKey("settlement:" + strconv.Itoa(g.sites[0].Tile))
	_, lines, _ = (inspector{v}).inspect()
	if !slices.ContainsFunc(lines, func(l line) bool { return l.label == "Travel time" && strings.Contains(l.value, "3.0 days") }) {
		t.Fatalf("no travel time in %v", lines)
	}
}

// The route: a click plans it (line, time, season), Confirm sends it.
func TestRouteModePlansAndConfirms(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	from, to := g.world.Caravans[0].Tile, landTile(t, 44)
	eg := &estGame{fakeGame: g, ticks: 30 * clock.TicksPerHour}
	sh, v := newFake(eg)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	clickWidget(t, sh, v, "world.act.travel")
	if !v.route.on || v.route.from != from {
		t.Fatalf("route %+v", v.route)
	}
	if w, ok := sh.kit.Find(ConfirmID); !ok || !w.Disabled {
		t.Fatalf("confirm before a target: %+v", w)
	}
	(picker{v}).clickTile(to, 0, 0)
	if len(v.route.path) < 2 || v.route.eta != "about 30 hours" || !strings.Contains(v.route.note, "tiles") {
		t.Fatalf("planned %+v", v.route)
	}
	if v.Selected() != "caravan:7" {
		t.Fatalf("a route click changed the selection: %q", v.Selected())
	}
	clickWidget(t, sh, v, ConfirmID)
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.CaravanTravel || sh.cmds[0].Target != 7 || sh.cmds[0].Count != to || v.route.on {
		t.Fatalf("confirm sent %+v, route %v", sh.cmds, v.route.on)
	}

	// the simulation's refusal and an unknown estimate
	eg.why = "Nobody can walk."
	(router{v}).startRoute(7, from)
	(router{v}).setRouteTarget(to)
	if v.route.why != "Nobody can walk." {
		t.Fatalf("why %q", v.route.why)
	}
	_, v2 := newFake(g)
	v2.Show("caravan:7")
	frame(v2.sh.(*fakeShell), v2, -1, -1, inputframe.Raw{})
	(router{v2}).startRoute(7, from)
	(router{v2}).setRouteTarget(to)
	if v2.route.eta != "—" || v2.route.why != "" {
		t.Fatalf("no estimator: %+v", v2.route)
	}
}

func TestDurationAndHoursLeftText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ticks int
		want  string
	}{{100, "under an hour"}, {2 * clock.TicksPerHour, "about 2 hours"}, {60 * clock.TicksPerHour, "about 2.5 days"}} {
		if got := durationText(c.ticks); got != c.want {
			t.Errorf("durationText(%d) = %q, want %q", c.ticks, got, c.want)
		}
	}
	if s := hoursLeft(10, 20); s != "moments" {
		t.Fatal(s)
	}
	if s := hoursLeft(3*clock.TicksPerDay, 0); s != "3 d" {
		t.Fatal(s)
	}
}

// launchView is the view targeting pod 99's group (a load aboard, range 8).
func launchView(t *testing.T) (*fakeShell, *View, *fakeGame) {
	t.Helper()
	g := colonyGame(t)
	sh, v := newFake(g)
	sh.world.Buildings = []game.BuildingView{{ID: 99}}
	sh.world.Buildings[0].AX.Range, sh.world.Buildings[0].AX.LoadKg = 8, 100
	g.view = 0
	v.LaunchFor(99)
	frame(sh, v, -1, -1, inputframe.Raw{})
	if !v.pods.on || v.pods.rng != 8 || len(v.pods.ring) == 0 {
		t.Fatalf("targeting %+v", v.pods.rng)
	}
	if v.g.HlLeft == 0 || len(v.g.Hl) != len(v.pods.ring) {
		t.Fatalf("the ring is not on the globe: %d of %d", len(v.g.Hl), len(v.pods.ring))
	}
	if v.Selected() != "" {
		t.Fatalf("targeting selected %q", v.Selected())
	}
	return sh, v, g
}

// The targeting list offers what the range reaches and opens the menu.
func TestPodTargetListOpensTheMenu(t *testing.T) {
	t.Parallel()
	sh, v, g := launchView(t)
	home := g.maps[0].Tile
	id := "world.pod.target." + strconv.Itoa(home)
	if _, ok := sh.kit.Find(id); !ok {
		t.Fatalf("no row for home; targets %+v", (targeting{v}).podTargets())
	}
	clickWidget(t, sh, v, id)
	if !v.pods.menu || v.pods.target != home {
		t.Fatalf("menu %v target %d", v.pods.menu, v.pods.target)
	}
	if _, ok := sh.kit.Find("world.pod.drop"); !ok {
		t.Fatal("the menu is not up")
	}
}

// inRange is a land tile within the pods' range that is free for a colony.
func inRange(t *testing.T, v *View) int {
	t.Helper()
	p := testPlanet()
	for ti, d := range v.pods.dist {
		if ok, _ := p.ValidLanding(ti); ok && d > 0 {
			return ti
		}
	}
	t.Fatal("nothing in range")
	return -1
}

func TestPodMenuReasonsMirrorTheRule(t *testing.T) {
	t.Parallel()
	sh, v, g := launchView(t)
	p := testPlanet()
	t0 := inRange(t, v)

	if why := (targeting{v}).mirrorReason(t0, command.PodArriveDrop); why != "" {
		t.Fatalf("drop on open land: %q", why)
	}
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveGift); !strings.Contains(why, "settlement") {
		t.Fatalf("gift on open land: %q", why)
	}
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveVisit); why == "" {
		t.Fatal("visit on open land")
	}
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveAttackEdge); why == "" {
		t.Fatal("attack on open land")
	}
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveFound); !strings.Contains(why, "colonies") {
		t.Fatalf("found at the cap: %q", why)
	}
	sh.settings.Gameplay.MaxColonies = 2
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveFound); why != "" {
		t.Fatalf("found with room: %q", why)
	}
	if why := (targeting{v}).podReason(t0, command.PodArriveSpot); !strings.Contains(why, "map") {
		t.Fatalf("spot off any map: %q", why)
	}

	// settlements: put them inside the range
	g.sites = []game.WorldSiteView{{Tile: t0, Name: "Near", Relation: "hostile"}}
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveDrop); !strings.Contains(why, "attack") {
		t.Fatalf("drop on a hostile base: %q", why)
	}
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveAttackCentre); why != "" {
		t.Fatalf("attack a hostile base: %q", why)
	}

	// out of range and empty pods
	far := -1
	for ti, d := range v.pods.dist {
		if d < 0 && p.Land(ti) {
			far = ti
			break
		}
	}
	if why := (targeting{v}).mirrorReason(far, command.PodArriveDrop); !strings.Contains(why, "Out of range") {
		t.Fatalf("far: %q", why)
	}
	v.pods.loadKg = 0
	if why := (targeting{v}).mirrorReason(t0, command.PodArriveDrop); !strings.Contains(why, "empty") {
		t.Fatalf("empty: %q", why)
	}
}

// The game's LaunchReason replaces the mirror.
func TestPodReasonPrefersTheGame(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	sh, v := newFake(&estGame{fakeGame: g, launch: "Nope."})
	sh.world.Buildings = []game.BuildingView{{ID: 99}}
	v.LaunchFor(99)
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := (targeting{v}).podReason(landTile(t, 3), command.PodArriveDrop); why != "Nope." {
		t.Fatalf("reason %q", why)
	}
}

// A click opens the anchored menu; a mode sends LaunchPods to the pods' map.
func TestPodMenuLaunches(t *testing.T) {
	t.Parallel()
	sh, v, _ := launchView(t)
	t0 := inRange(t, v)
	(picker{v}).clickTile(t0, 300, 200)
	if !v.pods.menu || v.pods.target != t0 {
		t.Fatalf("menu %+v", v.pods.menu)
	}
	if v.Selected() != "" {
		t.Fatalf("targeting selected %q", v.Selected())
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
	for _, m := range podModes() {
		if _, ok := sh.kit.Find("world.pod." + m.id); !ok {
			t.Fatalf("no row %s", m.id)
		}
	}
	if w, _ := sh.kit.Find("world.pod.gift"); !w.Disabled || w.Reason == "" {
		t.Fatalf("gift on open land: %+v", w)
	}
	clickWidget(t, sh, v, "world.pod.dropcentre")
	c := sh.cmds[len(sh.cmds)-1]
	if c.Kind != command.LaunchPods || c.Target != 99 || c.Count != t0 || c.Mode != command.PodArriveDropCentre || !c.MapSet || c.Map != 0 {
		t.Fatalf("launch sent %+v", c)
	}
	if v.pods.on || !v.Open() {
		t.Fatalf("after launch: targeting %v open %v", v.pods.on, v.Open())
	}
}

// Pick spot switches to our map at the target and sends the cell with the
// pods' own map.
func TestPickSpotCursor(t *testing.T) {
	t.Parallel()
	sh, v, g := launchView(t)
	t0 := inRange(t, v)
	g.maps = append(g.maps, game.MapView{ID: 4, Label: "Oak camp", Kind: "site", Tile: t0, Colonists: 1})
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	(picker{v}).clickTile(t0, 300, 200)
	frame(sh, v, -1, -1, inputframe.Raw{})
	clickWidget(t, sh, v, "world.pod.spot")
	if v.Open() || !v.Spotting() || sh.intents[len(sh.intents)-1] != (ports.SwitchMap{Map: 4}) {
		t.Fatalf("open %v spotting %v intents %v", v.Open(), v.Spotting(), sh.intents)
	}
	if len(sh.cmds) != 0 {
		t.Fatalf("launched too early: %+v", sh.cmds)
	}
	step := func(mx, my int, btn inputframe.MouseBtn, click bool) bool {
		var raw inputframe.Raw
		if click {
			raw.Press[btn] = true
		}
		sh.mx, sh.my = mx, my
		sh.f = inputframe.Frame{Raw: raw}
		return v.HandleSpot()
	}
	if !step(10, 10, inputframe.Left, true) || len(sh.cmds) != 0 {
		t.Fatal("the click that opened the cursor must not place")
	}
	if !strings.Contains(sh.hint, "Click the cell") {
		t.Fatalf("hint %q", sh.hint)
	}
	sh.chrome = []gfx.Rect{{X: 0, Y: 0, W: 20, H: 20}}
	if !step(10, 10, inputframe.Left, true) || len(sh.cmds) != 0 {
		t.Fatal("a click on the HUD places nothing")
	}
	sh.tile = [2]int{500, 500}
	if !step(200, 200, inputframe.Left, true) || len(sh.cmds) != 0 {
		t.Fatal("a click off the map places nothing")
	}
	sh.tile = [2]int{12, 34}
	if !step(200, 200, inputframe.Left, true) {
		t.Fatal("the click was not taken")
	}
	if len(sh.cmds) != 1 {
		t.Fatalf("cmds %+v", sh.cmds)
	}
	c := sh.cmds[0]
	if c.Mode != command.PodArriveSpot || c.X2 != 12 || c.Y2 != 34 || !c.MapSet || c.Map != 0 || c.Count != t0 || v.Spotting() {
		t.Fatalf("spot sent %+v, spotting %v", c, v.Spotting())
	}
	if v.HandleSpot() {
		t.Fatal("the cursor must be gone")
	}

	// Esc cancels without launching
	sh.cmds = nil
	v.spot = spotState{on: true, cmd: c}
	if !v.Escape() || v.Spotting() || len(sh.cmds) != 0 {
		t.Fatal("Esc must drop the cursor")
	}
	v.spot = spotState{on: true, cmd: c}
	if !step(1, 1, inputframe.Right, true) || v.Spotting() || len(sh.cmds) != 0 {
		t.Fatal("a right click must drop the cursor")
	}
}

// The marker widgets are real click targets under the pointer path; the
// HUD's own area keeps its clicks.
func TestMarkerWidgetsSelectThroughTheClickPath(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	sh, v := newFake(g)
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{}) // places the home marker facing the eye
	frame(sh, v, -1, -1, inputframe.Raw{})
	w, ok := sh.kit.Find("world.obj.map.0")
	if !ok {
		t.Fatalf("no marker widget; marks %d", len(v.marks))
	}
	clickWidget(t, sh, v, "world.obj.map.0")
	if v.Selected() != "map:0" {
		t.Fatalf("selected %q", v.Selected())
	}
	if v.g.Selected() != g.maps[0].Tile {
		t.Fatalf("globe selection %d", v.g.Selected())
	}
	sh.bar = w.R
	frame(sh, v, -1, -1, inputframe.Raw{})
	if _, ok := sh.kit.Find("world.obj.map.0"); ok {
		t.Fatal("a marker under the HUD must not take its clicks")
	}
}

// The object list is read again when the clock has moved far enough or
// the view asks, not every Update (a counting reader proves it).
func TestObjectsAreReadOnlyWhenNeeded(t *testing.T) {
	t.Parallel()
	g := &countingGame{fakeGame: colonyGame(t)}
	sh, v := newFake(g)
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	base := g.reads
	for range 5 {
		frame(sh, v, -1, -1, inputframe.Raw{})
	}
	if g.reads != base {
		t.Fatalf("%d reads on a paused clock", g.reads-base)
	}
	sh.tick += refreshTicks
	frame(sh, v, -1, -1, inputframe.Raw{})
	if g.reads != base+1 {
		t.Fatalf("%d reads after the clock moved", g.reads-base)
	}
	v.send(command.Command{Kind: command.CaravanReturn, Target: 7})
	frame(sh, v, -1, -1, inputframe.Raw{})
	if g.reads != base+2 {
		t.Fatalf("%d reads after a command", g.reads-base)
	}
	// a paused clock still sees the other features' commands
	for range refreshFrames {
		frame(sh, v, -1, -1, inputframe.Raw{})
	}
	if g.reads != base+3 {
		t.Fatalf("%d reads over %d frames", g.reads-base, refreshFrames)
	}
}

type countingGame struct {
	*fakeGame
	reads int
}

func (g *countingGame) Maps() []game.MapView { g.reads++; return g.fakeGame.Maps() }

func TestPodsFlyAlongTheirArc(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	home := g.maps[0].Tile
	to := landTile(t, 5)
	sh, v := newFake(g)
	sh.world.AX.Pods = []axview.PodFlight{{From: int32(home), To: int32(to), Arrive: 10000, Count: 2}}
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	i := find(v.objs, "pod:0")
	if i < 0 || v.objs[i].flight < caravan.PodTicksPerTile {
		t.Fatalf("pod object %d", i)
	}
	o := &v.objs[i]
	o.flight = 10000
	if a, b := podPoint(v.g, o, 0), v.g.Surface(home); !near3(a, b) {
		t.Fatalf("at the start %v, home %v", a, b)
	}
	if a, b := podPoint(v.g, o, 10000), v.g.Surface(to); !near3(a, b) {
		t.Fatalf("at the end %v, target %v", a, b)
	}
}

func near3(a, b [3]float32) bool {
	for i := range a {
		if d := a[i] - b[i]; d > 1e-3 || d < -1e-3 {
			return false
		}
	}
	return true
}

func (s *fakeShell) SettleReason(uint64) (string, bool) { return s.settle, s.settle != "" }
func (s *fakeShell) ArchitectOpen() bool                { return s.arch }
func (s *fakeShell) KeyHint(input.Action) string        { return " [F8]" }
func (s *fakeShell) WindowMode() (string, bool)         { return "", s.window }

// A new colony drops the old one's view.
func TestAnotherGameClosesAndForgets(t *testing.T) {
	t.Parallel()
	sh, v := newFake(colonyGame(t))
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if !v.Open() || v.g == nil {
		t.Fatal("not open")
	}
	sh.env.Game = colonyGame(t)
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.Open() || v.Selected() != "" || v.g != nil {
		t.Fatalf("open %v selected %q globe %v", v.Open(), v.Selected(), v.g != nil)
	}
}

// The inspector does not route a caravan every frame.
func TestEstimateIsCached(t *testing.T) {
	t.Parallel()
	g := &countEst{fakeGame: colonyGame(t)}
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	for range 5 {
		v.eta(7, 1, 2)
	}
	if g.n != 1 {
		t.Fatalf("%d estimates", g.n)
	}
	sh.tick += etaTicks
	v.eta(7, 1, 2)
	if g.n != 2 {
		t.Fatalf("%d estimates after the clock moved", g.n)
	}
}

type countEst struct {
	*fakeGame
	n int
}

func (g *countEst) TravelEstimate(uint64, int) (int, string) { g.n++; return clock.TicksPerHour, "" }

// peopledPlanet is a small world with factions (read-only).
var peopledPlanet = sync.OnceValue(func() *planet.Planet {
	b := planet.NewBuilder(planet.Params{SeedText: "world-view-bases", Coverage: 5, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()})
	for {
		if _, _, done := b.Step(50000); done {
			return b.Planet()
		}
	}
})

// A base the simulation has lost is still on the view's copy of the planet:
// its mark goes, a map open there is razed, the search skips it, and the
// planet stays untouched.
func TestRazedBaseLeavesTheGlobe(t *testing.T) {
	t.Parallel()
	p := peopledPlanet()
	tiles := p.SettlementTiles()
	if len(tiles) < 2 {
		t.Fatalf("%d settlements", len(tiles))
	}
	gone := int(tiles[0])
	g := colonyGame(t)
	g.params = p.Params
	g.sites = nil
	for _, ti := range tiles[1:] {
		g.sites = append(g.sites, game.WorldSiteView{Tile: int(ti), Name: p.SettlementName(int(ti))})
	}
	g.maps = append(g.maps, game.MapView{ID: 5, Label: "Ruin", Kind: "site", Tile: gone, Colonists: 2})
	sh, v := newFake(g)
	sh.env.Planet = p
	v.Show("map:5")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if len(v.g.Marks) != len(tiles)-1 {
		t.Fatalf("%d marks of %d settlements, one gone", len(v.g.Marks), len(tiles))
	}
	for i := range v.g.Marks {
		if int(v.g.Marks[i].Tile) == gone {
			t.Fatal("the razed base is still marked")
		}
	}
	o := v.selected()
	if o == nil || !o.Destroyed() {
		t.Fatalf("map on the razed base: %+v", o)
	}
	if key, _ := markLook(o); key != iconDestroyed {
		t.Fatalf("icon %q", key)
	}
	name := p.SettlementName(gone)
	if hits := (liveFinder{p, v}).Find(name); slices.ContainsFunc(hits, func(h planet.Hit) bool { return h.Tile == gone && h.Kind == planet.FeatSettlement }) {
		t.Fatalf("the search still finds it: %+v", hits)
	}
	if f, _ := p.SettlementFaction(gone); f < 0 {
		t.Fatal("the shared planet was changed")
	}
}

// The simulation's answer wins on Settle here.
func TestSettleReasonComesFromTheGame(t *testing.T) {
	t.Parallel()
	sh, v := newFake(colonyGame(t))
	sh.settle = "Only a caravan waiting at its destination can settle."
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if why := gizmoOf(t, v, "settle").why; why != sh.settle {
		t.Fatalf("settle: %q", why)
	}
}

// Attacking a settlement that is not hostile asks first and names the
// cost; a hostile one attacks at once (MAP-15). Abandoning the last colony
// with nobody out warns that it ends the game (MAP-14).
func TestAttackConfirmsAndAbandonWarns(t *testing.T) {
	t.Parallel()
	sh, v := newFake(colonyGame(t))
	in := inspector{v}
	friendly := game.WorldSiteView{Faction: "Oakhold", Relation: "friendly"}
	in.attackSettlement(7, friendly, true)
	if !sh.kit.ModalOpen() || len(sh.cmds) != 0 {
		t.Fatalf("friendly base: modal %v, commands %v", sh.kit.ModalOpen(), sh.cmds)
	}
	if txt := attackConfirmText(friendly); !strings.Contains(txt, "Oakhold") || !strings.Contains(txt, "200") || !strings.Contains(txt, "friendly") {
		t.Fatalf("confirm text %q", txt)
	}
	sh.kit.Modals = nil
	in.attackSettlement(7, game.WorldSiteView{Faction: "Ironhold", Relation: "hostile"}, true)
	if sh.kit.ModalOpen() || len(sh.cmds) != 1 || sh.cmds[0].Kind != command.AttackSettlement {
		t.Fatalf("hostile base: modal %v, commands %v", sh.kit.ModalOpen(), sh.cmds)
	}
	if !in.endsGame(game.MapView{ID: 0, Kind: "colony"}) {
		t.Fatal("the last colony with nobody out must end the game")
	}
	if in.endsGame(game.MapView{ID: 3, Kind: "site"}) {
		t.Fatal("a site map never ends the game")
	}
}
