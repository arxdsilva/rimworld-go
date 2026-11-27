// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pluginapi

import (
	"reflect"
	"testing"
)

// The capabilities are a closed enum of thirteen distinct names in
// canonical order; the zero value has no name and is in no set.
func TestCapabilityNames(t *testing.T) {
	want := []string{"query", "commands", "events", "world", "defs", "validate", "ui",
		"incidents", "jobs", "stats", "health", "recipes", "worldgen"}
	for i, n := range want {
		if c := Capability(i + 1); c.String() != n {
			t.Errorf("Capability(%d) = %q, want %q", i+1, c.String(), n)
		}
	}
	if Worldgen != Capability(len(want)) {
		t.Errorf("Worldgen = %d, want the last of %d", Worldgen, len(want))
	}
	if Capability(0).String() != "" || Capability(len(want)+1).String() != "" {
		t.Error("a value outside the enum has a name")
	}
	if Set(^uint32(0)).Has(0) || Set(^uint32(0)).Has(Worldgen+1) {
		t.Error("a full set has a value outside the enum")
	}
}

// Legacy is query, commands and events, nothing else.
func TestLegacy(t *testing.T) {
	l := Legacy()
	for c := Query; c <= Worldgen; c++ {
		if got, want := l.Has(c), c == Query || c == Commands || c == Events; got != want {
			t.Errorf("Legacy().Has(%s) = %v, want %v", c, got, want)
		}
	}
	if Version != 1 || MinVersion != 1 {
		t.Errorf("Version, MinVersion = %d, %d, want 1, 1", Version, MinVersion)
	}
}

func TestParse(t *testing.T) {
	s, unknown, dup := Parse([]string{"ui", "query", "ui", "bogus", "bogus", "Query", "query"})
	if want := []string{"query", "ui"}; !reflect.DeepEqual(s.Names(), want) {
		t.Errorf("Names = %v, want %v", s.Names(), want)
	}
	if !reflect.DeepEqual(unknown, []string{"bogus", "Query"}) || !reflect.DeepEqual(dup, []string{"ui", "query"}) {
		t.Errorf("unknown %v dup %v", unknown, dup)
	}
	if s, u, d := Parse(nil); s != 0 || u != nil || d != nil {
		t.Errorf("Parse(nil) = %v %v %v", s, u, d)
	}
	if got := Legacy().Names(); !reflect.DeepEqual(got, []string{"query", "commands", "events"}) {
		t.Errorf("Legacy names = %v", got)
	}
	if Set(0).Names() != nil {
		t.Error("empty set has names")
	}
}

// Parse never panics, never invents a capability, and Names round-trips
// through Parse into the same set in canonical order.
func FuzzParseCaps(f *testing.F) {
	f.Add("query,ui,ui,zzz")
	f.Add("")
	f.Add("worldgen,query")
	f.Fuzz(func(t *testing.T, in string) {
		var names []string
		start := 0
		for i := 0; i <= len(in); i++ {
			if i == len(in) || in[i] == ',' {
				names = append(names, in[start:i])
				start = i + 1
			}
		}
		s, unknown, _ := Parse(names)
		for _, u := range unknown {
			if _, ok := byName(u); ok {
				t.Fatalf("known name %q reported unknown", u)
			}
		}
		out := s.Names()
		again, u2, d2 := Parse(out)
		if again != s || len(u2) != 0 || len(d2) != 0 {
			t.Fatalf("not canonical: %v -> %v", out, again.Names())
		}
		for i := 1; i < len(out); i++ {
			a, _ := byName(out[i-1])
			b, _ := byName(out[i])
			if a >= b {
				t.Fatalf("names out of order: %v", out)
			}
		}
	})
}
