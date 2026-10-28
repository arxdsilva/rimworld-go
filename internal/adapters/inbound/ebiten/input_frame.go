package ebiten

import (
	"rimworld-go/internal/platform/input"
)

// pollInput reads the devices once and resolves this frame's actions.
func (a *App) pollInput() {
	if a.src == nil {
		a.src = &ebitenInput{}
	}
	raw := a.src.Poll()
	a.uiPreInput(&raw) // widget kit: held keys, Esc for a modal, Tab focus (ui_state.go)
	a.in = newFrame(raw)
	a.in.Ui.Mx, a.in.Ui.My = a.toLogical(raw.Mx, raw.My)
	a.in.Stack = a.contextStack()
	a.in.Res = input.Resolve(input.Frame{Pressed: raw.Pressed, Held: raw.Held, Mods: raw.Mods}, a.bindings(), a.in.Stack)
	a.uiBegin() // widget kit: tooltips, modals, the screen's widgets (ui_state.go)
}

// keyRepeat reports the key went down this frame or is repeating while
// held (first repeat after 24 frames, then every 3). Text fields and list
// navigation read it.
func (a *App) keyRepeat(name string) bool { return a.in.Key(name) || repeatFire(a.uiSt().heldN[name]) }

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
	case a.uiTextEditing(): // a widget-kit text field (ui_textfield.go)
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
		if a.uiModalOpen() {
			return []input.Context{input.CtxModal, input.CtxGlobal}
		}
		return []input.Context{input.CtxGlobal}
	}
	var s []input.Context
	switch {
	case a.modalOpen() || a.uiModalOpen():
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
func (a *App) queueing() bool { return a.in.Shift() }

// handleTextInput feeds the focused text field or key capture.
func (a *App) handleTextInput() {
	if a.in.Top() != input.CtxText {
		return
	}
	switch {
	case a.uiTextEditing():
		a.uiTextInput() // ui_textfield.go
	case a.rebind.on && a.nav.screen() == screenOptions:
		a.captureBinding() // controls_ui.go
	case a.search.open:
		a.searchTyping() // search_ui.go
	}
}

// pressAccept is Enter outside a text field: the wizard's Next/Start.
func (a *App) pressAccept() {
	if a.uiModalOpen() || a.uiTextEditing() {
		return // Enter belongs to the dialog or the field, not the wizard
	}
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
