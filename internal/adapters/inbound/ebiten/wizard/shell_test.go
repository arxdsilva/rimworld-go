package wizard

import (
	"errors"
	"sort"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeShell is a headless shell: a real kit over a fake host, a screen
// stack that routes the navigation intents the way the root's navigator
// does (Back asks GoBack, then Popped), and a recording Host.
type fakeShell struct {
	t        testing.TB
	w        *Flow
	kit      *uikit.Kit
	f        inputframe.Frame
	n        uint64
	mx, my   int
	cues     []string
	hint     string
	stack    []ports.Screen
	intents  []ports.Intent
	settings config.Settings
	saved    int
	store    ScenarioStore
	kept     *Draft
	ticks    int
	loads    int
	started  int
	loader   *fakeLoader
	loadErr  error
	sizeNote string
}

func newFakeShell(t testing.TB) (*fakeShell, *Flow) {
	sh := &fakeShell{t: t, stack: []ports.Screen{ports.ScreenMainMenu}, store: newMemStore()}
	sh.kit = uikit.New(fakeHost{sh})
	sh.w = New(sh)
	return sh, sh.w
}

// fakeHost is the kit's host.
type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return h.s.mx, h.s.my }
func (h fakeHost) Size() (int, int)         { return 1280, 720 }
func (h fakeHost) Updates() uint64          { return h.s.n }
func (h fakeHost) KeyRepeat(key string) bool {
	return h.s.f.Key(key) || inputframe.RepeatFire(h.s.kit.HeldN[key])
}
func (h fakeHost) Cue(id string)       { h.s.cues = append(h.s.cues, id) }
func (h fakeHost) Hint(text string)    { h.s.hint = text }
func (h fakeHost) ReducedMotion() bool { return false }

func (s *fakeShell) Hint(text string)               { s.hint = text }
func (s *fakeShell) Settings() config.Settings      { return s.settings }
func (s *fakeShell) KeyName(input.Action) string    { return "" }
func (s *fakeShell) KeyHint(input.Action) string    { return "" }
func (s *fakeShell) Kit() *uikit.Kit                { return s.kit }
func (s *fakeShell) KeepDraft(d *Draft)             { s.kept = d }
func (s *fakeShell) SaveSettings(c config.Settings) { s.settings, s.saved = c, s.saved+1 }
func (s *fakeShell) MapSizeNote(int) string         { return s.sizeNote }

func (s *fakeShell) top() ports.Screen { return s.stack[len(s.stack)-1] }

func (s *fakeShell) Do(it ports.Intent) {
	s.intents = append(s.intents, it)
	switch it := it.(type) {
	case ports.Push:
		s.stack = append(s.stack, it.Screen)
	case ports.Back:
		if len(s.stack) > 1 && s.w.GoBack() {
			s.kit.Modals = nil
			was := s.top()
			s.stack = s.stack[:len(s.stack)-1]
			s.w.Popped(was, s.top())
		}
	}
}

func (s *fakeShell) Env() Env {
	return Env{Screen: s.top(), Scenarios: s.store, Capture: true, DefaultSeed: 0}
}

func (s *fakeShell) SliderTick(sp uikit.SliderSpec) uikit.SliderSpec {
	sp.Tick = func(lo, hi, old, cur float64) { s.ticks++ }
	return sp
}

func (s *fakeShell) Load(st *game.Setup) (Loader, error) {
	s.loads++
	if st == nil {
		return nil, errors.New("the setup is incomplete")
	}
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.loader == nil {
		s.loader = &fakeLoader{need: 1}
	}
	s.loader.s, s.loader.setup = s, *st
	return s.loader, nil
}

func (s *fakeShell) DrawPortrait(*ebiten.Image, look.Look, float64, float64, float64, float64) bool {
	return false
}
func (s *fakeShell) DrawDiorama(*ebiten.Image, int, int, float64) bool { return false }

// fakeLoader is done after need steps; Finish starts the "colony" the way
// the root does (the draft is dropped).
type fakeLoader struct {
	s           *fakeShell
	setup       game.Setup
	steps, need int
	err         error
}

func (l *fakeLoader) Step(int) (string, float32, bool) {
	l.steps++
	return "", float32(l.steps) / float32(l.need), l.steps >= l.need
}

func (l *fakeLoader) Finish() error {
	if l.err != nil {
		return l.err
	}
	l.s.started++
	l.s.w.Drop()
	return nil
}

// frame runs one Update and Draw of the screen on top, the way the shell
// does: the kit's keys, the frame, the modals, the screen's widgets, then
// the screen and the kit's paint list.
func (s *fakeShell) frame(raw inputframe.Raw) {
	s.t.Helper()
	s.n++
	s.mx, s.my = raw.Mx, raw.My
	s.kit.CountHeld(&raw)
	s.kit.TakePages(&raw, raw.Mx, raw.My)
	s.kit.TakeTab(&raw, false)
	s.f = inputframe.NewFrame(raw)
	s.kit.Begin()
	s.kit.RunModals(nil)
	scr := s.top()
	s.w.Declare(scr, 1280, 720)
	img := ebiten.NewImage(1280, 720)
	s.w.Draw(scr, img, 1280, 720)
	s.kit.Paint(img)
}

func (s *fakeShell) idle() { s.frame(inputframe.Raw{}) }

// click runs a widget's click by id (the id-driven click path) and the
// frame after it; the widget must have been declared.
func (s *fakeShell) click(id string) {
	s.t.Helper()
	if !s.kit.Click(id) {
		s.t.Fatalf("no widget %q on %s (modal %q)", id, s.top(), s.kit.Top())
	}
	s.idle()
}

// memStore is an in-memory scenario folder.
type memStore struct{ files map[string][]byte }

func newMemStore() *memStore { return &memStore{files: map[string][]byte{}} }

func (m *memStore) List() ([]ScenarioFile, error) {
	var out []ScenarioFile
	for n := range m.files {
		out = append(out, ScenarioFile{Name: n, Modified: "2026-01-02 03:04"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memStore) Load(name string) ([]byte, error) {
	b, ok := m.files[name]
	if !ok {
		return nil, errors.New("no such file")
	}
	return b, nil
}

func (m *memStore) Save(name string, data []byte) error {
	m.files[name] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Delete(name string) error {
	delete(m.files, name)
	return nil
}
