package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/platform/input"
)

// pollInput reads the devices once and resolves this frame's actions.
func (a *App) pollInput() {
	if a.src == nil {
		a.src = &inputframe.EbitenSource{}
	}
	raw := a.src.Poll()
	a.uiPreInput(&raw) // widget kit: held keys, Esc for a modal, Tab focus (ui_state.go)
	a.in = inputframe.NewFrame(raw)
	a.in.Ui.Mx, a.in.Ui.My = a.toLogical(raw.Mx, raw.My)
	a.in.Stack = a.contextStack()
	a.in.Res = input.Resolve(input.Frame{Pressed: raw.Pressed, Held: raw.Held, Mods: raw.Mods}, a.bindings(), a.in.Stack)
	a.uiBegin() // widget kit: tooltips, modals, the screen's widgets (ui_state.go)
}

// keyRepeat reports the key went down this frame or is repeating while
// held (first repeat after 24 frames, then every 3). Text fields and list
// navigation read it.
func (a *App) keyRepeat(name string) bool {
	return a.in.Key(name) || inputframe.RepeatFire(a.uiSt().HeldN[name])
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
	case a.rebind.On && a.nav.screen() == ports.ScreenOptions:
		return true
	case a.ord.Search.Open && a.colonyLive():
		return true
	case a.uiSt().Editing(): // a widget-kit text field (ui_textfield.go)
		return true
	}
	return false
}

// colonyLive is the running colony with no overlay on top.
func (a *App) colonyLive() bool {
	return a.nav.screen() == ports.ScreenColony && a.nav.overlay() == overlayNone && a.renderer != nil
}

// toolActive reports a tool owns the pointer: Move mode, an Architect
// tool, or a formation drag.
func (a *App) toolActive() bool {
	return a.arch.Mover.State().On || (a.arch.Open && a.arch.ToolID != "") || a.ord.Form.On
}

// modalOpen reports a menu sits over the map.
func (a *App) modalOpen() bool { return a.ord.Float.Open || a.tools.State.Menu }

// contextStack is the active input layers, top first (UI-016): text
// entry, modal, tool, panel, colony map, global, dev.
func (a *App) contextStack() []input.Context {
	if a.textActive() {
		return []input.Context{input.CtxText}
	}
	if !a.colonyLive() {
		if a.uiSt().ModalOpen() {
			return []input.Context{input.CtxModal, input.CtxGlobal}
		}
		return []input.Context{input.CtxGlobal}
	}
	var s []input.Context
	switch {
	case a.modalOpen() || a.uiSt().ModalOpen():
		s = append(s, input.CtxModal)
	default:
		if a.toolActive() {
			s = append(s, input.CtxTool)
		}
		if a.win.Open {
			s = append(s, input.CtxPanel)
		}
		s = append(s, input.CtxMap)
	}
	s = append(s, input.CtxGlobal)
	if a.tools.Mode() {
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

// handleTextInput feeds the focused text field or key capture.
func (a *App) handleTextInput() {
	if a.in.Top() != input.CtxText {
		return
	}
	switch {
	case a.uiSt().Editing():
		a.uiSt().TextInput() // ui_textfield.go
	case a.rebind.On && a.nav.screen() == ports.ScreenOptions:
		a.opt.CaptureBinding() // controls_ui.go
	case a.ord.Search.Open:
		a.ord.SearchTyping() // search_ui.go
	}
}

// pressAccept is Enter outside a text field: the wizard's Next/Start.
func (a *App) pressAccept() {
	if a.uiSt().ModalOpen() || a.uiSt().Editing() {
		return // Enter belongs to the dialog or the field, not the wizard
	}
	sw, sh := a.size()
	for _, b := range a.layoutButtons(sw, sh) {
		switch b.ID {
		case "wizard.next", "wizard.start", "world.generate":
			if !b.Disabled {
				a.applyClick(b.ID)
			}
			return
		}
	}
}
