package uikit

import "rimworld-go/internal/adapters/inbound/ebiten/inputframe"

// Host is what the kit needs of the program around it: this Update's
// input, the cursor, the screen size, a frame counter, key repeat, the UI
// sounds, the hint line and the reduced-motion setting.
type Host interface {
	// Frame is this Update's input; widgets take the presses they use.
	Frame() *inputframe.Frame
	// Cursor is the pointer in logical UI pixels.
	Cursor() (x, y int)
	// Size is the logical screen.
	Size() (sw, sh int)
	// Updates counts Updates (the text caret blinks on it).
	Updates() uint64
	// KeyRepeat reports the key went down this frame or is repeating.
	KeyRepeat(key string) bool
	// Cue plays a UI sound ("ui.click", "ui.deny").
	Cue(id string)
	// Hint shows a line in the footer hint ("" clears it).
	Hint(text string)
	// ReducedMotion reports the accessibility setting (no caret blink).
	ReducedMotion() bool
}

// New is a kit bound to its host.
func New(h Host) *Kit { return &Kit{host: h} }

// Bind sets the kit's host (a zero Kit embedded in its host binds late).
func (k *Kit) Bind(h Host) { k.host = h }

// Bound reports the kit has a host.
func (k *Kit) Bound() bool { return k.host != nil }

// Host is the kit's host.
func (k *Kit) Host() Host { return k.host }

// in is this Update's input.
func (k *Kit) in() *inputframe.Frame { return k.host.Frame() }
