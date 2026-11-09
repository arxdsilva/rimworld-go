package contentid

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"
)

func TestFingerprintIsStable(t *testing.T) {
	t.Parallel()
	a, b := Fingerprint(), Fingerprint()
	if a == 0 || a != b {
		t.Fatalf("fingerprints %x, %x: want the same non-zero value", a, b)
	}
	if c := Current(); c.Hash != a || !reflect.DeepEqual(c.Plugins, []string{Core}) {
		t.Fatalf("current %+v", c)
	}
	for _, l := range registries() {
		if len(l.items) == 0 {
			t.Errorf("registry %s is empty", l.name)
		}
		seen := map[string]bool{}
		for _, it := range l.items {
			if it.key == "" || seen[it.key] {
				t.Errorf("registry %s: key %q is empty or repeated", l.name, it.key)
			}
			seen[it.key] = true
		}
	}
}

func TestFingerprintOfFollowsTheContent(t *testing.T) {
	t.Parallel()
	base := []list{
		{"a", []item{{"x", "1"}, {"y", "2"}}},
		{"b", []item{{"z", "3"}}},
	}
	h := fingerprintOf(base)
	reordered := []list{
		{"a", []item{{"y", "2"}, {"x", "1"}}},
		{"b", []item{{"z", "3"}}},
	}
	if fingerprintOf(reordered) != h {
		t.Fatal("a registry's order changed the fingerprint")
	}
	for name, ls := range map[string][]list{
		"a value":        {{"a", []item{{"x", "1"}, {"y", "9"}}}, {"b", []item{{"z", "3"}}}},
		"a key":          {{"a", []item{{"x", "1"}, {"w", "2"}}}, {"b", []item{{"z", "3"}}}},
		"a def added":    {{"a", []item{{"x", "1"}, {"y", "2"}}}, {"b", []item{{"z", "3"}, {"q", "4"}}}},
		"a def removed":  {{"a", []item{{"x", "1"}}}, {"b", []item{{"z", "3"}}}},
		"registry order": {{"b", []item{{"z", "3"}}}, {"a", []item{{"x", "1"}, {"y", "2"}}}},
		"run together":   {{"a", []item{{"x", "12"}, {"y", ""}}}, {"b", []item{{"z", "3"}}}},
	} {
		if fingerprintOf(ls) == h {
			t.Errorf("%s did not change the fingerprint", name)
		}
	}
}

// %+v prints a definition the same way in every run only if nothing in it
// prints an address: no funcs, pointers, interfaces or channels (maps
// print in key order).
func TestDefsPrintDeterministically(t *testing.T) {
	t.Parallel()
	for _, ty := range []reflect.Type{
		reflect.TypeFor[research.Def](), reflect.TypeFor[recipe.Def](), reflect.TypeFor[construction.Def](),
		reflect.TypeFor[biome.Def](), reflect.TypeFor[faction.Def](), reflect.TypeFor[incident.Def](),
		reflect.TypeFor[trait.Def](), reflect.TypeFor[drug.Def](), reflect.TypeFor[backstory.Def](),
		reflect.TypeFor[quest.Def](), reflect.TypeFor[animalDef](),
	} {
		var bad []string
		addressFree(ty, ty.String(), map[reflect.Type]bool{}, &bad)
		if len(bad) > 0 {
			t.Errorf("%s holds %v: fingerprint it field by field instead", ty, bad)
		}
	}
}

func addressFree(t reflect.Type, path string, seen map[reflect.Type]bool, bad *[]string) {
	if seen[t] {
		return
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Interface, reflect.Pointer:
		*bad = append(*bad, path+" "+t.Kind().String())
	case reflect.Slice, reflect.Array:
		addressFree(t.Elem(), path+"[]", seen, bad)
	case reflect.Map:
		addressFree(t.Key(), path+"{key}", seen, bad)
		addressFree(t.Elem(), path+"{}", seen, bad)
	case reflect.Struct:
		for i := range t.NumField() {
			addressFree(t.Field(i).Type, path+"."+t.Field(i).Name, seen, bad)
		}
	}
}
