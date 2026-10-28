package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// Options › Controls (UI-016, MAIN_UI §42 Controls): every action grouped
// by category on pages that fit the screen, two key slots each. Click a
// slot and press a key ("press a key..."): Esc cancels, Backspace clears.
// A key another action uses asks before swapping. Reset to defaults.
// Every change is saved through setSettings.

// rebindState is the Controls tab's capture and paging.
type rebindState struct {
	on      bool // waiting for a key
	act     input.Action
	slot    int
	page    int
	pending *pendingBind // a captured key that collides
}

// pendingBind is a captured key waiting for "swap?".
type pendingBind struct {
	act    input.Action
	slot   int
	key    input.Binding
	others []input.Action
}

// Layout (px).
const (
	ctlTop    = 164
	ctlRowH   = 26
	ctlBtnH   = 22
	ctlCatX   = 56
	ctlLabelX = 176
	ctlSlotX  = 470
	ctlSlotW  = 150
	ctlSlots  = 2
)

// ctlRowsPerPage is how many actions fit above the status line.
func ctlRowsPerPage(sh int) int {
	return max(1, (int(footerY(sh))-12-ctlTop)/ctlRowH-1)
}

func ctlPages(sh int) int {
	n := len(input.Defs())
	per := ctlRowsPerPage(sh)
	return (n + per - 1) / per
}

// ctlPageDefs is the actions on the current page.
func (a *App) ctlPageDefs(sh int) []input.Def {
	defs := input.Defs()
	per := ctlRowsPerPage(sh)
	a.rebind.page = max(0, min(a.rebind.page, ctlPages(sh)-1))
	lo := a.rebind.page * per
	return defs[lo:min(lo+per, len(defs))]
}

func ctlSlotID(act input.Action, slot int) string {
	return "options.ctl.bind." + string(act) + "." + strconv.Itoa(slot)
}

// controlsButtons are the tab's buttons below the tab row.
func (a *App) controlsButtons(sw, sh int) []button {
	var out []button
	b := a.bindings()
	for i, d := range a.ctlPageDefs(sh) {
		y := float32(ctlTop + i*ctlRowH)
		keys := b.Of(d.ID)
		for slot := 0; slot < ctlSlots; slot++ {
			label := "" // an empty slot
			if slot < len(keys) {
				label = keys[slot].Label()
			}
			bt := button{ID: ctlSlotID(d.ID, slot), Label: label, X: float32(ctlSlotX + slot*(ctlSlotW+8)), Y: y, W: ctlSlotW, H: ctlBtnH}
			switch {
			case a.rebind.on && a.rebind.act == d.ID && a.rebind.slot == slot:
				bt.Label, bt.Selected = "press a key...", true
			case d.Fixed && slot > 0:
				continue
			case d.Fixed:
				bt.Disabled, bt.Reason, bt.Quiet = true, "held with the mouse; not rebindable", true
			}
			out = append(out, bt)
		}
	}
	if p := a.rebind.pending; p != nil {
		y := float32(ctlTop + ctlRowsPerPage(sh)*ctlRowH + 2)
		out = append(out,
			button{ID: "options.ctl.swap", Label: "Swap", X: float32(sw) - 260, Y: y, W: 90, H: ctlBtnH},
			button{ID: "options.ctl.keep", Label: "Cancel", X: float32(sw) - 162, Y: y, W: 90, H: ctlBtnH})
	}
	fy := footerY(sh)
	out = append(out,
		button{ID: "options.ctl.prev", Label: "< Prev", X: 196, Y: fy, W: 90, H: 36, Disabled: a.rebind.page == 0, Quiet: true, Reason: "first page"},
		button{ID: "options.ctl.next", Label: "Next >", X: 294, Y: fy, W: 90, H: 36, Disabled: a.rebind.page >= ctlPages(sh)-1, Quiet: true, Reason: "last page"},
		button{ID: "options.ctl.reset", Label: "Reset to defaults", X: float32(sw) - 220, Y: fy, W: 180, H: 36})
	return out
}

// applyControlsClick handles an "options.ctl.*" click (key without the
// "options." prefix).
func (a *App) applyControlsClick(key string) {
	_, sh := a.size()
	switch key {
	case "ctl.prev":
		a.rebind.page = max(0, a.rebind.page-1)
	case "ctl.next":
		a.rebind.page = min(ctlPages(sh)-1, a.rebind.page+1)
	case "ctl.reset":
		a.rebind = rebindState{page: a.rebind.page}
		a.saveBindings(input.Defaults())
		a.toast("Controls reset to defaults")
	case "ctl.swap":
		if p := a.rebind.pending; p != nil {
			a.saveBindings(a.bindings().Swap(p.act, p.slot, p.key))
		}
		a.rebind.pending = nil
	case "ctl.keep":
		a.rebind.pending = nil
	default:
		rest, ok := strings.CutPrefix(key, "ctl.bind.")
		i := strings.LastIndex(rest, ".")
		if !ok || i < 0 {
			return
		}
		slot, err := strconv.Atoi(rest[i+1:])
		d, found := input.Lookup(input.Action(rest[:i]))
		if err != nil || !found || d.Fixed {
			return
		}
		a.rebind.on, a.rebind.act, a.rebind.slot, a.rebind.pending = true, d.ID, slot, nil
	}
}

// captureBinding takes the next key for the slot being rebound (the text
// context: nothing else sees the key).
func (a *App) captureBinding() {
	f := &a.in
	for _, k := range f.Raw.Pressed {
		switch k {
		case "Escape":
			a.rebind.on = false
			return
		case "Backspace", "Delete":
			a.rebind.on = false
			a.saveBindings(a.bindings().Set(a.rebind.act, a.rebind.slot, input.Binding{}))
			return
		}
		if input.IsModifier(k) || k == "Meta" {
			continue // wait for the key the modifiers go with
		}
		key := input.Binding{Key: k, Mods: f.Raw.Mods}
		a.rebind.on = false
		if others := a.bindings().ConflictsWith(a.rebind.act, key); len(others) > 0 {
			a.rebind.pending = &pendingBind{act: a.rebind.act, slot: a.rebind.slot, key: key, others: others}
			return
		}
		a.saveBindings(a.bindings().Set(a.rebind.act, a.rebind.slot, key))
		return
	}
}

// saveBindings stores the bindings as the settings' overrides.
func (a *App) saveBindings(b input.Bindings) {
	s := a.settings
	s.Controls.Bindings = input.ToSettings(b)
	a.setSettings(s)
}

func (a *App) drawControls(screen *ebiten.Image, sw, sh int) {
	defs := a.ctlPageDefs(sh)
	prev := ""
	if a.rebind.page > 0 {
		all := input.Defs()
		prev = all[a.rebind.page*ctlRowsPerPage(sh)-1].Category
	}
	drawText(screen, "Category", ctlCatX, ctlTop-20, tsLabel)
	drawText(screen, "Action", ctlLabelX, ctlTop-20, tsLabel)
	drawText(screen, "Keys (click to change)", ctlSlotX+4, ctlTop-20, tsLabel)
	for i, d := range defs {
		y := ctlTop + i*ctlRowH + 5
		if d.Category != prev {
			drawLabelClip(screen, ctlCatX, y, ctlLabelX-ctlCatX-8, d.Category)
			prev = d.Category
		}
		label := d.Label
		if d.Reserved != "" {
			label += " (" + d.Reserved + ")"
		}
		drawLabelClip(screen, ctlLabelX, y, ctlSlotX-ctlLabelX-8, label)
	}
	status := fmt.Sprintf("Page %d of %d", a.rebind.page+1, ctlPages(sh))
	switch {
	case a.rebind.pending != nil:
		p := a.rebind.pending
		var names []string
		for _, o := range p.others {
			if d, ok := input.Lookup(o); ok {
				names = append(names, d.Label)
			}
		}
		status = fmt.Sprintf("%s is used by %s. Swap?", p.key.Label(), strings.Join(names, ", "))
	case a.rebind.on:
		status = "Press a key (with Shift/Ctrl/Alt if you like). Esc cancels, Backspace clears."
	case len(input.Conflicts(a.bindings())) > 0:
		c := input.Conflicts(a.bindings())[0]
		status = fmt.Sprintf("Conflict: %s is bound twice (%s, %s)", c.Key.Label(), c.A, c.B)
	}
	drawLabelClip(screen, ctlCatX, ctlTop+ctlRowsPerPage(sh)*ctlRowH+7, sw-2*ctlCatX, status)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.controlsButtons(sw, sh), mx, my)
}
