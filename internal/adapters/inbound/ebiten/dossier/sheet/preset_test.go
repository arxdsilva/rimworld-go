package sheet

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/platform/config"
)

func TestBuiltInsAreSoundAndNamed(t *testing.T) {
	t.Parallel()
	names := BuiltInNames()
	ps := BuiltIns()
	if len(ps) != len(names) || len(ps) != 4 {
		t.Fatalf("%d built-ins", len(ps))
	}
	for i, p := range ps {
		if p.Name != names[i] {
			t.Errorf("%d: %q", i, p.Name)
		}
		for _, n := range []*config.HUDNode{p.Docked, p.Floating} {
			if n == nil || len(Validate(*n)) != 0 {
				t.Errorf("%s: bad tree", p.Name)
			}
		}
		if p.Text == nil || p.DockH == 0 || p.FloatW == 0 {
			t.Errorf("%s: no sizes", p.Name)
		}
		if _, ok := BuiltIn(strings.ToUpper(p.Name)); !ok {
			t.Errorf("%s not found by name", p.Name)
		}
		if err := ValidName(p.Name); err == nil {
			t.Errorf("%s: a player may not take a built-in's name", p.Name)
		}
		// each one survives its own file
		data, err := EncodePreset(p)
		if err != nil {
			t.Fatal(err)
		}
		back, err := DecodePreset(data)
		if err != nil || !reflect.DeepEqual(back, p) {
			t.Errorf("%s round trip: %v", p.Name, err)
		}
	}
	// the text differs where it should
	roomy, _ := BuiltIn("Roomy")
	snug, _ := BuiltIn("Snug")
	if roomy.Text.BodySize <= snug.Text.BodySize {
		t.Error("Roomy is larger than Snug")
	}
	if std, _ := BuiltIn("Standard"); *std.Text != config.Defaults().HUD.Text {
		t.Error("Standard is the default text")
	}
	withT, _ := BuiltIn("Snug with traits")
	if !hasKind(*withT.Docked, "traits") || hasKind(*snug.Docked, "traits") {
		t.Error("only Snug with traits adds the traits row to the pane")
	}
}

func hasKind(n config.HUDNode, key string) bool {
	if n.Kind == key {
		return true
	}
	for _, k := range n.Kids {
		if hasKind(k, key) {
			return true
		}
	}
	return false
}

func TestStandardGolden(t *testing.T) {
	t.Parallel()
	std, _ := BuiltIn("Standard")
	data, err := EncodePreset(std)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "standard.preset.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("the Standard preset's file changed; if intended run with UPDATE_GOLDEN=1 and bump PresetVersion when the format breaks")
	}
	// and the golden file loads
	if _, err := DecodePreset(want); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data string
		want error
	}{
		{"empty", ``, ErrPresetInvalid},
		{"not json", `{{`, ErrPresetInvalid},
		{"array", `[]`, ErrPresetInvalid},
		{"no version", `{"name":"x"}`, ErrPresetInvalid},
		{"newer", `{"version":99,"name":"x"}`, ErrPresetNewer},
		{"unknown element", `{"version":1,"docked":{"k":"column","c":[{"k":"zorp"}]}}`, ErrPresetInvalid},
		{"block of widgets", `{"version":1,"docked":{"k":"block","c":[{"k":"doing"}]}}`, ErrPresetInvalid},
		{"huge", `{"version":1,"name":"` + strings.Repeat("a", MaxPresetBytes) + `"}`, ErrPresetTooBig},
		{"name too long", `{"version":1,"name":"` + strings.Repeat("a", 400) + `"}`, ErrPresetInvalid},
	}
	for _, c := range cases {
		if _, err := DecodePreset([]byte(c.data)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestDecodeClampsSizes(t *testing.T) {
	t.Parallel()
	p, err := DecodePreset([]byte(`{"version":1,"name":"x","dock_h":5,"dock_tabs":99,"float_w":99999,"dock_w":1,
		"text":{"body_size":99,"label_w":1,"small_delta":-50}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.DockH != config.HUDPaneMinH || p.DockTabs != 12 || p.FloatW != 1000 || p.DockW != 72 {
		t.Errorf("sizes %+v", p)
	}
	if p.Text.BodySize != 20 || p.Text.LabelW != 30 || p.Text.SmallDelta != -5 || p.Text.BodyLinePct != config.HUDLineMinPct {
		t.Errorf("text %+v", p.Text)
	}
	// a file from before the compact pane: its tall pane comes down to the most
	tall, err := DecodePreset([]byte(`{"version":1,"name":"x","dock_h":480}`))
	if err != nil || tall.DockH != config.HUDPaneMaxH {
		t.Errorf("a 480 pane loads as %d (%v), want %d", tall.DockH, err, config.HUDPaneMaxH)
	}
	if IsOlder(p) {
		t.Error("current version is not older")
	}
	if !IsOlder(Preset{Version: 0}) {
		t.Error("version 0 is older")
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	good := map[string]string{"My card": "my-card", "tall_2": "tall_2", " Pane 3 ": "pane-3", "A-B": "a-b"}
	for name, slug := range good {
		if err := ValidName(name); err != nil {
			t.Errorf("%q: %v", name, err)
		}
		if got := Slug(name); got != slug {
			t.Errorf("slug of %q is %q, want %q", name, got, slug)
		}
	}
	for _, name := range []string{"", "  ", "../x", "a/b", "a.b", "naïve", strings.Repeat("a", MaxPresetName+1), "---", "standard", "Snug"} {
		if err := ValidName(name); !errors.Is(err, ErrPresetName) {
			t.Errorf("%q should be refused: %v", name, err)
		}
	}
	if Slug("../../etc") != "etc" || strings.ContainsAny(Slug("a/b\\c:d"), `/\:.`) {
		t.Error("a slug can never leave the folder")
	}
}

func TestCaptureAndApply(t *testing.T) {
	t.Parallel()
	h := config.Defaults().HUD
	custom := box(KindColumn, widget("doing"))
	h.Layouts.Docked = &custom
	h.Pane.Height, h.Pane.MinTabs, h.Pane.TabW, h.Card.W, h.Card.H = 400, 8, 90, 333, 444
	h.Text.BodySize = 15

	p := Capture(h, " Mine ", SaveFlags{Docked: true, Height: true})
	if p.Name != "Mine" || p.Docked == nil || p.Floating != nil || p.DockH != 400 || p.FloatH != 444 || p.FloatW != 0 || p.Text != nil || p.DockTabs != 0 {
		t.Fatalf("flags pick the parts: %+v", p)
	}
	all := Capture(h, "All", SaveFlags{true, true, true, true, true, true})
	if all.Floating == nil || all.DockW != 90 || all.DockTabs != 8 || all.FloatW != 333 || all.Text.BodySize != 15 {
		t.Fatalf("all flags: %+v", all)
	}
	if all.Floating == nil || !reflect.DeepEqual(*all.Floating, DefaultFloating()) {
		t.Fatal("an unedited tree is captured as the built-in one")
	}

	fresh := config.Defaults().HUD
	Apply(all, &fresh)
	if fresh.Layouts.Docked == nil || !reflect.DeepEqual(*fresh.Layouts.Docked, custom) {
		t.Fatal("docked layout")
	}
	if fresh.Layouts.Floating != nil {
		t.Fatal("a tree equal to the built-in is stored as nil")
	}
	if fresh.Pane.Height != 400 || fresh.Pane.MinTabs != 8 || fresh.Pane.TabW != 90 || fresh.Card.W != 333 || fresh.Card.H != 444 ||
		fresh.Text.BodySize != 15 || fresh.Preset != "All" {
		t.Fatalf("applied: %+v", fresh)
	}

	partial := config.Defaults().HUD
	partial.Card.W = 222
	Apply(Preset{Name: "Tall", DockH: 500}, &partial)
	if partial.Card.W != 222 || partial.Pane.Height != 500 || partial.Layouts.Docked != nil {
		t.Fatalf("what a preset lacks stays: %+v", partial)
	}
	std, _ := BuiltIn("Standard")
	Apply(std, &fresh)
	if fresh.Layouts.Docked != nil || fresh.Preset != "Standard" {
		t.Fatal("Standard clears the edited layouts")
	}
}

func FuzzPresetDecode(f *testing.F) {
	for _, p := range BuiltIns() {
		data, _ := EncodePreset(p)
		f.Add(data)
	}
	f.Add([]byte(`{"version":1}`))
	f.Add([]byte(`{"version":1,"docked":{"k":"column","c":[{"k":"skill","def":"melee","who":"cw"}]}}`))
	f.Add([]byte(`{"version":2}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := DecodePreset(data)
		if err != nil {
			return
		}
		for _, n := range []*config.HUDNode{p.Docked, p.Floating} {
			if n == nil {
				continue
			}
			if ps := Validate(*n); len(ps) != 0 {
				t.Fatalf("accepted a tree with problems: %v", ps)
			}
			// pruning and measuring any accepted tree must be safe
			m := MetricsFor(config.Defaults().HUD.Text)
			for w := WhoColonist; w <= WhoWild; w++ {
				_ = Measure(Prune(*n, w), m, 100)
			}
		}
		out, err := EncodePreset(p)
		if err != nil {
			t.Fatal(err)
		}
		back, err := DecodePreset(out)
		if err != nil {
			t.Fatalf("own output refused: %v", err)
		}
		p.Version = PresetVersion
		if !reflect.DeepEqual(back, p) {
			t.Fatalf("round trip changed it:\n%+v\n%+v", p, back)
		}
	})
}
