package architect

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// fakeShell answers the calls the package tests make; the nil Shell it
// embeds panics on any other, so a test that reaches one says so.
type fakeShell struct {
	Shell
	env     Env
	w       game.WorldView
	cmds    []command.Command
	hint    string
	intents []ports.Intent
	floats  int // CloseFloat calls
	syncs   int // SyncOverlays calls
	locked  map[string]string
	hover   [2]int // the tile under the pointer
}

func newFake(sim Sim) (*Tool, *fakeShell) {
	f := &fakeShell{env: Env{Sim: sim, Sel: &selection.Model{}}}
	return New(f), f
}

func (f *fakeShell) Env() Env                               { return f.env }
func (f *fakeShell) World() game.WorldView                  { return f.w }
func (f *fakeShell) Pawns() []game.PawnView                 { return nil }
func (f *fakeShell) Tick() clock.Tick                       { return 1 }
func (f *fakeShell) Enqueue(c command.Command)              { f.cmds = append(f.cmds, c) }
func (f *fakeShell) Hint(s string)                          { f.hint = s }
func (f *fakeShell) Do(it ports.Intent)                     { f.intents = append(f.intents, it) }
func (f *fakeShell) CloseFloat()                            { f.floats++ }
func (f *fakeShell) SyncOverlays()                          { f.syncs++ }
func (f *fakeShell) LockedBy(key string) string             { return f.locked[key] }
func (f *fakeShell) MapCursor() (int, int)                  { return f.hover[0], f.hover[1] }
func (f *fakeShell) TileAt(mx, my int) (int, int)           { return mx, my }
func (f *fakeShell) Layout() hudlayout.Layout               { return hudlayout.Layout{} }
func (f *fakeShell) BuildingTitle(game.BuildingView) string { return "Thing" }
