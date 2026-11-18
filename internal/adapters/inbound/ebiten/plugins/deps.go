// Package plugins is the Plugin Manager (PLG-070, MAIN_UI §45), its load
// order explanation (PLG-071, §46) and the content problems view (PLG-041,
// §48). It reads one read model, View, filled once at start by the
// composition root, and changes nothing but the player's settings: turning
// a plugin on or off rewrites Settings.Plugins.Enabled, which the next start
// applies. It never touches a file or the simulation.
package plugins

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/platform/config"
)

// Host is what the screens need of the program that no ports role carries.
type Host interface {
	// Plugins is the run's plugins as of the start.
	Plugins() View
	// SetSettings applies changed settings and stores them.
	SetSettings(config.Settings)
}

// Shell composes the roles the screens call with their Host.
type Shell interface {
	ports.Settings
	ports.Widgets
	ports.Texts
	ports.Dispatcher
	Host
}
