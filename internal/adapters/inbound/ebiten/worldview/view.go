package worldview

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// Tuning (presentation only).
const (
	buildUnits = 6000 // planet builder work per Update while charting
	// refreshTicks is how often (in game ticks) the objects are read
	// again while the clock runs, refreshFrames how often (in Updates)
	// whatever the clock does (commands apply at once while paused, and
	// the other features' dialogs send them); a command the view sends or
	// opening it re-reads at once.
	refreshTicks  = 15
	refreshFrames = 30
	noMap         = "This colony has no world map."
)

// chart is the planet being shown: the colony's params, the builder
// while it is charted a slice per Update, and the finished planet.
type chart struct {
	params  planet.Params
	hasMap  bool
	builder *planet.Builder
	planet  *planet.Planet
}

// readStamp is when the objects were last read (the game tick), and
// whether something the view did asks for a fresh read.
type readStamp struct {
	tick  clock.Tick
	frame uint64
	ok    bool
	dirty bool
}

// selState is the world selection: the object key ("caravan:7", or
// "tile:<n>" for a bare tile), the last click for the double-click, and
// the caravan last selected (settlements read their distance from it).
type selState struct {
	key       string
	home      bool // select the map on screen at the next read (opening with nothing selected)
	lastFrame uint64
	lastKey   string
	caravan   uint64
	pick      int // the last tile clicked that is not one of our maps (Form caravan's destination; -1 none)
}

// toolState is the toolbar's toggles and the search box.
type toolState struct {
	search   worldinfo.SearchBox
	searchOn bool
	legendOn bool
}

// View is the world view: whether it is open, the globe, the objects and
// their markers, the selection and the modes (route, pod targeting, the
// pick-spot cursor).
type View struct {
	sh    Shell
	open  bool
	chart chart
	g     *globe.View
	objs  []Object
	live  []int // the tiles of the simulation's settlements, sorted
	marks []mark
	tiles []int // onTile scratch
	read  readStamp
	sel   selState
	route routeState
	pods  podState
	spot  spotState
	tools toolState
	card  card     // the tile card of the bare tile shown
	etaC  etaCache // the selected caravan's travel time and distance
	frame uint64   // Updates seen (the double-click window)
	hops  planet.HopScratch
	game  Reader // the colony the state above belongs to
}

// New builds the view over its shell (closed).
func New(sh Shell) *View { return &View{sh: sh, sel: selState{pick: -1}} }

func (v *View) kit() *uikit.Kit { return v.sh.Kit() }

// Open reports the world view is up (the root skips the map draw, the
// map keys and the map mouse while it is).
func (v *View) Open() bool { return v.open }

// Toggle opens the view, or closes it when it is open (F8, the World tab).
func (v *View) Toggle() {
	if v.open {
		v.Close()
		return
	}
	v.Show("")
}

// Show opens the view with object sel selected (ports.OpenWorld: ""
// keeps the selection; "caravan:<id>", "map:<id>", "settlement:<tile>",
// "site:<tile>", "pod:<n>", "tile:<n>"). The Architect and the main-tab
// windows close.
func (v *View) Show(sel string) {
	env := v.sh.Env()
	if env.Game == nil {
		return
	}
	v.follow(env)
	if !v.open {
		v.sh.Do(ports.CloseArchitect{})
		v.sh.Do(ports.CloseWindows{})
	}
	v.open = true
	v.read.dirty = true
	if src, ok := env.Game.(WorldSource); ok {
		params, has := src.WorldParams()
		if has != v.chart.hasMap || params.Key() != v.chart.params.Key() {
			v.chart, v.g = chart{params: params, hasMap: has}, nil
		}
	}
	if sel != "" {
		v.selectKey(sel)
		v.centreOnSelection()
	}
	v.sel.home = v.sel.key == ""
}

// follow drops everything of another colony when the game has changed (a
// load, a new game): the view is closed and forgets its selection and
// planet.
func (v *View) follow(env Env) {
	if env.Game == v.game {
		return
	}
	*v = View{sh: v.sh, sel: selState{pick: -1}, frame: v.frame, game: env.Game}
}

// Close returns to the map; the selection is kept for the next open, the
// route and pod modes are dropped.
func (v *View) Close() {
	v.open = false
	v.route = routeState{}
	v.pods = podState{}
}

// Escape is the view's share of the Esc chain: the pick-spot cursor,
// then the pod menu, pod targeting or route mode, then the selection,
// then the view itself. False when the view is closed and no cursor is up.
func (v *View) Escape() bool {
	if v.spot.on {
		v.cancelSpot()
		return true
	}
	if !v.open {
		return false
	}
	switch {
	case v.pods.menu:
		v.pods.menu = false
	case v.pods.on:
		v.pods = podState{}
	case v.route.on:
		v.route = routeState{}
	case v.sel.key != "":
		v.selectKey("")
	default:
		v.Close()
	}
	return true
}

// Selected is the selection key ("" none).
func (v *View) Selected() string { return v.sel.key }

// planetFor is the finished planet (charting it a slice per Update), nil
// while it is being charted or when the colony has none.
func (v *View) planetFor(env Env) *planet.Planet {
	c := &v.chart
	if c.planet != nil || !c.hasMap {
		return c.planet
	}
	if f := env.Planet; f != nil && f.Params.Key() == c.params.Key() {
		c.planet = f // the planet the colony was founded on
		return f
	}
	if c.builder == nil {
		c.builder = planet.NewBuilder(c.params)
	}
	if _, _, done := c.builder.Step(buildUnits); !done {
		return nil
	}
	c.planet, c.builder = c.builder.Planet(), nil
	return c.planet
}

// chartPct is the charting progress in percent.
func (v *View) chartPct() int {
	if v.chart.builder == nil {
		return 0
	}
	_, f, _ := v.chart.builder.Step(0)
	return int(f * 100)
}

// Declare runs once per Update: the globe's input, the objects, the
// marker widgets and, unless the UI is hidden, the inspector, toolbar,
// search, legend, route bar and pod menu.
func (v *View) Declare(sw, sh int) {
	v.frame++
	env := v.sh.Env()
	v.follow(env)
	if !v.open {
		return
	}
	if _, win := v.sh.WindowMode(); win || v.sh.ArchitectOpen() {
		v.Close() // another tab took the screen (its opening closed nothing of ours)
		return
	}
	hidden := v.sh.UIHidden()
	p := v.planetFor(env)
	if p == nil {
		if !hidden {
			(chromeBar{v}).declareTools(sw, nil)
		}
		return
	}
	if v.g == nil {
		v.g = globe.New(p, env.Art)
		v.g.Reduced = v.kit().Host().ReducedMotion()
		v.read.dirty = true
		v.refresh(env, p)
		if !v.centreOnSelection() {
			v.g.Center(v.homeTile())
		}
	}
	v.refresh(env, p)
	(targeting{v}).podRing(p)
	v.g.SetArea(0, 0, float64(sw), float64(sh))
	v.marks = placeMarks(v.marks, v.objs, v.g, uint64(v.sh.Tick()))
	v.declareMarks()
	var panes []gfx.Rect
	if !hidden {
		panes = (chromeBar{v}).declareChrome(sw, sh, p)
	}
	v.updateGlobe(sw, sh, panes)
}

// refresh reads the objects again when the clock has moved on far
// enough or the view asked for it.
func (v *View) refresh(env Env, p *planet.Planet) {
	now := v.sh.Tick()
	if v.read.ok && !v.read.dirty && now/refreshTicks == v.read.tick/refreshTicks && v.frame-v.read.frame < refreshFrames {
		return
	}
	v.read = readStamp{tick: now, frame: v.frame, ok: true}
	r := readModels{maps: env.Game.Maps(), world: env.Game.WorldMap(), ax: v.sh.World().AX, tiles: p.TileCount()}
	if src, ok := env.Game.(WorldSource); ok {
		r.sites = src.WorldSites()
	}
	v.objs = buildObjects(v.objs, r)
	v.syncLive(p)
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindPod {
			o.flight = max(1, p.HopsWith(&v.hops, int(o.Pod.From), int(o.Pod.To), caravan.PodMaxRange+1)) * caravan.PodTicksPerTile
		}
	}
	if k := v.sel.key; k != "" && !strings.HasPrefix(k, "tile:") && find(v.objs, k) < 0 {
		v.sel.key = "" // the object is gone (a caravan home, a map closed)
	}
	if v.sel.home {
		v.sel.home = false
		cur := env.Game.ViewedMap()
		for i := range v.objs {
			if o := &v.objs[i]; o.Kind == KindMap && o.Map.ID == cur {
				v.selectKey(o.Key)
				break
			}
		}
	}
}

// syncLive squares the view's planet with the simulation's settlements. The
// view's planet is a read-only copy (shared with the charting cache), so a
// base the game has lost is still on it: the globe stops marking it, a map
// open there is razed, and the search leaves it out (liveFinder).
func (v *View) syncLive(p *planet.Planet) {
	v.live = v.live[:0]
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindSettlement {
			v.live = append(v.live, o.Tile)
		}
	}
	slices.Sort(v.live)
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap {
			f, _ := p.SettlementFaction(o.Tile)
			o.razed = f >= 0 && !v.isLive(o.Tile)
		}
	}
	if v.g == nil || len(v.g.Marks) == len(v.live) {
		return
	}
	n := 0
	for i := range v.g.Marks {
		if v.isLive(int(v.g.Marks[i].Tile)) {
			v.g.Marks[n] = v.g.Marks[i]
			n++
		}
	}
	v.g.Marks = v.g.Marks[:n]
}

// isLive reports a settlement stands on tile t in the simulation.
func (v *View) isLive(t int) bool {
	_, ok := slices.BinarySearch(v.live, t)
	return ok
}

// liveFinder is the planet's name search without the bases the game has
// lost.
type liveFinder struct {
	p *planet.Planet
	v *View
}

// Find is the planet's ranked search, the razed settlements left out.
func (f liveFinder) Find(q string) []planet.Hit {
	return slices.DeleteFunc(f.p.Find(q), func(h planet.Hit) bool {
		return h.Kind == planet.FeatSettlement && !f.v.isLive(h.Tile)
	})
}

// homeTile is the tile of the map on screen (the colony's), -1 none.
func (v *View) homeTile() int {
	g := v.sh.Env().Game
	if g == nil {
		return -1
	}
	cur := g.ViewedMap()
	for i := range v.objs {
		if o := &v.objs[i]; o.Kind == KindMap && o.Map.ID == cur {
			return o.Tile
		}
	}
	return g.WorldMap().Tile
}

// declareMarks registers a click widget on every visible marker that is
// not under the HUD (journeys press world.obj.<kind>.<id>). The widgets go
// in reverse pick order, so on a shared tile the first in pick order is on
// top of the kit's hit test.
func (v *View) declareMarks() {
	k := v.kit()
	chrome := v.sh.Layout().Chrome()
	for i := len(v.marks) - 1; i >= 0; i-- {
		m := v.marks[i]
		r := markRect(m)
		if underChrome(r, chrome) {
			continue
		}
		o := &v.objs[m.obj]
		key := o.Key
		k.Add(uikit.Widget{ID: o.ID, R: r, Tip: o.Label, Click: func() { (picker{v}).clickMark(key) }})
	}
}

// underChrome reports r touches any of the HUD's rectangles.
func underChrome(r gfx.Rect, chrome []gfx.Rect) bool {
	for _, c := range chrome {
		if !r.Intersect(c).Empty() {
			return true
		}
	}
	return false
}

// updateGlobe feeds the globe this Update's input (blocked over the HUD,
// the view's panes, the markers and any dialog) and acts on its click.
func (v *View) updateGlobe(sw, sh int, panes []gfx.Rect) {
	k := v.kit()
	in := k.Host().Frame()
	mx, my := v.sh.Cursor()
	blocked := k.ModalOpen() || v.sh.ChromeHit(mx, my) || nearest(v.marks, mx, my) >= 0 ||
		in.Raw.Press[inputframe.Left] && !in.Clicked(inputframe.Left) // a dialog's button used this press, then closed
	for _, r := range panes {
		blocked = blocked || r.Hit(mx, my)
	}
	gi := globe.Input{
		W: sw, H: sh, MX: in.Raw.Mx, MY: in.Raw.My, Wheel: in.Raw.WheelY,
		Down: in.Raw.Down, Press: in.Raw.Press, Release: in.Raw.Release,
		Held: in.Raw.Held, Keys: in.Raw.Pressed, Shift: in.Shift(), Blocked: blocked,
	}
	if k.Editing() || k.ModalOpen() {
		gi.Held, gi.Keys = nil, nil // typing in the search, or a dialog: the keys are not the camera's
	}
	rightClick := in.Raw.Press[inputframe.Right] && !blocked
	clicked, _ := v.g.Update(gi, k.Host().ReducedMotion())
	switch {
	case clicked >= 0:
		(picker{v}).clickTile(clicked, mx, my)
	case clicked == globe.ModeUncovered && !v.pods.on && !v.route.on:
		v.selectKey("")
	case rightClick:
		(picker{v}).cancelMode()
	}
	v.g.Select(v.selTile())
	if v.pods.on && len(v.pods.ring) > 0 {
		v.g.Highlight(v.pods.ring, 2) // the range ring, a tinted overlay on the relief
	}
}

// cancelMode is a right click on the globe: the pod menu, pod targeting
// or the route, else the selection.
func (v picker) cancelMode() {
	switch {
	case v.pods.menu:
		v.pods.menu = false
	case v.pods.on:
		v.pods = podState{}
	case v.route.on:
		v.route = routeState{}
	default:
		v.selectKey("")
	}
}

// clickTile is a click on globe tile t (no marker under the pointer):
// a target in the route and pod modes, else the tile's first object or
// the bare tile.
func (v picker) clickTile(t, mx, my int) {
	switch {
	case v.pods.on:
		(targeting{v.View}).openPodMenu(t, float32(mx), float32(my))
		return
	case v.route.on:
		(router{v.View}).setRouteTarget(t)
		return
	}
	v.notePick(t)
	if v.tiles = onTile(v.tiles, v.objs, t); len(v.tiles) > 0 {
		v.pickObject(v.tiles[0])
		return
	}
	v.selectKey("tile:" + strconv.Itoa(t))
}

// clickMark is a click on a marker widget: the nearest marker to the
// pointer when another one is strictly closer than the widget's own.
func (v picker) clickMark(key string) {
	o := find(v.objs, key)
	if o < 0 {
		return
	}
	mx, my := v.sh.Cursor()
	if i := nearest(v.marks, mx, my); i >= 0 && v.marks[i].obj != o && !v.sameSpot(v.marks[i], o) {
		o = v.marks[i].obj
	}
	mx, my = v.markAt(o, mx, my)
	if v.pods.on {
		(targeting{v.View}).openPodMenu(v.objs[o].Tile, float32(mx), float32(my))
		return
	}
	if v.route.on {
		(router{v.View}).setRouteTarget(v.objs[o].Tile)
		return
	}
	v.notePick(v.objs[o].Tile)
	v.pickObject(o)
}

// sameSpot reports m and object o's marker are at the same place.
func (v picker) sameSpot(m mark, o int) bool {
	for _, n := range v.marks {
		if n.obj == o {
			return n.x == m.x && n.y == m.y
		}
	}
	return false
}

// markAt is object o's marker position (logical), or (mx, my) when it
// has none this frame.
func (v picker) markAt(o, mx, my int) (int, int) {
	for _, m := range v.marks {
		if m.obj == o {
			return int(m.x), int(m.y)
		}
	}
	return mx, my
}

// notePick remembers t as the Form caravan destination unless it is one
// of our maps.
func (v picker) notePick(t int) {
	v.tiles = onTile(v.tiles, v.objs, t)
	for _, i := range v.tiles {
		if v.objs[i].Kind == KindMap {
			return
		}
	}
	v.sel.pick = t
}

// pickObject selects object o: a second click on our selected map within
// the double-click window enters it; another click on the selected
// object's tile cycles through the objects there.
func (v picker) pickObject(o int) {
	obj := &v.objs[o]
	double := v.frame-v.sel.lastFrame <= globe.DoubleFrames && v.sel.lastKey == obj.Key
	v.sel.lastFrame, v.sel.lastKey = v.frame, obj.Key
	cur := find(v.objs, v.sel.key)
	if double && cur >= 0 && v.objs[cur].Kind == KindMap {
		v.jump(v.objs[cur].Map.ID)
		return
	}
	if cur >= 0 && obj.Kind != KindPod && v.objs[cur].Kind != KindPod && v.objs[cur].Tile == obj.Tile {
		v.tiles = onTile(v.tiles, v.objs, obj.Tile)
		for n, i := range v.tiles {
			if i == cur {
				next := v.tiles[(n+1)%len(v.tiles)]
				v.sel.lastKey = v.objs[next].Key
				v.selectKey(v.objs[next].Key)
				return
			}
		}
	}
	v.selectKey(obj.Key)
}

// selectKey sets the selection (and the caravan settlements measure from).
func (v *View) selectKey(k string) {
	v.sel.key = k
	if i := find(v.objs, k); i >= 0 && v.objs[i].Kind == KindCaravan {
		v.sel.caravan = v.objs[i].Caravan.ID
	}
}

// selected is the selected object, nil for none or a bare tile.
func (v *View) selected() *Object {
	if i := find(v.objs, v.sel.key); i >= 0 {
		return &v.objs[i]
	}
	return nil
}

// selTile is the selection's tile, -1 none.
func (v *View) selTile() int {
	if o := v.selected(); o != nil {
		return o.Tile
	}
	if t, ok := strings.CutPrefix(v.sel.key, "tile:"); ok {
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return -1
}

// centreOnSelection flies the globe to the selection; false when there
// is no globe or no selection.
func (v *View) centreOnSelection() bool {
	t := v.selTile()
	if v.g == nil || t < 0 {
		return false
	}
	v.g.Center(t)
	return true
}

// jump shows map m and closes the view.
func (v *View) jump(m game.MapID) {
	v.sh.Do(ports.SwitchMap{Map: m})
	v.Close()
}

// send enqueues a command and asks for a fresh read.
func (v *View) send(c command.Command) {
	v.sh.Enqueue(c)
	v.read.dirty = true
}

// DrawGlobe paints the world view in place of the map (before the HUD):
// space, the globe, the route line and the markers, or a note while there
// is no globe. A closed view draws nothing.
func (v *View) DrawGlobe(dst *ebiten.Image, sw, sh int) {
	if !v.open {
		return
	}
	if v.g == nil {
		uikit.FillRect(dst, gfx.WinRect(sw, sh), uikit.Space)
		if !v.sh.UIHidden() {
			msg := noMap
			if v.chart.hasMap {
				msg = fmt.Sprintf("Charting the world… %d%%", v.chartPct())
			}
			gfx.Text(dst, msg, float64(sw)/2-120, float64(sh)/2, uikit.DimText())
		}
		return
	}
	v.g.Draw(dst, sw, sh)
	(router{v}).drawRoute(dst)
	v.drawMarks(dst)
}
