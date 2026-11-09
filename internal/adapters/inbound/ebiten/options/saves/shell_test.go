package saves

import (
	"errors"
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
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
	hints    []string
	saved    []string
	metas    []ports.SaveMeta
	saveErr  error
	slots    []ports.SaveSlot
	listErr  error
	loadErr  error
	loaded   []string
	deleted  []string
	season   string  // the World read model's season, which every save's header carries
	ui       *testUI // the widget kit, made on first use (dialog_test.go)
}

func newFake() (*fakeShell, *Book) {
	f := &fakeShell{settings: config.Defaults()}
	return f, New(f)
}

func (f *fakeShell) SaveEnv() Env              { return f.env }
func (f *fakeShell) Settings() config.Settings { return f.settings }
func (f *fakeShell) Hint(s string)             { f.hints = append(f.hints, s) }
func (f *fakeShell) Date() clock.Date          { return f.date }
func (f *fakeShell) Tick() clock.Tick          { return f.tick }
func (f *fakeShell) World() game.WorldView     { return game.WorldView{Season: f.season} }
func (f *fakeShell) Save(meta ports.SaveMeta) error {
	f.saved = append(f.saved, meta.Name)
	f.metas = append(f.metas, meta)
	return f.saveErr
}

var errDisk = errors.New("disk full")

func (f *fakeShell) List() ([]ports.SaveSlot, error) { return f.slots, f.listErr }
func (f *fakeShell) Load(path string) (ports.SaveMeta, clock.Tick, error) {
	f.loaded = append(f.loaded, path)
	return ports.SaveMeta{Name: "colony-autosave"}, clock.Tick(3 * clock.TicksPerDay), f.loadErr
}
func (f *fakeShell) Delete(name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

// T and Tf are the English catalog: the tests read the source text.
func (f *fakeShell) T(src string) string { return src }

func (f *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
