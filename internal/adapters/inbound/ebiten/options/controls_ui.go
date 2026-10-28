package options

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/input"
)

// Options › Controls (UI-016, MAIN_UI §42 Controls): every action grouped
// by category on pages that fit the screen, two key slots each. Click a
// slot and press a key ("press a key..."): Esc cancels, Backspace clears.
// A key another action uses asks before swapping. Reset to defaults.
// Every change is saved through SetSettings.

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

// CtlPages is how many pages the actions take at screen height sh.
func CtlPages(sh int) int {
	n := len(input.Defs())
	per := ctlRowsPerPage(sh)
	return (n + per - 1) / per
}

// ctlPageDefs is the actions on the current page.
func (s *Screen) ctlPageDefs(sh int) []input.Def {
	defs := input.Defs()
	per := ctlRowsPerPage(sh)
	rb := s.sh.Rebind()
	rb.Page = max(0, min(rb.Page, CtlPages(sh)-1))
	lo := rb.Page * per
	return defs[lo:min(lo+per, len(defs))]
}

// CtlSlotID is the button id of an action's key slot.
func CtlSlotID(act input.Action, slot int) string {
	return "options.ctl.bind." + string(act) + "." + strconv.Itoa(slot)
}

// controlsButtons are the tab's buttons below the tab row.
func (s *Screen) controlsButtons(sw, sh int) []uikit.Button {
	var out []uikit.Button
	b := s.sh.Env().Bindings
	rb := s.sh.Rebind()
	for i, d := range s.ctlPageDefs(sh) {
		y := float32(ctlTop + i*ctlRowH)
		keys := b.Of(d.ID)
		for slot := 0; slot < ctlSlots; slot++ {
			label := "" // an empty slot
			if slot < len(keys) {
				label = keys[slot].Label()
			}
			bt := uikit.Button{ID: CtlSlotID(d.ID, slot), Label: label, X: float32(ctlSlotX + slot*(ctlSlotW+8)), Y: y, W: ctlSlotW, H: ctlBtnH}
			switch {
			case rb.On && rb.act == d.ID && rb.slot == slot:
				bt.Label, bt.Selected = "press a key...", true
			case d.Fixed && slot > 0:
				continue
			case d.Fixed:
				bt.Disabled, bt.Reason, bt.Quiet = true, "held with the mouse; not rebindable", true
			}
			out = append(out, bt)
		}
	}
	if p := rb.Pending; p != nil {
		y := float32(ctlTop + ctlRowsPerPage(sh)*ctlRowH + 2)
		out = append(out,
			uikit.Button{ID: "options.ctl.swap", Label: "Swap", X: float32(sw) - 260, Y: y, W: 90, H: ctlBtnH},
			uikit.Button{ID: "options.ctl.keep", Label: "Cancel", X: float32(sw) - 162, Y: y, W: 90, H: ctlBtnH})
	}
	fy := footerY(sh)
	out = append(out,
		uikit.Button{ID: "options.ctl.prev", Label: "< Prev", X: 196, Y: fy, W: 90, H: 36, Disabled: rb.Page == 0, Quiet: true, Reason: "first page"},
		uikit.Button{ID: "options.ctl.next", Label: "Next >", X: 294, Y: fy, W: 90, H: 36, Disabled: rb.Page >= CtlPages(sh)-1, Quiet: true, Reason: "last page"},
		uikit.Button{ID: "options.ctl.reset", Label: "Reset to defaults", X: float32(sw) - 220, Y: fy, W: 180, H: 36})
	return out
}

// applyControlsClick handles an "options.ctl.*" click (key without the
// "options." prefix).
func (s *Screen) applyControlsClick(key string) {
	_, sh := s.sh.Size()
	rb := s.sh.Rebind()
	switch key {
	case "ctl.prev":
		rb.Page = max(0, rb.Page-1)
	case "ctl.next":
		rb.Page = min(CtlPages(sh)-1, rb.Page+1)
	case "ctl.reset":
		*rb = RebindState{Page: rb.Page}
		s.saveBindings(input.Defaults())
		s.sh.Toast("Controls reset to defaults")
	case "ctl.swap":
		if p := rb.Pending; p != nil {
			s.saveBindings(s.sh.Env().Bindings.Swap(p.act, p.slot, p.key))
		}
		rb.Pending = nil
	case "ctl.keep":
		rb.Pending = nil
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
		rb.On, rb.act, rb.slot, rb.Pending = true, d.ID, slot, nil
	}
}

// CaptureBinding takes the next key for the slot being rebound (the text
// context: nothing else sees the key).
func (s *Screen) CaptureBinding() {
	env := s.sh.Env()
	rb := s.sh.Rebind()
	for _, k := range env.Pressed {
		switch k {
		case "Escape":
			rb.On = false
			return
		case "Backspace", "Delete":
			rb.On = false
			s.saveBindings(env.Bindings.Set(rb.act, rb.slot, input.Binding{}))
			return
		}
		if input.IsModifier(k) || k == "Meta" {
			continue // wait for the key the modifiers go with
		}
		key := input.Binding{Key: k, Mods: env.Mods}
		rb.On = false
		if others := env.Bindings.ConflictsWith(rb.act, key); len(others) > 0 {
			rb.Pending = &PendingBind{act: rb.act, slot: rb.slot, key: key, others: others}
			return
		}
		s.saveBindings(env.Bindings.Set(rb.act, rb.slot, key))
		return
	}
}

// saveBindings stores the bindings as the settings' overrides.
func (s *Screen) saveBindings(b input.Bindings) {
	st := s.sh.Settings()
	st.Controls.Bindings = input.ToSettings(b)
	s.sh.SetSettings(st)
}

func (s *Screen) drawControls(screen *ebiten.Image, sw, sh int) {
	defs := s.ctlPageDefs(sh)
	rb := s.sh.Rebind()
	b := s.sh.Env().Bindings
	prev := ""
	if rb.Page > 0 {
		all := input.Defs()
		prev = all[rb.Page*ctlRowsPerPage(sh)-1].Category
	}
	gfx.Text(screen, "Category", ctlCatX, ctlTop-20, gfx.Label)
	gfx.Text(screen, "Action", ctlLabelX, ctlTop-20, gfx.Label)
	gfx.Text(screen, "Keys (click to change)", ctlSlotX+4, ctlTop-20, gfx.Label)
	for i, d := range defs {
		y := ctlTop + i*ctlRowH + 5
		if d.Category != prev {
			uikit.DrawLabelClip(screen, ctlCatX, y, ctlLabelX-ctlCatX-8, d.Category)
			prev = d.Category
		}
		label := d.Label
		if d.Reserved != "" {
			label += " (" + d.Reserved + ")"
		}
		uikit.DrawLabelClip(screen, ctlLabelX, y, ctlSlotX-ctlLabelX-8, label)
	}
	status := fmt.Sprintf("Page %d of %d", rb.Page+1, CtlPages(sh))
	switch {
	case rb.Pending != nil:
		p := rb.Pending
		var names []string
		for _, o := range p.others {
			if d, ok := input.Lookup(o); ok {
				names = append(names, d.Label)
			}
		}
		status = fmt.Sprintf("%s is used by %s. Swap?", p.key.Label(), strings.Join(names, ", "))
	case rb.On:
		status = "Press a key (with Shift/Ctrl/Alt if you like). Esc cancels, Backspace clears."
	case len(input.Conflicts(b)) > 0:
		c := input.Conflicts(b)[0]
		status = fmt.Sprintf("Conflict: %s is bound twice (%s, %s)", c.Key.Label(), c.A, c.B)
	}
	uikit.DrawLabelClip(screen, ctlCatX, ctlTop+ctlRowsPerPage(sh)*ctlRowH+7, sw-2*ctlCatX, status)
	mx, my := s.sh.Cursor()
	s.drawButtons(screen, s.controlsButtons(sw, sh), mx, my)
}

// drawButtons paints buttons with the one under (mx, my) hovered.
func (s *Screen) drawButtons(screen *ebiten.Image, buttons []uikit.Button, mx, my int) {
	for _, b := range buttons {
		uikit.DrawButton(screen, b, b.Contains(mx, my))
	}
}
