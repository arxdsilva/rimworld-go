// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/platform/input"
)

// The strip asks the shell which tab's window is open and for key hints.
func (f *fakeShell) TabOn(string) bool { return false }

func (f *fakeShell) KeyHint(input.Action) string { return "" }

// TestTabsHavePluginsTextTab: the strip has a keyless text tab "Plugins",
// and every tab still gets a width at the smallest screen.
func TestTabsHavePluginsTextTab(t *testing.T) {
	c, _ := newFake()
	var found bool
	for _, b := range c.Tabs().Buttons(1280, 720) {
		if b.ID != ids.TabPlugins {
			continue
		}
		found = true
		if b.Label != "Plugins" || b.Disabled || b.W <= 0 {
			t.Fatalf("Plugins tab = %+v", b)
		}
	}
	if !found {
		t.Fatal("no Plugins tab")
	}
	if ws := TabWidths(1280); len(ws) != len(c.Tabs().Buttons(1280, 720)) {
		t.Fatalf("%d widths", len(ws))
	}
}
