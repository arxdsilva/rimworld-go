package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// fakeShell answers the calls the package tests make; the nil Shell it
// embeds panics on any other, so a test that reaches one says so.
type fakeShell struct {
	Shell
	env      Env
	w        game.WorldView
	pawns    []game.PawnView
	lay      hudlayout.Layout
	set      config.Settings
	present  uint64
	cmds     []command.Command
	centred  [][2]float64
	ui       uikit.Kit
	selected bool
}

// fakeGame is the simulation's extra read models: the pawns only.
type fakeGame struct{ pawns []game.PawnView }

func (g fakeGame) Snapshot() []game.PawnView    { return g.pawns }
func (g fakeGame) TemperatureAt(x, y int) int   { return 150 }
func (g fakeGame) Factions() []game.FactionView { return nil }
func (g fakeGame) Activity() schedule.Activity  { return schedule.Work }
func newFake(pawns ...game.PawnView) (*Chrome, *fakeShell) {
	f := &fakeShell{env: Env{Game: fakeGame{pawns}, ViewCached: true, Colony: true}, pawns: pawns}
	return New(f), f
}

func (f *fakeShell) Env() Env                          { return f.env }
func (f *fakeShell) World() game.WorldView             { return f.w }
func (f *fakeShell) Pawns() []game.PawnView            { return f.pawns }
func (f *fakeShell) Layout() hudlayout.Layout          { return f.lay }
func (f *fakeShell) Settings() config.Settings         { return f.set }
func (f *fakeShell) SetSettings(s config.Settings)     { f.set = s }
func (f *fakeShell) Present() uint64                   { return f.present }
func (f *fakeShell) Cursor() (int, int)                { return -1, -1 }
func (f *fakeShell) Size() (int, int)                  { return 1280, 720 }
func (f *fakeShell) Kit() *uikit.Kit                   { return &f.ui }
func (f *fakeShell) Enqueue(c command.Command)         { f.cmds = append(f.cmds, c) }
func (f *fakeShell) CenterOnMap(wx, wy float64)        { f.centred = append(f.centred, [2]float64{wx, wy}) }
func (f *fakeShell) Picked(id entity.EntityID) bool    { return false }
func (f *fakeShell) Selected() (entity.EntityID, bool) { return 0, f.selected }
