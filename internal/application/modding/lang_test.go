// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"testing"
)

func langPlugin(id string, files ...File) Plugin {
	return Plugin{Manifest: Manifest{ID: id}, Lang: files}
}

func langOrder(ids ...string) Order {
	o := Order{Loaded: []Placed{{ID: Core}}}
	for _, id := range ids {
		o.Loaded = append(o.Loaded, Placed{ID: id})
	}
	return o
}

func TestLangMergeOrderAndOverride(t *testing.T) {
	a := langPlugin("a", File{Path: "lang/pt-BR.json", Data: map[string]any{"Hello": "Ola A", "Only A": "So A", "_meta": map[string]any{"name": "x"}}})
	b := langPlugin("b", File{Path: "lang/pt-BR.json", Data: map[string]any{"Hello": "Ola B"}}, File{Path: "lang/de.json", Data: map[string]any{"Hello": "Hallo"}})
	for _, tc := range []struct {
		order Order
		want  string
	}{{langOrder("a", "b"), "Ola B"}, {langOrder("b", "a"), "Ola A"}} {
		tr, probs := BuildLang([]Plugin{b, a}, tc.order, nil)
		if len(probs) != 0 {
			t.Fatalf("problems: %v", probs)
		}
		if got := tr.For("pt-BR")["Hello"]; got != tc.want {
			t.Errorf("order %v: Hello = %q, want %q", tc.order.Loaded, got, tc.want)
		}
		if tr.For("pt-BR")["Only A"] != "So A" || tr.For("pt-BR")["_meta"] != "" {
			t.Errorf("table = %v", tr.For("pt-BR"))
		}
		if !reflect.DeepEqual(tr.Locales(), []string{"de", "pt-BR"}) {
			t.Errorf("locales = %v", tr.Locales())
		}
	}
}

func TestLangOnlyLoadedPluginsCount(t *testing.T) {
	a := langPlugin("a", File{Path: "lang/pt-BR.json", Data: map[string]any{"Hello": "Ola"}})
	tr, _ := BuildLang([]Plugin{a}, langOrder(), nil)
	if tr.For("pt-BR") != nil || len(tr.Locales()) != 0 {
		t.Errorf("a plugin that did not load translated: %v", tr.For("pt-BR"))
	}
}

func TestLangBadEntriesAreProblemsNotBreakage(t *testing.T) {
	a := langPlugin("a",
		File{Path: "lang/pt-BR.json", Data: map[string]any{"Good": "Bom", "Num": 3.0, "Blank": " ", "Count %d": "Contar %s"}},
		File{Path: "lang/not a locale.json", Data: map[string]any{"x": "y"}},
		File{Path: "lang/fr.json", Data: []any{"x"}})
	tr, probs := BuildLang([]Plugin{a}, langOrder("a"), nil)
	if tr.For("pt-BR")["Good"] != "Bom" || len(tr.For("pt-BR")) != 1 {
		t.Errorf("table = %v", tr.For("pt-BR"))
	}
	if len(probs) != 5 {
		t.Fatalf("problems = %v", probs)
	}
	for _, p := range probs {
		if p.Plugin != "a" || p.File == "" || !p.Warn {
			t.Errorf("problem lacks provenance or is fatal: %+v", p)
		}
	}
	if Errors(probs) {
		t.Error("a bad translation must not count as an error")
	}
}

func TestLangDefKeysResolveToSourceText(t *testing.T) {
	defs := []Def{{Class: "backstory", Key: "a.surveyor", Body: map[string]any{"title": "Surveyor"}}}
	a := langPlugin("a", File{Path: "lang/pt-BR.json", Data: map[string]any{
		"def.backstory.a.surveyor.title": "Agrimensor",
		"def.backstory.a.surveyor.desc":  "x",
		"def.recipe.a.none.label":        "x"}})
	tr, probs := BuildLang([]Plugin{a}, langOrder("a"), defs)
	if tr.For("pt-BR")["Surveyor"] != "Agrimensor" || len(tr.For("pt-BR")) != 1 {
		t.Errorf("table = %v", tr.For("pt-BR"))
	}
	if len(probs) != 2 {
		t.Errorf("problems = %v", probs)
	}
}

func TestPluginLocales(t *testing.T) {
	p := langPlugin("a", File{Path: "lang/pt-BR.json"}, File{Path: "lang/de.json"}, File{Path: "lang/Bad.json"})
	if got := PluginLocales(p); !reflect.DeepEqual(got, []string{"de", "pt-BR"}) {
		t.Errorf("locales = %v", got)
	}
}
