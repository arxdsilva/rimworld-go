package windows

import (
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
)

// Research tab (RSCH-2, MAIN_UI §24; wiki: Research): RimWorld's research
// screen. The right side is the tech tree: every project a box in its tech
// level's columns, prerequisite lines between them (hidden prerequisites
// are not drawn), done boxes green, available ones dark, unavailable ones
// grey and the project under way outlined with its progress filling it.
// The graph pans with a drag on empty space (or a middle drag), the wheel
// and Shift+wheel; the search field dims the boxes that do not match and
// Enter jumps to the first match. Clicking a box selects it: its
// prerequisite chain lights up and the left panel shows it (research_panel
// .go), with Start research (command.SetResearch) and, in developer mode,
// Finish now (command.DevFinishResearch). Colonists with Research work
// then study it at a bench that can work it. There is no queue (vanilla):
// research stops when a project finishes.
//
// research_tree.go keeps the state and caches, research_layout.go lays
// the prerequisite graph out.

// The window's wanted height is researchHeightShare of the room above the
// tab bar, at least researchMinH (UIS-065: 731 of 1046 measured, one data
// point); its width is the room left of the right column.
const (
	researchHeightShare = 0.68
	researchMinH        = 480
)

// LockedBy is the research project still gating an unlock key ("" if none).
func (w Research) LockedBy(key string) string {
	if !w.live() {
		return ""
	}
	return w.sh.World().Locked[key]
}

// want is the Research window's wanted size.
func (w Research) want() (float32, float32) {
	sw, sh := w.sh.Size()
	return float32(sw - hudlayout.RightW - hudlayout.HudMargin), max(researchMinH, researchHeightShare*float32(sh-hudlayout.TabsH))
}

// storyReader is the optional reader of the colony's story settings (a
// commitment colony has no developer tools).
type storyReader interface{ Story() game.StoryView }

// devMode reports developer mode is on (Finish now shows): the setting,
// and not a commitment colony.
func (w Research) devMode() bool {
	s, ok := w.sh.(ports.Settings)
	if !ok || !s.Settings().Gameplay.DevMode {
		return false
	}
	if sr, ok := w.sh.Env().Game.(storyReader); ok && sr.Story().Commitment {
		return false
	}
	return true
}

// geometry is the window's regions this frame.
func (w Research) geometry() researchGeom {
	r := w.Rect()
	return researchGeometry(gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, w.devMode())
}

// sync brings the caches up to this frame's research list and buildings.
func (w Research) sync(st *researchState) {
	world := w.sh.World()
	st.refresh(world.Research)
	bs := world.Buildings
	if !st.benchSrc.same(bs) {
		st.hasBench = false
		for _, b := range bs {
			if isResearchBench(b.Kind) {
				st.hasBench = true
				break
			}
		}
	}
}

// Declare declares the window's widgets once per Update: the search field,
// the graph (its wheel and drag pan it) and the panel's links and buttons.
// A fresh opening selects the project under way (else the first available
// one) and centres the graph on it.
func (w Research) Declare(sw, sh int) {
	st := w.rs()
	if !w.live() || !w.Open || w.Mode != "research" {
		st.close()
		return
	}
	r := w.Rect()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	w.sync(st)
	g := w.geometry()
	st.view = g.view
	if !st.open {
		st.opened(g.view)
	}
	k := w.kit()
	fr := k.Host().Frame()
	mx, my := w.sh.Cursor()
	modal := len(k.Modals) > 0
	if !g.search.Empty() {
		before := st.query
		changed, committed := k.TextField(researchSearchID, g.search, &st.query, uikit.TextFieldSpec{MaxLen: 32, Placeholder: w.sh.T("Search")})
		// typing edits st.query in the kit's text input, before this call,
		// so the value alone cannot tell a change: trust the field's events
		if changed || committed || st.query != before {
			st.searchAgain()
		}
		if committed {
			if i := st.firstMatch(); i >= 0 {
				st.jump(i)
			}
		}
	}
	k.Add(uikit.Widget{ID: researchTreeID, R: g.view, Wheel: true})
	st.pan(fr, g.view, mx, my, modal)
	w.declarePanel(st, g, w.devMode(), fr, mx, my)
}

// Buttons are the graph's boxes shown in the view (research.pick.<id>),
// each clipped to it; a click selects the project. The slice is reused
// from frame to frame.
func (w Research) Buttons() []uikit.Button {
	st := w.rs()
	if !w.live() {
		return nil
	}
	w.sync(st)
	g := w.geometry()
	st.btns = st.btns[:0]
	for i := range st.nodes {
		n := &st.nodes[i]
		r := st.nodeAt(g.view, i).Intersect(g.view)
		if r.Empty() {
			continue
		}
		st.btns = append(st.btns, uikit.Button{ID: n.idStr, X: r.X, Y: r.Y, W: r.W, H: r.H, Label: n.v.Label, Tip: n.tip,
			Selected: i == st.sel})
	}
	return st.btns
}

// Pick handles a research.pick.<id> click: select that project.
func (w Research) Pick(id string) {
	n, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixResearchPick))
	if err != nil || !w.live() {
		return
	}
	st := w.rs()
	w.sync(st)
	for i := range st.nodes {
		if int(st.nodes[i].id) == n {
			st.selectNode(i)
			return
		}
	}
}

// Selected is the selected project's id (ok false: none).
func (w Research) Selected() (uint8, bool) {
	st := w.rs()
	if st.sel < 0 || st.sel >= len(st.nodes) {
		return 0, false
	}
	return st.nodes[st.sel].id, true
}

func (w Research) draw(screen *ebiten.Image, sw, sh int) {
	st := w.rs()
	w.sync(st)
	r := w.Rect()
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	g := w.geometry()
	mx, my := w.sh.Cursor()
	pad := float64(hudlayout.WindowPad)
	gfx.Text(screen, w.sh.T("Research"), float64(r.X)+pad, float64(r.Y)+pad, gfx.WithColor(gfx.Label, uikit.Heading))
	hintX := float64(r.X) + pad + float64(gfx.TextW(w.sh.T("Research"), gfx.Label)) + 16
	hint, c := w.headline(st)
	if right := float64(r.X+r.W) - pad; right-hintX > 40 {
		gfx.Text(screen, gfx.Ellipsize(hint, gfx.Small, right-hintX), hintX, gfx.CenterTop(float64(r.Y)+pad, gfx.LineH(gfx.Label), gfx.Small),
			gfx.WithColor(gfx.Small, c))
	}
	if st.qlow != "" && !g.search.Empty() {
		s := w.sh.T("no match")
		if st.matches > 0 {
			s = lang.Tf(w.sh, "%d found", st.matches)
		}
		if tw := gfx.TextW(s, gfx.Small); g.search.X-tw-8 > g.right.X {
			gfx.Text(screen, s, float64(g.search.X-tw-8), gfx.CenterTop(float64(g.search.Y), rtSearchH, gfx.Small),
				gfx.WithColor(gfx.Small, uikit.TextDim))
		}
	}
	st.drawTree(screen, g, mx, my)
	w.drawPanel(screen, st, g, w.devMode(), mx, my)
}

// headline is the title row's note: no bench, nothing picked, or the
// project under way.
func (w Research) headline(st *researchState) (string, color.RGBA) {
	if !st.hasBench {
		return w.sh.T("No research bench yet: build one from Architect > Production."), uikit.Warning
	}
	for i := range st.nodes {
		if v := st.nodes[i].v; v.Current {
			return st.nodes[i].sub, uikit.TextDim
		}
	}
	return w.sh.T("No project under way: select one and press Start research."), uikit.Warning
}

// isResearchBench reports the building kind is a research bench of any
// tier (its recipe bench is the research bench).
func isResearchBench(kind string) bool {
	if k, ok := construction.Parse(kind); ok {
		return construction.RecipeBench(k) == construction.ResearchBench
	}
	return false
}

// HasBench reports a research bench of any tier stands.
func (w Research) HasBench() bool {
	if !w.live() {
		return false
	}
	st := w.rs()
	w.sync(st)
	return st.hasBench
}
