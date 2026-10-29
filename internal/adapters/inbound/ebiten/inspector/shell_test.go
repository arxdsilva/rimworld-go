package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeSim is a colony with nothing to report (the shell is live).
type fakeSim struct{ Sim }

// fakeShell is the inspector's shell for unit tests: the embedded Shell
// is nil, so a call the fake does not override fails loudly.
type fakeShell struct {
	Shell
	env      Env
	sel      *selection.Model
	pawn     entity.EntityID // the selected pawn (0: none)
	hint     string
	cmds     []command.Command
	clicks   []string
	pawns    []game.PawnView
	hostiles []game.PawnView
	world    game.WorldView
	kit      *uikit.Kit
	frame    inputframe.Frame
}

// fakeHost is the kit's host: a fixed 1280x720 screen, the cursor off it.
type fakeHost struct{ f *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.f.frame }
func (h fakeHost) Cursor() (int, int)       { return -1, -1 }
func (h fakeHost) Size() (int, int)         { return 1280, 720 }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(string)              {}
func (h fakeHost) ReducedMotion() bool      { return true }

func (f *fakeShell) Kit() *uikit.Kit {
	if f.kit == nil {
		f.kit = uikit.New(fakeHost{f})
	}
	return f.kit
}

// frameOnce runs one kit frame the way a host does: Begin, then the
// modals declare their widgets.
func (f *fakeShell) frameOnce() {
	k := f.Kit()
	f.frame = inputframe.NewFrame(inputframe.Raw{Mx: -1, My: -1})
	k.Begin()
	k.RunModals(nil)
}

// clickWidget declares a frame, clicks widget id and declares the next
// frame, which hands the click to the control's handler.
func (f *fakeShell) clickWidget(id string) bool {
	f.frameOnce()
	ok := f.Kit().Click(id)
	f.frameOnce()
	return ok
}

func newFake() *fakeShell {
	return &fakeShell{env: Env{Sim: fakeSim{}}, sel: &selection.Model{}}
}

func (f *fakeShell) Env() Env                         { return f.env }
func (f *fakeShell) Selection() *selection.Model      { return f.sel }
func (f *fakeShell) Hint(s string)                    { f.hint = s }
func (f *fakeShell) Enqueue(c command.Command)        { f.cmds = append(f.cmds, c) }
func (f *fakeShell) Click(id string)                  { f.clicks = append(f.clicks, id) }
func (f *fakeShell) Layout() hudlayout.Layout         { return hudlayout.Layout{} }
func (f *fakeShell) Pawns() []game.PawnView           { return f.pawns }
func (f *fakeShell) Hostiles() []game.PawnView        { return f.hostiles }
func (f *fakeShell) World() game.WorldView            { return f.world }
func (f *fakeShell) PawnLabel(p game.PawnView) string { return p.Name }
func (f *fakeShell) Selected() (entity.EntityID, bool) {
	return f.pawn, f.pawn != 0
}
func (f *fakeShell) Temp(int) string { return "20C" }

func (f *fakeShell) MoveState(game.BuildingView) (bool, string) { return true, "" }
func (f *fakeShell) LockedBy(string) string                     { return "" }
func (f *fakeShell) Buildable(string) bool                      { return true }
