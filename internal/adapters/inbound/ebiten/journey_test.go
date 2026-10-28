package ebiten

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Player journeys (QA-001). A journey drives a real colony the way a player
// does: keys and mouse through fakeInput into App.Update, frames through
// App.Draw, and asserts the command that left the client or the read model
// after it. Nothing here calls a handler, a click dispatcher or the
// renderer directly (TestJourneysUseTheRealInputPath), so a key or a
// button that stops reaching its handler fails here. Every input action and
// every command kind the client can emit needs one
// (TestEveryInputActionHasAJourney).
//
// Conventions: the colony is a 24x24 game.New map (no fog, no intro
// letter), paused, with the inputColony camera: tile (x, y) sits at screen
// (64x, 64y+32), rows 0..8 and columns 6..19 are open map clear of the HUD.
// The wall clock is pinned (a.now), so Draws step the camera by the 8 ms
// the journey adds and the Update budget never holds a tick back.

type journey struct {
	t     *testing.T
	a     *App
	g     *game.Game
	f     *fakeInput
	clock time.Time
	img   *ebiten.Image
	cmds  []command.Command
}

type jCfg struct {
	w, h  int
	pawns [][2]int
	game  *game.Game
}

type jOpt func(*jCfg)

// jSize is the map's side lengths (game.New).
func jSize(w, h int) jOpt { return func(c *jCfg) { c.w, c.h = w, h } }

// jPawns places colonists at these tiles (default: one at (8, 4)).
func jPawns(at ...[2]int) jOpt { return func(c *jCfg) { c.pawns = at } }

// jGame runs the journey on a ready-made game (a scenario) instead of a
// fresh one; jSize and jPawns are ignored then.
func jGame(g *game.Game) jOpt { return func(c *jCfg) { c.game = g } }

// newJourney starts a paused colony with fake input and a pinned clock.
func newJourney(t *testing.T, opts ...jOpt) *journey {
	t.Helper()
	c := jCfg{w: 24, h: 24, pawns: [][2]int{{8, 4}}}
	for _, o := range opts {
		o(&c)
	}
	g := c.game
	if g == nil {
		g = game.New(game.Config{Seed: 1, Width: c.w, Height: c.h})
		for _, p := range c.pawns {
			if _, err := g.SpawnPawn(p[0], p[1]); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := testApp(g)
	a.finishBoot()
	a.startGame()
	if a.renderer == nil || a.nav.screen() != screenColony {
		t.Fatalf("no colony: screen %s, %s", a.nav.screen(), a.sessionErr)
	}
	j := &journey{t: t, a: a, g: g, clock: time.Unix(1000, 0)}
	a.now = func() time.Time { return j.clock }
	j.f = fakeOn(a)
	j.f.Cur.Focused = true
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	a.renderer.speed = speedPaused // before the settling frame, so no tick moves a pawn off its spawn tile
	j.f.Frame(t, a)                // settles the session: the camera is placed, the learn hook installed
	r := a.renderer
	prev := r.onEnqueue
	r.onEnqueue = func(cmd command.Command) {
		if prev != nil {
			prev(cmd)
		}
		j.cmds = append(j.cmds, cmd)
	}
	r.smooth.Instant = false
	r.speed = speedPaused
	if c.game == nil {
		r.cam.Zoom, r.cam.OffsetX, r.cam.OffsetY, r.cam.Placed = 1, 0, -32, true
	}
	return j
}

// hold keeps keys down until release; the next Update reads them.
func (j *journey) hold(keys ...string) { j.f.Cur.Held = append([]string(nil), keys...) }

// release lets every held key and modifier go.
func (j *journey) release() { j.f.Cur.Held, j.f.Cur.Mods = nil, 0 }

// key presses a key for one Update.
func (j *journey) key(name string, mods input.Mod) {
	j.t.Helper()
	j.f.Key(j.t, j.a, name, mods)
}

// typeText types s one character per Update.
func (j *journey) typeText(s string) {
	j.t.Helper()
	j.f.TypeText(j.t, j.a, s)
}

// update runs n Updates.
func (j *journey) update(n int) {
	j.t.Helper()
	for i := 0; i < n; i++ {
		j.f.Frame(j.t, j.a)
	}
}

// draw runs n Draws 8 ms apart (a 120 Hz display). The first Draw of a
// journey only primes the per-Draw camera step.
func (j *journey) draw(n int) {
	j.t.Helper()
	if j.img == nil {
		j.img = ebiten.NewImage(j.a.sw, j.a.sh)
	}
	for i := 0; i < n; i++ {
		j.clock = j.clock.Add(8 * time.Millisecond)
		j.a.Draw(j.img)
	}
}

// tick runs the colony at normal speed for exactly n ticks and pauses it
// again, through the speed keys.
func (j *journey) tick(n int) {
	j.t.Helper()
	before := j.g.Tick()
	j.key("Digit1", 0)
	for i := 0; i < n*2+2 && int(j.g.Tick()-before) < n; i++ {
		j.update(1)
	}
	j.key("Space", 0)
	if got := int(j.g.Tick() - before); got != n {
		j.t.Fatalf("tick(%d) ran %d ticks", n, got)
	}
}

// lookAt centres the camera on a tile (presentation-only, as a jump would).
func (j *journey) lookAt(tx, ty int) {
	v := j.a.mapViewport()
	j.a.renderer.centerOnView(float64(tx)+0.5, float64(ty)+0.5, v, v)
}

// clickTile clicks a tile's centre with btn; the tile must not be under
// the HUD.
func (j *journey) clickTile(x, y int, btn mouseBtn, mods input.Mod) {
	j.t.Helper()
	j.f.MoveTo(j.a, x, y)
	if j.a.colonyChromeHit(j.f.Cur.Mx, j.f.Cur.My, j.a.sw, j.a.sh) {
		j.t.Fatalf("tile (%d, %d) is under the HUD at (%d, %d)", x, y, j.f.Cur.Mx, j.f.Cur.My)
	}
	j.f.Click(j.t, j.a, btn, mods)
}

// dragTiles drags btn from one tile centre to another (three Updates).
func (j *journey) dragTiles(btn mouseBtn, x0, y0, x1, y1 int, mods input.Mod) {
	j.t.Helper()
	j.f.Drag(j.t, j.a, btn, x0, y0, x1, y1, mods)
}

// settle puts enough Updates between two clicks that they are never a
// double click.
func (j *journey) settle() { j.a.updates += doubleClickFrames + 1 }

// clickID clicks a HUD button or a kit widget by id with the mouse, the
// real path: one Update to lay the screen out, a press and a release at
// its centre, then the cursor parks off the window. A disabled element is
// not clicked and clickID returns false, so the test can assert the hint.
func (j *journey) clickID(id string) bool {
	j.t.Helper()
	j.update(1)
	var x, y float32
	found := false
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.id == id {
			if b.disabled {
				return false
			}
			x, y, found = b.x+b.w/2, b.y+b.h/2, true
			break
		}
	}
	if !found {
		w, ok := j.a.uiSt().find(id)
		if !ok {
			j.t.Fatalf("no button or widget %q on %s", id, j.a.nav.screen())
		}
		x, y = w.r.Center()
	}
	s := float32(j.a.scale.S)
	j.f.clickAt(j.t, j.a, int(x*s), int(y*s))
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	return true
}

// pick clicks the open float menu's row whose label starts with label.
func (j *journey) pick(label string) {
	j.t.Helper()
	var labels []string
	for _, b := range j.a.floatButtons() {
		labels = append(labels, b.label)
		if strings.HasPrefix(b.label, label) {
			s := float32(j.a.scale.S)
			j.f.clickAt(j.t, j.a, int((b.x+b.w/2)*s), int((b.y+b.h/2)*s))
			j.f.Cur.Mx, j.f.Cur.My = -50, -50
			return
		}
	}
	j.t.Fatalf("no menu row starting %q; rows: %q", label, labels)
}

// lastCmd is the newest command of that kind the client sent.
func (j *journey) lastCmd(kind command.Kind) (command.Command, bool) {
	for i := len(j.cmds) - 1; i >= 0; i-- {
		if j.cmds[i].Kind == kind {
			return j.cmds[i], true
		}
	}
	return command.Command{}, false
}

// cmdsOf lists the commands of that kind, oldest first.
func (j *journey) cmdsOf(kind command.Kind) []command.Command {
	var out []command.Command
	for _, c := range j.cmds {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

// reset forgets the commands seen so far.
func (j *journey) reset() { j.cmds = j.cmds[:0] }

// pawn is a colonist's current view.
func (j *journey) pawn(id uint64) game.PawnView {
	j.t.Helper()
	for _, p := range j.g.Snapshot() {
		if uint64(p.ID) == id {
			return p
		}
	}
	j.t.Fatalf("no pawn %d", id)
	return game.PawnView{}
}

// world is the colony's current read model.
func (j *journey) world() game.WorldView { return j.g.World() }

// until runs Updates at the current speed until cond holds, at most max.
func (j *journey) until(cond func() bool, max int) {
	j.t.Helper()
	for i := 0; i < max; i++ {
		if cond() {
			return
		}
		j.update(1)
	}
	if !cond() {
		j.t.Fatalf("condition not met after %d Updates (tick %d)", max, j.g.Tick())
	}
}

// ---- guards ---------------------------------------------------------------

// journeyKindTODO and journeyActionTODO list the command kinds and input
// actions no journey covers yet, with the journey file that owes them
// (each file registers its own debt with owe at the top). The guard fails
// on a kind or action that is neither covered nor owed, and on an owed one
// that a journey now covers, so the lists only shrink: cover it, then
// delete it from the file's owe call.
var journeyKindTODO, journeyActionTODO = map[string]string{}, map[string]string{}

// owe registers what a journey file still owes: command kinds and input
// action ids. It returns nothing useful; call it from a package-level var.
func owe(file string, kinds, actions []string) struct{} {
	for _, k := range kinds {
		journeyKindTODO[k] = "owed by " + file
	}
	for _, a := range actions {
		journeyActionTODO[a] = "owed by " + file
	}
	return struct{}{}
}

// journeyFiles are the journey test files of this package.
func journeyFiles(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob("journey_*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no journey files: %v", err)
	}
	sort.Strings(files)
	return files
}

func parseGo(t testing.TB, fset *token.FileSet, path string) *ast.File {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	af, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return af
}

// calleeName is the bare name a call goes to: f(...) -> f, x.f(...) -> f.
func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func strLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, "`\""), true
}

// journeyKeys are the key names journeys mention: every string literal in
// a journey file (keys are pressed from tables as often as inline, and a
// literal only counts when it equals a bound key's name), plus the
// modifier selectors used (input.ModShift ...).
func journeyKeys(t testing.TB) (keys, mods map[string]bool) {
	t.Helper()
	keys, mods = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range journeyFiles(t) {
		ast.Inspect(parseGo(t, fset, path), func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if s, ok := strLit(x); ok {
					keys[s] = true
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "input" && strings.HasPrefix(x.Sel.Name, "Mod") {
					mods[x.Sel.Name] = true
				}
			}
			return true
		})
	}
	return keys, mods
}

// commandKinds are the names of command.Kind's constants.
func commandKinds(t testing.TB) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	af := parseGo(t, fset, filepath.Join("..", "..", "..", "domain", "sim", "command", "command.go"))
	kinds := map[string]bool{}
	for _, d := range af.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		isKind := false
		for _, s := range gd.Specs {
			if vs, ok := s.(*ast.ValueSpec); ok {
				if id, ok := vs.Type.(*ast.Ident); ok && id.Name == "Kind" {
					isKind = true
				}
			}
		}
		if !isKind {
			continue
		}
		for _, s := range gd.Specs {
			for _, n := range s.(*ast.ValueSpec).Names {
				kinds[n.Name] = true
			}
		}
	}
	if len(kinds) < 20 {
		t.Fatalf("found only %d command kinds", len(kinds))
	}
	return kinds
}

// kindSelectors are the command.<Kind> names mentioned in these files.
func kindSelectors(t testing.TB, files []string, kinds map[string]bool) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range files {
		ast.Inspect(parseGo(t, fset, path), func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "command" && kinds[sel.Sel.Name] {
					out[sel.Sel.Name] = true
				}
			}
			return true
		})
	}
	return out
}

// TestEveryInputActionHasAJourney: every input action (but the reserved
// ones) is pressed or held in some journey, and every command kind the
// client emits is asserted by one, or is on journeyKindTODO with a reason.
func TestEveryInputActionHasAJourney(t *testing.T) {
	keys, mods := journeyKeys(t)
	var missing, staleActions []string
	for _, d := range input.Defs() {
		if d.Reserved != "" {
			continue
		}
		ok := false
		var names []string
		for _, b := range d.Defaults {
			names = append(names, b.String())
			if keys[b.Key] || mods["Mod"+b.Key] {
				ok = true
			}
		}
		id := string(d.ID)
		switch {
		case !ok && journeyActionTODO[id] == "":
			missing = append(missing, fmt.Sprintf("%s (%s)", id, strings.Join(names, ", ")))
		case ok && journeyActionTODO[id] != "":
			staleActions = append(staleActions, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("input actions no journey presses or holds (add one to a journey_*_test.go, or list it in journeyActionTODO):\n\t%s", strings.Join(missing, "\n\t"))
	}
	if len(staleActions) > 0 {
		t.Errorf("journeyActionTODO entries a journey now covers (remove them): %s", strings.Join(staleActions, ", "))
	}

	kinds := commandKinds(t)
	var emitters []string
	for _, f := range packageFiles(t, ".") { // the sub-packages too: a moved emitter stays covered
		if f == "lessons.go" || f == "learn_ui.go" {
			continue // the lessons observe commands; they don't send them
		}
		emitters = append(emitters, f)
	}
	emitted := kindSelectors(t, emitters, kinds)
	covered := kindSelectors(t, journeyFiles(t), kinds)
	var uncovered, stale []string
	for k := range emitted {
		if !covered[k] && journeyKindTODO[k] == "" {
			uncovered = append(uncovered, k)
		}
	}
	for k := range journeyKindTODO {
		if covered[k] || !emitted[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(uncovered)
	sort.Strings(stale)
	if len(uncovered) > 0 {
		t.Errorf("command kinds the client emits that no journey asserts (assert one, or list it in journeyKindTODO with a reason):\n\t%s", strings.Join(uncovered, "\n\t"))
	}
	if len(stale) > 0 {
		t.Errorf("journeyKindTODO entries that are covered now or never emitted: %s", strings.Join(stale, ", "))
	}
}

// TestJourneysUseTheRealInputPath: a journey never calls a handler, the
// click dispatcher or the renderer directly; it only presses keys and
// clicks through fakeInput, and reads the result.
func TestJourneysUseTheRealInputPath(t *testing.T) {
	denied := map[string]string{
		"click": "the id shortcut skips the mouse: use j.clickID", "applyClick": "use j.clickID",
		"pressEscape": "use j.key(\"Escape\", 0)", "pressAccept": "use j.key(\"Enter\", 0)",
		"openArchitect": "use j.key(\"Tab\", 0)", "selectArchTool": "use j.clickID(\"arch.tool.*\")",
		"selectArchCategory": "use j.clickID(\"arch.cat.*\")", "openPick": "click the picker",
		"openOrders": "right-click the tile", "Select": "click the thing", "enqueue": "send the command through the UI",
		"toggleDraft": "use j.key(\"R\", 0)", "setSpeed": "use the speed keys", "togglePause": "use Space",
		"LayoutF": "journeys run at scale 1 (parallel-safe)", "setSettings": "use the Options screen", "applyOption": "use j.clickID(\"options.*\")",
		// the canvas's process-wide state (gfx/state.go) is not parallel-safe
		"SetScale": "journeys run at scale 1 (parallel-safe)", "SetDrawTrace": "not parallel-safe", "SetCanvasTrace": "not parallel-safe",
		"ForcePass": "not parallel-safe", "SetPass": "not parallel-safe", "SetVia": "not parallel-safe",
	}
	fset := token.NewFileSet()
	var bad []string
	for _, path := range journeyFiles(t) {
		ast.Inspect(parseGo(t, fset, path), func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				name := calleeName(x)
				if why, ok := denied[name]; ok {
					if name == "click" && len(x.Args) != 3 { // f.click(t, a, btn, mods) is the mouse
						return true
					}
					bad = append(bad, fmt.Sprintf("%s: %s(): %s", fset.Position(x.Pos()), name, why))
				}
			}
			return true
		})
	}
	if len(bad) > 0 {
		t.Errorf("journeys must go through the player's input path:\n\t%s", strings.Join(bad, "\n\t"))
	}
}
