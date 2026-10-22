package ebiten

import (
	"rimworld-go/internal/platform/input"
)

// inputFrame is this Update's input: the raw devices plus the actions the
// bindings resolved for the context stack. Handlers read it instead of
// Ebitengine, and take() a mouse button they used so no later handler
// acts on the same click.
type inputFrame struct {
	raw   rawInput
	keys  map[string]bool // keys pressed this frame
	res   input.Result
	stack []input.Context
	taken [numMouse]bool
}

func newFrame(raw rawInput) inputFrame {
	f := inputFrame{raw: raw, keys: make(map[string]bool, len(raw.pressed))}
	for _, k := range raw.pressed {
		f.keys[k] = true
	}
	return f
}

// fired reports a Press/Toggle action fired this frame.
func (f *inputFrame) fired(a input.Action) bool { return f.res.Fired(a) }

// held reports a Hold action is active.
func (f *inputFrame) held(a input.Action) bool { return f.res.Held(a) }

// key reports a raw key went down (text fields and key capture read it).
func (f *inputFrame) key(name string) bool { return f.keys[name] }

// clicked reports button b went down this frame and nobody took it yet.
func (f *inputFrame) clicked(b mouseBtn) bool { return f.raw.press[b] && !f.taken[b] }

// take marks b's press as used.
func (f *inputFrame) take(b mouseBtn) { f.taken[b] = true }

// down reports b is held.
func (f *inputFrame) down(b mouseBtn) bool { return f.raw.down[b] }

// released reports b came up this frame.
func (f *inputFrame) released(b mouseBtn) bool { return f.raw.release[b] }

func (f *inputFrame) shift() bool { return f.raw.mods&input.ModShift != 0 }
func (f *inputFrame) ctrl() bool  { return f.raw.mods&input.ModCtrl != 0 }
func (f *inputFrame) alt() bool   { return f.raw.mods&input.ModAlt != 0 }

// top is the topmost active context.
func (f *inputFrame) top() input.Context {
	if len(f.stack) == 0 {
		return input.CtxGlobal
	}
	return f.stack[0]
}

// pollInput reads the devices once and resolves this frame's actions.
func (a *App) pollInput() {
	if a.src == nil {
		a.src = &ebitenInput{}
	}
	raw := a.src.poll()
	a.in = newFrame(raw)
	a.in.stack = a.contextStack()
	a.in.res = input.Resolve(input.Frame{Pressed: raw.pressed, Held: raw.held, Mods: raw.mods}, a.bindings(), a.in.stack)
}

// bindings are the live key bindings (the settings' overrides over the
// defaults).
func (a *App) bindings() input.Bindings {
	if a.binds == nil {
		a.binds = input.FromSettings(a.settings.Controls.Bindings)
	}
	return a.binds
}

// textActive reports a text field or key capture has the keyboard.
func (a *App) textActive() bool {
	switch {
	case a.rebind.on && a.nav.screen() == screenOptions:
		return true
	case a.search.open && a.colonyLive():
		return true
	case a.nav.screen() == screenWorldGen && a.wiz.editing:
		return true
	}
	return false
}

// colonyLive is the running colony with no overlay on top.
func (a *App) colonyLive() bool {
	return a.nav.screen() == screenColony && a.nav.overlay() == overlayNone && a.renderer != nil
}

// toolActive reports a tool owns the pointer: Move mode, an Architect
// tool, or a formation drag.
func (a *App) toolActive() bool {
	return a.bldgMove.on || (a.archOpen && a.archTool != "") || a.form.on
}

// modalOpen reports a menu sits over the map.
func (a *App) modalOpen() bool { return a.float.open || a.dev.menu }

// contextStack is the active input layers, top first (UI-016): text
// entry, modal, tool, panel, colony map, global, dev.
func (a *App) contextStack() []input.Context {
	if a.textActive() {
		return []input.Context{input.CtxText}
	}
	if !a.colonyLive() {
		return []input.Context{input.CtxGlobal}
	}
	var s []input.Context
	switch {
	case a.modalOpen():
		s = append(s, input.CtxModal)
	default:
		if a.toolActive() {
			s = append(s, input.CtxTool)
		}
		if a.workOpen {
			s = append(s, input.CtxPanel)
		}
		s = append(s, input.CtxMap)
	}
	s = append(s, input.CtxGlobal)
	if a.devMode() {
		s = append(s, input.CtxDev)
	}
	return s
}

// keyHint is " [R]" for an action's first key ("" when unbound), for
// labels and tooltips that show the live binding.
func (a *App) keyHint(act input.Action) string {
	if l := a.bindings().Label(act); l != "" {
		return " [" + l + "]"
	}
	return ""
}

// queueing reports the queue modifier (Shift) held.
func (a *App) queueing() bool { return a.in.shift() }

// handleTextInput feeds the focused text field or key capture.
func (a *App) handleTextInput() {
	if a.in.top() != input.CtxText {
		return
	}
	switch {
	case a.rebind.on && a.nav.screen() == screenOptions:
		a.captureBinding() // controls_ui.go
	case a.search.open:
		a.searchTyping() // search_ui.go
	case a.wiz.editing:
		a.handleSeedInput()
	}
}

// pressAccept is Enter outside a text field: the wizard's Next/Start.
func (a *App) pressAccept() {
	sw, sh := a.size()
	for _, b := range a.layoutButtons(sw, sh) {
		switch b.id {
		case "wizard.next", "wizard.start", "world.generate":
			if !b.disabled {
				a.applyClick(b.id)
			}
			return
		}
	}
}
