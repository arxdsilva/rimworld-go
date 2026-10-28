package learn

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeShell is a colony on hand-built settings: it records saves and
// observers and answers the layout, input and state the test sets.
type fakeShell struct {
	s       config.Settings
	scripts bool
	hooked  int
	hint    string
	sensed  Sensed
	lay     hudlayout.Layout
	frame   inputframe.Frame
	now     uint64
}

func newFake() *fakeShell { return &fakeShell{s: config.Defaults()} }

func (f *fakeShell) Hint(t string)                { f.hint = t }
func (f *fakeShell) Size() (int, int)             { return 1280, 720 }
func (f *fakeShell) Cursor() (int, int)           { return 0, 0 }
func (f *fakeShell) MapCursor() (int, int)        { return 0, 0 }
func (f *fakeShell) MapScale() float64            { return 1 }
func (f *fakeShell) Settings() config.Settings    { return f.s }
func (f *fakeShell) KeyName(input.Action) string  { return "K" }
func (f *fakeShell) KeyHint(input.Action) string  { return "[K]" }
func (f *fakeShell) Kit() *uikit.Kit              { return nil }
func (f *fakeShell) Colony() (any, bool)          { return f, f.scripts }
func (f *fakeShell) Chain(func(command.Command))  { f.hooked++ }
func (f *fakeShell) Frame() *inputframe.Frame     { return &f.frame }
func (f *fakeShell) Sense() Sensed                { return f.sensed }
func (f *fakeShell) Layout() hudlayout.Layout     { return f.lay }
func (f *fakeShell) Updates() uint64              { return f.now }
func (f *fakeShell) Save(s config.Settings) error { f.s = s.Normalize(); return nil }
