package devtools

import (
	"fmt"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

type keys map[input.Action]bool

func (k keys) Fired(a input.Action) bool { return k[a] }

func testLayers() []game.MapFieldLayer {
	return []game.MapFieldLayer{
		{Name: "Elevation", W: 2, H: 2, Scalar: []float32{0, 0.5, 1, 0.25}},
		{Name: "Caves", W: 2, H: 2, Classes: []uint8{0, 1, 2, 1}, Legend: []game.LegendEntry{
			{Label: "open", RGBA: [4]uint8{10, 20, 30, 255}}, {Label: "rock", RGBA: [4]uint8{200, 0, 0, 255}}, {Label: "cave", RGBA: [4]uint8{0, 200, 0, 255}}}},
		{Name: "Start score", W: 2, H: 2, Scalar: []float32{0, 0, 0, 0}, Marks: [][2]int{{1, 1}}},
	}
}

func TestMapFieldPixels(t *testing.T) {
	t.Parallel()
	l := testLayers()
	px := FieldPixels(l[1]) // classes
	if len(px) != 16 || px[3] != FieldAlpha {
		t.Fatalf("pixels %v", px)
	}
	if px[4] != 200*FieldAlpha/255 { // rock red, premultiplied
		t.Fatalf("premultiplied red = %d", px[4])
	}
	s := FieldPixels(l[0])
	if s[3] != FieldAlpha || s[0] == s[8] && s[1] == s[9] && s[2] == s[10] {
		t.Fatalf("scalar ramp should differ between 0 and 1: %v", s)
	}
	m := FieldPixels(l[2]) // mark at (1,1)
	if m[12] == m[0] && m[13] == m[1] {
		t.Fatal("mark not painted")
	}
}

func TestMapFieldLegendListsClasses(t *testing.T) {
	t.Parallel()
	l := testLayers()
	got := FieldLegendLines(l[1])
	if len(got) != 3 || got[0] != "open" || got[2] != "cave" {
		t.Fatalf("legend %v", got)
	}
	if s := FieldLegendLines(l[0]); len(s) != 2 {
		t.Fatalf("scalar legend is low/high: %v", s)
	}
}

// devLayouts are busy HUD states at the sizes the plan pins: every region
// that could collide with the developer panels is up.
func devLayouts() map[string]hudlayout.Layout {
	busy := hudlayout.Inputs{Colonists: 30, PaneShown: true, Commands: 13, GlobalH: 190, MinimapOn: true, LearnH: 200,
		Alerts: 8, Letters: 8, ResourcesH: 600, Messages: 5, ExpandedW: 500, ExpandedH: 300}
	arch := busy
	arch.PaneShown, arch.ArchOpen, arch.ArchDescH, arch.ExpandedW, arch.ExpandedH = false, true, 120, 0, 0
	win := busy
	win.PaneShown, win.WindowW, win.WindowH, win.ExpandedW, win.ExpandedH = false, 900, 500, 0, 0
	idle := hudlayout.Inputs{GlobalH: 150, ResourcesH: 100}
	out := map[string]hudlayout.Layout{}
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		for name, st := range map[string]hudlayout.Inputs{"busy": busy, "architect": arch, "window": win, "idle": idle} {
			out[fmt.Sprintf("%s %dx%d", name, sz[0], sz[1])] = hudlayout.Compute(sz[0], sz[1], st)
		}
	}
	return out
}

// The developer panels live in a column that touches no HUD region: right
// of the resource list, under the colonist bar and the messages, above the
// lower-left stack, left of the right column.
func TestDevColumnAvoidsEveryRegion(t *testing.T) {
	t.Parallel()
	for name, l := range devLayouts() {
		col := Column(l)
		if col.Empty() {
			continue // no room: nothing is drawn
		}
		regions := append(l.Chrome(), l.Minimap)
		for _, r := range regions {
			if i := col.Intersect(r); i.W > 0 && i.H > 0 {
				t.Errorf("%s: dev column %+v collides with %+v", name, col, r)
			}
		}
		if col.X < l.Resources.Right() || col.Right() > l.Right.X {
			t.Errorf("%s: dev column %+v is not between the resource list and the right column", name, col)
		}
	}
}

func TestPanelsOffWithoutDevMode(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1280x720"]}
	p := New(f)
	p.State.Insp, p.State.Menu = true, true
	if st := p.Stack(); !st.Insp.Empty() || p.Buttons() != nil || p.Mode() {
		t.Fatalf("dev mode off draws nothing: %+v", st)
	}
	f.env = Env{On: true} // on, but no colony
	if st := p.Stack(); !st.Insp.Empty() || p.Buttons() != nil {
		t.Fatal("a menu screen has no panels")
	}
}

func TestPanelsStackInOrder(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1920x1080"], env: Env{On: true, Game: struct{}{}}}
	p := New(f)
	p.State.Log, p.State.Insp = true, true
	p.State.Fields = FieldsOverlay{Shown: true, Layers: testLayers()[1:2]}
	st := p.Stack()
	if st.Fields.Empty() || st.Insp.Empty() || st.Log.Empty() {
		t.Fatalf("stack = %+v: with room all three panels show", st)
	}
	if !(st.Fields.Y < st.Insp.Y && st.Insp.Y < st.Log.Y) {
		t.Errorf("stack order is legend, inspector, log: %+v", st)
	}
}

func TestHandleKeys(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{On: true, Game: struct{}{}}, tileX: 3, tileY: 4, cx: 50, cy: 60}
	p := New(f)
	p.HandleKeys(keys{input.DevUltrafast: true})
	if f.tier != speedUltra {
		t.Fatalf("tier %d", f.tier)
	}
	p.HandleKeys(keys{input.DevUltrafast: true})
	if f.tier != speedNormal {
		t.Fatalf("second press returns to normal, got %d", f.tier)
	}
	p.HandleKeys(keys{input.DevStep: true}) // running: refused
	if f.steps != 0 || len(f.toasts) != 1 {
		t.Fatalf("steps %d toasts %v", f.steps, f.toasts)
	}
	f.tier = speedPaused
	p.HandleKeys(keys{input.DevStep: true})
	if f.steps != 1 {
		t.Fatalf("steps %d", f.steps)
	}
	p.HandleKeys(keys{input.DevActions: true})
	if s := p.State; !s.Menu || s.TX != 3 || s.TY != 4 || s.MX != 50 || s.MY != 60 {
		t.Fatalf("state %+v", *s)
	}
	p.HandleKeys(keys{input.DevLog: true})
	p.HandleKeys(keys{input.DevInspector: true})
	f.env.On = false
	p.HandleKeys(keys{input.DevLog: true})
	if !p.State.Log || !p.State.Insp {
		t.Fatalf("log %v insp %v", p.State.Log, p.State.Insp)
	}
}

func TestActionsNeedASelection(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{On: true, Game: struct{}{}}}
	p := New(f)
	reason := func(label string) string {
		for _, a := range p.Actions(1, 1) {
			if a.Label == label {
				return a.Reason
			}
		}
		t.Fatalf("no %q action", label)
		return ""
	}
	if reason("Kill selected animal") == "" || reason("Lose a part...") == "" {
		t.Fatal("selection actions are disabled without a selection")
	}
	f.sel = Selected{Pawn: &game.PawnView{ID: entity.EntityID(5), Name: "Ada"}}
	if reason("Lose a part...") != "" || reason("Spawn implants") != "" || reason("Kill selected animal") == "" {
		t.Fatal("a colonist enables the colonist actions only")
	}
	if lines := p.inspectLines(); len(lines) != 4 {
		t.Fatalf("pawn inspector %v", lines)
	}
	for _, a := range p.Actions(2, 3) {
		if a.Label == "Start fire here" {
			a.Run(nil)
		}
	}
	if len(f.cmds) != 1 || f.cmds[0].X != 2 || f.cmds[0].Y != 3 {
		t.Fatalf("cmds %+v", f.cmds)
	}
}

func TestClickWithoutFullGameToasts(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{On: true, Game: struct{}{}}}
	p := New(f)
	p.State.Menu = true
	p.Click("dev.act.0")
	if p.State.Menu || len(f.toasts) != 1 {
		t.Fatalf("menu %v toasts %v", p.State.Menu, f.toasts)
	}
}

func TestDrawEveryPanel(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1920x1080"], env: Env{On: true, Game: struct{}{}}}
	p := New(f)
	p.State.Menu, p.State.Log, p.State.Insp = true, true, true
	screen := ebiten.NewImage(1920, 1080)
	for range 4 { // every layer, then off
		p.DrawMapFields(screen)
		p.Draw(screen, 1920, 1080)
		if p.State.Fields.Cycle(layerSource(testLayers())) != nil {
			t.Fatal("cycle")
		}
	}
	if p.State.Fields.On() {
		t.Fatal("cycled back to off")
	}
	if r := p.MenuRect(); r.W <= 0 || len(p.Buttons()) == 0 {
		t.Fatalf("menu %+v", r)
	}
}

type layerSource []game.MapFieldLayer

func (l layerSource) MapFieldLayers() ([]game.MapFieldLayer, error) { return l, nil }
