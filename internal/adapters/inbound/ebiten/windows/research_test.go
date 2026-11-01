package windows

import (
	"strconv"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// RSCH-2: the research tab's tech tree. The views are faked from the
// project table (the simulation fills the RSCH-0 fields separately), so
// the tests hold whether or not those fields are set yet.

// treeViews are the table's projects as views: done ones finished, the
// rest available once their prerequisites are done; cur is under way.
func treeViews(done map[research.Project]bool, cur research.Project) []game.ResearchView {
	var out []game.ResearchView
	for _, d := range research.All() {
		v := game.ResearchView{ID: uint8(d.Project), Name: d.Name, Label: d.Label, Desc: d.Desc, Cost: d.Cost, Done: done[d.Project],
			Current: d.Project == cur && !done[d.Project]}
		if v.Done {
			v.Progress = d.Cost
		}
		for _, r := range append(append([]research.Project(nil), d.Requires...), d.Hidden...) {
			if !done[r] {
				v.Requires = append(v.Requires, research.DefOf(r).Label)
			}
		}
		v.Available = !v.Done && len(v.Requires) == 0
		for _, r := range d.Requires { // the drawn prerequisites, as the game fills them
			v.Prereqs = append(v.Prereqs, int(r))
		}
		for _, u := range d.Unlocks {
			_, name, _ := strings.Cut(u, ":")
			v.Unlocks = append(v.Unlocks, name)
		}
		out = append(out, v)
	}
	return out
}

// treeWindow is the Research window open over views.
func treeWindow(t *testing.T, views []game.ResearchView) (*fakeShell, Research) {
	t.Helper()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.Research = views
	w.Show("research")
	sh.f = inputframe.NewFrame(inputframe.Raw{Mx: -1, My: -1})
	w.kit().Begin()
	w.Research().Declare(1280, 720)
	return sh, w.Research()
}

// nodeOf is the project's node index in the window's tree.
func nodeOf(t *testing.T, w Research, p research.Project) int {
	t.Helper()
	for i, n := range w.rs().nodes {
		if n.id == uint8(p) {
			return i
		}
	}
	t.Fatalf("no node for %v", p)
	return -1
}

func TestResearchLayoutColumnsFollowLevelsAndPrerequisites(t *testing.T) {
	t.Parallel()
	// 0 -> 1 -> 2 at level 0; 3 at level 2 needs 2; 4 at level 2 needs 3; 5 at level 0, alone
	level := []int{0, 0, 0, 2, 2, 0}
	pre := [][]int{nil, {0}, {1}, {2}, {3}, nil}
	g := layoutTree(level, pre)
	if want := []int{0, 1, 2, 3, 4, 0}; !equalInts(g.Col, want) {
		t.Fatalf("columns %v, want %v", g.Col, want)
	}
	if len(g.Bands) != 2 || g.Bands[0] != (treeBand{Level: 0, Col0: 0, Col1: 3}) || g.Bands[1] != (treeBand{Level: 2, Col0: 3, Col1: 5}) {
		t.Fatalf("bands %+v", g.Bands)
	}
	if g.Cols != 5 || g.Rows != 2 {
		t.Fatalf("grid %dx%d", g.Cols, g.Rows)
	}
	// the chain is one straight row; the loner gets its own
	if g.Row[0] != g.Row[1] || g.Row[1] != g.Row[2] || g.Row[2] != g.Row[3] || g.Row[5] == g.Row[0] {
		t.Fatalf("rows %v", g.Row)
	}
	// no two nodes share a cell
	seen := map[[2]int]bool{}
	for i := range level {
		c := [2]int{g.Col[i], g.Row[i]}
		if seen[c] {
			t.Fatalf("two nodes in cell %v: %v %v", c, g.Col, g.Row)
		}
		seen[c] = true
	}
	// a cycle or a bad index does not hang or panic
	g = layoutTree([]int{0, 0}, [][]int{{1}, {0, 7, -1}})
	if len(g.Col) != 2 {
		t.Fatal("cycle layout")
	}
}

func TestResearchLayoutUncrossesWhenCheap(t *testing.T) {
	t.Parallel()
	// roots A(0) B(1); in table order the child of B (2) comes before the
	// child of A (3), which crosses the lines; so do the grandchildren.
	level := []int{0, 0, 0, 0, 0, 0}
	pre := [][]int{nil, nil, {1}, {0}, {3}, {2}}
	naive := countCrossings([]int{0, 0, 1, 1, 2, 2}, func(i int) float64 { return float64(i % 2) }, pre)
	if naive == 0 {
		t.Fatal("the test graph should cross in table order")
	}
	g := layoutTree(level, pre)
	if x := g.crossings(pre); x != 0 {
		t.Fatalf("%d crossings left: cols %v rows %v", x, g.Col, g.Row)
	}
}

// On the real table the layout never crosses more than the table order.
func TestResearchLayoutRealTableNoWorseThanTableOrder(t *testing.T) {
	t.Parallel()
	_, w := treeWindow(t, treeViews(nil, research.NumProjects))
	st := w.rs()
	pre := make([][]int, len(st.nodes))
	for i := range st.nodes {
		pre[i] = st.nodes[i].pre
	}
	idx := map[int]int{}
	naiveRow := make([]float64, len(st.nodes))
	for i := range st.nodes {
		naiveRow[i] = float64(idx[st.g.Col[i]])
		idx[st.g.Col[i]]++
	}
	naive := countCrossings(st.g.Col, func(i int) float64 { return naiveRow[i] }, pre)
	if got := st.g.crossings(pre); got > naive {
		t.Fatalf("layout crosses %d lines, the table order %d", got, naive)
	}
	for i := range st.nodes {
		for _, p := range st.nodes[i].pre {
			if st.g.Col[p] >= st.g.Col[i] {
				t.Errorf("%d sits left of or with its prerequisite %d", i, p)
			}
		}
	}
}

// deepProject is a project whose prerequisite has a prerequisite of its
// own (the table is the simulation's, so the test finds one).
func deepProject(t *testing.T) (p, mid, root research.Project) {
	t.Helper()
	for _, d := range research.All() {
		for _, m := range d.Requires {
			if r := research.DefOf(m).Requires; len(r) > 0 {
				return d.Project, m, r[0]
			}
		}
	}
	t.Skip("no project two prerequisites deep")
	return
}

func TestResearchTreeSelectionLightsThePrerequisiteChain(t *testing.T) {
	t.Parallel()
	p, mid, root := deepProject(t)
	done := map[research.Project]bool{} // everything mid needs, so mid can start
	var need func(q research.Project)
	need = func(q research.Project) {
		d := research.DefOf(q)
		for _, r := range append(append([]research.Project(nil), d.Requires...), d.Hidden...) {
			if !done[r] {
				done[r] = true
				need(r)
			}
		}
	}
	need(mid)
	sh, w := treeWindow(t, treeViews(done, research.NumProjects))
	pi, mi, ri := nodeOf(t, w, p), nodeOf(t, w, mid), nodeOf(t, w, root)
	w.Pick("research.pick." + strconv.Itoa(int(p)))
	st := w.rs()
	if st.sel != pi || !st.chain[mi] || !st.chain[ri] || st.chain[pi] {
		t.Fatalf("selected %d (want %d), chain %v %v %v", st.sel, pi, st.chain[mi], st.chain[ri], st.chain[pi])
	}
	// a locked project shows why it cannot start, and Start sends nothing
	st.buildPanel(200)
	if !st.panel.startOff || !strings.Contains(st.panel.startReason, research.DefOf(mid).Label) {
		t.Fatalf("start %q off %v", st.panel.startReason, st.panel.startOff)
	}
	w.start()
	w.finish()
	if len(sh.cmds) != 0 {
		t.Fatalf("a locked project sent %+v", sh.cmds)
	}
	// the panel lists the prerequisite as a link that jumps to it
	var link *panelRow
	for i := range st.panel.rows {
		if st.panel.rows[i].link == mi {
			link = &st.panel.rows[i]
		}
	}
	if link == nil || link.id != researchLinkPfx+strconv.Itoa(int(mid)) {
		t.Fatalf("no link to %v in %+v", mid, st.panel.rows)
	}
	link.click()
	if st.sel != mi {
		t.Fatalf("the link selected %d, want %d", st.sel, mi)
	}
	if r := st.nodeAt(st.view, mi); !st.view.Contains(r.X+r.W/2, r.Y+r.H/2) {
		t.Fatalf("%v at %+v is not in view %+v after the jump", mid, r, st.view)
	}
	if v := st.nodes[mi].v; !v.Available {
		t.Fatalf("%v is not available with %v done", mid, done)
	}
	// Start research on an available project sends SetResearch
	w.start()
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetResearch || sh.cmds[0].Build != uint8(mid) {
		t.Fatalf("start sent %+v", sh.cmds)
	}
	// Finish now: made current first, then finished
	sh.cmds = nil
	w.finish()
	if len(sh.cmds) != 2 || sh.cmds[0].Kind != command.SetResearch || sh.cmds[1].Kind != command.DevFinishResearch || sh.cmds[1].Build != uint8(mid) {
		t.Fatalf("finish sent %+v", sh.cmds)
	}
	// a done project's box still selects, and its button says so
	w.Pick("research.pick." + strconv.Itoa(int(root)))
	st.buildPanel(200)
	if st.sel != ri || !st.panel.startOff || st.panel.startLabel != "Finished" {
		t.Fatalf("done project: sel %d start %q off %v", st.sel, st.panel.startLabel, st.panel.startOff)
	}
}

func TestResearchTreeOpensOnTheCurrentProject(t *testing.T) {
	t.Parallel()
	last := research.Project(research.NumProjects - 1)
	_, w := treeWindow(t, treeViews(nil, last))
	st := w.rs()
	i := nodeOf(t, w, last)
	if st.sel != i {
		t.Fatalf("selected %d, want the current project %d", st.sel, i)
	}
	if r := st.nodeAt(st.view, i); r.Intersect(st.view).Empty() {
		t.Fatalf("the current project %+v is out of view %+v", r, st.view)
	}
}

func TestResearchSearchDimsNonMatches(t *testing.T) {
	t.Parallel()
	_, w := treeWindow(t, treeViews(nil, research.NumProjects))
	st := w.rs()
	gunLabel := research.DefOf(research.Gunsmithing).Label
	st.query = " " + strings.ToUpper(gunLabel) + " "
	st.searchAgain()
	gun, other := nodeOf(t, w, research.Gunsmithing), nodeOf(t, w, research.FoodPreservation)
	if st.matches < 1 || !st.match[gun] || st.match[other] {
		t.Fatalf("matches %d gun %v other %v", st.matches, st.match[gun], st.match[other])
	}
	if f := st.firstMatch(); f < 0 || !st.match[f] {
		t.Fatal("no first match")
	}
	st.query = ""
	st.searchAgain()
	if st.matches != 0 || !st.match[gun] || !st.match[other] {
		t.Fatal("an empty search dims something")
	}
}

// The graph pans with the wheel and a drag on empty space, and stays over
// its content.
func TestResearchTreePans(t *testing.T) {
	t.Parallel()
	_, w := treeWindow(t, treeViews(nil, research.NumProjects))
	st := w.rs()
	v := st.view
	st.panX, st.panY = 0, 0
	mx, my := int(v.Right()-2), int(v.Bottom()-2) // a corner, clear of boxes
	if st.hit(v, mx, my) >= 0 {
		t.Skip("a box covers the corner")
	}
	fr := inputframe.NewFrame(inputframe.Raw{Mx: mx, My: my, WheelY: -1})
	st.pan(&fr, v, mx, my, false)
	if st.cw > v.W && st.ch <= v.H && st.panX != rtWheel {
		t.Fatalf("the wheel on a one-view-tall graph pans sideways: %v,%v", st.panX, st.panY)
	}
	if fr.Raw.WheelY != 0 {
		t.Fatal("the wheel was not consumed")
	}
	st.panX, st.panY = 0, 0
	raw := inputframe.Raw{Mx: mx, My: my}
	raw.Press[inputframe.Left], raw.Down[inputframe.Left] = true, true
	fr = inputframe.NewFrame(raw)
	st.pan(&fr, v, mx, my, false)
	if !st.drag.on || fr.Clicked(inputframe.Left) {
		t.Fatal("a press on empty space does not start a drag")
	}
	raw.Press[inputframe.Left] = false
	fr = inputframe.NewFrame(raw)
	st.pan(&fr, v, mx-40, my, false)
	if st.cw > v.W && st.panX != 40 {
		t.Fatalf("dragging 40 px left pans %v", st.panX)
	}
	st.pan(&fr, v, mx-1e6, my, false)
	if st.panX > st.cw-v.W+0.01 {
		t.Fatalf("the pan ran past the content: %v > %v", st.panX, st.cw-v.W)
	}
	raw.Down[inputframe.Left] = false
	fr = inputframe.NewFrame(raw)
	st.pan(&fr, v, mx, my, false)
	if st.drag.on {
		t.Fatal("the drag outlived the button")
	}
}

// The layout is made once per change of the projects: new progress keeps
// it, and an open window's frame allocates nothing for the tree.
func TestResearchTreeCachesAndDoesNotAllocatePerFrame(t *testing.T) {
	views := treeViews(nil, research.Traps)
	sh, w := treeWindow(t, views)
	st := w.rs()
	col := &st.g.Col[0]
	next := append([]game.ResearchView(nil), views...)
	next[research.Traps].Progress = 100
	sh.world.Research = next
	w.Buttons()
	if &st.g.Col[0] != col {
		t.Fatal("new progress laid the tree out again")
	}
	if !strings.Contains(st.nodes[research.Traps].sub, "%") {
		t.Fatalf("Traps' line %q missed the progress", st.nodes[research.Traps].sub)
	}
	// the window's own per-frame work: the caches' check, the geometry,
	// the boxes as buttons and the hit test (the canvas primitives that
	// draw them allocate on their own, as everywhere in the HUD)
	if n := testing.AllocsPerRun(20, func() {
		w.sync(st)
		g := w.geometry()
		w.Buttons()
		st.hit(g.view, 400, 400)
		st.clampPan(g.view)
	}); n > 0 {
		t.Fatalf("a frame allocates %v times", n)
	}
	img := ebiten.NewImage(1280, 720)
	st.drawTree(img, w.geometry(), 400, 400) // draws without panicking
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The "X / Y" line rescales base-cost progress to the colony's cost, and
// BenchMissing's constants pick the red line (bench or facility).
func TestResearchPanelCostAndMissingBench(t *testing.T) {
	t.Parallel()
	v := game.ResearchView{ID: 1, Label: "Smithing", Cost: 1000, CostNow: 1500, Progress: 400, Available: true,
		Bench: "hi-tech research bench + multi-analyzer", BenchMissing: rlab.MissingAnalyzer}
	_, w := treeWindow(t, []game.ResearchView{v})
	st := w.rs()
	st.selectNode(0)
	st.buildPanel(260)
	var prog, bench, fac, why panelRow
	for _, r := range st.panel.rows {
		switch {
		case strings.HasPrefix(r.text, "Progress:"):
			prog = r
		case strings.HasPrefix(r.text, "Bench:"):
			bench = r
		case strings.HasPrefix(r.text, "Facility:"):
			fac = r
		case strings.HasPrefix(r.text, "Needs a powered"):
			why = r
		}
	}
	if prog.text != "Progress: 600 / 1500 (40%)" {
		t.Fatalf("progress line %q", prog.text)
	}
	if bench.st.Color == panelRed() || fac.st.Color != panelRed() || why.text == "" {
		t.Fatalf("bench %q %v, facility %q %v, reason %q", bench.text, bench.st.Color, fac.text, fac.st.Color, why.text)
	}
	if st.nodes[0].sub != "40% of 1500" {
		t.Fatalf("box line %q", st.nodes[0].sub)
	}
	// a view list without Prereqs draws no lines
	if len(st.nodes[0].pre) != 0 {
		t.Fatal("lines without Prereqs")
	}
}
