package game

import "rimworld-go/internal/adapters/inbound/ebiten/display"

// Display is the window and monitor the client runs in; display.Screen
// fills it.
type Display interface {
	Window(fullscreen, vsync bool)      // AppConfig.Window
	Apply(size string, borderless bool) // AppConfig.Display (OPT-029)
	Cursor(visible bool)                // AppConfig.Cursor (OPT-028)
	scaler
	sizer
	Run(g display.Game, win display.Window) error
}

// scaler is the monitor's device scale (the Medium tier's rows read it).
type scaler interface{ DeviceScale() float64 }

// sizer is the monitor's size (usage data reports it).
type sizer interface{ MonitorSize() (w, h int) }
