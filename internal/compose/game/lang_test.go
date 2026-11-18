// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"path/filepath"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/application/modding"
)

// The example plugin's pt-BR file reaches the client's table when the plugin
// is enabled (PLG-055), with its def.* key resolved to the def's source
// text, and nothing when it is off.
func TestPluginTextFromExample(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	found, err := src.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	text, lp := pluginText(found, cat)
	if len(probs)+len(lp) != 0 {
		t.Fatalf("problems: %v %v", probs, lp)
	}
	pt := text["pt-BR"]
	if pt["Salt-flat surveyor"] != "Agrimensor das salinas" || pt["Lash slat carpet"] != "Tapete de ripas amarradas" {
		t.Errorf("pt-BR = %v", pt)
	}
	for _, p := range found {
		if p.Manifest.ID == "example_frontier" && !slices.Equal(modding.PluginLocales(p), []string{"pt-BR"}) {
			t.Errorf("locales = %v", modding.PluginLocales(p))
		}
	}
	off, _ := modding.Build(found, nil, "0.1.35")
	if text, _ := pluginText(found, off); len(text) != 0 {
		t.Errorf("a disabled plugin translated: %v", text)
	}
}
