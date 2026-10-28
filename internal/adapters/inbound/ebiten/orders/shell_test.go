package orders

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeShell answers the calls the tests reach; the embedded Shell is nil,
// so a call outside this list panics and names itself.
type fakeShell struct {
	Shell
	world    game.WorldView
	pawns    []game.PawnView
	selected entity.EntityID
	hostiles []game.PawnView
	opts     []game.OrderOption
	in       inputframe.Frame
	cmds     []command.Command
	hint     string
	sel      *selection.Model
	tx, ty   int
}

func newFake() *fakeShell {
	f := &fakeShell{in: inputframe.NewFrame(inputframe.Raw{})}
	f.sel = selection.New(f, nil)
	return f
}

func (f *fakeShell) Hint(s string)                                         { f.hint = s }
func (f *fakeShell) Size() (int, int)                                      { return 1280, 720 }
func (f *fakeShell) Cursor() (int, int)                                    { return 0, 0 }
func (f *fakeShell) Enqueue(c command.Command)                             { f.cmds = append(f.cmds, c) }
func (f *fakeShell) World() game.WorldView                                 { return f.world }
func (f *fakeShell) Pawns() []game.PawnView                                { return f.pawns }
func (f *fakeShell) Tick() clock.Tick                                      { return 1 }
func (f *fakeShell) Selected() (entity.EntityID, bool)                     { return f.selected, f.selected != 0 }
func (f *fakeShell) Hostiles() []game.PawnView                             { return f.hostiles }
func (f *fakeShell) Layout() hudlayout.Layout                              { return hudlayout.Layout{} }
func (f *fakeShell) Frame() (*inputframe.Frame, uint64)                    { return &f.in, 0 }
func (f *fakeShell) BuildingTitle(b game.BuildingView) string              { return "B:" + b.Kind }
func (f *fakeShell) OrdersAt(entity.EntityID, int, int) []game.OrderOption { return f.opts }
func (f *fakeShell) Selection() *selection.Model                           { return f.sel }
func (f *fakeShell) MapCursor() (int, int)                                 { return 0, 0 }
func (f *fakeShell) TileAt(int, int) (int, int)                            { return f.tx, f.ty }
func (f *fakeShell) ChromeHit(int, int) bool                               { return false }
func (f *fakeShell) MapSize() (int, int)                                   { return 20, 20 }
func (f *fakeShell) TilePx() float64                                       { return 16 }
func (f *fakeShell) WorldToScreen(x, y int) (float32, float32) {
	return float32(x * 16), float32(y * 16)
}
