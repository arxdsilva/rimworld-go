// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"rimworld-go/internal/application/scripting"
)

// queryAttrs are the world capability's ctx attributes this group serves.
var queryAttrs = []string{"animals", "buildings", "map", "world"}

// queryAttr is ctx.<name> for the world queries; ok is false for a name
// this group does not serve. The capability check ran before this.
func (c *call) queryAttr(name string) (starlark.Value, bool) {
	switch name {
	case "animals":
		return starlark.NewBuiltin("animals", c.animals), true
	case "buildings":
		return starlark.NewBuiltin("buildings", c.buildings), true
	case "map":
		return c.mapValue(), true
	case "world":
		return c.worldValue(), true
	}
	return nil, false
}

// onceRows is a list reader's result for one call: read on first use,
// truncated to MaxQueryRows and frozen.
type onceRows struct {
	done bool
	rows starlark.Tuple
}

// get builds the rows on first use only.
func (o *onceRows) get(build func() starlark.Tuple) starlark.Tuple {
	if !o.done {
		o.done = true
		o.rows = build()
		o.rows.Freeze()
	}
	return o.rows
}

// cachedAnimals, cachedBuildings and cachedWorld wrap a call's readers so
// each is read at most once per call (no field on call is needed: c.q is
// swapped for the wrappers on first query use).
type cachedAnimals struct {
	scripting.AnimalReader
	once onceRows
}

type cachedBuildings struct {
	scripting.BuildingReader
	once onceRows
}

type cachedWorld struct {
	scripting.WorldReader
	factions, sites onceRows
}

// cached is c.q with its list readers wrapped, once.
func (c *call) cached() *scripting.Queries {
	if _, ok := c.q.Animals.(*cachedAnimals); !ok {
		c.q.Animals = &cachedAnimals{AnimalReader: c.q.Animals}
	}
	if _, ok := c.q.Buildings.(*cachedBuildings); !ok {
		c.q.Buildings = &cachedBuildings{BuildingReader: c.q.Buildings}
	}
	if _, ok := c.q.World.(*cachedWorld); !ok {
		c.q.World = &cachedWorld{WorldReader: c.q.World}
	}
	return &c.q
}

// rowsOf is n rows built by row(i), capped at MaxQueryRows.
func rowsOf(n int, row func(i int) starlark.Value) starlark.Tuple {
	n = min(n, scripting.MaxQueryRows)
	t := make(starlark.Tuple, n)
	for i := range t {
		t[i] = row(i)
	}
	return t
}

func rowStruct(name string, d starlark.StringDict) starlark.Value {
	s := starlarkstruct.FromStringDict(starlark.String(name), d)
	s.Freeze()
	return s
}

// noArgs rejects arguments to a zero-argument builtin.
func noArgs(b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) error {
	return starlark.UnpackPositionalArgs(b.Name(), args, kw, 0)
}

// animals is ctx.animals(): one struct (species, label, count, predator)
// per wild species, species ascending; empty without a reader.
func (c *call) animals(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	if err := noArgs(b, args, kw); err != nil {
		return nil, err
	}
	a, ok := c.cached().Animals.(*cachedAnimals)
	if !ok || a.AnimalReader == nil {
		return starlark.Tuple{}, nil
	}
	return a.once.get(func() starlark.Tuple {
		l := a.Animals()
		return rowsOf(len(l), func(i int) starlark.Value {
			return rowStruct("animal", starlark.StringDict{
				"species": starlark.String(l[i].Species), "label": starlark.String(l[i].Label),
				"count": starlark.MakeInt(l[i].Count), "predator": starlark.Bool(l[i].Predator)})
		})
	}), nil
}

// buildings is ctx.buildings(): one struct (kind, count) per building
// kind, kind ascending; empty without a reader.
func (c *call) buildings(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	if err := noArgs(b, args, kw); err != nil {
		return nil, err
	}
	bl, ok := c.cached().Buildings.(*cachedBuildings)
	if !ok || bl.BuildingReader == nil {
		return starlark.Tuple{}, nil
	}
	return bl.once.get(func() starlark.Tuple {
		l := bl.Buildings()
		return rowsOf(len(l), func(i int) starlark.Value {
			return rowStruct("building", starlark.StringDict{
				"kind": starlark.String(l[i].Kind), "count": starlark.MakeInt(l[i].Count)})
		})
	}), nil
}

// mapValue is ctx.map: width, height, weather and the terrain and
// temperature lookups; with no reader the size is 0x0, weather "" and
// every cell None.
func (c *call) mapValue() starlark.Value {
	m := c.q.Map
	w, h, weather := 0, 0, ""
	if m != nil {
		w, h = m.Size()
		weather = m.Weather()
	}
	return rowStruct("map", starlark.StringDict{
		"width": starlark.MakeInt(w), "height": starlark.MakeInt(h), "weather": starlark.String(weather),
		"terrain": starlark.NewBuiltin("terrain", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
			x, y, err := cellArgs(b, args, kw)
			if err != nil || m == nil {
				return starlark.None, err
			}
			if t, ok := m.Terrain(x, y); ok {
				return starlark.String(t), nil
			}
			return starlark.None, nil
		}),
		"temperature": starlark.NewBuiltin("temperature", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
			x, y, err := cellArgs(b, args, kw)
			if err != nil || m == nil {
				return starlark.None, err
			}
			if t, ok := m.Temperature(x, y); ok {
				return starlark.MakeInt(t), nil
			}
			return starlark.None, nil
		}),
	})
}

// cellArgs unpacks (x, y).
func cellArgs(b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (x, y int, err error) {
	err = starlark.UnpackPositionalArgs(b.Name(), args, kw, 2, &x, &y)
	return
}

// worldValue is ctx.world: factions() and sites().
func (c *call) worldValue() starlark.Value {
	return rowStruct("world", starlark.StringDict{
		"factions": starlark.NewBuiltin("factions", c.factions),
		"sites":    starlark.NewBuiltin("sites", c.sites),
	})
}

// worldCache is the call's wrapped world reader; nil without a reader.
func (c *call) worldCache() *cachedWorld {
	w, ok := c.cached().World.(*cachedWorld)
	if !ok || w.WorldReader == nil {
		return nil
	}
	return w
}

func (c *call) factions(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	if err := noArgs(b, args, kw); err != nil {
		return nil, err
	}
	w := c.worldCache()
	if w == nil {
		return starlark.Tuple{}, nil
	}
	return w.factions.get(func() starlark.Tuple {
		l := w.Factions()
		return rowsOf(len(l), func(i int) starlark.Value {
			return rowStruct("faction", starlark.StringDict{
				"name": starlark.String(l[i].Name), "goodwill": starlark.MakeInt(l[i].Goodwill),
				"hostile": starlark.Bool(l[i].Hostile)})
		})
	}), nil
}

func (c *call) sites(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	if err := noArgs(b, args, kw); err != nil {
		return nil, err
	}
	w := c.worldCache()
	if w == nil {
		return starlark.Tuple{}, nil
	}
	return w.sites.get(func() starlark.Tuple {
		l := w.Sites()
		return rowsOf(len(l), func(i int) starlark.Value {
			return rowStruct("site", starlark.StringDict{
				"name": starlark.String(l[i].Name), "faction": starlark.String(l[i].Faction)})
		})
	}), nil
}
