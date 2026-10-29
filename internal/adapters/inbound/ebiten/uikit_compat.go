package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	uikitpkg "rimworld-go/internal/adapters/inbound/ebiten/uikit"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
)

// The widget kit lives in uikit (ARCH-003). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move
// out one package at a time; each mover repoints its own files to uikit
// and the last one deletes this file (SOLID_GO.md §11). The App's own
// side of the kit (binding, the screen dispatch, the modal window) is in
// ui_state.go and ui_modal.go.

// uiHost is the App as the kit's host.
type uiHost struct{ a *App }

var _ uikit.Host = uiHost{}

func (h uiHost) Frame() *inputframe.Frame  { return &h.a.in }
func (h uiHost) Cursor() (int, int)        { return h.a.currentCursor() }
func (h uiHost) Size() (int, int)          { return h.a.size() }
func (h uiHost) Updates() uint64           { return h.a.updates }
func (h uiHost) KeyRepeat(key string) bool { return h.a.keyRepeat(key) }
func (h uiHost) Cue(id string)             { h.a.cfg.Sound.Play(id, 1, 0) }
func (h uiHost) Hint(text string)          { h.a.hint = text }
func (h uiHost) ReducedMotion() bool       { return h.a.settings.Accessibility.ReducedMotion }

// ---- free functions

// ---- the setup-screen composites (uikit/form.go)

func (a *App) ngButton(id string, r gfx.Rect, label string, o uikitpkg.BtnOpts, fn func()) {
	uikit.StyledButton(a.uiSt(), id, r, label, o, fn)
}
func (a *App) ngChoice(id string, r gfx.Rect, title string, opts []string, cur int, disabled string, pick func(i int)) {
	uikit.Choice(a.uiSt(), id, r, title, opts, cur, disabled, pick)
}

func (a *App) declareOnScreenLayer() { uikit.OnScreenLayer(a.uiSt()) }

// diffSlider is a labelled slider row over *v with the range of the
// difficulty field key; a reason disables it.
func (a *App) diffSlider(f *uikitpkg.Form, id, label string, v *int, key string, format func(int) string, disabled, tip string) {
	f.Slider(a, id, label, v, a.withTick(wizard.DiffSpec(key)), format, disabled, tip)
}
func (a *App) diffCheck(f *uikitpkg.Form, id, label string, v *bool, disabled, tip string) {
	f.Check(a, id, label, v, disabled, tip)
}
func (a *App) diffStepper(f *uikitpkg.Form, id, label string, v *int, lo, hi int, tip string) {
	f.Stepper(a, id, label, v, lo, hi, tip)
}

// ---- the kit's methods, on the App

// withTick gives a slider the App's step sound (sound_director.go).
func (a *App) withTick(sp uikitpkg.SliderSpec) uikitpkg.SliderSpec {
	if sp.Tick == nil {
		sp.Tick = func(lo, hi, old, cur float64) { a.sliderCue(sp, lo, hi, old, cur) }
	}
	return sp
}

// Kit is the widget kit as uikit.Owner sees the App (the form composites
// take an Owner; ports.Widgets is the same method on the shell).
func (a *App) Kit() *uikit.Kit { return a.uiSt() }
