package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
func cmdColony(t *testing.T, n int, cmds ...cmdButton) (*App, *stubSim, *fakeInput) {
	t.Helper()
	at := make([][2]int, n)
	for i := range at {
		at[i] = [2]int{3 + i, 3}
	}
	a, s, f := inputColony(t, at...)
	a.renderer.Select(1)
	a.setTestPane(paneSpec{Title: "Ada", Close: "inspector.close"})
	a.setTestCommands(func(*App, selKind, entity.EntityID) []cmdButton { return cmds })
	f.Frame(t, a)
	return a, s, f
}

func nCmds(n int) []cmdButton {
	out := make([]cmdButton, n)
	for i := range out {
		out[i] = cmdButton{ID: fmt.Sprintf("cmd.test.c%d", i), Label: fmt.Sprintf("Command %d", i), Icon: "draft"}
	}
	return out
}

func TestCommandGridCellsFollowTheLayout(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 9, 13, 27} {
		a, _, _ := cmdColony(t, 1, nCmds(n)...)
		if got := a.commandCount(); got != n {
			t.Fatalf("%d commands: commandCount = %d", n, got)
		}
		l := a.hud()
		if len(l.GridCells) != n {
			t.Fatalf("%d commands: layout has %d cells", n, len(l.GridCells))
		}
		bs := a.commandGridButtons()
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
	a.setTestPane(paneSpec{}) // the kind has not migrated: its legacy panel draws
	if len(a.hud().GridCells) != 0 {
		t.Fatal("the grid needs the pane (or the Architect) to sit beside")
	}
	if len(a.commandGridButtons()) != 0 {
		t.Fatal("no cells, no grid buttons")
	}
}

func TestCommandClickRunsItsCommand(t *testing.T) {
	t.Parallel()
	ran := 0
	enq := command.Command{Kind: command.DraftPawn, Pawn: 1, Enabled: true}
	optRan := ""
	a, s, _ := cmdColony(t, 1,
		cmdButton{ID: "cmd.test.run", Label: "Run", Run: func(*App) { ran++ }},
		cmdButton{ID: "cmd.test.cmds", Label: "Enqueue", Cmds: []command.Command{enq}},
		cmdButton{ID: "cmd.test.opts", Label: "Pick", Options: func(*App) []cmdOption {
			return []cmdOption{
				{Label: "off", Disabled: true, Reason: "no", Run: func(*App) { optRan = "off" }},
				{Label: "first", Run: func(*App) { optRan = "first" }},
				{Label: "second", Run: func(*App) { optRan = "second" }},
			}
		}},
		cmdButton{ID: "cmd.test.off", Label: "Off", Disabled: true, Reason: "needs a bed", Run: func(*App) { ran += 100 }},
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
	a.applyCommandClick("cmd.test.off") // even called directly
	if ran != 1 {
		t.Fatal("applyCommandClick ran a disabled command")
	}
	a.applyCommandClick("cmd.test.gone") // unknown ids are ignored
}

// A legacy id has no Run or Cmds: the click goes through app.go's handlers.
func TestLegacyCommandIDGoesThroughApplyClick(t *testing.T) {
	t.Parallel()
	a, s, _ := cmdColony(t, 1, cmdButton{ID: "pawn.clearorders", Label: "Clear orders"})
	click(t, a, "pawn.clearorders")
	if got := cmdsOf(s, command.ClearOrders); len(got) != 1 || got[0].Pawn != 1 {
		t.Fatalf("applyClick(pawn.clearorders) enqueued %+v", s.cmds)
	}
}

func TestCommandHotkeysFollowDeclarationOrder(t *testing.T) {
	t.Parallel()
	var ran []string
	mk := func(name string) cmdButton {
		return cmdButton{ID: "cmd.test." + name, Label: name, Run: func(*App) { ran = append(ran, name) }}
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
	a.setTestCommands(func(*App, selKind, entity.EntityID) []cmdButton { return []cmdButton{mk("c"), mk("a"), mk("b")} })
	f.Frame(t, a)
	f.Key(t, a, key(1), 0)
	if got := strings.Join(ran, ","); got != "a" {
		t.Fatalf("after reordering key 2 ran %q, want a", got)
	}
	// moving the layout (a different command count) never changes a command's key
	if got := a.commandKeyLabel(1, mk("a")); got != a.bindings().Label(input.Gizmos[1]) {
		t.Fatalf("hint for slot 1 = %q", got)
	}
}

func TestCommandHotkeyOnADisabledCommandShowsTheReason(t *testing.T) {
	t.Parallel()
	ran := false
	a, _, f := cmdColony(t, 1, cmdButton{ID: "cmd.test.x", Label: "X", Disabled: true, Reason: "nobody can reach it", Run: func(*App) { ran = true }})
	f.Key(t, a, a.bindings().Of(input.Gizmos[0])[0].Key, 0)
	if ran || a.hint != "nobody can reach it" {
		t.Fatalf("ran=%v hint=%q", ran, a.hint)
	}
}

func TestGizmoKeysAreBeyondTwelveSilent(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, nCmds(14)...)
	if got := a.commandKeyLabel(12, nCmds(14)[12]); got != "" {
		t.Fatalf("the 13th command has no hotkey, got %q", got)
	}
	// the twelve command keys reach exactly the first twelve commands
	if len(input.Gizmos) != cmdMaxKeys {
		t.Fatalf("%d command keys, want %d", len(input.Gizmos), cmdMaxKeys)
	}
	for i := range cmdMaxKeys {
		if a.commandKeyLabel(i, nCmds(14)[i]) == "" {
			t.Errorf("command %d has no hotkey", i)
		}
	}
	a.pressCommandKey(12) // no 13th key: nothing happens, nothing panics
	a.pressCommandKey(-1)
}

func TestDedicatedHotkeyIsTheOneShown(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, cmdButton{ID: "pawn.draft", Label: "Draft", Hotkey: input.Draft})
	if got, want := a.commandKeyLabel(0, a.selectionCommands()[0]), a.bindings().Label(input.Draft); got == "" || got != want {
		t.Fatalf("hint = %q, want the Draft key %q", got, want)
	}
}

// The grid draws each command's key hint, once, on its own cell (no
// separate key badges are drawn over the HUD).
func TestCommandGridDrawsTheKeyHints(t *testing.T) {
	a, _, _ := cmdColony(t, 1, nCmds(3)...) // serial: it installs the draw trace
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	a.drawCommandGrid(ebiten.NewImage(1280, 720))
	cells := a.hud().GridCells
	for i, c := range a.selectionCommands() {
		key := a.commandKeyLabel(i, c)
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
		cmdButton{ID: "cmd.test.opts", Label: "Pick", Options: func(*App) []cmdOption {
			return []cmdOption{
				{Label: "Ada", Cmd: &enq},
				{Label: "Bo", Run: func(*App) { picked = "Bo" }},
				{Label: "Cy", Disabled: true, Reason: "asleep"},
			}
		}},
		cmdButton{ID: "cmd.test.plain", Label: "Plain", Run: func(*App) {}},
	)
	cells := a.hud().GridCells
	cx, cy := cells[0].Center()
	if !a.rightClickCommand(int(cx), int(cy)) || !a.ord.Float.Open || len(a.ord.Float.Opts) != 3 {
		t.Fatalf("right-click on a command with options opens its menu: open=%v opts=%+v", a.ord.Float.Open, a.ord.Float.Opts)
	}
	if a.ord.Float.Opts[0].Label != "Ada" || !a.ord.Float.Opts[2].Disabled || a.ord.Float.Opts[2].Reason != "asleep" {
		t.Fatalf("menu entries = %+v", a.ord.Float.Opts)
	}
	if r := a.floatRect(); r.Y+r.H > cells[0].Y+0.5 {
		t.Errorf("the menu opens upward from the cell: menu %+v, cell %+v", r, cells[0])
	}
	click(t, a, "float.1")
	if picked != "Bo" || a.ord.Float.Open {
		t.Fatalf("picking an option runs it and closes the menu: picked=%q open=%v", picked, a.ord.Float.Open)
	}
	a.rightClickCommand(int(cx), int(cy))
	click(t, a, "float.0")
	if got := cmdsOf(s, command.DraftPawn); len(got) != 1 {
		t.Fatalf("an option with a Cmd enqueues it: %+v", s.cmds)
	}
	a.rightClickCommand(int(cx), int(cy))
	click(t, a, "float.2")
	if !a.ord.Float.Open || a.hint != "asleep" {
		t.Fatalf("a disabled option shows its reason and keeps the menu: open=%v hint=%q", a.ord.Float.Open, a.hint)
	}
	a.closeFloat()

	// a command without options still swallows the click (it is over the HUD)
	px, py := cells[1].Center()
	if !a.rightClickCommand(int(px), int(py)) || a.ord.Float.Open {
		t.Fatal("right-click on a plain command is consumed and opens nothing")
	}
	// empty space is not the grid's
	if a.rightClickCommand(2, 2) {
		t.Fatal("right-click off the grid must fall through")
	}
}

func TestOptionsArrowOpensTheMenuOnLeftClick(t *testing.T) {
	t.Parallel()
	a, _, _ := cmdColony(t, 1, cmdButton{ID: "cmd.test.opts", Label: "Pick", Options: func(*App) []cmdOption {
		return []cmdOption{{Label: "one"}, {Label: "two"}}
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
	next.X += cmdStride
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
	if x != l.HintX || b != l.HintBottom || b != l.Grid.Y-hudMargin {
		t.Fatalf("hintAnchor = %v,%v, want the layout's %v,%v above the grid at %v", x, b, l.HintX, l.HintBottom, l.Grid.Y)
	}
	// with no pane or grid the hint rests at the left margin above the tabs
	a.setTestPane(paneSpec{})
	a.setTestCommands(noCommands)
	l = a.hud()
	x, b = a.hintAnchor()
	if !l.Pane.Empty() || !l.Grid.Empty() || x != hudMargin || b != l.Tabs.Y-hudMargin {
		t.Fatalf("with no pane or grid the hint sits at %v,%v, want %v,%v", x, b, float32(hudMargin), l.Tabs.Y-hudMargin)
	}
}

func TestCommandTooltipsAreDeclared(t *testing.T) {
	t.Parallel()
	a, _, f := cmdColony(t, 1,
		cmdButton{ID: "cmd.test.ok", Label: "Haul", Desc: "Carry it to a stockpile.", Hotkey: input.Draft},
		cmdButton{ID: "cmd.test.off", Label: "Equip", Desc: "Pick up the weapon.", Disabled: true, Reason: "already armed"},
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
		cmdButton{ID: "cmd.test.a", Label: "A long label that must wrap and be cut", Icon: "draft", Hotkey: input.Draft},
		cmdButton{ID: "cmd.test.b", Label: "On", Toggle: true, On: true},
		cmdButton{ID: "cmd.test.c", Label: "Some", Toggle: true, Mixed: true},
		cmdButton{ID: "cmd.test.d", Label: "Off", Disabled: true, Reason: "no"},
		cmdButton{ID: "cmd.test.e", Label: "Menu", Options: func(*App) []cmdOption { return nil }},
	)
	l := a.hud()
	var rects []rect
	gfx.SetCanvasTrace(func(_ string, x, y, w, h float32) { rects = append(rects, rect{X: x, Y: y, W: w, H: h}) })
	t.Cleanup(func() { gfx.SetCanvasTrace(nil) })
	a.drawCommandGrid(ebiten.NewImage(1280, 720))
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

// A one-word label too wide for a command button ("Deconstruct") steps its
// type down instead of breaking the word in two; it still fits the cell.
func TestCommandLabelShrinksBeforeBreakingAWord(t *testing.T) {
	maxW := float64(cmdCell - 2*cmdLabelPad)
	for _, label := range []string{"Deconstruct", "Mark for hauling", "Cancel blueprint", "Draft"} {
		st, lines := cmdLabelLines(label, maxW, false)
		for _, ln := range lines {
			if w, _ := measure(ln, st); w > maxW+0.01 {
				t.Errorf("%q: line %q is %.1f wide, cell text room %.1f", label, ln, w, maxW)
			}
		}
		for _, word := range strings.Fields(label) {
			found := false
			for _, ln := range lines {
				found = found || strings.Contains(ln, word)
			}
			if !found {
				t.Errorf("%q: word %q was broken across lines %q", label, word, lines)
			}
		}
		if len(lines) > 2 {
			t.Errorf("%q: %d lines, want at most 2", label, len(lines))
		}
	}
	if st, _ := cmdLabelLines("Draft", maxW, false); st.Size != tsSmall.Size {
		t.Errorf("a short label shrank to %v", st.Size)
	}
}
