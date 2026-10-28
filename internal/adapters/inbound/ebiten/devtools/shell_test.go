package devtools

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeShell is a headless shell: fixed screen, cursor and layout, a
// recording Host. The ports roles the tests do not reach stay nil.
type fakeShell struct {
	ports.Widgets
	ports.View
	env    Env
	layout hudlayout.Layout
	sel    Selected
	tier   int
	steps  int
	toasts []string
	cmds   []command.Command
	pawns  []game.PawnView
	cx, cy int
	tileX  int
	tileY  int
}

func (f *fakeShell) TilePx() float64                           { return 32 }
func (f *fakeShell) WorldToScreen(int, int) (float32, float32) { return 0, 0 }
func (f *fakeShell) Size() (int, int)                          { return 1280, 720 }
func (f *fakeShell) Cursor() (int, int)                        { return f.cx, f.cy }
func (f *fakeShell) MapCursor() (int, int)                     { return f.cx, f.cy }
func (f *fakeShell) MapScale() float64                         { return 1 }
func (f *fakeShell) Enqueue(c command.Command)                 { f.cmds = append(f.cmds, c) }
func (f *fakeShell) World() game.WorldView                     { return game.WorldView{} }
func (f *fakeShell) Pawns() []game.PawnView                    { return f.pawns }
func (f *fakeShell) MapSize() (int, int)                       { return 64, 64 }
func (f *fakeShell) Tick() clock.Tick                          { return 7 }
func (f *fakeShell) Date() clock.Date                          { return clock.Date{} }
func (f *fakeShell) TileAt(int, int) (int, int)                { return f.tileX, f.tileY }
func (f *fakeShell) Paused() bool                              { return f.tier == 0 }
func (f *fakeShell) TogglePause()                              {}
func (f *fakeShell) SetSpeed(t int)                            { f.tier = t }
func (f *fakeShell) SpeedTier() int                            { return f.tier }
func (f *fakeShell) PauseReason() string                       { return "" }
func (f *fakeShell) Settings() config.Settings                 { return config.Settings{} }
func (f *fakeShell) KeyName(input.Action) string               { return "K" }
func (f *fakeShell) KeyHint(input.Action) string               { return "[K]" }
func (f *fakeShell) Env() Env                                  { return f.env }
func (f *fakeShell) HUD() hudlayout.Layout                     { return f.layout }
func (f *fakeShell) Toast(s string)                            { f.toasts = append(f.toasts, s) }
func (f *fakeShell) Selected() Selected                        { return f.sel }
func (f *fakeShell) StepTick()                                 { f.steps++ }
func (f *fakeShell) Icon(string) *ebiten.Image                 { return nil }
