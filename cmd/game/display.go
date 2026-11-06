package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// applyDisplay sets the window's size preset and borderless mode (OPT-029):
// size is a config.WindowSizes entry ("WxH"; "" leaves the size free), and
// borderless covers the monitor with an undecorated window instead of
// switching to exclusive fullscreen. Fullscreen itself stays with Window.
func applyDisplay(size string, borderless bool) {
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
