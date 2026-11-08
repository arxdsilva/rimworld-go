package display

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Game is what Run drives: the client's App.
type Game = ebiten.Game

// Window is the window Run opens.
type Window struct {
	TPS           int
	Width, Height int
	Title         string
	// Probe opens a floating window the size of the display (PERF-002).
	Probe bool
}

// Screen is the Ebitengine window and monitor. Its zero value is ready.
type Screen struct{}

// Window switches fullscreen and vsync.
func (Screen) Window(fullscreen, vsync bool) {
	ebiten.SetFullscreen(fullscreen)
	ebiten.SetVsyncEnabled(vsync)
}

// Apply sets the window's size preset and borderless mode (OPT-029):
// size is a config.WindowSizes entry ("WxH"; "" leaves the size free), and
// borderless covers the monitor with an undecorated window instead of
// switching to exclusive fullscreen. Fullscreen itself stays with Window.
func (Screen) Apply(size string, borderless bool) {
	ebiten.SetWindowDecorated(!borderless)
	if borderless {
		w, h := ebiten.Monitor().Size()
		ebiten.SetWindowPosition(0, 0)
		ebiten.SetWindowSize(w, h)
		return
	}
	if w, h, ok := parseSize(size); ok {
		ebiten.SetWindowSize(w, h)
	}
}

// parseSize reads "1600x900".
func parseSize(s string) (w, h int, ok bool) {
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// Cursor shows or hides the OS cursor: hidden while the game draws its
// own pointer (OPT-028).
func (Screen) Cursor(visible bool) {
	if visible {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
}

// DeviceScale is the monitor's device scale factor.
func (Screen) DeviceScale() float64 { return ebiten.Monitor().DeviceScaleFactor() }

// MonitorSize is the monitor's size in device-independent pixels.
func (Screen) MonitorSize() (w, h int) { return ebiten.Monitor().Size() }

// Run sets up the window and runs g until it ends.
func (Screen) Run(g Game, win Window) error {
	ebiten.SetTPS(win.TPS)
	ebiten.SetWindowSize(win.Width, win.Height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(win.Title)
	if win.Probe {
		// PERF-002: a probe draws into a window the size of the display that
		// stays on top (the Fullscreen setting is overridden off), so it
		// renders every pixel a fullscreen game would while macOS never
		// throttles it as a hidden window (ENG-004: about 9 fps).
		ebiten.SetWindowSize(ebiten.Monitor().Size())
		ebiten.SetWindowFloating(true)
	}
	return ebiten.RunGame(g)
}
