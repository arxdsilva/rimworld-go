// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package infocard

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

const bellKey = "example_frontier.trail_bell"

// pluginKinds is the core book plus one plugin item.
func pluginKinds(desc string) Kinds {
	keys := item.Core().Keys()
	defs := make([]item.Def, 0, len(keys)+1)
	for k := range item.CoreKinds {
		defs = append(defs, item.CoreDef(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "trail bell", Desc: desc, Path: "Manufactured/Components",
		Mass: 250, Value: 12, Stack: 75})
	keys = append(keys[:item.CoreKinds:item.CoreKinds], bellKey)
	return Kinds{
		Items: item.NewBook(defs, keys),
		SourceOf: func(class, key string) game.Source {
			if class == "item" && key == bellKey {
				return game.Source{AddedBy: "example_frontier"}
			}
			return game.Source{}
		},
	}
}

func TestInfoCardPluginItem(t *testing.T) {
	s, _ := newFake()
	k := pluginKinds("A small brass bell for the trail.")
	c := Build(s, k, ports.OpenInfoCard{Kind: bellKey})
	if c.Title != "Trail bell" || c.Desc != "A small brass bell for the trail." {
		t.Fatalf("title/desc = %q / %q", c.Title, c.Desc)
	}
	wants(t, c, "Manufactured", "Mass: 250 g", "Value: 12 silver", "Stack limit: 75", "Added by example_frontier")
	if strings.Contains(text(c), "Nutrition") || strings.Contains(text(c), "Armour") {
		t.Errorf("plugin item shows core sections:\n%s", text(c))
	}
}

func TestInfoCardPluginItemWithoutDesc(t *testing.T) {
	s, _ := newFake()
	c := Build(s, pluginKinds(""), ports.OpenInfoCard{Kind: bellKey})
	if c.Desc != "A made good. It is hauled, stored and installed rather than used up as raw material." {
		t.Fatalf("desc = %q", c.Desc)
	}
}

func TestInfoCardUnknownName(t *testing.T) {
	s, _ := newFake()
	for _, n := range []string{"zzz", "example_frontier.gone"} {
		c := Build(s, pluginKinds("x"), ports.OpenInfoCard{Kind: n})
		if c.Desc != wordsUnknown {
			t.Errorf("%q: desc = %q", n, c.Desc)
		}
	}
}
