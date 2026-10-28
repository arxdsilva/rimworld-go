// Package uikit is the immediate-mode widget kit (NG-1 §1.4): per-Update
// widget declarations with hit testing, focus, keyboard navigation,
// scrolling, text editing, modals, tooltips and the paint list, drawn
// through gfx. It reaches its host through one small interface (Host) and
// knows nothing of screens or the game.
//
// A screen declares its widgets once per Update (right after the input is
// polled, before the click path): each widget call reads the input,
// mutates the value it is bound to, registers a hit rectangle and queues
// its drawing. The host's Draw then calls Paint to draw the queue, the
// modal stack and the tooltip.
//
// Widget ids are strings with a per-screen prefix (scen., sced., story.,
// diff., world., land., pawns., fac., modal.).
package uikit

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// Kit is everything the kit remembers between frames. Build it with New;
// a zero Kit works once its host is bound (Bind).
type Kit struct {
	host Host

	// Hot is the topmost widget under the cursor this frame; Active the
	// widget that captured the mouse (slider knob, scroll thumb, text
	// selection drag); Focus the keyboard-focused widget.
	Hot, Active, Focus string
	// Scroll holds the Scroll offsets by widget id.
	Scroll map[string]float32
	// Caret and the selection [SelStart, SelEnd) of the text field being
	// edited, in runes.
	Caret, SelStart, SelEnd int
	// tip is the pending tooltip; TipFrames the frames the cursor has
	// rested on its widget.
	tip       tipState
	TipFrames int
	// dragY is the grab offset of the active scrollbar drag.
	dragY float32

	frame    uint64
	mx, my   int
	Layer    int  // the layer widgets are being declared into: 0 screen, i+1 modal i
	Suppress bool // the screen is too small: no widgets this frame
	kbd      bool // focus came from the keyboard: draw the focus ring
	TabTo    string
	// LastScreen is the host's screen key at its last screen-change check
	// (the host compares it and calls Reset; the kit never reads it).
	LastScreen any
	Hook       func(sw, sh int) // test and capture override of the screen hook

	Cur, last []Widget
	Paints    []paintCmd
	events    map[string]uiEventRec
	HeldN     map[string]int // frames each key has been held (Host.KeyRepeat)
	Edit      textEdit
	Modals    []Modal
	sorts     map[string]sortState
	lines     map[string]lineCache
	pageDelta int // PageUp/PageDown taken for a scroll view this frame (-1, 0, +1)
}

// tipState is the pending tooltip: its widget, its text and where the
// cursor came to rest.
type tipState struct {
	id, text string
	x, y     int
}

// Widget is one registered hit rectangle.
type Widget struct {
	ID        string
	Label     string
	R         gfx.Rect
	layer     int
	Focusable bool
	Disabled  bool
	Reason    string // why it is disabled
	Tip       string // "Title\nbody": the hover card
	Wheel     bool   // a scroll view: it owns the mouse wheel over r
	Click     func() // what a click on the id does (the host's click path)
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

// Add registers a widget in the layer being declared. It reports whether
// the widget is live (accepts input): widgets under a modal or on a
// screen that is too small are not.
func (k *Kit) Add(w Widget) bool {
	if k.Suppress || k.Layer != len(k.Modals) {
		return false
	}
	w.layer = k.Layer
	k.Cur = append(k.Cur, w)
	if w.R.Hit(k.mx, k.my) {
		k.Hot = w.ID
	}
	return true
}

// Queue adds a drawing to the layer being declared.
func (k *Kit) Queue(fn func(dst *ebiten.Image)) {
	if k.Suppress {
		return
	}
	k.Paints = append(k.Paints, paintCmd{layer: k.Layer, fn: fn})
}

// Find returns the registered widget with id (this frame's).
func (k *Kit) Find(id string) (Widget, bool) {
	for i := len(k.Cur) - 1; i >= 0; i-- {
		if k.Cur[i].ID == id {
			return k.Cur[i], true
		}
	}
	return Widget{}, false
}

// fire records an event for the widget id; the widget's next call reads it.
func (k *Kit) fire(id string, flag uint8) {
	if k.events == nil {
		k.events = map[string]uiEventRec{}
	}
	e := k.events[id]
	if e.frame+1 < k.frame {
		e.flags = 0
	}
	e.flags |= flag
	e.frame = k.frame
	k.events[id] = e
}

// take reads and clears the widget's events from this or the last frame.
func (k *Kit) take(id string) uint8 {
	e, ok := k.events[id]
	if !ok {
		return 0
	}
	delete(k.events, id)
	if e.frame+1 < k.frame {
		return 0
	}
	return e.flags
}

// Reset forgets focus, drags, modals, any edit and the hit list (the
// host changed screens).
func (k *Kit) Reset() {
	k.Hot, k.Active, k.Focus = "", "", ""
	k.tip.id, k.TipFrames, k.tip.text = "", 0, ""
	k.kbd, k.TabTo = false, ""
	k.Edit = textEdit{}
	k.Modals = nil
	k.events = nil
	k.Cur = nil
}

// focusables are the widgets Tab visits, in declaration order.
func (k *Kit) focusables() []string {
	var out []string
	for _, w := range k.Cur {
		if w.Focusable && !w.Disabled {
			out = append(out, w.ID)
		}
	}
	return out
}

// cycleFocus moves keyboard focus to the next (dir>0) or previous focusable
// widget, wrapping. A text field it lands on starts editing.
func (k *Kit) cycleFocus(dir int) {
	ids := k.focusables()
	if len(ids) == 0 {
		return
	}
	i := -1
	for j, id := range ids {
		if id == k.Focus {
			i = j
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
	k.Focus = ids[i]
	k.TabTo = ids[i]
	k.kbd = true
}

// ---- shared widget plumbing -------------------------------------------

// Press reports a fresh left press inside r and takes it, so nothing
// else acts on it. Only live widgets call it.
func (k *Kit) Press(r gfx.Rect) bool {
	if !k.in().Clicked(inputframe.Left) {
		return false
	}
	mx, my := k.host.Cursor()
	if !r.Hit(mx, my) {
		return false
	}
	k.in().Take(inputframe.Left)
	k.kbd = false
	return true
}

// IsHot reports the cursor's topmost widget is id (hover drawing).
func (k *Kit) IsHot(id string) bool { return k.Hot == id }

// IsFocused reports id has keyboard focus.
func (k *Kit) IsFocused(id string) bool { return k.Focus == id }

// release consumes this frame's left release so nothing else sees it.
func (k *Kit) release() { k.in().Raw.Release[inputframe.Left] = false }

// KeyAct reports Space went down on the focused widget.
func (k *Kit) KeyAct(id string) bool {
	return k.Focus == id && k.in().Key("Space")
}

// Deny is a click on a disabled widget: the reason is shown and a deny
// sound plays.
func (k *Kit) Deny(reason string) {
	k.host.Hint(reason)
	k.host.Cue("ui.deny")
}

// OK is the click sound and a cleared hint.
func (k *Kit) OK() {
	k.host.Hint("")
	k.host.Cue("ui.click")
}

// ---- button ------------------------------------------------------------

type BtnStyle uint8

const (
	BtnNormal  BtnStyle = iota // the plain button
	BtnPrimary                 // accent: the screen's forward action
	BtnDanger                  // danger
)

// BtnOpts are a button's optional parts.
type BtnOpts struct {
	Style    BtnStyle
	Disabled bool
	Reason   string // shown as the tooltip while disabled
	Tip      string // hover card: "Title\nbody"
	Selected bool
}

func (s BtnStyle) Colors(hover, down, selected bool) (fill, border, text color.RGBA) {
	switch s {
	case BtnPrimary:
		fill, border, text = LerpRGBA(BtnSelected, Accent, 0.6), Accent, Field
	case BtnDanger:
		fill, border, text = LerpRGBA(Btn, Danger, 0.35), Danger, Text
	default:
		fill, border, text = Btn, BtnBorder, Text
		if selected {
			fill, border = BtnSelected, Accent
		}
	}
	switch {
	case down:
		fill = LerpRGBA(fill, color.RGBA{A: 0xFF}, 0.2)
	case hover:
		fill = LerpRGBA(fill, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, 0.12)
	}
	return
}

// Button is a labelled button; it reports a click. Disabled buttons
// take the click, say why (tooltip, footer hint) and return false.
func (k *Kit) Button(id string, r gfx.Rect, label string, o BtnOpts) bool {
	tip := o.Tip
	if o.Disabled && o.Reason != "" {
		tip = o.Reason
	}
	w := Widget{ID: id, Label: label, R: r, Focusable: true, Disabled: o.Disabled, Reason: o.Reason, Tip: tip}
	clicked := false
	w.Click = func() {
		if o.Disabled {
			k.Deny(o.Reason)
			return
		}
		k.OK()
		k.fire(id, evClick)
	}
	if k.Add(w) {
		switch {
		case k.Press(r):
			w.Click()
			clicked = !o.Disabled
		case !o.Disabled && k.KeyAct(id):
			w.Click()
			clicked = true
		}
		if k.take(id)&evClick != 0 {
			clicked = true // the click path got there first
		}
	}
	k.Queue(func(dst *ebiten.Image) {
		hover := k.Hot == id && !o.Disabled
		down := hover && k.in().Down(inputframe.Left) && k.Active == ""
		fill, border, text := o.Style.Colors(hover, down, o.Selected)
		if o.Disabled {
			fill, border, text = BtnDisabled, BtnBorder, TextDisabled
		}
		gfx.FillRect(dst, r.X, r.Y, r.W, r.H, fill, false)
		gfx.StrokeRect(dst, r.X, r.Y, r.W, r.H, 1, border, false)
		st := gfx.Label
		st.Color, st.Align = text, gfx.AlignCenter
		cx, cy := r.Center()
		gfx.Text(dst, FitText(label, gfx.Label, r.W-12), float64(cx), float64(cy)-gfx.LineH(st)/2, st)
	})
	return clicked
}

// Label draws s at (x, y) in style st (queued, so it layers with the
// widgets).
func (k *Kit) Label(x, y float32, s string, st gfx.TextStyle) {
	k.Queue(func(dst *ebiten.Image) { gfx.Text(dst, s, float64(x), float64(y), st) })
}

// Panel queues a panel background.
func (k *Kit) Panel(r gfx.Rect) {
	k.Queue(func(dst *ebiten.Image) { DrawPanel(dst, r.X, r.Y, r.W, r.H) })
}

// StrokeRect outlines r with a 1 px line.
func StrokeRect(dst *ebiten.Image, r gfx.Rect, c color.Color) {
	gfx.StrokeRect(dst, r.X, r.Y, r.W, r.H, 1, c, false)
}

// FillRect fills r.
func FillRect(dst *ebiten.Image, r gfx.Rect, c color.Color) {
	gfx.FillRect(dst, r.X, r.Y, r.W, r.H, c, false)
}

func Num(n int) string { return strconv.Itoa(n) }

// FitText shortens s with an ellipsis to fit w px in style st.
func FitText(s string, st gfx.TextStyle, w float32) string { return gfx.Ellipsize(s, st, float64(w)) }
