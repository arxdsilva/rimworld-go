package windows

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// fakeShell is a headless shell: a real kit over a fixed 1280x720 host,
// a fixed Window rect and a recording Commands, Dispatcher and Float. The
// embedded Shell is nil: a test that reaches any other method panics.
type fakeShell struct {
	Shell
	kit     *uikit.Kit
	f       inputframe.Frame
	env     Env
	pawns   []game.PawnView
	world   game.WorldView
	cmds    []command.Command
	intents []ports.Intent
	floats  int
}

func newFake() (*fakeShell, *Set) {
	sh := &fakeShell{}
	sh.kit = uikit.New(fakeHost{sh})
	return sh, New(sh)
}

func (s *fakeShell) Kit() *uikit.Kit                          { return s.kit }
func (s *fakeShell) Env() Env                                 { return s.env }
func (s *fakeShell) Size() (int, int)                         { return 1280, 720 }
func (s *fakeShell) Cursor() (int, int)                       { return -1, -1 }
func (s *fakeShell) Pawns() []game.PawnView                   { return s.pawns }
func (s *fakeShell) World() game.WorldView                    { return s.world }
func (s *fakeShell) Enqueue(c command.Command)                { s.cmds = append(s.cmds, c) }
func (s *fakeShell) Do(it ports.Intent)                       { s.intents = append(s.intents, it) }
func (s *fakeShell) Float(_, _ float32, _ []game.OrderOption) { s.floats++ }
func (s *fakeShell) WindowRect() gfx.Rect                     { return gfx.Rect{X: 8, Y: 300, W: 900, H: 380} }
func (s *fakeShell) PawnLabel(p game.PawnView) string         { return p.Name }

// fakeHost is the kit's host.
type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return h.s.Cursor() }
func (h fakeHost) Size() (int, int)         { return h.s.Size() }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(string)              {}
func (h fakeHost) ReducedMotion() bool      { return true }
