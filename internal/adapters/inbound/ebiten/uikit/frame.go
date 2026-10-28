package uikit

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// Tooltip metrics (NG-1 §1.4).
const (
	TipDelay = 24  // frames the cursor rests before the card shows
	TipMaxW  = 360 // card width cap in px
)

// CountHeld runs before the frame is resolved against the bindings: it
// counts held keys (KeyRepeat reads HeldN) and clears last frame's
// PageUp/PageDown. The host then calls TakePages and TakeTab unless a
// text field or key capture has the keyboard.
func (k *Kit) CountHeld(raw *inputframe.Raw) {
	if k.HeldN == nil {
		k.HeldN = map[string]int{}
	}
	now := map[string]bool{}
	for _, key := range raw.Held {
		now[key] = true
	}
	for _, key := range raw.Pressed {
		now[key] = true
	}
	for key := range k.HeldN {
		if !now[key] {
			delete(k.HeldN, key)
		}
	}
	for key := range now {
		k.HeldN[key]++
	}
	k.pageDelta = 0
}

// TakePages takes PageUp and PageDown out of the frame while a scroll
// view is focused or under the cursor (mx, my): the view scrolls, the
// colony camera does not zoom.
func (k *Kit) TakePages(raw *inputframe.Raw, mx, my int) {
	if !k.pageOwner(mx, my) {
		return
	}
	for _, p := range []struct {
		key string
		dir int
	}{{"PageUp", -1}, {"PageDown", 1}} {
		if inputframe.RepeatFire(k.HeldN[p.key]) {
			k.pageDelta += p.dir
		}
		DropPressed(raw, p.key)
	}
}

// TakeTab takes Tab out of the frame and moves keyboard focus (back with
// Shift) when the frame has focusable widgets.
func (k *Kit) TakeTab(raw *inputframe.Raw, shift bool) {
	if !RawPressed(raw, "Tab") || len(k.focusables()) == 0 {
		return
	}
	dir := 1
	if shift {
		dir = -1
	}
	k.cycleFocus(dir)
	DropPressed(raw, "Tab")
}

// pageOwner reports a scroll view is focused or under the cursor, so it
// takes PageUp and PageDown.
func (k *Kit) pageOwner(mx, my int) bool {
	for _, w := range k.Cur {
		if w.Wheel && (k.Focus == w.ID || w.R.Hit(mx, my)) {
			return true
		}
	}
	return false
}

// RawPressed reports key went down in the raw frame.
func RawPressed(raw *inputframe.Raw, key string) bool {
	for _, p := range raw.Pressed {
		if p == key {
			return true
		}
	}
	return false
}

// DropPressed removes key from the frame's pressed keys (copying, since
// the source may reuse the slice).
func DropPressed(raw *inputframe.Raw, key string) {
	out := make([]string, 0, len(raw.Pressed))
	for _, p := range raw.Pressed {
		if p != key {
			out = append(out, p)
		}
	}
	raw.Pressed = out
}

// Begin starts the kit's frame: tooltip timing, a new hit list and paint
// list. The host then declares the modals (RunModals) and the screen's
// widgets. A host that changes screens calls Reset first.
func (k *Kit) Begin() {
	k.frame++
	mx, my := k.host.Cursor()
	pressing := k.in().Down(inputframe.Left) || k.in().Down(inputframe.Right)
	k.tipTick(mx, my, pressing)
	if k.Active != "" && !k.in().Down(inputframe.Left) && !k.in().Released(inputframe.Left) {
		k.Active = "" // a drag whose release was missed
	}
	if pressing {
		k.TabTo = ""
	}
	k.last, k.Cur = k.Cur, k.last[:0]
	k.Paints = k.Paints[:0]
	k.Hot, k.Layer, k.Suppress = "", 0, false
	k.mx, k.my = mx, my
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// tipTick advances the tooltip timer from the frame that just ended.
func (k *Kit) tipTick(mx, my int, pressing bool) {
	id := k.Hot
	dragging := false
	if k.Active != "" { // a captured widget keeps its tooltip while it is dragged
		if _, ok := k.Find(k.Active); ok {
			id, dragging = k.Active, true
		}
	}
	if id == "" || pressing && !dragging {
		k.tip.id, k.TipFrames, k.tip.text = "", 0, ""
		k.tip.x, k.tip.y = mx, my
		return
	}
	moved := absInt(mx-k.tip.x) > 3 || absInt(my-k.tip.y) > 3
	if id != k.tip.id || moved {
		k.tip.id, k.TipFrames = id, 1
		k.tip.x, k.tip.y = mx, my
	} else {
		k.TipFrames++
	}
	k.tip.text = ""
	if w, ok := k.Find(id); ok {
		k.tip.text = w.Tip
	}
	if dragging {
		k.TipFrames = max(k.TipFrames, TipDelay) // a dragged slider shows its value at once
	}
}

// Paint draws the queued widgets, the modal stack over their scrims
// and the tooltip. The host's Draw calls it after the screen's own drawing.
func (k *Kit) Paint(dst *ebiten.Image) {
	for layer := 0; layer <= len(k.Modals); layer++ {
		for _, p := range k.Paints {
			if p.layer == layer {
				p.fn(dst)
			}
		}
		k.paintFocusRing(dst, layer)
	}
	k.paintTooltip(dst)
}

// Click runs a clicked widget id's action (the host's click path calls it first);
// it reports the id was a widget's.
func (k *Kit) Click(id string) bool {
	if w, ok := k.Find(id); ok && w.Click != nil {
		w.Click()
		return true
	}
	return false
}

// WheelOver reports the cursor is over a scroll view, which owns the
// mouse wheel there (the colony camera does not zoom).
func (k *Kit) WheelOver(mx, my int) bool {
	for _, w := range k.Cur {
		if w.Wheel && w.R.Hit(mx, my) {
			return true
		}
	}
	return false
}

// ModalOpen reports a kit modal is on screen.
func (k *Kit) ModalOpen() bool { return len(k.Modals) > 0 }

// Editing reports a kit text field has the keyboard.
func (k *Kit) Editing() bool { return k.Edit.ID != "" }

// SetupTooSmall reports the window is below the setup minimum and, if so,
// turns this frame's widget declarations into no-ops. Screens call it
// first in their hook and draw drawTooSmall instead of their content.
func (k *Kit) SetupTooSmall(sw, sh int) bool {
	if !TooSmall(sw, sh) {
		return false
	}
	k.Suppress = true
	return true
}

// paintFocusRing outlines the keyboard-focused widget of a layer.
func (k *Kit) paintFocusRing(dst *ebiten.Image, layer int) {
	if !k.kbd || k.Focus == "" || k.Suppress {
		return
	}
	for _, w := range k.Cur {
		if w.ID == k.Focus && w.layer == layer && w.Focusable {
			StrokeRect(dst, w.R.Inset(-2), FieldFocus)
			return
		}
	}
}
