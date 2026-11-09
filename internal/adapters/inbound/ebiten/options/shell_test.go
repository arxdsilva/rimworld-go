package options

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
)

// fakeShell is a headless shell: the roles the tests do not call are the
// nil embedded Shell (a call to one panics), the rest record what happens.
type fakeShell struct {
	Shell
	env      Env
	settings config.Settings
	date     clock.Date
	tick     clock.Tick
	rb       RebindState
	set      []config.Settings
	hints    []string
	cues     []string
	intents  []ports.Intent
}

func newFake() (*fakeShell, *Screen) {
	f := &fakeShell{settings: config.Defaults()}
	return f, New(f)
}

func (f *fakeShell) Do(it ports.Intent)        { f.intents = append(f.intents, it) }
func (f *fakeShell) Env() Env                  { return f.env }
func (f *fakeShell) Settings() config.Settings { return f.settings }
func (f *fakeShell) SetSettings(s config.Settings) {
	f.settings = s
	f.set = append(f.set, s)
}
func (f *fakeShell) Hint(s string)        { f.hints = append(f.hints, s) }
func (f *fakeShell) Cue(id string)        { f.cues = append(f.cues, id) }
func (f *fakeShell) Toast(string)         {}
func (f *fakeShell) Size() (int, int)     { return 1280, 720 }
func (f *fakeShell) Rebind() *RebindState { return &f.rb }
func (f *fakeShell) Date() clock.Date     { return f.date }
func (f *fakeShell) Tick() clock.Tick     { return f.tick }

// T and Tf are the English catalog: the tests read the source text.
func (f *fakeShell) T(src string) string { return src }

func (f *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
