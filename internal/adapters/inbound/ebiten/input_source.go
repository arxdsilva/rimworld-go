package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"rimworld-go/internal/platform/input"
)

// Input layer (UI-016): the only place the client reads Ebitengine's
// keyboard and mouse. inputSource polls once per App.Update into a
// rawInput; App.pollInput resolves it against the bindings and the
// context stack into the frame every handler reads (input_frame.go).
// Tests swap in a fake source.

type mouseBtn int

const (
	mbLeft mouseBtn = iota
	mbRight
	mbMiddle
	numMouse
)

// rawInput is one frame of device state.
type rawInput struct {
	pressed []string // keys that went down this frame (input key names)
	held    []string // keys down now
	mods    input.Mod
	mx, my  int
	wheelX  float64
	wheelY  float64
	down    [numMouse]bool // held
	press   [numMouse]bool // went down this frame
	release [numMouse]bool // came up this frame
	chars   []rune         // typed characters
	focused bool
}

// inputSource is what the App needs from the input devices.
type inputSource interface {
	poll() rawInput
}

// ebitenInput reads the real keyboard and mouse.
type ebitenInput struct{ buf []ebiten.Key }

var ebitenButtons = [numMouse]ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle}

func (e *ebitenInput) poll() rawInput {
	var in rawInput
	e.buf = inpututil.AppendJustPressedKeys(e.buf[:0])
	in.pressed = keyNames(e.buf)
	e.buf = inpututil.AppendPressedKeys(e.buf[:0])
	in.held = keyNames(e.buf)
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		in.mods |= input.ModShift
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		in.mods |= input.ModCtrl
	}
	if ebiten.IsKeyPressed(ebiten.KeyAlt) {
		in.mods |= input.ModAlt
	}
	in.mx, in.my = ebiten.CursorPosition()
	in.wheelX, in.wheelY = ebiten.Wheel()
	for i, b := range ebitenButtons {
		in.down[i] = ebiten.IsMouseButtonPressed(b)
		in.press[i] = inpututil.IsMouseButtonJustPressed(b)
		in.release[i] = inpututil.IsMouseButtonJustReleased(b)
	}
	in.chars = ebiten.AppendInputChars(nil)
	in.focused = ebiten.IsFocused()
	return in
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
