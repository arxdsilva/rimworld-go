package orders

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func entries() []game.OrderOption {
	return []game.OrderOption{{Label: "Ada"}, {Label: "Bo", Disabled: true, Reason: "asleep"}, {Label: "Cy"}}
}

func TestPickMenuCallsBackAndCloses(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	picked := -1
	c.OpenPick(300, 300, false, entries(), func(i int) { picked = i })
	c.ChooseFloat("float.1")
	if picked != -1 || !c.Float.Open || f.hint != "asleep" {
		t.Fatalf("disabled entry: picked=%d open=%v hint=%q", picked, c.Float.Open, f.hint)
	}
	c.ChooseFloat("float.2")
	if picked != 2 || c.Float.Open {
		t.Fatalf("picked=%d open=%v", picked, c.Float.Open)
	}
}

func TestFloatRectFooterAndUpward(t *testing.T) {
	t.Parallel()
	c := New(newFake())
	c.OpenPick(300, 400, true, entries(), func(int) {})
	r := c.FloatRect()
	if r.H != 3*FloatRowH || r.Y+r.H != 400 {
		t.Fatalf("a pick menu is its rows and ends at its anchor: %+v", r)
	}
	c.Float = Float{Open: true, X: 300, Y: 300, Opts: entries()}
	if r := c.FloatRect(); r.H != 3*FloatRowH+FloatHintH {
		t.Fatalf("the order menu keeps its footer: %+v", r)
	}
	if got := len(c.FloatButtons()); got != 3 {
		t.Fatalf("buttons = %d", got)
	}
}

func TestChooseFloatEnqueuesTheOrder(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	c.Float = Float{Open: true, Opts: []game.OrderOption{{Label: "Go", Cmd: command.Command{Kind: command.MovePawn, X: 2}}}}
	c.ChooseFloat("float.0")
	if len(f.cmds) != 1 || f.cmds[0].X != 2 || c.Float.Open {
		t.Fatalf("cmds=%+v open=%v", f.cmds, c.Float.Open)
	}
	c.ChooseFloat("float.9") // out of range: nothing
	c.ChooseFloat("float.x")
}

func TestRightClickDraftedWalksAndUndraftedOpensMenu(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	c := New(f)
	c.RightClickTile(4, 5, 10, 10)
	if len(f.cmds) != 1 || f.cmds[0].Kind != command.MovePawn || f.cmds[0].X != 4 || f.cmds[0].Y != 5 {
		t.Fatalf("a drafted colonist walks: %+v", f.cmds)
	}
	f.cmds, f.pawns = nil, []game.PawnView{{ID: 1}}
	f.opts = []game.OrderOption{{Label: "Go here"}}
	c.RightClickTile(4, 5, 10, 10)
	if len(f.cmds) != 0 || !c.Float.Open || c.Float.X != 16 {
		t.Fatalf("an undrafted colonist gets the menu: %+v %+v", f.cmds, c.Float)
	}
	f.opts = nil
	c.OpenOrders(1, 1, 0, 0)
	if c.Float.Open {
		t.Fatal("no options closes the menu")
	}
}

func TestRightClickOnRaiderOffersTheMenu(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	f.hostiles = []game.PawnView{{ID: 9, X: 4, Y: 5}}
	f.opts = []game.OrderOption{{Label: "Shoot"}}
	c := New(f)
	c.RightClickTile(4, 5, 0, 0)
	if len(f.cmds) != 0 || !c.Float.Open {
		t.Fatalf("a raider gets the order menu: %+v", f.cmds)
	}
}

func TestToggleDraft(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	c.ToggleDraft()
	if len(f.cmds) != 0 {
		t.Fatal("nobody selected")
	}
	f.selected, f.pawns = 3, []game.PawnView{{ID: 3, Drafted: true}}
	c.ToggleDraft()
	if len(f.cmds) != 1 || f.cmds[0].Kind != command.DraftPawn || f.cmds[0].Enabled {
		t.Fatalf("cmds %+v", f.cmds)
	}
}

func TestFireOrdersOnlyForDraftedOnFire(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Fires: []game.FireView{{X: 4, Y: 5, Size: 30}}}
	if FireOrders(game.PawnView{ID: 1}, w, 4, 5) != nil || FireOrders(game.PawnView{ID: 1, Drafted: true}, w, 0, 0) != nil {
		t.Fatal("menu for an undrafted pawn or off the fire")
	}
	opts := FireOrders(game.PawnView{ID: 7, Drafted: true}, w, 4, 5)
	if len(opts) != 2 || opts[1].Cmd.Order != command.OrderExtinguish || FloatIcon(opts[1]) != "extinguish" || FloatIcon(opts[0]) != "" {
		t.Fatalf("opts %+v", opts)
	}
}

func TestFormationDragSendsOneMovePerDraftedColonist(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	f.tx, f.ty = 6, 7
	c := New(f)
	f.in.Raw.Press[inputframe.Right], f.in.Raw.Down[inputframe.Right] = true, true
	c.HandleOrders()
	if !c.Form.On || c.Form.X0 != 6 {
		t.Fatalf("the press starts a formation: %+v", c.Form)
	}
	f.in = inputframe.NewFrame(inputframe.Raw{}) // released
	c.HandleOrders()
	if c.Form.On || len(f.cmds) != 1 || f.cmds[0].X != 6 || f.cmds[0].Y != 7 {
		t.Fatalf("the release sends the move: %+v %+v", c.Form, f.cmds)
	}
}

func TestDrawSmoke(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	c := New(f)
	img := ebiten.NewImage(64, 64)
	c.OpenPick(10, 10, false, entries(), func(int) {})
	c.DrawFloat(img)
	c.Form = Formation{On: true, X0: 1, Y0: 1, X1: 2, Y1: 1}
	c.DrawFormation(img)
	c.OpenSearch()
	c.DrawSearch(img, 64)
	DrawRoute(img, 16, game.PawnView{HasOrder: true, OrderX: 2, OrderY: 2}, 0, 0, func(x, y float64) (float32, float32) { return float32(x * 16), float32(y * 16) })
}

func TestHasPawnOrder(t *testing.T) {
	t.Parallel()
	opt := func(c command.Command) game.OrderOption { return game.OrderOption{Cmd: c} }
	cases := []struct {
		name string
		opts []game.OrderOption
		want bool
	}{
		{"emergency tend", []game.OrderOption{opt(command.Command{Kind: command.EmergencyTend})}, true},
		{"prioritize with target", []game.OrderOption{opt(command.Command{Kind: command.Prioritize, Order: command.OrderRescue, Target: 7})}, true},
		{"prioritize without target", []game.OrderOption{opt(command.Command{Kind: command.Prioritize})}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		if got := hasPawnOrder(tc.opts); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestDraftedRightClickOnPawnOrderOpensMenu(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	f.opts = []game.OrderOption{{Label: "Rescue Ann", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderRescue, Pawn: 1, Target: 7}}}
	c := New(f)
	c.RightClickTile(4, 5, 10, 10)
	if len(f.cmds) != 0 || !c.Float.Open {
		t.Fatalf("a pawn-targeted option opens the menu: %+v open=%v", f.cmds, c.Float.Open)
	}
}
