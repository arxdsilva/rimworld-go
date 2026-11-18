// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package zone

import "rimworld-go/internal/domain/economy/item"

// Plugin item kinds in storage (PLG-026). A plugin kind is a plain good:
// a zone's older switches and every ceiling take it as they take
// components (its twin), and the filter tree files it under its def's
// category (the run's item.Book, passed to the …In functions; the zero
// Book is core). Core zones and filters never gain a plugin bit unless the
// run has plugin kinds, so core state hashes are unchanged.

// pluginTwin is the core kind a plugin kind is stored like.
const pluginTwin = item.Components

// twin is the kind k is stored like: pluginTwin for a plugin kind, else k.
func twin(k item.Kind) item.Kind {
	if int(k) >= item.CoreKinds && k != item.None {
		return pluginTwin
	}
	return k
}

// pluginsOf is every plugin kind of b.
func pluginsOf(b item.Book) (f Filter) {
	for _, k := range b.Plugins() {
		f = f.With(k, true)
	}
	return f
}

// categoriesIn is the filter tree's categories with b's plugin kinds filed
// under their paths (a path the tree lacks files nothing).
func categoriesIn(b item.Book) []category {
	cats := categories()
	for _, k := range b.Plugins() {
		for i := range cats {
			if cats[i].path == b.Def(k).Path {
				cats[i].kinds = append(cats[i].kinds, k)
				break
			}
		}
	}
	return cats
}

// UniverseHas reports a storage zone of kind k may ever take item kind it
// (Universe(k).Has, a plugin kind as its twin).
func UniverseHas(k Kind, it item.Kind) bool { return Universe(k).Has(twin(it)) }

// universeIn is UniverseOf(k, p) with b's plugin kinds when it takes
// their twin.
func universeIn(k Kind, p Profile, b item.Book) Filter {
	u := UniverseOf(k, p)
	if u.Has(pluginTwin) {
		u = u.Or(pluginsOf(b))
	}
	return u
}

// EffectiveIn is Effective in a run whose item kinds are b: a zone its
// older switches still govern takes b's plugin kinds with their twin.
func (z Zone) EffectiveIn(b item.Book) Filter {
	f := z.Effective()
	if !z.Custom && f.Has(pluginTwin) {
		f = f.Or(pluginsOf(b))
	}
	return f
}

// SetAllowedIn is SetAllowed in a run whose item kinds are b.
func (z *Zone) SetAllowedIn(set Filter, on bool, b item.Book) {
	f := z.EffectiveIn(b)
	if on {
		f = f.Or(set.And(universeIn(z.Kind, z.Profile, b)))
	} else {
		f = f.AndNot(set)
	}
	z.Filter, z.Custom = f, true
}

// PasteFromIn is PasteFrom in a run whose item kinds are b.
func (z *Zone) PasteFromIn(src Zone, b item.Book) {
	z.PasteFrom(src)
	z.Filter = src.EffectiveIn(b).And(universeIn(z.Kind, z.Profile, b))
}

// AdmitPlugins gives a freshly made zone b's plugin kinds when its filter
// takes their twin (a new stockpile takes the run's plugin goods as it
// takes components).
func (z *Zone) AdmitPlugins(b item.Book) {
	if z.Custom && z.Filter.Has(pluginTwin) {
		z.Filter = z.Filter.Or(pluginsOf(b))
	}
}
