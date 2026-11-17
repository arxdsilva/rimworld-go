package modding

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
)

// bsJSON sends defs through encoding/json and back, as a defs file would
// reach Decode.
func bsJSON(t *testing.T, defs []Def) []Def {
	t.Helper()
	out := make([]Def, len(defs))
	for i, d := range defs {
		raw, err := json.Marshal(d.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		d.Body = body
		out[i] = d
	}
	return out
}

func bsSame(t *testing.T, got, want backstory.Book) {
	t.Helper()
	if got.Len() != want.Len() {
		t.Fatalf("%d backstories, want %d", got.Len(), want.Len())
	}
	for i := 0; i <= want.Len()+1; i++ {
		id := backstory.ID(i)
		if g, w := got.DefOf(id), want.DefOf(id); !reflect.DeepEqual(g, w) {
			t.Fatalf("id %d: %+v, want %+v", id, g, w)
		}
		if g, w := got.Key(id), want.Key(id); g != w {
			t.Fatalf("id %d: key %q, want %q", id, g, w)
		}
	}
}

// TestBackstoryCodecRoundTripsCore: decoding core's encoded defs (straight,
// and through JSON) gives core back, def for def and key for key.
func TestBackstoryCodecRoundTripsCore(t *testing.T) {
	core := backstory.Core()
	defs := EncodeBackstories(core)
	if len(defs) != core.Len() {
		t.Fatalf("%d defs for %d backstories", len(defs), core.Len())
	}
	for i, d := range defs {
		if d.Class != BackstoryClass || d.Plugin != Core || d.Key != core.Key(backstory.ID(i+1)) {
			t.Fatalf("def %d: %s by %q", i, d.Ref(), d.Plugin)
		}
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeBackstories(in)
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		bsSame(t, got, core)
	}
}

// TestBackstoryEncodeOmitsEmpty: no skills, incapabilities or tags means
// no such fields; they decode back to the zero values.
func TestBackstoryEncodeOmitsEmpty(t *testing.T) {
	b := backstory.NewBook([]backstory.Def{{Slot: backstory.Adulthood, Title: "Plain", Short: "plain"}}, []string{"p.plain"})
	defs := EncodeBackstories(b)
	if len(defs) != 1 || defs[0].Plugin != "p" {
		t.Fatalf("defs %+v", defs)
	}
	for _, k := range []string{"skills", "incapable", "tags"} {
		if _, ok := defs[0].Body[k]; ok {
			t.Fatalf("empty %s encoded", k)
		}
	}
	got, probs := DecodeBackstories(bsJSON(t, defs))
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	bsSame(t, got, b)
}

// TestBackstoryPluginDefFollowsCore: a plugin's backstory after core's
// takes ID Len()+1 under its own key.
func TestBackstoryPluginDefFollowsCore(t *testing.T) {
	defs := EncodeBackstories(backstory.Core())
	defs = append(defs, Def{Class: BackstoryClass, Key: "moremods.lamplighter", Plugin: "moremods", File: "defs/b.json",
		Body: map[string]any{"slot": "adulthood", "title": "Lamplighter", "short": "lamplighter",
			"desc": "{name} lit the lamps.", "skills": map[string]any{"construction": 3.0, "social": -1.0},
			"incapable": []any{"mining", "violence"}, "weight": 7.0, "tags": []any{"spacer", "industrial"}}})
	got, probs := DecodeBackstories(defs)
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := backstory.Core().Len()
	id := backstory.ID(n + 1)
	if got.Len() != n+1 || got.Key(id) != "moremods.lamplighter" {
		t.Fatalf("len %d key %q", got.Len(), got.Key(id))
	}
	d := got.DefOf(id)
	if d.ID != id || d.Title != "Lamplighter" || d.Weight != 7 || d.Slot != backstory.Adulthood ||
		d.Incapable != backstory.Violence|backstory.Mining || d.Tags != backstory.TagIndustrial|backstory.TagSpacer ||
		len(d.Skills) != 2 {
		t.Fatalf("plugin def %+v", d)
	}
	bsSame(t, backstory.NewBook(got.All()[:n], bsKeysOf(got, n)), backstory.Core())
}

func bsKeysOf(b backstory.Book, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = b.Key(backstory.ID(i + 1))
	}
	return out
}

// TestBackstoryDecodeProblems: every strictness rule names its field, and
// a def with any problem is left out while the rest load.
func TestBackstoryDecodeProblems(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{"slot": "childhood", "title": "Tinker", "short": "tinker", "desc": "{name} fixed things.",
			"skills": map[string]any{"crafting": 2.0}, "weight": 10.0}
	}
	long := func(n int) string { return strings.Repeat("é", n) }
	cases := []struct {
		name, field, msg string
		edit             func(map[string]any)
	}{
		{"unknown field", "colour", "unknown field", func(b map[string]any) { b["colour"] = "red" }},
		{"slot type", "slot", "want a string", func(b map[string]any) { b["slot"] = 1.0 }},
		{"slot value", "slot", "not childhood or adulthood", func(b map[string]any) { b["slot"] = "elderhood" }},
		{"slot missing", "slot", "required", func(b map[string]any) { delete(b, "slot") }},
		{"title missing", "title", "required", func(b map[string]any) { delete(b, "title") }},
		{"short missing", "short", "required", func(b map[string]any) { delete(b, "short") }},
		{"title type", "title", "want a string", func(b map[string]any) { b["title"] = []any{"x"} }},
		{"title empty", "title", "want 1 to 40", func(b map[string]any) { b["title"] = "" }},
		{"title long", "title", "want 1 to 40", func(b map[string]any) { b["title"] = long(41) }},
		{"short empty", "short", "want 1 to 20", func(b map[string]any) { b["short"] = "" }},
		{"short long", "short", "want 1 to 20", func(b map[string]any) { b["short"] = long(21) }},
		{"desc long", "desc", "want 0 to 800", func(b map[string]any) { b["desc"] = long(801) }},
		{"desc type", "desc", "want a string", func(b map[string]any) { b["desc"] = true }},
		{"weight type", "weight", "want a number", func(b map[string]any) { b["weight"] = "10" }},
		{"weight fraction", "weight", "not a whole number", func(b map[string]any) { b["weight"] = 2.5 }},
		{"weight low", "weight", "out of range", func(b map[string]any) { b["weight"] = -1.0 }},
		{"weight high", "weight", "out of range", func(b map[string]any) { b["weight"] = 1001.0 }},
		{"skills type", "skills", "want an object", func(b map[string]any) { b["skills"] = []any{} }},
		{"skill unknown", "skills.dancing", "unknown skill", func(b map[string]any) { b["skills"] = map[string]any{"dancing": 1.0} }},
		{"skill type", "skills.crafting", "want a number", func(b map[string]any) { b["skills"] = map[string]any{"crafting": nil} }},
		{"skill fraction", "skills.crafting", "not a whole number", func(b map[string]any) { b["skills"] = map[string]any{"crafting": 0.5} }},
		{"skill low", "skills.crafting", "out of range", func(b map[string]any) { b["skills"] = map[string]any{"crafting": -21.0} }},
		{"skill high", "skills.crafting", "out of range", func(b map[string]any) { b["skills"] = map[string]any{"crafting": 21.0} }},
		{"incapable type", "incapable", "want a list", func(b map[string]any) { b["incapable"] = "violence" }},
		{"incapable item", "incapable.1", "want a string", func(b map[string]any) { b["incapable"] = []any{"violence", 3.0} }},
		{"incapable unknown", "incapable.0", "unknown key", func(b map[string]any) { b["incapable"] = []any{"flying"} }},
		{"tags unknown", "tags.0", "unknown key", func(b map[string]any) { b["tags"] = []any{"medieval"} }},
		{"tags type", "tags", "want a list", func(b map[string]any) { b["tags"] = map[string]any{} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := valid()
			c.edit(bad)
			defs := []Def{
				{Class: BackstoryClass, Key: "p.ok", Plugin: "p", File: "defs/a.json", Body: valid()},
				{Class: BackstoryClass, Key: "p.bad", Plugin: "p", File: "defs/a.json", Body: bad},
			}
			got, probs := DecodeBackstories(defs)
			if len(probs) != 1 {
				t.Fatalf("problems %v", probs)
			}
			p := probs[0]
			if p.Plugin != "p" || p.File != "defs/a.json" || p.Def != "backstory/p.bad" || p.Field != c.field ||
				!strings.Contains(p.Msg, c.msg) || p.Warn {
				t.Fatalf("problem %+v, want field %q msg %q", p, c.field, c.msg)
			}
			if got.Len() != 1 || got.Key(1) != "p.ok" {
				t.Fatalf("book: %d defs, first %q", got.Len(), got.Key(1))
			}
		})
	}
}

// TestBackstoryRangeEdgesLoad: the range limits themselves are allowed.
func TestBackstoryRangeEdgesLoad(t *testing.T) {
	body := map[string]any{"slot": "adulthood", "title": strings.Repeat("é", 40), "short": strings.Repeat("é", 20),
		"desc": strings.Repeat("é", 800), "weight": 1000.0, "skills": map[string]any{"melee": -20.0, "shooting": 20.0}}
	got, probs := DecodeBackstories([]Def{{Class: BackstoryClass, Key: "p.edge", Plugin: "p", Body: body}})
	if len(probs) > 0 || got.Len() != 1 || got.DefOf(1).Weight != 1000 {
		t.Fatalf("problems %v, len %d", probs, got.Len())
	}
	body["weight"] = 0.0
	if _, probs := DecodeBackstories([]Def{{Class: BackstoryClass, Key: "p.edge", Plugin: "p", Body: body}}); len(probs) > 0 {
		t.Fatalf("weight 0: %v", probs)
	}
}

// TestBackstoryKeysRoundTrip: every bit and tag has a distinct key that
// reads back; nothing else has one.
func TestBackstoryKeysRoundTrip(t *testing.T) {
	seen := map[string]bool{}
	for bit := backstory.Bits(1); bit&backstory.AllBits != 0; bit <<= 1 {
		k := backstory.BitKey(bit)
		if b, ok := backstory.BitOfKey(k); k == "" || seen[k] || !ok || b != bit {
			t.Fatalf("bit %b: key %q reads back %b", bit, k, b)
		}
		seen[k] = true
	}
	for tag := backstory.Tag(1); tag&backstory.AllTags != 0; tag <<= 1 {
		k := backstory.TagKey(tag)
		if g, ok := backstory.TagOfKey(k); k == "" || seen[k] || !ok || g != tag {
			t.Fatalf("tag %b: key %q reads back %b", tag, k, g)
		}
		seen[k] = true
	}
	if backstory.BitKey(backstory.Violence|backstory.Caring) != "" || backstory.BitKey(backstory.AllBits+1) != "" ||
		backstory.TagKey(backstory.AllTags) != "" || backstory.TagKey(backstory.AllTags+1) != "" {
		t.Fatal("a key for a non-single bit or tag")
	}
}
