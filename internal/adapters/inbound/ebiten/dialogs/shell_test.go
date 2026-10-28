package dialogs

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// fakeShell is a headless shell over a real kit.
type fakeShell struct {
	kit   *uikit.Kit
	f     inputframe.Frame
	cmds  []command.Command
	tier  int
	tv    *game.TradeView
	cv    *game.CommsView
	sites []game.WorldSiteView
}

func newFake() (*fakeShell, *Dealings) {
	s := &fakeShell{tier: 2}
	s.kit = uikit.New(fakeHost{s})
	return s, New(s)
}

type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return -1, -1 }
func (h fakeHost) Size() (int, int)         { return 1280, 720 }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(string)              {}
func (h fakeHost) ReducedMotion() bool      { return true }

func (s *fakeShell) Enqueue(c command.Command)     { s.cmds = append(s.cmds, c) }
func (s *fakeShell) World() game.WorldView         { return game.WorldView{} }
func (s *fakeShell) Pawns() []game.PawnView        { return nil }
func (s *fakeShell) MapSize() (int, int)           { return 0, 0 }
func (s *fakeShell) Tick() clock.Tick              { return 0 }
func (s *fakeShell) Date() clock.Date              { return clock.Date{} }
func (s *fakeShell) Paused() bool                  { return s.tier == 0 }
func (s *fakeShell) TogglePause()                  {}
func (s *fakeShell) SetSpeed(t int)                { s.tier = t }
func (s *fakeShell) SpeedTier() int                { return s.tier }
func (s *fakeShell) PauseReason() string           { return "" }
func (s *fakeShell) HoldSpeed(t int)               { s.tier = t }
func (s *fakeShell) Kit() *uikit.Kit               { return s.kit }
func (s *fakeShell) Sites() []game.WorldSiteView   { return s.sites }
func (s *fakeShell) Trade() (game.TradeView, bool) { return opt(s.tv) }
func (s *fakeShell) Comms() (game.CommsView, bool) { return opt(s.cv) }
