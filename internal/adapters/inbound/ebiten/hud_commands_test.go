package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// The command-button grid (PLAY-018, CORE-PANE). The per-kind producers
// belong to other owners, so these tests inject the commands through the
// test seam (cmdsState.testProducer / paneState.testProducer).

// cmdColony is a paused stub colony with n colonists (ids 1..n) and the
// first one selected; its commands and pane come from cmds and a minimal
// pane.
func cmdColony(t *testing.T, n int, cmds ...inspector.Button) (*App, *stubSim, *fakeInput) {
	t.Helper()
	at := make([][2]int, n)
	for i := range at {
		at[i] = [2]int{3 + i, 3}
	}
	a, s, f := inputColony(t, at...)
	a.renderer.Select(1)
	a.setTestPane(inspector.Spec{Title: "Ada", Close: "inspector.close"})
	a.setTestCommands(func(*App, selection.Kind, entity.EntityID) []inspector.Button { return cmds })
	f.Frame(t, a)
	return a, s, f
}

func nCmds(n int) []inspector.Button {
	out := make([]inspector.Button, n)
	for i := range out {
		out[i] = inspector.Button{ID: fmt.Sprintf("cmd.test.c%d", i), Label: fmt.Sprintf("Command %d", i), Icon: "draft"}
	}
	return out
}

func TestCommandGridCellsFollowTheLayout(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 9, 13, 27} {
		a, _, _ := cmdColony(t, 1, nCmds(n)...)
		if got := a.insp.Commands().CommandCount(); got != n {
			t.Fatalf("%d commands: commandCount = %d", n, got)
		}
		l := a.hud()
		if len(l.GridCells) != n {
			t.Fatalf("%d commands: layout has %d cells", n, len(l.GridCells))
		}
		bs := a.insp.Commands().CommandGridButtons()
		if len(bs) != n {
			t.Fatalf("%d commands: %d grid buttons", n, len(bs))
		}
		for i, b := range bs {
			c := l.GridCells[i]
			if b.ID != fmt.Sprintf("cmd.test.c%d", i) || b.X != c.X || b.Y != c.Y || b.W != c.W || b.H != c.H {
				t.Errorf("%d commands: button %d = %+v, want id c%d at %+v", n, i, b, i, c)
			}
		}
	}
}

func TestGridCellsAreNotThereWithoutAPane(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, nCmds(3)...)
	a.setTestPane(inspector.Spec{}) // the kind has not migrated: its legacy panel draws
	if len(a.hud().GridCells) != 0 {
		t.Fatal("the grid needs the pane (or the Architect) to sit beside")
	}
	if len(a.insp.Commands().CommandGridButtons()) != 0 {
		t.Fatal("no cells, no grid buttons")
	}
}

func TestCommandClickRunsItsCommand(t *testing.T) {
	t.Parallel()
	ran := 0
	enq := command.Command{Kind: command.DraftPawn, Pawn: 1, Enabled: true}
	optRan := ""
	a, s, _ := cmdColony(t, 1,
		inspector.Button{ID: "cmd.test.run", Label: "Run", Run: func() { ran++ }},
		inspector.Button{ID: "cmd.test.cmds", Label: "Enqueue", Cmds: []command.Command{enq}},
		inspector.Button{ID: "cmd.test.opts", Label: "Pick", Options: func() []inspector.Option {
			return []inspector.Option{
				{Label: "off", Disabled: true, Reason: "no", Run: func() { optRan = "off" }},
				{Label: "first", Run: func() { optRan = "first" }},
				{Label: "second", Run: func() { optRan = "second" }},
			}
		}},
		inspector.Button{ID: "cmd.test.off", Label: "Off", Disabled: true, Reason: "needs a bed", Run: func() { ran += 100 }},
	)
	click(t, a, "cmd.test.run")
	if ran != 1 {
		t.Fatalf("Run ran %d times", ran)
	}
	click(t, a, "cmd.test.cmds")
	if got := cmdsOf(s, command.DraftPawn); len(got) != 1 || got[0].Pawn != 1 {
		t.Fatalf("Cmds enqueued %+v", s.cmds)
	}
	click(t, a, "cmd.test.opts")
	if optRan != "first" {
		t.Fatalf("a click runs the first enabled option, ran %q", optRan)
	}
	click(t, a, "cmd.test.off")
	if ran != 1 || a.hint != "needs a bed" {
		t.Fatalf("a disabled command shows its reason and does nothing: ran=%d hint=%q", ran, a.hint)
	}
	a.insp.Commands().ApplyCommandClick("cmd.test.off") // even called directly
	if ran != 1 {
		t.Fatal("applyCommandClick ran a disabled command")
	}
	a.insp.Commands().ApplyCommandClick("cmd.test.gone") // unknown ids are ignored
}

// A legacy id has no Run or Cmds: the click goes through app.go's handlers.
func TestLegacyCommandIDGoesThroughApplyClick(t *testing.T) {
	t.Parallel()
	a, s, _ := cmdColony(t, 1, inspector.Button{ID: "pawn.clearorders", Label: "Clear orders"})
	click(t, a, "pawn.clearorders")
	if got := cmdsOf(s, command.ClearOrders); len(got) != 1 || got[0].Pawn != 1 {
		t.Fatalf("applyClick(pawn.clearorders) enqueued %+v", s.cmds)
	}
}

func TestCommandHotkeysFollowDeclarationOrder(t *testing.T) {
	t.Parallel()
	var ran []string
	mk := func(name string) inspector.Button {
		return inspector.Button{ID: "cmd.test." + name, Label: name, Run: func() { ran = append(ran, name) }}
	}
	a, _, f := cmdColony(t, 1, mk("a"), mk("b"), mk("c"))
	key := func(i int) string { return a.bindings().Of(input.Gizmos[i])[0].Key }
	f.Key(t, a, key(1), 0)
	f.Key(t, a, key(0), 0)
	f.Key(t, a, key(2), 0)
	if got := strings.Join(ran, ","); got != "b,a,c" {
		t.Fatalf("keys 2,1,3 ran %q, want b,a,c", got)
	}
	// reorder: the same key presses another command
	ran = nil
	a.setTestCommands(func(*App, selection.Kind, entity.EntityID) []inspector.Button {
		return []inspector.Button{mk("c"), mk("a"), mk("b")}
	})
	f.Frame(t, a)
	f.Key(t, a, key(1), 0)
	if got := strings.Join(ran, ","); got != "a" {
		t.Fatalf("after reordering key 2 ran %q, want a", got)
	}
	// moving the layout (a different command count) never changes a command's key
	if got := a.insp.Commands().CommandKeyLabel(1, mk("a")); got != a.bindings().Label(input.Gizmos[1]) {
		t.Fatalf("hint for slot 1 = %q", got)
	}
}

func TestCommandHotkeyOnADisabledCommandShowsTheReason(t *testing.T) {
	t.Parallel()
	ran := false
	a, _, f := cmdColony(t, 1, inspector.Button{ID: "cmd.test.x", Label: "X", Disabled: true, Reason: "nobody can reach it", Run: func() { ran = true }})
	f.Key(t, a, a.bindings().Of(input.Gizmos[0])[0].Key, 0)
	if ran || a.hint != "nobody can reach it" {
		t.Fatalf("ran=%v hint=%q", ran, a.hint)
	}
}

func TestGizmoKeysAreBeyondTwelveSilent(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, nCmds(14)...)
	if got := a.insp.Commands().CommandKeyLabel(12, nCmds(14)[12]); got != "" {
		t.Fatalf("the 13th command has no hotkey, got %q", got)
	}
	// the twelve command keys reach exactly the first twelve commands
	if len(input.Gizmos) != inspector.CmdMaxKeys {
		t.Fatalf("%d command keys, want %d", len(input.Gizmos), inspector.CmdMaxKeys)
	}
	for i := range inspector.CmdMaxKeys {
		if a.insp.Commands().CommandKeyLabel(i, nCmds(14)[i]) == "" {
			t.Errorf("command %d has no hotkey", i)
		}
	}
	a.insp.Commands().PressCommandKey(12) // no 13th key: nothing happens, nothing panics
	a.insp.Commands().PressCommandKey(-1)
}

func TestDedicatedHotkeyIsTheOneShown(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, inspector.Button{ID: "pawn.draft", Label: "Draft", Hotkey: input.Draft})
	if got, want := a.insp.Commands().CommandKeyLabel(0, a.insp.Commands().SelectionCommands()[0]), a.bindings().Label(input.Draft); got == "" || got != want {
		t.Fatalf("hint = %q, want the Draft key %q", got, want)
	}
}

// The grid draws each command's key hint, once, on its own cell (no
// separate key badges are drawn over the HUD).
func TestCommandGridDrawsTheKeyHints(t *testing.T) {
	a, _, _ := cmdColony(t, 1, nCmds(3)...) // serial: it installs the draw trace
	var ev []gfx.TraceEvent
	gfx.SetDrawTrace(func(e gfx.TraceEvent) { ev = append(ev, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	a.insp.Commands().DrawCommandGrid(ebiten.NewImage(1280, 720))
	cells := a.hud().GridCells
	for i, c := range a.insp.Commands().SelectionCommands() {
		key := a.insp.Commands().CommandKeyLabel(i, c)
		n := 0
		for _, e := range ev {
			if e.Kind == "text" && e.Text == key {
				n++
				if !visInside(e, float64(cells[i].X), float64(cells[i].Y), float64(cells[i].Right()), float64(cells[i].Bottom()), 1) {
					t.Errorf("key %q of command %d drawn at %s, off its cell %+v", key, i, e, cells[i])
				}
			}
		}
		if key == "" || n != 1 {
			t.Errorf("command %d: key %q drawn %d times, want once", i, key, n)
		}
	}
}

func TestRightClickOpensTheOptionsMenu(t *testing.T) {
	t.Parallel()
	var picked string
	enq := command.Command{Kind: command.DraftPawn, Pawn: 1, Enabled: true}
	a, s, _ := cmdColony(t, 1,
		inspector.Button{ID: "cmd.test.opts", Label: "Pick", Options: func() []inspector.Option {
			return []inspector.Option{
				{Label: "Ada", Cmd: &enq},
				{Label: "Bo", Run: func() { picked = "Bo" }},
				{Label: "Cy", Disabled: true, Reason: "asleep"},
			}
		}},
		inspector.Button{ID: "cmd.test.plain", Label: "Plain", Run: func() {}},
	)
	cells := a.hud().GridCells
	cx, cy := cells[0].Center()
	if !a.insp.Commands().RightClickCommand(int(cx), int(cy)) || !a.ord.Float.Open || len(a.ord.Float.Opts) != 3 {
		t.Fatalf("right-click on a command with options opens its menu: open=%v opts=%+v", a.ord.Float.Open, a.ord.Float.Opts)
	}
	if a.ord.Float.Opts[0].Label != "Ada" || !a.ord.Float.Opts[2].Disabled || a.ord.Float.Opts[2].Reason != "asleep" {
		t.Fatalf("menu entries = %+v", a.ord.Float.Opts)
	}
	if r := a.ord.FloatRect(); r.Y+r.H > cells[0].Y+0.5 {
		t.Errorf("the menu opens upward from the cell: menu %+v, cell %+v", r, cells[0])
	}
	click(t, a, "float.1")
	if picked != "Bo" || a.ord.Float.Open {
		t.Fatalf("picking an option runs it and closes the menu: picked=%q open=%v", picked, a.ord.Float.Open)
	}
	a.insp.Commands().RightClickCommand(int(cx), int(cy))
	click(t, a, "float.0")
	if got := cmdsOf(s, command.DraftPawn); len(got) != 1 {
		t.Fatalf("an option with a Cmd enqueues it: %+v", s.cmds)
	}
	a.insp.Commands().RightClickCommand(int(cx), int(cy))
	click(t, a, "float.2")
	if !a.ord.Float.Open || a.hint != "asleep" {
		t.Fatalf("a disabled option shows its reason and keeps the menu: open=%v hint=%q", a.ord.Float.Open, a.hint)
	}
	a.ord.CloseFloat()

	// a command without options still swallows the click (it is over the HUD)
	px, py := cells[1].Center()
	if !a.insp.Commands().RightClickCommand(int(px), int(py)) || a.ord.Float.Open {
		t.Fatal("right-click on a plain command is consumed and opens nothing")
	}
	// empty space is not the grid's
	if a.insp.Commands().RightClickCommand(2, 2) {
		t.Fatal("right-click off the grid must fall through")
	}
}

func TestOptionsArrowOpensTheMenuOnLeftClick(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, inspector.Button{ID: "cmd.test.opts", Label: "Pick", Options: func() []inspector.Option {
		return []inspector.Option{{Label: "one"}, {Label: "two"}}
	}})
	click(t, a, "cmd.options.0") // the corner arrow is its own hit target
	if !a.ord.Float.Open || len(a.ord.Float.Opts) != 2 {
		t.Fatalf("the corner arrow opens the menu: open=%v", a.ord.Float.Open)
	}
}

func TestChromeBlocksOccupiedCellsOnly(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, nCmds(3)...)
	l := a.hud()
	blocks := func(x, y float32) bool {
		for _, r := range l.Chrome() {
			if r.Contains(x, y) {
				return true
			}
		}
		return false
	}
	for i, c := range l.GridCells {
		cx, cy := c.Center()
		if !blocks(cx, cy) {
			t.Errorf("occupied cell %d does not block clicks", i)
		}
	}
	// the cell after the last command is free map
	next := l.GridCells[2]
	next.X += hudlayout.CmdStride
	if cx, cy := next.Center(); blocks(cx, cy) {
		t.Error("an empty grid cell must let clicks reach the map")
	}
	assertChromeIsTheLayout(t, a) // the layout covers the cells, and only the occupied ones
}

func TestHintAnchorFollowsTheLayout(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, nCmds(3)...)
	l := a.hud()
	x, b := a.hintAnchor()
	if x != l.HintX || b != l.HintBottom || b != l.Grid.Y-hudlayout.HudMargin {
		t.Fatalf("hintAnchor = %v,%v, want the layout's %v,%v above the grid at %v", x, b, l.HintX, l.HintBottom, l.Grid.Y)
	}
	// with no pane or grid the hint rests at the left margin above the tabs
	a.setTestPane(inspector.Spec{})
	a.setTestCommands(noCommands)
	l = a.hud()
	x, b = a.hintAnchor()
	if !l.Pane.Empty() || !l.Grid.Empty() || x != hudlayout.HudMargin || b != l.Tabs.Y-hudlayout.HudMargin {
		t.Fatalf("with no pane or grid the hint sits at %v,%v, want %v,%v", x, b, float32(hudlayout.HudMargin), l.Tabs.Y-hudlayout.HudMargin)
	}
}

func TestCommandTooltipsAreDeclared(t *testing.T) {
	t.Parallel()
	a, _, f := cmdColony(t, 1,
		inspector.Button{ID: "cmd.test.ok", Label: "Haul", Desc: "Carry it to a stockpile.", Hotkey: input.Draft},
		inspector.Button{ID: "cmd.test.off", Label: "Equip", Desc: "Pick up the weapon.", Disabled: true, Reason: "already armed"},
	)
	f.Frame(t, a)
	tip := map[string]string{}
	for _, w := range a.uiSt().Cur {
		if id, ok := strings.CutPrefix(w.ID, "cmd.tip."); ok {
			tip[id] = w.Tip
		}
	}
	ok, off := tip["cmd.test.ok"], tip["cmd.test.off"]
	if !strings.HasPrefix(ok, "Haul") || !strings.Contains(ok, a.bindings().Label(input.Draft)) || !strings.Contains(ok, "Carry it to a stockpile.") {
		t.Errorf("tooltip = %q", ok)
	}
	if !strings.Contains(off, "Equip") || !strings.Contains(off, "Pick up the weapon.") || !strings.Contains(off, "already armed") {
		t.Errorf("a disabled command's tooltip carries its reason: %q", off)
	}
}

func TestCommandGridDrawsInsideItsCells(t *testing.T) {
	// serial: installs the canvas trace
	a, _, _ := cmdColony(t, 1,
		inspector.Button{ID: "cmd.test.a", Label: "A long label that must wrap and be cut", Icon: "draft", Hotkey: input.Draft},
		inspector.Button{ID: "cmd.test.b", Label: "On", Toggle: true, On: true},
		inspector.Button{ID: "cmd.test.c", Label: "Some", Toggle: true, Mixed: true},
		inspector.Button{ID: "cmd.test.d", Label: "Off", Disabled: true, Reason: "no"},
		inspector.Button{ID: "cmd.test.e", Label: "Menu", Options: func() []inspector.Option { return nil }},
	)
	l := a.hud()
	var rects []gfx.Rect
	gfx.SetCanvasTrace(func(_ string, x, y, w, h float32) { rects = append(rects, gfx.Rect{X: x, Y: y, W: w, H: h}) })
	t.Cleanup(func() { gfx.SetCanvasTrace(nil) })
	a.insp.Commands().DrawCommandGrid(ebiten.NewImage(1280, 720))
	if len(rects) == 0 {
		t.Fatal("the grid drew nothing")
	}
	for _, r := range rects {
		in := false
		for _, c := range l.GridCells {
			if r.X >= c.X-0.01 && r.Y >= c.Y-0.01 && r.Right() <= c.Right()+0.01 && r.Bottom() <= c.Bottom()+0.01 {
				in = true
			}
		}
		if !in {
			t.Errorf("draw %+v lies outside every grid cell", r)
		}
	}
}

// Thirty commands (and sixty) are all reachable at every UI scale
// (UIS-052): each has its own cell or sits behind the "more" cell, in
// declaration order, and a pick in the more cell's menu presses it.
func TestThirtyCommandsAreAllReachable(t *testing.T) {
	// serial: it sets the UI scale
	overflowed := false
	for _, n := range []int{30, 60} {
		for _, sc := range []float64{1, 1.5, 2} {
			withUIScale(t, sc)
			sz := fmt.Sprintf("scale %v", sc)
			ran := map[string]int{}
			cmds := nCmds(n)
			for i := range cmds {
				id := cmds[i].ID
				cmds[i].Run = func() { ran[id]++ }
			}
			a, _, _ := cmdColony(t, 1, cmds...)
			c := a.insp.Commands()
			var got []string
			for _, s := range c.CommandCells() {
				got = append(got, s.ID())
			}
			more := c.MoreCommands()
			for _, m := range more {
				got = append(got, m.ID)
			}
			if len(got) != n {
				t.Fatalf("%d commands at %s: %d reachable (%d cells, %d more)", n, sz, len(got), len(c.CommandCells()), len(more))
			}
			for i, id := range got {
				if id != cmds[i].ID {
					t.Fatalf("%d commands at %s: slot %d is %s, want %s", n, sz, i, id, cmds[i].ID)
				}
			}
			l := a.hud()
			if (l.GridMore > 0) != (len(more) > 0) || (len(more) > 0 && l.GridMore != len(more)) {
				t.Fatalf("%d commands at %s: layout more %d, grid more %d", n, sz, l.GridMore, len(more))
			}
			if len(more) == 0 {
				continue
			}
			overflowed = true
			found := false
			for _, b := range c.CommandGridButtons() {
				found = found || b.ID == inspector.MoreID
			}
			if !found {
				t.Fatalf("%d commands at %s: no more cell among the grid buttons", n, sz)
			}
			click(t, a, inspector.MoreID)
			if !a.ord.Float.Open || len(a.ord.Float.Opts) != len(more) {
				t.Fatalf("%d commands at %s: the more cell's menu has %d entries, want %d", n, sz, len(a.ord.Float.Opts), len(more))
			}
			last := len(more) - 1
			a.ord.ChooseFloat(fmt.Sprintf("float.%d", last))
			if ran[more[last].ID] != 1 {
				t.Fatalf("%d commands at %s: picking %s in the more menu did not run it", n, sz, more[last].ID)
			}
		}
	}
	if !overflowed {
		t.Fatal("no case overflowed the grid: the more cell went untested")
	}
}

// TestCommandTextStaysInItsCell: every label, hotkey and count a command
// cell draws lies inside the cell (user, 2026-11-07: text never leaves its
// box), for the pawn commands, every Architect category's designators, the
// "more" cell and worst-case labels, at every UI scale, two window sizes
// and with the larger small text (OPT-009). It reads the draw trace, which
// sees text (the canvas trace does not).
func TestCommandTextStaysInItsCell(t *testing.T) {
	// serial: it sets the UI scale, the text size and the draw trace
	worst := []inspector.Button{
		{ID: "cmd.test.w0", Label: "Butcher table", Icon: "draft", Hotkey: input.Draft},
		{ID: "cmd.test.w1", Label: "Supercalifragilisticexpialidocious", Icon: "draft"},
		{ID: "cmd.test.w2", Label: "Rest when hurt badly", Icon: "draft", Options: func() []inspector.Option { return nil }},
		{ID: "cmd.test.w3", Label: "Prioritize tending everyone nearby now", Disabled: true, Reason: "no"},
		{ID: "cmd.test.w4", Label: "Draft", Icon: "draft"},
	}
	type scene struct {
		name  string
		build func(t *testing.T) *App
	}
	scenes := []scene{
		{"worst", func(t *testing.T) *App { a, _, _ := cmdColony(t, 1, worst...); return a }},
		{"more", func(t *testing.T) *App { a, _, _ := cmdColony(t, 1, append(nCmds(60), worst...)...); return a }},
		{"pawn", func(t *testing.T) *App {
			a, _, f := inputColony(t, [2]int{3, 3})
			a.renderer.Select(1)
			f.Frame(t, a)
			return a
		}},
		{"drafted", func(t *testing.T) *App {
			a, s, f := inputColony(t, [2]int{3, 3})
			s.pawns[0].Drafted = true
			a.renderer.Select(1)
			f.Frame(t, a)
			return a
		}},
	}
	for _, c := range architect.Categories {
		id := c.ID
		scenes = append(scenes, scene{"arch." + id, func(t *testing.T) *App {
			a, _ := archColony(t)
			a.applyClick("arch.cat." + id)
			fakeOn(a).Frame(t, a)
			return a
		}})
	}
	drawn := map[string]bool{}
	for _, larger := range []bool{false, true} {
		for _, sc := range []float64{1, 1.25, 1.5, 2, 4} {
			for _, win := range [][2]int{{1280, 720}, {1920, 1080}} {
				for _, sn := range scenes {
					old := gfx.LargerSmall()
					a := sn.build(t)
					a.settings.Interface.LargerSmall = larger
					layoutScreen(t, a, win[0], win[1], sc)
					fakeOn(a).Frame(t, a)
					if gfx.LargerSmall() != larger || gfx.UIScale() != sc {
						t.Fatalf("the frame reset the larger small text (%v) or the scale (%v)", gfx.LargerSmall(), gfx.UIScale())
					}
					l := a.hud()
					var ev []gfx.TraceEvent
					gfx.SetDrawTrace(func(e gfx.TraceEvent) { ev = append(ev, e) })
					a.insp.Commands().DrawCommandGrid(ebiten.NewImage(a.pw, a.ph))
					gfx.SetDrawTrace(nil)
					gfx.SetLargerSmall(old)
					where := fmt.Sprintf("%s at scale %v, %dx%d, larger small %v", sn.name, sc, win[0], win[1], larger)
					cells := append([]gfx.Rect(nil), l.GridCells...)
					for _, e := range ev {
						if e.Kind != "text" {
							continue
						}
						drawn[sn.name] = true
						er := gfx.Rect{X: e.X, Y: e.Y, W: e.W, H: e.H}
						var home *gfx.Rect
						for i := range cells {
							p := gfx.Rect{X: cells[i].X * float32(sc), Y: cells[i].Y * float32(sc), W: cells[i].W * float32(sc), H: cells[i].H * float32(sc)}
							if !p.Intersect(er).Empty() {
								home = &p
								break
							}
						}
						if home == nil {
							t.Errorf("%s: %q at %.1f,%.1f %.1fx%.1f meets no cell", where, e.Text, e.X, e.Y, e.W, e.H)
							continue
						}
						const tol = 0.5
						if e.X < home.X-tol || e.Y < home.Y-tol || e.X+e.W > home.Right()+tol || e.Y+e.H > home.Bottom()+tol {
							t.Errorf("%s: %q (%.1f,%.1f %.1fx%.1f) leaves its cell %+v by %.1f px below, %.1f right",
								where, e.Text, e.X, e.Y, e.W, e.H, *home, e.Y+e.H-home.Bottom(), e.X+e.W-home.Right())
						}
					}
				}
			}
		}
	}
	for _, sn := range scenes {
		if !drawn[sn.name] {
			t.Errorf("%s drew no command text", sn.name)
		}
	}
}
