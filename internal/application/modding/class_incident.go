// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"strings"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/world/biome"
)

// IncidentClass is the incident class name in plugin files.
const IncidentClass = incident.Class

// Incident field limits (PLG-026): what a def may hold.
const (
	icMaxWeight = 10000
	icMaxDays   = 3650
	icMaxLabel  = 60 // runes
	icMaxPoints = 100000
	icMaxPct    = 1000
)

// incidentClass is the incident class (PLG-026).
func incidentClass() class {
	return class{
		name: IncidentClass,
		core: func() []Def { return EncodeIncidents(incident.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeIncidents(defs)
			if cat != nil {
				cat.Incidents = b
			}
			return probs
		},
	}
}

// EncodeIncidents turns a book's incidents into trees under their keys, one
// per incident in ID order (PLG-026). The key and ID are the position's,
// not fields. A core incident has no "like" (its behaviour is its own); a
// plugin one names the core incident it runs.
func EncodeIncidents(b incident.Book) []Def {
	out := make([]Def, 0, b.Len())
	for i := range b.Len() {
		id := incident.ID(i)
		d, key := b.DefOf(id), b.Key(id)
		plugin, _, _ := strings.Cut(key, ".")
		body := map[string]any{"label": d.Name, "category": icCategory(d.Category),
			"weight": float64(d.Weight), "cooldown": float64(d.Cooldown), "min_day": float64(d.MinDay)}
		rcPut(body, "min_points", d.MinPoints != 0, float64(d.MinPoints))
		rcPut(body, "points_pct", d.PointsPct != 0, float64(d.PointsPct))
		rcPut(body, "season", d.Season != incident.AnySeason, icSeasonKey(d.Season))
		rcPut(body, "target", d.Target != incident.HomeMap, "world")
		if d.Like != id {
			body["like"] = b.Key(d.Like)
		}
		rcPut(body, "helpful", d.Helpful, true)
		rcPut(body, "disease", d.Disease, true)
		rcPut(body, "biomes", d.Biomes != 0, icBiomeKeys(d.Biomes))
		rcPut(body, "not", d.Not != 0, icBiomeKeys(d.Not))
		out = append(out, Def{Class: IncidentClass, Key: key, Plugin: plugin, Body: body})
	}
	return out
}

func icCategory(c incident.Category) string {
	if c == incident.Threat {
		return "threat"
	}
	return "nature"
}

func icSeasonKey(s incident.Season) string {
	if s == incident.WinterOnly {
		return "winter"
	}
	return "not_winter"
}

func icBiomeKeys(s biome.Set) []any {
	var l []any
	for _, d := range biome.All() {
		if s.Has(d.ID) {
			l = append(l, d.Key)
		}
	}
	return l
}

// DecodeIncidents types the incident defs (abstract ones already dropped)
// in order and freezes them into a book (PLG-026): core's come first, so
// each keeps its ID. A plugin incident is data only: "like" names the core
// incident whose behaviour it runs, and it has its own label, weight,
// cooldown and first day; a patch may change a core incident's numbers,
// label and biomes but not its category, disease flag or behaviour. Typing
// is strict; a bad def is a problem and is left out.
func DecodeIncidents(defs []Def) (incident.Book, []Problem) {
	var list []incident.Def
	var keys []string
	var probs []Problem
	core := incident.Core()
	for _, d := range defs {
		id := icDecoder{bsDecoder: bsDecoder{def: d}, core: core}
		v := id.decode()
		if len(id.probs) == 0 && len(list) >= incident.MaxBook {
			id.fail("", "too many incidents: a game holds at most %d", incident.MaxBook)
		}
		if len(id.probs) > 0 {
			probs = append(probs, id.probs...)
			continue
		}
		list, keys = append(list, v), append(keys, d.Key)
	}
	return incident.NewBook(list, keys), probs
}

// icDecoder types one incident def.
type icDecoder struct {
	bsDecoder
	core incident.Book
}

func (id *icDecoder) decode() incident.Def {
	var d incident.Def
	if id.def.Class != IncidentClass {
		id.fail("", "not a %s def", IncidentClass)
		return d
	}
	own, isCore := incident.ID(0), false
	if id.def.Plugin == Core {
		own, isCore = id.core.IDOf(id.def.Key)
	}
	if isCore {
		d = id.core.DefOf(own) // numbers the body leaves out stay core's
	} else {
		d.Like, d.Category = 0, incident.Nature
	}
	body := id.def.Body
	for _, k := range rcSorted(body) {
		id.field(&d, k, body[k], isCore)
	}
	if isCore {
		id.checkCore(d, own)
		return d
	}
	for _, k := range []string{"like", "label", "weight"} {
		if _, ok := body[k]; !ok {
			id.fail(k, "required")
		}
	}
	if !strings.Contains(id.def.Key, ".") || strings.HasPrefix(id.def.Key, Core+".") {
		id.fail("", "a plugin incident's key is <plugin>.<name>")
	}
	d.Key = id.def.Key
	return d
}

// checkCore refuses a patch that changes what is the game's own.
func (id *icDecoder) checkCore(d incident.Def, own incident.ID) {
	c := id.core.DefOf(own)
	if d.Category != c.Category {
		id.fail("category", "a patch cannot change a core incident's category")
	}
	if d.Disease != c.Disease {
		id.fail("disease", "a patch cannot change a core incident's disease flag")
	}
}

func (id *icDecoder) field(d *incident.Def, k string, v any, isCore bool) {
	switch k {
	case "label":
		d.Name = id.text(k, v, 1, icMaxLabel)
	case "like":
		if isCore {
			id.fail(k, "a core incident runs its own behaviour")
			return
		}
		d.Like = id.like(v)
	case "category":
		id.category(d, v, isCore)
	case "weight":
		d.Weight = id.num(k, v, 0, icMaxWeight)
	case "cooldown":
		d.Cooldown = id.num(k, v, 0, icMaxDays)
	case "min_day":
		d.MinDay = id.num(k, v, 0, icMaxDays)
	case "min_points":
		d.MinPoints = id.num(k, v, 0, icMaxPoints)
	case "points_pct":
		d.PointsPct = id.num(k, v, 0, icMaxPct)
	case "season":
		d.Season = icChoice(id, k, v, map[string]incident.Season{"any": incident.AnySeason, "winter": incident.WinterOnly, "not_winter": incident.NotWinter})
	case "target":
		d.Target = icChoice(id, k, v, map[string]incident.Target{"map": incident.HomeMap, "world": incident.WorldObject})
	case "helpful":
		d.Helpful = id.flag(k, v)
	case "disease":
		d.Disease = id.flag(k, v)
	case "biomes":
		d.Biomes = biome.Set(id.keys(k, v, icBiome)) // #nosec G115 -- bits from biome.Parse, ID < NumBiomes=12 <= 16
	case "not":
		d.Not = biome.Set(id.keys(k, v, icBiome)) // #nosec G115 -- bits from biome.Parse, ID < NumBiomes=12 <= 16
	default:
		id.fail(k, "unknown field")
	}
}

// icChoice reads a string field that must be one of the table's names.
func icChoice[T any](id *icDecoder, field string, v any, names map[string]T) T {
	var zero T
	s, ok := v.(string)
	if !ok {
		id.fail(field, "want a string, got %s", bsJSONType(v))
		return zero
	}
	t, ok := names[s]
	if !ok {
		id.fail(field, "unknown %s %q", field, s)
	}
	return t
}

func icBiome(s string) (uint64, bool) {
	b, ok := biome.Parse(s)
	return 1 << b, ok
}

func (id *icDecoder) category(d *incident.Def, v any, isCore bool) {
	s, ok := v.(string)
	switch {
	case !ok:
		id.fail("category", "want a string, got %s", bsJSONType(v))
	case s == "nature":
		d.Category = incident.Nature
	case s == "threat" && isCore:
		d.Category = incident.Threat
	case s == "threat":
		id.fail("category", "a plugin incident is a nature incident (the storyteller draws only those)")
	default:
		id.fail("category", "unknown category %q", s)
	}
}

// like resolves "core.<key>" to the core incident a plugin incident runs.
func (id *icDecoder) like(v any) incident.ID {
	s, ok := v.(string)
	if !ok {
		id.fail("like", "want a string, got %s", bsJSONType(v))
		return 0
	}
	l, ok := id.core.IDOf(s)
	switch {
	case !ok:
		id.fail("like", "unknown core incident %q (name one as core.<key>)", s)
	case l == incident.RivalRaid:
		id.fail("like", "core.rival_raid is queued by the story, not drawn: pick another")
	case l >= incident.HiveSwarm && l <= incident.MechRaid:
		id.fail("like", "%s is a storyteller threat face, not a drawn incident: pick another", s)
	}
	return l
}

func (id *icDecoder) flag(field string, v any) bool {
	b, ok := v.(bool)
	if !ok {
		id.fail(field, "want a boolean, got %s", bsJSONType(v))
	}
	return b
}
