// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import "rimworld-go/internal/application/modding"

// pluginText is what the loaded plugins translate (PLG-055): each locale's
// table merged in load order, ready for the client to lay over its catalog,
// and the problems their lang files raised (shown with the other content
// problems, never stopping the base table).
func pluginText(found []modding.Plugin, cat modding.Catalog) (map[string]map[string]string, []modding.Problem) {
	tr, probs := modding.BuildLang(found, cat.Order, cat.Defs)
	out := make(map[string]map[string]string, len(tr.Locales()))
	for _, loc := range tr.Locales() {
		out[loc] = tr.For(loc)
	}
	return out, probs
}
