// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

// TraitClass is the trait class name in plugin files.
const TraitClass = trait.Class

// Trait field limits (PLG-026): what a def may hold.
const (
	trMaxLabel  = 30 // runes
	trMaxDesc   = 400
	trMaxGroup  = 30
	trMaxAdd    = 100_000
	trMaxMul    = 10_000
	trMaxSkill  = 20
	trMaxPct    = 1000
	trMaxRarity = 100
	trMaxMods   = 12
)

// trBehaviourMsg is what a plugin trait cannot set: the game's own
// behaviour, written in code against particular core traits.
const trBehaviourMsg = "are the game's own behaviour, written for particular traits: a plugin trait cannot set it, and a patch leaves a core trait's as it is"

// The Def fields read as code, not data (PLG-026): a note that claims an
// effect no stat mod expresses, and the thoughts the food, drug, social and
// body systems scale or add for named traits (note, thought_scale,
// add_thought) are not fields of a trait def.

// EncodeTraits turns a book's traits into trees under their keys, one per
// trait in ID order (PLG-026). The name and ID are the key's and the
// position's, not fields. Zero fields are left out, except the required
// label. The behaviour fields (note, thought_scale, add_thought) are not encoded: a core
// trait keeps the game's own whatever a patch does.
func EncodeTraits(b trait.Book) []Def {
	out := make([]Def, 0, b.Len())
	for i := 1; i <= b.Len(); i++ {
		d, key := b.DefOf(trait.ID(i)), b.Key(trait.ID(i))
		plugin, _, _ := strings.Cut(key, ".")
		body := map[string]any{"label": d.Label}
		rcPut(body, "desc", d.Desc != "", d.Desc)
		rcPut(body, "group", d.Group != trait.NoGroup, trait.GroupKey(d.Group))
		if len(d.Mods) > 0 {
			l := make([]any, len(d.Mods))
			for j, m := range d.Mods {
				mm := map[string]any{"stat": m.Stat.Key()}
				rcPut(mm, "add", m.Add != 0, float64(m.Add))
				rcPut(mm, "mul", m.Mul != 0, float64(m.Mul))
				l[j] = mm
			}
			body["mods"] = l
		}
		trPutSkills(body, "work_speed", d.WorkSpeed)
		trPutSkills(body, "learn_mul", d.LearnMul)
		trPutSkills(body, "skill_bonus", d.SkillBonus)
		if l := bsBitKeys(d.Incapable); len(l) > 0 {
			body["incapable"] = l
		}
		if l := bsBitKeys(d.Needs); len(l) > 0 {
			body["needs"] = l
		}
		rcPut(body, "rarity", d.Rarity != 0, float64(d.Rarity))
		rcPut(body, "beauty", d.Beauty != 0, float64(d.Beauty))
		out = append(out, Def{Class: TraitClass, Key: key, Plugin: plugin, Body: body})
	}
	return out
}

func trPutSkills[V int8 | int16](body map[string]any, k string, m map[skill.ID]V) {
	if len(m) == 0 {
		return
	}
	t := make(map[string]any, len(m))
	for s, v := range m {
		t[s.Key()] = float64(v)
	}
	body[k] = t
}

// DecodeTraits types the trait defs (abstract ones already dropped) in
// order and freezes them into a book (PLG-026): core's come first, so each
// keeps its ID. Typing is strict: an unknown field, a wrong JSON type, a
// fractional number, an unknown stat, skill or incapability, a value out of
// range or a behaviour field is a problem, and its def is left out. Traits
// past trait.MaxBook are refused (an ID is a byte). A group name core does
// not know starts a group of the plugin's own, shared by name.
func DecodeTraits(defs []Def) (trait.Book, []Problem) {
	var list []trait.Def
	var keys []string
	var probs []Problem
	groups := map[string]uint8{}
	for _, d := range defs {
		td := trDecoder{bsDecoder: bsDecoder{def: d}, groups: groups}
		v := td.decode()
		if len(td.probs) == 0 && len(list) >= trait.MaxBook {
			td.fail("", "too many traits: a game holds at most %d", trait.MaxBook)
		}
		if len(td.probs) > 0 {
			probs = append(probs, td.probs...)
			continue
		}
		list, keys = append(list, v), append(keys, d.Key)
	}
	return trait.NewBook(list, keys), probs
}

// trDecoder types one trait def.
type trDecoder struct {
	bsDecoder
	groups map[string]uint8 // plugin group names to numbers, shared by a book's defs
}

func (td *trDecoder) decode() trait.Def {
	var d trait.Def
	if td.def.Class != TraitClass {
		td.fail("", "not a %s def", TraitClass)
		return d
	}
	core, isCore := trait.Core().IDOf(td.def.Key)
	if isCore && td.def.Plugin == Core {
		// What is not data stays the game's own.
		c := trait.Core().DefOf(core)
		d = trait.Def{Key: c.Key, Note: c.Note, ThoughtScale: c.ThoughtScale, AddThought: c.AddThought}
	} else {
		d.Key = td.def.Key
	}
	body := td.def.Body
	for _, k := range rcSorted(body) {
		td.field(&d, k, body[k])
	}
	if _, ok := body["label"]; !ok {
		td.fail("label", "required")
	}
	return d
}

func (td *trDecoder) field(d *trait.Def, k string, v any) {
	switch k {
	case "label":
		d.Label = td.text(k, v, 1, trMaxLabel)
	case "desc":
		d.Desc = td.text(k, v, 0, trMaxDesc)
	case "group":
		d.Group = td.group(v)
	case "mods":
		d.Mods = td.mods(v)
	case "work_speed":
		d.WorkSpeed = trSkillMap(&td.bsDecoder, k, v, 0, trMaxPct, func(n int) int16 { return int16(n) }) // #nosec G115 -- trSkillMap num bounds [0, trMaxPct=1000] fits int16
	case "learn_mul":
		d.LearnMul = trSkillMap(&td.bsDecoder, k, v, 0, trMaxPct, func(n int) int16 { return int16(n) }) // #nosec G115 -- trSkillMap num bounds [0, trMaxPct=1000] fits int16
	case "skill_bonus":
		d.SkillBonus = trSkillMap(&td.bsDecoder, k, v, -trMaxSkill, trMaxSkill, func(n int) int8 { return int8(n) }) // #nosec G115 -- trSkillMap num bounds [-20, 20] (trMaxSkill) fits int8
	case "incapable":
		d.Incapable = backstory.Bits(td.keys(k, v, trBit)) // #nosec G115 -- bits from backstory.BitOfKey, all < numBits=10 <= 16
	case "needs":
		d.Needs = backstory.Bits(td.keys(k, v, trBit)) // #nosec G115 -- bits from backstory.BitOfKey, all < numBits=10 <= 16
	case "rarity":
		d.Rarity = uint8(td.num(k, v, 0, trMaxRarity)) // #nosec G115 -- num rejects outside [0, trMaxRarity=100], fits uint8
	case "beauty":
		d.Beauty = int8(td.num(k, v, -2, 2)) // #nosec G115 -- num rejects outside [-2, 2], fits int8
	case "note", "thought_scale", "add_thought":
		td.fail(k, "note, thought_scale and add_thought "+trBehaviourMsg)
	default:
		td.fail(k, "unknown field")
	}
}

func trBit(s string) (uint64, bool) {
	b, ok := backstory.BitOfKey(s)
	return uint64(b), ok
}

// group names a conflict group: one of core's, or a plugin's own.
func (td *trDecoder) group(v any) uint8 {
	s, ok := v.(string)
	if !ok {
		td.fail("group", "want a string, got %s", bsJSONType(v))
		return trait.NoGroup
	}
	if n := len(s); n < 1 || n > trMaxGroup || strings.ToLower(s) != s || strings.ContainsAny(s, " .") {
		td.fail("group", "%q: want a lowercase name of 1 to %d characters without spaces or dots", s, trMaxGroup)
		return trait.NoGroup
	}
	if g, ok := trait.GroupOfKey(s); ok {
		return g
	}
	g, ok := td.groups[s]
	if !ok {
		if int(trait.FirstPluginGroup)+len(td.groups) > 255 {
			td.fail("group", "too many conflict groups")
			return trait.NoGroup
		}
		g = trait.FirstPluginGroup + uint8(len(td.groups)) // #nosec G115 -- group count <= 255 checked at :200
		td.groups[s] = g
	}
	return g
}

func (td *trDecoder) mods(v any) []stat.Mod {
	l, ok := v.([]any)
	if !ok {
		td.fail("mods", "want a list, got %s", bsJSONType(v))
		return nil
	}
	if len(l) > trMaxMods {
		td.fail("mods", "%d mods, want at most %d", len(l), trMaxMods)
		return nil
	}
	var out []stat.Mod
	for i, x := range l {
		field := fmt.Sprintf("mods.%d", i)
		m, ok := x.(map[string]any)
		if !ok {
			td.fail(field, "want an object, got %s", bsJSONType(x))
			continue
		}
		var mod stat.Mod
		before := len(td.probs)
		for _, k := range rcSorted(m) {
			f := field + "." + k
			switch k {
			case "stat":
				s, ok := m[k].(string)
				if !ok {
					td.fail(f, "want a string, got %s", bsJSONType(m[k]))
					break
				}
				id, ok := stat.ParseKey(s)
				if !ok {
					td.fail(f, "unknown stat %q", s)
				}
				mod.Stat = id
			case "add":
				mod.Add = int32(td.num(f, m[k], -trMaxAdd, trMaxAdd)) // #nosec G115 -- num rejects outside [-trMaxAdd, trMaxAdd]=±100000, fits int32
			case "mul":
				mod.Mul = int32(td.num(f, m[k], 0, trMaxMul)) // #nosec G115 -- num rejects outside [0, trMaxMul=10000], fits int32
			default:
				td.fail(f, "unknown field")
			}
		}
		if _, ok := m["stat"]; !ok {
			td.fail(field+".stat", "required")
		} else if len(td.probs) == before && mod.Add == 0 && mod.Mul == 0 {
			td.fail(field, "a mod needs a non-zero add or mul")
		}
		out = append(out, mod)
	}
	return out
}

// trSkillMap types an object of skill key to whole number.
func trSkillMap[V int8 | int16](bd *bsDecoder, field string, v any, lo, hi int, conv func(int) V) map[skill.ID]V {
	m, ok := v.(map[string]any)
	if !ok {
		bd.fail(field, "want an object, got %s", bsJSONType(v))
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	out := make(map[skill.ID]V, len(m))
	for k, x := range m {
		id, ok := skill.Parse(k)
		if !ok {
			bd.fail(field+"."+k, "unknown skill")
			continue
		}
		out[id] = conv(bd.num(field+"."+k, x, lo, hi))
	}
	return out
}

// traitClass is the trait class (PLG-026).
func traitClass() class {
	return class{
		name: TraitClass,
		core: func() []Def { return EncodeTraits(trait.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeTraits(defs)
			if cat != nil {
				cat.Traits = b
			}
			return probs
		},
	}
}
