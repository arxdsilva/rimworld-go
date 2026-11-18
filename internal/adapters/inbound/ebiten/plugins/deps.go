// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package plugins is the Plugin Manager (PLG-070, MAIN_UI §45), its load
// order explanation (PLG-071, §46) and the content problems view (PLG-041,
// §48). It reads one read model, View, filled once at start by the
// composition root, and changes nothing but the player's settings: turning
// a plugin on or off rewrites Settings.Plugins.Enabled, which the next start
// applies. It never touches a file or the simulation.
package plugins

import (
	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// Host is what the screens need of the program that no ports role carries.
type Host interface {
	// Plugins is the run's plugins as of the start.
	Plugins() View
	// SetSettings applies changed settings and stores them.
	SetSettings(config.Settings)
	// PaintMenu draws the menu backdrop (the dimmed diorama when there is
	// one) and the title, as the other main-menu screens do.
	PaintMenu(dst *ebiten.Image, sw, sh int, title string, body []string, buttons []uikit.Button)
}

// Shell composes the roles the screens call with their Host.
type Shell interface {
	ports.Settings
	ports.Widgets
	ports.Texts
	ports.Dispatcher
	Host
}
