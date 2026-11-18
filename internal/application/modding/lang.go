// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Translations are the loaded plugins' lang/<locale>.json files merged per
// locale (PLG-055). A file is a flat object of source string -> translation,
// the shape of the game's own catalogs, so the client can lay it over its
// table. Two forms of key are understood:
//
//   - the source text itself ("Salt-flat surveyor"), which also translates
//     core strings;
//   - def.<class>.<key>.<field> ("def.backstory.example_frontier.salt_flat_surveyor.title"),
//     resolved here to the def's source text, so a plugin need not repeat it.
//
// "_meta" (the catalog's own name) is not a translation.
type Translations struct {
	byLocale map[string]map[string]string
}

// For is the merged table for locale: plugins in load order, a later plugin
// winning. Nil when no plugin translates it. The map is shared; do not write.
func (t Translations) For(locale string) map[string]string { return t.byLocale[locale] }

// Locales lists the locales some loaded plugin translates, sorted.
func (t Translations) Locales() []string {
	out := make([]string, 0, len(t.byLocale))
	for l := range t.byLocale {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// localeRE is the shape of a locale code ("pt-BR", "zh-Hans"); it is the
// client's catalog code shape, so a plugin file can only name a language the
// client could load.
var localeRE = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// verbRE finds a fmt verb ("%d", "%-5s", "%[1]s"); "%%" is matched apart.
var verbRE = regexp.MustCompile(`%%|%[-+# 0-9.*\[\]]*[a-zA-Z]`)

// LocaleOf is the locale a lang file path names ("lang/pt-BR.json" ->
// "pt-BR"), or false when it names none.
func LocaleOf(file string) (string, bool) {
	loc, ok := strings.CutSuffix(path.Base(file), ".json")
	return loc, ok && localeRE.MatchString(loc)
}

// PluginLocales are the locales p ships a usable file for, sorted.
func PluginLocales(p Plugin) []string {
	var out []string
	for _, f := range p.Lang {
		if loc, ok := LocaleOf(f.Path); ok && !slices.Contains(out, loc) {
			out = append(out, loc)
		}
	}
	slices.Sort(out)
	return out
}

// BuildLang merges the lang files of the plugins that loaded (order), in load
// order, against the defs of cat (for def.* keys). A file that is not an
// object, names no locale, or an entry that is not a string, is blank, or
// changes its source's fmt verbs, is reported with plugin and file and
// skipped; it never stops another entry or file.
func BuildLang(found []Plugin, order Order, defs []Def) (Translations, []Problem) {
	byID := make(map[string]Plugin, len(found))
	for _, p := range found {
		byID[p.Manifest.ID] = p
	}
	var probs []Problem
	t := Translations{byLocale: map[string]map[string]string{}}
	for _, placed := range order.Loaded {
		p, ok := byID[placed.ID]
		if !ok || placed.ID == Core {
			continue
		}
		for _, f := range p.Lang {
			loc, ok := LocaleOf(f.Path)
			if !ok {
				probs = append(probs, langProblem(p, f.Path, "", "not a locale file name (want lang/<locale>.json, e.g. lang/pt-BR.json)", true))
				continue
			}
			obj, ok := f.Data.(map[string]any)
			if !ok {
				probs = append(probs, langProblem(p, f.Path, "", "a translation file is one object of source text to translation", true))
				continue
			}
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				src, tr, msg := langEntry(k, obj[k], defs)
				if msg != "" {
					if msg != "-" {
						probs = append(probs, langProblem(p, f.Path, k, msg, true))
					}
					continue
				}
				if t.byLocale[loc] == nil {
					t.byLocale[loc] = map[string]string{}
				}
				t.byLocale[loc][src] = tr
			}
		}
	}
	return t, probs
}

// langEntry is one entry's source text and translation, or a message ("-":
// skip silently, the "_meta" key).
func langEntry(key string, v any, defs []Def) (src, tr, msg string) {
	if key == "_meta" {
		return "", "", "-"
	}
	s, ok := v.(string)
	if !ok {
		return "", "", fmt.Sprintf("translation must be a string, not %s", treeKind(v))
	}
	if strings.TrimSpace(s) == "" {
		return "", "", "empty translation (the source text is shown)"
	}
	src = key
	if rest, ok := strings.CutPrefix(key, "def."); ok {
		var err string
		if src, err = defSource(rest, defs); err != "" {
			return "", "", err
		}
	}
	if !sameVerbs(src, s) {
		return "", "", "translation changes the fmt verbs of its source text"
	}
	return src, s, ""
}

// defSource resolves "<class>.<key>.<field>" to the def's text.
func defSource(rest string, defs []Def) (string, string) {
	parts := strings.Split(rest, ".")
	if len(parts) < 3 {
		return "", "def key must be def.<class>.<key>.<field>"
	}
	class, field, key := parts[0], parts[len(parts)-1], strings.Join(parts[1:len(parts)-1], ".")
	for _, d := range defs {
		if d.Class != class || d.Key != key {
			continue
		}
		if s, ok := d.Body[field].(string); ok && s != "" {
			return s, ""
		}
		return "", fmt.Sprintf("def %s has no text field %q", d.Ref(), field)
	}
	return "", fmt.Sprintf("no def %s/%s", class, key)
}

func sameVerbs(a, b string) bool {
	return slices.Equal(verbList(a), verbList(b))
}

func verbList(s string) []string {
	var out []string
	for _, v := range verbRE.FindAllString(s, -1) {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

func langProblem(p Plugin, file, field, msg string, warn bool) Problem {
	return Problem{Plugin: p.Manifest.ID, File: file, Field: field, Msg: msg, Warn: warn}
}

func treeKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64, int, int64:
		return "a number"
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("%T", v)
}
