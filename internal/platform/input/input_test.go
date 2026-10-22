package input_test

import (
	"bytes"
	"testing"

	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

func TestDefaultsHaveNoConflicts(t *testing.T) {
	t.Parallel()
	if cs := input.Conflicts(input.Defaults()); len(cs) != 0 {
		t.Fatalf("default conflicts: %+v", cs)
	}
}

func TestDefsAreWellFormed(t *testing.T) {
	t.Parallel()
	seen := map[input.Action]bool{}
	for _, d := range input.Defs() {
		if seen[d.ID] {
			t.Fatalf("duplicate action %s", d.ID)
		}
		seen[d.ID] = true
		if d.Label == "" || d.Category == "" {
			t.Fatalf("%s: missing label or category", d.ID)
		}
		if len(d.Defaults) == 0 {
			t.Fatalf("%s: no default key", d.ID)
		}
		for _, k := range d.Defaults {
			if (d.Kind == input.Modifier) != input.IsModifier(k.Key) {
				t.Fatalf("%s: modifier kind and key %q disagree", d.ID, k.Key)
			}
		}
	}
}

func TestConflictSameContextAndGlobal(t *testing.T) {
	t.Parallel()
	b := input.Defaults()
	b[input.Forbid] = []input.Binding{{Key: "R"}} // same context as Draft
	if cs := input.Conflicts(b); len(cs) != 1 || cs[0].A != input.Draft || cs[0].B != input.Forbid {
		t.Fatalf("map conflict = %+v", cs)
	}
	b = input.Defaults()
	b[input.RotateLeft] = []input.Binding{{Key: "Space"}} // tool key on the global pause
	if cs := input.Conflicts(b); len(cs) != 1 {
		t.Fatalf("global conflict = %+v", cs)
	}
	b = input.Defaults()
	b[input.RotateLeft] = []input.Binding{{Key: "R"}} // tool vs map: separate layers
	if cs := input.Conflicts(b); len(cs) != 0 {
		t.Fatalf("tool and map may share a key, got %+v", cs)
	}
	b = input.Defaults()
	b[input.Draft] = []input.Binding{{Key: "W", Mods: input.ModShift}} // holds ignore mods
	if cs := input.Conflicts(b); len(cs) != 1 || cs[0].A != input.CamUp {
		t.Fatalf("hold conflict = %+v", cs)
	}
}

func frame(mods input.Mod, pressed ...string) input.Frame {
	return input.Frame{Pressed: pressed, Held: pressed, Mods: mods}
}

func TestResolveContextOrderAndSingleUse(t *testing.T) {
	t.Parallel()
	b := input.Defaults()
	b[input.RotateLeft] = []input.Binding{{Key: "R"}}
	full := []input.Context{input.CtxTool, input.CtxMap, input.CtxGlobal}
	r := input.Resolve(frame(0, "R"), b, full)
	if !r.Fired(input.RotateLeft) || r.Fired(input.Draft) {
		t.Fatal("the tool context above the map takes R")
	}
	r = input.Resolve(frame(0, "R"), b, full[1:])
	if !r.Fired(input.Draft) {
		t.Fatal("without a tool R drafts")
	}
	r = input.Resolve(frame(input.ModShift, "R"), input.Defaults(), full[1:])
	if !r.Fired(input.RoofOverlay) || r.Fired(input.Draft) {
		t.Fatal("Shift+R is the roof overlay, not draft")
	}
	r = input.Resolve(frame(0, "R"), input.Defaults(), []input.Context{input.CtxText, input.CtxMap})
	if r.Fired(input.Draft) || len(r.FiredList()) != 0 {
		t.Fatal("a text field swallows every key")
	}
	r = input.Resolve(frame(0, "R"), input.Defaults(), []input.Context{input.CtxModal, input.CtxGlobal})
	if r.Fired(input.Draft) {
		t.Fatal("a map key is inactive under a modal")
	}
	r = input.Resolve(frame(0, "Digit4"), input.Defaults(), full)
	if r.Fired(input.DevUltrafast) {
		t.Fatal("dev keys need the dev context")
	}
}

func TestResolveHoldsAndModifiers(t *testing.T) {
	t.Parallel()
	f := input.Frame{Held: []string{"W"}, Mods: input.ModShift | input.ModAlt}
	r := input.Resolve(f, input.Defaults(), []input.Context{input.CtxMap, input.CtxGlobal})
	if !r.Held(input.CamUp) || !r.Held(input.CamFast) || !r.Held(input.Detail) || !r.Held(input.Queue) {
		t.Fatal("held pan, fast pan, detail and queue should all be active")
	}
	if r.Held(input.CamDown) || r.Held(input.Constrain) {
		t.Fatal("unheld keys and inactive contexts stay off")
	}
}

func TestBindingStringsRoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"R", "Shift+R", "Ctrl+Shift+F1", "Ctrl+Shift+Alt+Comma"} {
		b, ok := input.ParseBinding(s)
		if !ok || b.String() != s {
			t.Fatalf("%q -> %+v %v -> %q", s, b, ok, b.String())
		}
	}
	for _, s := range []string{"", "Shift+", "Hyper+R", "Ctrl+Shift"} {
		if _, ok := input.ParseBinding(s); ok {
			t.Fatalf("%q should not parse", s)
		}
	}
	if l := (input.Binding{Key: "Comma"}).Label(); l != "," {
		t.Fatalf("comma label %q", l)
	}
	if l := (input.Binding{Key: "Digit1", Mods: input.ModShift}).Label(); l != "Shift+1" {
		t.Fatalf("shift digit label %q", l)
	}
}

func TestRebindRoundTripsThroughSettingsJSON(t *testing.T) {
	t.Parallel()
	b := input.Defaults().Set(input.Draft, 0, input.Binding{Key: "Y", Mods: input.ModCtrl})
	b = b.Set(input.Forbid, 0, input.Binding{}) // cleared
	s := config.Defaults()
	s.Controls.Bindings = input.ToSettings(b)
	var buf bytes.Buffer
	if err := config.Encode(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := config.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	back := input.FromSettings(got.Controls.Bindings)
	if l := back.Label(input.Draft); l != "Ctrl+Y" {
		t.Fatalf("draft = %q after the round trip", l)
	}
	if len(back.Of(input.Forbid)) != 0 {
		t.Fatalf("forbid should stay unbound, got %v", back.Of(input.Forbid))
	}
	if back.Label(input.Pause) != "Space" {
		t.Fatal("untouched actions keep their defaults")
	}
	if len(got.Controls.Bindings) != 2 {
		t.Fatalf("only overrides are stored, got %v", got.Controls.Bindings)
	}
}

func TestNormalizeDropsUnknownAndDuplicates(t *testing.T) {
	t.Parallel()
	m := input.Normalize(map[string][]string{
		"no.such.action":    {"K"},
		string(input.Draft): {"Y", "Y", "Hyper+Q", "Shift"},
		string(input.Pause): {"Space"}, // equal to the default
		string(input.Queue): {"Ctrl"},  // fixed modifier
	})
	if len(m) != 1 || len(m[string(input.Draft)]) != 1 || m[string(input.Draft)][0] != "Y" {
		t.Fatalf("normalized = %v", m)
	}
}

func TestSwapTradesKeys(t *testing.T) {
	t.Parallel()
	b := input.Defaults()
	k := input.Binding{Key: "R"}
	if cs := b.ConflictsWith(input.Forbid, k); len(cs) != 1 || cs[0] != input.Draft {
		t.Fatalf("conflicts = %v", cs)
	}
	b = b.Swap(input.Forbid, 0, k)
	if b.Label(input.Forbid) != "R" || b.Label(input.Draft) != "F" {
		t.Fatalf("after swap forbid=%q draft=%q", b.Label(input.Forbid), b.Label(input.Draft))
	}
	if cs := input.Conflicts(b); len(cs) != 0 {
		t.Fatalf("swap left conflicts: %+v", cs)
	}
}
