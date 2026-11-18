package modding

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
)

// BackstoryClass is the class name in defs files and patch targets.
const BackstoryClass = backstory.Class

// Backstory field limits (PLG-021): what a def may hold.
const (
	bsMaxSkill  = 20
	bsMaxWeight = 1000
	bsMaxTitle  = 40 // runes
	bsMaxShort  = 20 // runes
	bsMaxDesc   = 800
)

// bsSlots names the slots in a def ("slot": "childhood").
var bsSlots = [...]string{backstory.Childhood: "childhood", backstory.Adulthood: "adulthood"}

// EncodeBackstories turns a book's defs into trees under their keys, one
// per backstory in ID order (PLG-021). Plugin is the key's namespace;
// trees hold only what encoding/json decodes into (float64, []any,
// map[string]any, string), and empty skills, incapable and tags are left
// out.
func EncodeBackstories(b backstory.Book) []Def {
	out := make([]Def, 0, b.Len())
	for i := 1; i <= b.Len(); i++ {
		id := backstory.ID(i)
		d, key := b.DefOf(id), b.Key(id)
		plugin, _, _ := strings.Cut(key, ".")
		body := map[string]any{
			"slot": bsSlots[d.Slot], "title": d.Title, "short": d.Short, "desc": d.Desc,
			"weight": float64(d.Weight),
		}
		if len(d.Skills) > 0 {
			sk := make(map[string]any, len(d.Skills))
			for s, v := range d.Skills {
				sk[s.Key()] = float64(v)
			}
			body["skills"] = sk
		}
		if l := bsBitKeys(d.Incapable); len(l) > 0 {
			body["incapable"] = l
		}
		if l := bsTagKeys(d.Tags); len(l) > 0 {
			body["tags"] = l
		}
		out = append(out, Def{Class: BackstoryClass, Key: key, Plugin: plugin, Body: body})
	}
	return out
}

func bsBitKeys(b backstory.Bits) []any {
	var out []any
	for bit := backstory.Bits(1); bit&backstory.AllBits != 0; bit <<= 1 {
		if b&bit != 0 {
			out = append(out, backstory.BitKey(bit))
		}
	}
	return out
}

func bsTagKeys(t backstory.Tag) []any {
	var out []any
	for tag := backstory.Tag(1); tag&backstory.AllTags != 0; tag <<= 1 {
		if t&tag != 0 {
			out = append(out, backstory.TagKey(tag))
		}
	}
	return out
}

// DecodeBackstories types the backstory defs (abstract ones already
// dropped) in order and freezes them into a book (PLG-024, PLG-042):
// core's come first, so each keeps its ID. Typing is strict: an unknown
// field, a wrong JSON type, a fractional number or a value out of range
// is a problem, and its def is left out.
func DecodeBackstories(defs []Def) (backstory.Book, []Problem) {
	var list []backstory.Def
	var keys []string
	var probs []Problem
	for _, d := range defs {
		bd := bsDecoder{def: d}
		v := bd.decode()
		if len(bd.probs) > 0 {
			probs = append(probs, bd.probs...)
			continue
		}
		list, keys = append(list, v), append(keys, d.Key)
	}
	return backstory.NewBook(list, keys), probs
}

// bsDecoder types one def, collecting its problems.
type bsDecoder struct {
	def   Def
	probs []Problem
}

func (bd *bsDecoder) fail(field, format string, args ...any) {
	bd.probs = append(bd.probs, Problem{Plugin: bd.def.Plugin, File: bd.def.File, Def: bd.def.Ref(),
		Field: field, Msg: fmt.Sprintf(format, args...)})
}

func (bd *bsDecoder) decode() backstory.Def {
	var d backstory.Def
	if bd.def.Class != BackstoryClass {
		bd.fail("", "not a %s def", BackstoryClass)
		return d
	}
	body := bd.def.Body
	fields := make([]string, 0, len(body))
	for k := range body {
		fields = append(fields, k)
	}
	slices.Sort(fields) // problems in a stable order
	for _, k := range fields {
		v := body[k]
		switch k {
		case "slot":
			d.Slot = bd.slot(v)
		case "title":
			d.Title = bd.text(k, v, 1, bsMaxTitle)
		case "short":
			d.Short = bd.text(k, v, 1, bsMaxShort)
		case "desc":
			d.Desc = bd.text(k, v, 0, bsMaxDesc)
		case "skills":
			d.Skills = bd.skills(v)
		case "incapable":
			d.Incapable = backstory.Bits(bd.keys(k, v, func(s string) (uint64, bool) {
				b, ok := backstory.BitOfKey(s)
				return uint64(b), ok
			}))
		case "weight":
			d.Weight = bd.num(k, v, 0, bsMaxWeight)
		case "tags":
			d.Tags = backstory.Tag(bd.keys(k, v, func(s string) (uint64, bool) {
				t, ok := backstory.TagOfKey(s)
				return uint64(t), ok
			}))
		default:
			bd.fail(k, "unknown field")
		}
	}
	for _, k := range []string{"slot", "title", "short"} {
		if _, ok := body[k]; !ok {
			bd.fail(k, "required")
		}
	}
	return d
}

func (bd *bsDecoder) slot(v any) backstory.Slot {
	s, ok := v.(string)
	if !ok {
		bd.fail("slot", "want a string, got %s", bsJSONType(v))
		return 0
	}
	for i, name := range bsSlots {
		if s == name {
			return backstory.Slot(i)
		}
	}
	bd.fail("slot", "%q is not childhood or adulthood", s)
	return 0
}

func (bd *bsDecoder) text(field string, v any, lo, hi int) string {
	s, ok := v.(string)
	if !ok {
		bd.fail(field, "want a string, got %s", bsJSONType(v))
		return ""
	}
	if n := utf8.RuneCountInString(s); n < lo || n > hi {
		bd.fail(field, "%d characters, want %d to %d", n, lo, hi)
	}
	return s
}

func (bd *bsDecoder) num(field string, v any, lo, hi int) int {
	f, ok := v.(float64)
	switch {
	case !ok:
		bd.fail(field, "want a number, got %s", bsJSONType(v))
	case math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f):
		bd.fail(field, "%v is not a whole number", f)
	case f < float64(lo) || f > float64(hi):
		bd.fail(field, "%v is out of range %d to %d", f, lo, hi)
	default:
		return int(f)
	}
	return 0
}

func (bd *bsDecoder) skills(v any) map[skill.ID]int8 {
	m, ok := v.(map[string]any)
	if !ok {
		bd.fail("skills", "want an object, got %s", bsJSONType(v))
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	out := make(map[skill.ID]int8, len(m))
	for k, x := range m {
		id, ok := skill.Parse(k)
		if !ok {
			bd.fail("skills."+k, "unknown skill")
			continue
		}
		out[id] = int8(bd.num("skills."+k, x, -bsMaxSkill, bsMaxSkill))
	}
	return out
}

// keys ORs the bits a list of keys names.
func (bd *bsDecoder) keys(field string, v any, of func(string) (uint64, bool)) uint64 {
	l, ok := v.([]any)
	if !ok {
		bd.fail(field, "want a list, got %s", bsJSONType(v))
		return 0
	}
	var out uint64
	for i, x := range l {
		s, ok := x.(string)
		if !ok {
			bd.fail(fmt.Sprintf("%s.%d", field, i), "want a string, got %s", bsJSONType(x))
			continue
		}
		b, ok := of(s)
		if !ok {
			bd.fail(fmt.Sprintf("%s.%d", field, i), "unknown key %q", s)
			continue
		}
		out |= b
	}
	return out
}

// bsJSONType names a tree value's JSON type for messages.
func bsJSONType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("%T", v)
}
