package ebiten

import (
	globepkg "rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// globeInputFrom adapts the App's frame: w x h is the logical screen, the
// pointer the raw (physical) one the globe picks and drags with. blocked
// is whether the pointer is over screen chrome.
func globeInputFrom(in *inputframe.Frame, w, h int, blocked bool) globepkg.Input {
	return globepkg.Input{
		W: w, H: h, MX: in.Raw.Mx, MY: in.Raw.My, Wheel: in.Raw.WheelY,
		Down: in.Raw.Down, Press: in.Raw.Press, Release: in.Raw.Release,
		Held: in.Raw.Held, Keys: in.Raw.Pressed, Shift: in.Shift(), Blocked: blocked,
	}
}
