// Package inputframe is the client's input abstraction: the raw device state polled once per Update, the resolved frame of fired actions and mouse buttons, and the fake source tests drive. It knows nothing of screens or the game.
package inputframe

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"rimworld-go/internal/platform/input"
)

// Frame is this Update's input: the raw devices plus the actions the
// bindings resolved for the context stack. Handlers read it instead of
// Ebitengine, and take() a mouse button they used so no later handler
// acts on the same click.
type Frame struct {
	// Raw is the devices as polled: Raw.mx/my are physical pixels (the
	// map reads them through App.mapCursor). ui is the same pointer in
	// logical UI pixels (Raw / s), read through App.currentCursor.
	Raw   Raw
	Ui    UIPoint
	keys  map[string]bool // keys pressed this frame
	Res   input.Result
	Stack []input.Context
	taken [NumMouse]bool
}

// UIPoint is a pointer position in logical UI pixels.
type UIPoint struct{ Mx, My int }

func NewFrame(raw Raw) Frame {
	f := Frame{Raw: raw, keys: make(map[string]bool, len(raw.Pressed))}
	for _, k := range raw.Pressed {
		f.keys[k] = true
	}
	return f
}

// Fired reports a Press/Toggle action Fired this frame.
func (f *Frame) Fired(a input.Action) bool { return f.Res.Fired(a) }

// Held reports a Hold action is active.
func (f *Frame) Held(a input.Action) bool { return f.Res.Held(a) }

// Key reports a raw Key went down (text fields and Key capture read it).
func (f *Frame) Key(name string) bool { return f.keys[name] }

// Clicked reports button b went down this frame and nobody took it yet.
func (f *Frame) Clicked(b MouseBtn) bool { return f.Raw.Press[b] && !f.taken[b] }

// Take marks b's press as used.
func (f *Frame) Take(b MouseBtn) { f.taken[b] = true }

// Down reports b is held.
func (f *Frame) Down(b MouseBtn) bool { return f.Raw.Down[b] }

// Released reports b came up this frame.
func (f *Frame) Released(b MouseBtn) bool { return f.Raw.Release[b] }

func (f *Frame) Shift() bool { return f.Raw.Mods&input.ModShift != 0 }

func (f *Frame) Ctrl() bool { return f.Raw.Mods&input.ModCtrl != 0 }

func (f *Frame) Alt() bool { return f.Raw.Mods&input.ModAlt != 0 }

// Top is the topmost active context.
func (f *Frame) Top() input.Context {
	if len(f.Stack) == 0 {
		return input.CtxGlobal
	}
	return f.Stack[0]
}

// RepeatFire reports a key held for n consecutive frames fires: on the
// first, then after 24 frames and every 3 frames after that.
func RepeatFire(n int) bool { return n == 1 || (n >= 25 && (n-25)%3 == 0) }

// MouseBtn is a mouse button.
type MouseBtn int

const (
	Left MouseBtn = iota
	Right
	Middle
	NumMouse
)

// Raw is one frame of device state.
type Raw struct {
	Pressed []string // keys that went down this frame (input key names)
	Held    []string // keys down now
	Mods    input.Mod
	Mx, My  int
	WheelX  float64
	WheelY  float64
	Down    [NumMouse]bool // held
	Press   [NumMouse]bool // went down this frame
	Release [NumMouse]bool // came up this frame
	Chars   []rune         // typed characters
	Focused bool
	// Blurred: the window had focus and lost it (backgrounded or
	// minimised). The zero value is "in the foreground", so a source that
	// never reports focus (a fake, a headless run) never counts as
	// backgrounded.
	Blurred bool
}

// Source is what the App needs from the input devices.
type Source interface {
	Poll() Raw
}

// EbitenSource reads the real keyboard and mouse. seenFocus remembers the
// window has been focused once (see BlurredFor).
type EbitenSource struct {
	buf       []ebiten.Key
	seenFocus bool
}

var ebitenButtons = [NumMouse]ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle}

func (e *EbitenSource) Poll() Raw {
	var in Raw
	e.buf = inpututil.AppendJustPressedKeys(e.buf[:0])
	in.Pressed = keyNames(e.buf)
	e.buf = inpututil.AppendPressedKeys(e.buf[:0])
	in.Held = keyNames(e.buf)
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		in.Mods |= input.ModShift
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		in.Mods |= input.ModCtrl
	}
	if ebiten.IsKeyPressed(ebiten.KeyAlt) {
		in.Mods |= input.ModAlt
	}
	in.Mx, in.My = ebiten.CursorPosition()
	in.WheelX, in.WheelY = ebiten.Wheel()
	for i, b := range ebitenButtons {
		in.Down[i] = ebiten.IsMouseButtonPressed(b)
		in.Press[i] = inpututil.IsMouseButtonJustPressed(b)
		in.Release[i] = inpututil.IsMouseButtonJustReleased(b)
	}
	in.Chars = ebiten.AppendInputChars(nil)
	in.Focused = ebiten.IsFocused()
	in.Blurred = e.BlurredFor(in.Focused)
	return in
}

// BlurredFor reports the window is in the background: it had focus before
// and has none now. A window that was never focused (Ebitengine reports
// false before the game runs, and a capture or a launch behind the
// terminal may never get focus) is not "in the background", so the colony
// never pauses itself without the player having left the window.
func (e *EbitenSource) BlurredFor(focused bool) bool {
	e.seenFocus = e.seenFocus || focused
	return e.seenFocus && !focused
}

// keyNames maps keys to the input package's names: Ebitengine's own,
// with left/right modifiers folded into Shift, Ctrl and Alt.
func keyNames(ks []ebiten.Key) []string {
	out := make([]string, 0, len(ks))
	seen := map[string]bool{}
	for _, k := range ks {
		n := keyName(k)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func keyName(k ebiten.Key) string {
	switch k {
	case ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyShift:
		return input.KeyShift
	case ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyControl:
		return input.KeyCtrl
	case ebiten.KeyAltLeft, ebiten.KeyAltRight, ebiten.KeyAlt:
		return input.KeyAlt
	case ebiten.KeyMetaLeft, ebiten.KeyMetaRight, ebiten.KeyMeta:
		return "Meta"
	}
	return k.String()
}
