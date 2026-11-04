package ebiten

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier"
	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/platform/config"
)

// The setup window's Sections and Presets tabs, driven with the real mouse.

// memCards is a preset folder in memory.
type memCards struct{ files map[string][]byte }

func (m *memCards) List() (out []string, _ error) {
	for k := range m.files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
func (m *memCards) Load(n string) ([]byte, error) { return m.files[n], nil }
func (m *memCards) Save(n string, d []byte) error {
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[n] = d
	return nil
}
func (m *memCards) Delete(n string) error { delete(m.files, n); return nil }

// setupJourney is a journey with the card docked and the setup window open on
// a tab (0 Look, 1 Sections, 2 Presets).
func setupJourney(t *testing.T, tab int, st dossier.PresetStore) *journey {
	t.Helper()
	j := newJourney(t)
	if st != nil {
		j.a.cfg.CardFiles = CardFiles{Store: st, Open: func(string) error { return nil }, Folder: "/cards"}
		j.a.dos = newDossier(j.a)
	}
	dossierOn(j, true)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	if j.a.renderer.Selected() != p.ID {
		t.Fatalf("selected %d, want %d", j.a.renderer.Selected(), p.ID)
	}
	j.a.dos.setup.Open() // the gear shows only under the pointer; its press runs this
	j.update(2)
	if tab > 0 {
		if !j.clickID("dossier.setup.tabs." + strconv.Itoa(tab)) {
			t.Fatal("the tab is disabled")
		}
		j.update(2)
	}
	return j
}

// clickMenu picks the nth row of the list a "pick.<id>" menu shows.
func clickMenu(j *journey, id string, n int) {
	j.t.Helper()
	j.clickID("modal.pick." + id + ".list." + strconv.Itoa(n))
	j.update(2)
}

func TestJourneyDossierEditAddBar(t *testing.T) {
	j := setupJourney(t, 1, nil)
	before := j.a.settings.HUD
	if before.Layouts.Docked != nil {
		t.Fatal("the pane starts on the built-in layout")
	}
	j.a.settings.HUD.Preset = "Roomy"

	j.clickID("sec.add.bar")
	j.update(2)
	// the bars group: health, mood, food, rest, recreation, other need
	clickMenu(j, "sec.add", 4)
	clickMenu(j, "sec.style", 1) // only when low
	h := j.a.settings.HUD
	if h.Layouts.Docked == nil {
		t.Fatal("the edit did not reach the settings")
	}
	if h.Preset != "" {
		t.Fatalf("the preset %q stays after an edit", h.Preset)
	}
	var found *config.HUDNode
	var walk func(n *config.HUDNode)
	walk = func(n *config.HUDNode) {
		if n.Kind == "fun_bar" && n.BarStyle == sheet.BarLowOnly {
			found = n
		}
		for i := range n.Kids {
			walk(&n.Kids[i])
		}
	}
	walk(h.Layouts.Docked)
	if found == nil {
		t.Fatalf("no recreation bar in the stored tree")
	}
	if n0, _ := sheet.Count(sheet.DefaultDocked()); true {
		if n1, _ := sheet.Count(*h.Layouts.Docked); n1 != n0+1 {
			t.Fatalf("nodes %d -> %d", n0, n1)
		}
	}
	j.draw(2) // the card draws the new tree

	// back to the built-in layout
	if !j.clickID("sec.default") {
		t.Fatal("the built-in layout is not reachable")
	}
	if j.a.settings.HUD.Layouts.Docked != nil {
		t.Fatal("the built-in layout did not return")
	}
}

func TestJourneyDossierPresetSaveLoad(t *testing.T) {
	st := &memCards{}
	j := setupJourney(t, 2, st)
	j.a.settings.HUD.Text.BodySize = 11

	// keep it: name, then the button
	if j.clickID("pre.save") {
		t.Fatal("an unnamed look can be kept")
	}
	j.clickID("pre.name")
	j.typeText("Tight and tidy")
	j.key("Enter", 0)
	j.update(2)
	if !j.clickID("pre.save") {
		t.Fatal("a named look cannot be kept")
	}
	data, ok := st.files["tight-and-tidy"]
	if !ok {
		t.Fatalf("files %v", st.files)
	}
	p, err := sheet.DecodePreset(data)
	if err != nil || p.Name != "Tight and tidy" || p.Text == nil || p.Text.BodySize != 11 {
		t.Fatalf("kept %+v %v", p, err)
	}
	if got := j.a.settings.HUD.Preset; got != "Tight and tidy" {
		t.Fatalf("in use %q", got)
	}

	// change the card, then load the look back
	j.a.settings.HUD.Text.BodySize = 15
	j.a.settings.HUD.Preset = ""
	j.update(2)
	if !j.clickID("pre.load") { // the kept look is still the chosen row
		t.Fatal("cannot use a look")
	}
	if h := j.a.settings.HUD; h.Text.BodySize != 11 || h.Preset != "Tight and tidy" {
		t.Fatalf("loaded %+v", h.Text)
	}

	// a hostile file dropped in the folder shows when the tab opens again, and
	// using it sends the card back to the built-in layouts
	st.files["evil"] = []byte(`{"version":1,"name":"Evil","docked":{"k":"column","c":[{"k":"launch_missiles"}]}}`)
	j.clickID("dossier.setup.tabs.0")
	j.update(8)
	j.clickID("dossier.setup.tabs.2")
	j.update(2)
	tree := sheet.DefaultDocked()
	tree.Kids = append(tree.Kids, sheet.NewNode("traits"))
	j.a.settings.HUD.Layouts.Docked = &tree
	j.clickID("pre.list." + strconv.Itoa(len(sheet.BuiltIns()))) // "evil" sorts before "tight-and-tidy"
	if !j.clickID("pre.load") {
		t.Fatal("cannot use a look")
	}
	if !strings.Contains(j.a.hint, "built-in card is back") {
		t.Fatalf("hint %q", j.a.hint)
	}
	if h := j.a.settings.HUD; h.Layouts.Docked != nil || h.Preset != "" {
		t.Fatalf("the bad look left %+v / %q behind", h.Layouts, h.Preset)
	}
	j.draw(2)
}
