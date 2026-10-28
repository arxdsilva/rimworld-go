package options

import (
	"errors"

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
	saved    []string
	saveErr  error
	slots    []ports.SaveSlot
	listErr  error
	loadErr  error
	loaded   []string
}

func newFake() (*fakeShell, *Screen) {
	f := &fakeShell{settings: config.Defaults()}
	return f, New(f)
}

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
func (f *fakeShell) Save(slot string) error {
	f.saved = append(f.saved, slot)
	return f.saveErr
}

var errDisk = errors.New("disk full")

func (f *fakeShell) List() ([]ports.SaveSlot, error) { return f.slots, f.listErr }
func (f *fakeShell) Load(path string) (string, clock.Tick, error) {
	f.loaded = append(f.loaded, path)
	return "colony-autosave", clock.Tick(3 * clock.TicksPerDay), f.loadErr
}
