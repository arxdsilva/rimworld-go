package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
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
func (f *fakeShell) World() game.WorldView            { return game.WorldView{} }
func (f *fakeShell) PawnLabel(p game.PawnView) string { return p.Name }
func (f *fakeShell) Selected() (entity.EntityID, bool) {
	return f.pawn, f.pawn != 0
}
func (f *fakeShell) Temp(int) string { return "20C" }
