package ebiten

// journey_wave2_helpers_test.go: fixtures for the STO-2/STO-3/RES-1/CBT-1/
// CBT-2 journeys (journey_wave2_test.go). Read-model fields and controls
// the parallel agents land are looked up at run time (reflection, id
// substrings) so a missing one fails the journey with a message listing
// what is there instead of breaking the package build.

import (
	"reflect"
	"sort"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// w2Find is the first on-screen id holding every substring in inc and none
// in exc (lower case compare); "" when none does.
func w2Find(j *journey, inc []string, exc ...string) string {
	j.t.Helper()
	for _, id := range wbIDs(j) {
		low := strings.ToLower(id)
		ok := true
		for _, s := range inc {
			ok = ok && strings.Contains(low, s)
		}
		for _, s := range exc {
			ok = ok && !strings.Contains(low, s)
		}
		if ok {
			return id
		}
	}
	return ""
}

// w2Click clicks the on-screen control holding every substring of inc and
// none of exc; fatal, listing the cmd/storage/zone ids on screen, when
// there is none or it is disabled.
func w2Click(j *journey, what string, inc []string, exc ...string) {
	j.t.Helper()
	id := w2Find(j, inc, exc...)
	if id == "" {
		var have []string
		for _, s := range wbIDs(j) {
			l := strings.ToLower(s)
			if strings.HasPrefix(l, "cmd.") || strings.Contains(l, "storage") || strings.Contains(l, "zone") || strings.Contains(l, "inspector") {
				have = append(have, s)
			}
		}
		j.t.Fatalf("%s: no control with ids containing %q (excluding %q); candidate ids on screen: %q", what, inc, exc, have)
	}
	if !j.clickID(id) {
		j.t.Fatalf("%s: %q is disabled", what, id)
	}
}

// w2Opt reads a field by reflection; ok is false when the type has none.
func w2Opt(v any, name string) (reflect.Value, bool) {
	f := reflect.Indirect(reflect.ValueOf(v)).FieldByName(name)
	return f, f.IsValid()
}

// w2Bool reads a bool field; fatal when the field is missing.
func w2Bool(j *journey, v any, name string) bool {
	j.t.Helper()
	f := wcField(j, v, name)
	if f.Kind() != reflect.Bool {
		j.t.Fatalf("%T.%s is %s, not bool", v, name, f.Kind())
	}
	return f.Bool()
}

// w2Zone is the zone view with that id.
func w2Zone(j *journey, id uint16) (game.ZoneView, bool) {
	for _, z := range j.world().Zones {
		if z.ID == id {
			return z, true
		}
	}
	return game.ZoneView{}, false
}

// w2ShelfZone is the storage zone view of the shelf building b.
func w2ShelfZone(j *journey, b entity.EntityID) game.ZoneView {
	j.t.Helper()
	var kinds []string
	for _, z := range j.world().Zones {
		if z.Kind == game.ZoneShelf && z.Shelf == uint64(b) {
			return z
		}
		kinds = append(kinds, z.Kind)
	}
	j.t.Fatalf("no shelf zone for building %d; zone kinds %q", b, kinds)
	return game.ZoneView{}
}

// w2Item is the item view with that id.
func w2Item(j *journey, id entity.EntityID) (game.ItemView, bool) {
	for _, it := range j.world().Items {
		if it.ID == id {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// w2ForbidWood switches the wood category off in the open Storage window
// (opening it first) and returns the command's name.
func w2ForbidWood(j *journey) {
	j.t.Helper()
	if wbFind(j, "storage.filter.") == "" {
		w2Click(j, "storage settings button", []string{"cmd.", "storage"})
	}
	if wbFind(j, "storage.filter.", "wood") == "" { // Wood sits under Raw resources
		j.clickID(weButton(j, "raw resources expander", "storage.expand.", "raw resources"))
	}
	j.clickID(weButton(j, "wood filter row", "storage.filter.", "wood"))
}

// w2KeyFor is the first default binding of the input action whose id holds
// every substring; fatal listing the zone/overlay ids when none does.
func w2KeyFor(j *journey, subs ...string) input.Binding {
	j.t.Helper()
	var near []string
	for _, d := range input.Defs() {
		id := strings.ToLower(string(d.ID))
		ok := true
		for _, s := range subs {
			ok = ok && strings.Contains(id, s)
		}
		if ok && len(d.Defaults) > 0 {
			return d.Defaults[0]
		}
		if strings.Contains(id, "zone") || strings.Contains(id, "overlay") {
			near = append(near, string(d.ID))
		}
	}
	sort.Strings(near)
	j.t.Fatalf("no input action with id containing %q (U1 has not landed it); zone/overlay actions: %q", subs, near)
	return input.Binding{}
}
