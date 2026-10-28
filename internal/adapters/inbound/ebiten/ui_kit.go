package ebiten

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Immediate-mode widget kit (NG-1 §1.4). A screen declares its widgets
// once per Update from a hook registered with registerScreenUI (they run
// right after the input is polled, before the click path): each widget
// call reads the input, mutates the value it is bound to, registers a hit
// rectangle and queues its drawing. App.Draw then calls uiPaint to draw
// the queue, the modal stack and the tooltip.
//
// Widget ids are strings with a per-screen prefix (scen., sced., story.,
// diff., world., land., pawns., fac., modal.).

// uiState is everything the kit remembers between frames. The zero value
// is usable; App reaches it through uiSt (ui_state.go).
type uiState struct {
	// hot is the topmost widget under the cursor this frame; active the
	// widget that captured the mouse (slider knob, scroll thumb, text
	// selection drag); focus the keyboard-focused widget.
	hot, active, focus string
	// scroll holds the scroll offsets by widget id.
	scroll map[string]float32
	// caret and the selection [selStart, selEnd) of the text field being
	// edited, in runes.
	caret, selStart, selEnd int
	// tipID is the widget whose tooltip is pending; tipFrames the frames
	// the cursor has rested on it.
	tipID     string
	tipFrames int
	// dragY is the grab offset of the active scrollbar drag.
	dragY float32

	frame      uint64
	mx, my     int
	layer      int  // the layer widgets are being declared into: 0 screen, i+1 modal i
	suppress   bool // the screen is too small: no widgets this frame
	kbd        bool // focus came from the keyboard: draw the focus ring
	tabTo      string
	lastScreen screenID
	hook       func(a *App, sw, sh int) // test and capture override of the screen hook

	cur, last []uiWidget
	paints    []paintCmd
	events    map[string]uiEventRec
	heldN     map[string]int // frames each key has been held (keyRepeat)
	edit      textEdit
	modals    []modal
	sorts     map[string]sortState
	lines     map[string]lineCache
	tipText   string
	pageDelta int // PageUp/PageDown taken for a scroll view this frame (-1, 0, +1)
	tipX      int
	tipY      int
}

// uiWidget is one registered hit rectangle.
type uiWidget struct {
	id        string
	label     string
	r         rect
	layer     int
	focusable bool
	disabled  bool
	reason    string // why it is disabled
	tip       string // "Title\nbody": the hover card
	wheel     bool   // a scroll view: it owns the mouse wheel over r
	click     func() // what a click on the id does (applyClick path)
}

// paintCmd is a queued drawing.
type paintCmd struct {
	layer int
	fn    func(dst *ebiten.Image)
}

// Event flags: what happened to a widget since its last call.
const (
	evChanged uint8 = 1 << iota
	evCommit
	evCancel
	evClick
)

// uiEventRec is the flags recorded for a widget and the frame they were set.
type uiEventRec struct {
	flags uint8
	frame uint64
}

// add registers a widget in the layer being declared. It reports whether
// the widget is live (accepts input): widgets under a modal or on a
// screen that is too small are not.
func (u *uiState) add(w uiWidget) bool {
	if u.suppress || u.layer != len(u.modals) {
		return false
	}
	w.layer = u.layer
	u.cur = append(u.cur, w)
	if w.r.Hit(u.mx, u.my) {
		u.hot = w.id
	}
	return true
}

// queue adds a drawing to the layer being declared.
func (u *uiState) queue(fn func(dst *ebiten.Image)) {
	if u.suppress {
		return
	}
	u.paints = append(u.paints, paintCmd{layer: u.layer, fn: fn})
}

// find returns the registered widget with id (this frame's).
func (u *uiState) find(id string) (uiWidget, bool) {
	for i := len(u.cur) - 1; i >= 0; i-- {
		if u.cur[i].id == id {
			return u.cur[i], true
		}
	}
	return uiWidget{}, false
}

// fire records an event for the widget id; the widget's next call reads it.
func (u *uiState) fire(id string, flag uint8) {
	if u.events == nil {
		u.events = map[string]uiEventRec{}
	}
	e := u.events[id]
	if e.frame+1 < u.frame {
		e.flags = 0
	}
	e.flags |= flag
	e.frame = u.frame
	u.events[id] = e
}

// take reads and clears the widget's events from this or the last frame.
func (u *uiState) take(id string) uint8 {
	e, ok := u.events[id]
	if !ok {
		return 0
	}
	delete(u.events, id)
	if e.frame+1 < u.frame {
		return 0
	}
	return e.flags
}

// reset forgets focus, drags and any edit (a screen change).
func (u *uiState) reset() {
	u.hot, u.active, u.focus = "", "", ""
	u.tipID, u.tipFrames, u.tipText = "", 0, ""
	u.kbd, u.tabTo = false, ""
	u.edit = textEdit{}
	u.modals = nil
	u.events = nil
}

// focusables are the widgets Tab visits, in declaration order.
func (u *uiState) focusables() []string {
	var out []string
	for _, w := range u.cur {
		if w.focusable && !w.disabled {
			out = append(out, w.id)
		}
	}
	return out
}

// cycleFocus moves keyboard focus to the next (dir>0) or previous focusable
// widget, wrapping. A text field it lands on starts editing.
func (u *uiState) cycleFocus(dir int) {
	ids := u.focusables()
	if len(ids) == 0 {
		return
	}
	i := -1
	for k, id := range ids {
		if id == u.focus {
			i = k
		}
	}
	switch {
	case i < 0 && dir > 0:
		i = 0
	case i < 0:
		i = len(ids) - 1
	default:
		i = (i + dir + len(ids)) % len(ids)
	}
	u.focus = ids[i]
	u.tabTo = ids[i]
	u.kbd = true
}

// ---- shared widget plumbing -------------------------------------------

// uiPress reports a fresh left press inside r and takes it, so nothing
// else acts on it. Only live widgets call it.
func (a *App) uiPress(r rect) bool {
	if !a.in.clicked(mbLeft) {
		return false
	}
	mx, my := a.currentCursor()
	if !r.Hit(mx, my) {
		return false
	}
	a.in.take(mbLeft)
	a.uiSt().kbd = false
	return true
}

// uiHot reports the cursor's topmost widget is id (hover drawing).
func (a *App) uiHot(id string) bool { return a.uiSt().hot == id }

// uiFocused reports id has keyboard focus.
func (a *App) uiFocused(id string) bool { return a.uiSt().focus == id }

// uiRelease consumes this frame's left release so nothing else sees it.
func (a *App) uiRelease() { a.in.raw.release[mbLeft] = false }

// uiKeyAct reports Space went down on the focused widget.
func (a *App) uiKeyAct(id string) bool {
	return a.uiSt().focus == id && a.in.key("Space")
}

// uiDeny is a click on a disabled widget: the reason is shown and a deny
// sound plays.
func (a *App) uiDeny(reason string) {
	a.hint = reason
	a.cfg.Sound.Play("ui.deny", 1, 0)
}

// uiOK is the click sound and a cleared hint.
func (a *App) uiOK() {
	a.hint = ""
	a.cfg.Sound.Play("ui.click", 1, 0)
}

// ---- button ------------------------------------------------------------

type btnStyle uint8

const (
	btnNormal  btnStyle = iota // the plain button
	btnPrimary                 // accent: the screen's forward action
	btnDanger                  // danger
)

// btnOpts are a button's optional parts.
type btnOpts struct {
	Style    btnStyle
	Disabled bool
	Reason   string // shown as the tooltip while disabled
	Tip      string // hover card: "Title\nbody"
	Selected bool
}

func (s btnStyle) colors(hover, down, selected bool) (fill, border, text color.RGBA) {
	switch s {
	case btnPrimary:
		fill, border, text = lerpRGBA(uiBtnSelected, uiAccent, 0.6), uiAccent, uiField
	case btnDanger:
		fill, border, text = lerpRGBA(uiBtn, uiDanger, 0.35), uiDanger, uiText
	default:
		fill, border, text = uiBtn, uiBtnBorder, uiText
		if selected {
			fill, border = uiBtnSelected, uiAccent
		}
	}
	switch {
	case down:
		fill = lerpRGBA(fill, color.RGBA{A: 0xFF}, 0.2)
	case hover:
		fill = lerpRGBA(fill, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, 0.12)
	}
	return
}

// uiButton is a labelled button; it reports a click. Disabled buttons
// take the click, say why (tooltip, footer hint) and return false.
func (a *App) uiButton(id string, r rect, label string, o btnOpts) bool {
	u := a.uiSt()
	tip := o.Tip
	if o.Disabled && o.Reason != "" {
		tip = o.Reason
	}
	w := uiWidget{id: id, label: label, r: r, focusable: true, disabled: o.Disabled, reason: o.Reason, tip: tip}
	clicked := false
	w.click = func() {
		if o.Disabled {
			a.uiDeny(o.Reason)
			return
		}
		a.uiOK()
		u.fire(id, evClick)
	}
	if u.add(w) {
		switch {
		case a.uiPress(r):
			w.click()
			clicked = !o.Disabled
		case !o.Disabled && a.uiKeyAct(id):
			w.click()
			clicked = true
		}
		if u.take(id)&evClick != 0 {
			clicked = true // the click path got there first
		}
	}
	u.queue(func(dst *ebiten.Image) {
		hover := u.hot == id && !o.Disabled
		down := hover && a.in.down(mbLeft) && u.active == ""
		fill, border, text := o.Style.colors(hover, down, o.Selected)
		if o.Disabled {
			fill, border, text = uiBtnDisabled, uiBtnBorder, uiTextDisabled
		}
		uiFillRect(dst, r.X, r.Y, r.W, r.H, fill, false)
		uiStrokeRect(dst, r.X, r.Y, r.W, r.H, 1, border, false)
		st := tsLabel
		st.Color, st.Align = text, alignCenter
		cx, cy := r.Center()
		drawText(dst, fitText(label, tsLabel, r.W-12), float64(cx), float64(cy)-lineH(st)/2, st)
	})
	return clicked
}

// uiLabel draws s at (x, y) in style st (queued, so it layers with the
// widgets).
func (a *App) uiLabel(x, y float32, s string, st textStyle) {
	a.uiSt().queue(func(dst *ebiten.Image) { drawText(dst, s, float64(x), float64(y), st) })
}

// uiPanel queues a panel background.
func (a *App) uiPanel(r rect) {
	a.uiSt().queue(func(dst *ebiten.Image) { drawPanel(dst, r.X, r.Y, r.W, r.H) })
}

// strokeRect outlines r with a 1 px line.
func strokeRect(dst *ebiten.Image, r rect, c color.Color) {
	uiStrokeRect(dst, r.X, r.Y, r.W, r.H, 1, c, false)
}

// fillRect fills r.
func fillRect(dst *ebiten.Image, r rect, c color.Color) {
	uiFillRect(dst, r.X, r.Y, r.W, r.H, c, false)
}

func uiNum(n int) string { return strconv.Itoa(n) }

// fitText shortens s with an ellipsis to fit w px in style st.
func fitText(s string, st textStyle, w float32) string { return ellipsize(s, st, float64(w)) }
