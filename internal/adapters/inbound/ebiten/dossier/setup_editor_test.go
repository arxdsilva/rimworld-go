package dossier

import (
	"errors"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/platform/config"
)

// memStore is a PresetStore in memory.
type memStore struct {
	files map[string][]byte
	fail  error
}

func (m *memStore) List() ([]string, error) {
	var out []string
	for k := range m.files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, m.fail
}
func (m *memStore) Load(n string) ([]byte, error) {
	b, ok := m.files[n]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return b, nil
}
func (m *memStore) Save(n string, d []byte) error {
	if m.fail != nil {
		return m.fail
	}
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[n] = d
	return nil
}
func (m *memStore) Delete(n string) error { delete(m.files, n); return nil }

var testArea = gfx.Rect{X: 20, Y: 20, W: 620, H: 360}

// tall shows a whole tab without scrolling.
var tall = gfx.Rect{X: 20, Y: 20, W: 620, H: 1200}

// setupShell is a setup modal over the fake shell; it declares a tab for a frame.
func setupShell(t *testing.T) (*fakeShell, *Setup, *memStore) {
	t.Helper()
	s := newShell(t)
	st := &memStore{}
	su := NewSetup(s.panel, s.kit, s, s, st, func(string) error { return nil }, "/cards", s)
	return s, su, st
}

func (s *fakeShell) frame() { s.step(-50, -50, false, gfx.Rect{X: 0, Y: 0, W: 10, H: 10}, true) }

func (s *fakeShell) paintAll() {
	img := ebiten.NewImage(700, 420)
	s.kit.Paint(img)
}

func lastHint(s *fakeShell) string {
	if len(s.hints) == 0 {
		return ""
	}
	return s.hints[len(s.hints)-1]
}

func TestSectionsAddBarWritesTheLayoutAndClearsThePreset(t *testing.T) {
	s, su, _ := setupShell(t)
	s.set.HUD.Preset = "Roomy"
	s.set.HUD.Docked = true
	s.frame()
	su.sections.Declare(testArea)
	if len(su.sections.rows) < 3 {
		t.Fatalf("the tree shows %d rows", len(su.sections.rows))
	}
	before := su.sections.work
	nodes0, _ := sheet.Count(before)

	n := sheet.NewNode("fun_bar")
	n.BarStyle = sheet.BarLowOnly
	su.sections.edit(func(root *config.HUDNode) ([]int, error) { return su.sections.place(root, n) })

	h := s.set.HUD
	if h.Preset != "" {
		t.Fatalf("an edit leaves the preset %q", h.Preset)
	}
	if h.Layouts.Docked == nil {
		t.Fatal("the edited tree is not stored")
	}
	nodes1, _ := sheet.Count(*h.Layouts.Docked)
	if nodes1 != nodes0+1 {
		t.Fatalf("nodes %d -> %d", nodes0, nodes1)
	}
	last := h.Layouts.Docked.Kids[len(h.Layouts.Docked.Kids)-1]
	if last.Kind != "fun_bar" || last.BarStyle != sheet.BarLowOnly {
		t.Fatalf("added %+v", last)
	}
	// the stored tree is its own copy: a later edit of the working copy does not leak
	su.sections.work.Kids[0].Fill = !su.sections.work.Kids[0].Fill
	if reflect.DeepEqual(*s.set.HUD.Layouts.Docked, su.sections.work) {
		t.Fatal("the settings share memory with the working copy")
	}
	su.sections.work.Kids[0].Fill = !su.sections.work.Kids[0].Fill

	// removing it again gives the built-in tree, stored as nil
	su.sections.sel = []int{len(h.Layouts.Docked.Kids) - 1}
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		sheet.Remove(root, su.sections.sel)
		return nil, nil
	})
	if s.set.HUD.Layouts.Docked != nil {
		t.Fatal("a tree equal to the built-in one is stored as nil")
	}
	s.frame()
	su.sections.Declare(testArea)
	s.paintAll()
}

func TestSectionsRefuseWhatCannotBeHeld(t *testing.T) {
	s, su, _ := setupShell(t)
	s.set.HUD.Docked = true
	s.frame()
	su.sections.Declare(testArea)
	// a row inside a row is refused and nothing is stored
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		return sheet.Add(root, nil, 0, sheet.NewNode(sheet.KindLine))
	})
	su.sections.sel = []int{0}
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		_, err := sheet.Add(root, []int{0}, 0, sheet.NewNode(sheet.KindColumn))
		return nil, err
	})
	if !strings.Contains(lastHint(s), "cannot hold") {
		t.Fatalf("hint %q", lastHint(s))
	}
	// the root cannot be removed
	su.sections.sel = nil
	n := len(s.hints)
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		if !sheet.Remove(root, nil) {
			return nil, sheet.ErrNoSuchNode
		}
		return nil, nil
	})
	if len(s.hints) != n+1 {
		t.Fatal("removing the root gave no reason")
	}
}

func TestSectionsTargetsKeepOne(t *testing.T) {
	s, su, _ := setupShell(t)
	s.set.HUD.Docked = true
	s.frame()
	su.sections.Declare(testArea)
	su.sections.sel = []int{0}
	for _, w := range []sheet.Who{sheet.WhoStranger, sheet.WhoPrisoner, sheet.WhoWild} {
		w := w
		su.sections.edit(func(root *config.HUDNode) ([]int, error) {
			return su.sections.sel, sheet.SetWho(sheet.At(root, su.sections.sel), w, false)
		})
	}
	got := sheet.WhoSet(*sheet.At(s.set.HUD.Layouts.Docked, []int{0}))
	if got != "c" {
		t.Fatalf("targets %q", got)
	}
	n := len(s.hints)
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		return su.sections.sel, sheet.SetWho(sheet.At(root, su.sections.sel), sheet.WhoColonist, false)
	})
	if !errors.Is(sheet.SetWho(&config.HUDNode{Who: "c"}, sheet.WhoColonist, false), sheet.ErrLastTarget) || len(s.hints) != n+1 {
		t.Fatal("the last target can be turned off")
	}
}

func TestSectionsFollowTheModeAndTheBuiltInIsReachable(t *testing.T) {
	s, su, _ := setupShell(t)
	s.set.HUD.Docked = true
	s.frame()
	su.sections.Declare(testArea)
	su.sections.edit(func(root *config.HUDNode) ([]int, error) {
		return su.sections.place(root, sheet.NewNode("traits"))
	})
	if s.set.HUD.Layouts.Docked == nil || s.set.HUD.Layouts.Floating != nil {
		t.Fatal("the edit went to the wrong mode")
	}
	s.set.HUD.Docked = false
	s.frame()
	su.sections.Declare(testArea)
	if !reflect.DeepEqual(su.sections.work, sheet.DefaultFloating()) {
		t.Fatal("the floating mode does not show its own tree")
	}
	s.set.HUD.Docked = true
	s.frame()
	su.sections.Declare(testArea)
	su.sections.useDefault()
	if s.set.HUD.Layouts.Docked != nil {
		t.Fatal("the built-in layout is not back")
	}
	s.frame()
	su.sections.Declare(testArea)
	if !reflect.DeepEqual(su.sections.work, sheet.DefaultDocked()) {
		t.Fatal("the tree does not follow the settings")
	}
}

func TestSectionsHostileTreeDoesNotPanic(t *testing.T) {
	s, su, _ := setupShell(t)
	s.set.HUD.Docked = true
	bad := config.HUDNode{Kind: "column", Kids: []config.HUDNode{{Kind: "nope", Def: "x"}, {Kind: "line", Kids: []config.HUDNode{{Kind: "column"}}}, {Kind: "skill", Def: "zzz", Who: "qq"}}}
	s.set.HUD.Layouts.Docked = &bad
	s.frame()
	su.sections.Declare(testArea)
	s.paintAll()
	su.sections.edit(func(root *config.HUDNode) ([]int, error) { return su.sections.place(root, sheet.NewNode("rule")) })
}

func TestPresetsSaveAndLoad(t *testing.T) {
	s, su, st := setupShell(t)
	s.set.HUD.Docked = true
	s.set.HUD.Text.BodySize = 11
	n := sheet.NewNode("traits")
	tree := sheet.DefaultDocked()
	tree.Kids = append(tree.Kids, n)
	s.set.HUD.Layouts.Docked = &tree
	s.frame()
	p := su.presets
	p.Declare(testArea)
	if p.saveBlock() == "" {
		t.Fatal("an empty name can be saved")
	}
	p.name = "My look"
	if p.saveBlock() != "" {
		t.Fatalf("blocked: %s", p.saveBlock())
	}
	p.save()
	if _, ok := st.files["my-look"]; !ok {
		t.Fatalf("not stored: %v", st.files)
	}
	if s.set.HUD.Preset != "My look" {
		t.Fatalf("preset %q", s.set.HUD.Preset)
	}
	// edit away, then load it again
	s.set.HUD.Layouts = config.HUDLayouts{}
	s.set.HUD.Text.BodySize = 14
	s.set.HUD.Preset = ""
	e, ok := p.chosen()
	if !ok || e.label != "My look" || e.builtIn {
		t.Fatalf("the kept look is not selected: %+v %v", e, ok)
	}
	p.load(e)
	if s.set.HUD.Text.BodySize != 11 || s.set.HUD.Layouts.Docked == nil || s.set.HUD.Preset != "My look" {
		t.Fatalf("loaded %+v", s.set.HUD)
	}
	// loading the same name again keeps the name
	p.load(e)
	if s.set.HUD.Preset != "My look" {
		t.Fatal("a reload cleared the name")
	}
	// the standard look returns the built-in layouts
	p.load(presetEntry{label: "Standard", p: mustBuiltIn(t, "Standard"), builtIn: true})
	if s.set.HUD.Layouts.Docked != nil || s.set.HUD.Preset != "Standard" || !strings.Contains(lastHint(s), "standard") {
		t.Fatalf("standard: %+v %q", s.set.HUD.Layouts, lastHint(s))
	}
}

func mustBuiltIn(t *testing.T, n string) sheet.Preset {
	p, ok := sheet.BuiltIn(n)
	if !ok {
		t.Fatalf("no built-in %q", n)
	}
	return p
}

func TestPresetsRefuseBadNamesAndReasons(t *testing.T) {
	s, su, st := setupShell(t)
	s.frame()
	p := su.presets
	p.Declare(testArea)
	for _, n := range []string{"", "  ", "..", "a/b", "Standard", "x.y", strings.Repeat("a", 41)} {
		p.name = n
		if p.saveBlock() == "" {
			t.Errorf("name %q can be saved", n)
		}
	}
	p.name = "ok"
	p.flags = sheet.SaveFlags{}
	if p.saveBlock() == "" {
		t.Error("saving nothing is allowed")
	}
	p.flags.Text = true
	st.fail = errors.New("disk full")
	p.save()
	if !strings.Contains(lastHint(s), "not kept") {
		t.Fatalf("hint %q", lastHint(s))
	}
	su2 := NewSetup(s.panel, s.kit, s, s, nil, nil, "", s)
	su2.presets.Declare(testArea)
	if su2.presets.saveBlock() == "" {
		t.Error("a run without a store can save")
	}
}

func TestPresetsHostileFilesFallBackToTheDefault(t *testing.T) {
	s, su, st := setupShell(t)
	s.set.HUD.Docked = true
	tree := sheet.DefaultDocked()
	tree.Kids = append(tree.Kids, sheet.NewNode("traits"))
	s.set.HUD.Layouts.Docked = &tree
	st.files = map[string][]byte{
		"junk":    []byte("not json at all"),
		"newer":   []byte(`{"version":99}`),
		"deep":    []byte(`{"version":1,"name":"Deep","docked":` + strings.Repeat(`{"k":"column","c":[`, 40) + strings.Repeat(`]}`, 40) + `}`),
		"unknown": []byte(`{"version":1,"name":"U","docked":{"k":"column","c":[{"k":"launch_missiles"}]}}`),
		"huge":    make([]byte, sheet.MaxPresetBytes+1),
		"empty":   nil,
		"nul":     []byte{0, 0, 0},
	}
	s.frame()
	p := su.presets
	p.Declare(testArea)
	s.paintAll()
	bad := 0
	for i, e := range p.entries {
		if e.builtIn {
			continue
		}
		if e.err == nil {
			t.Errorf("%s loads", e.stem)
			continue
		}
		bad++
		p.sel = i
		p.load(e)
		if s.set.HUD.Layouts.Docked != nil || s.set.HUD.Preset != "" || !strings.Contains(lastHint(s), "built-in card is back") {
			t.Fatalf("%s: %+v hint %q", e.stem, s.set.HUD.Layouts, lastHint(s))
		}
		s.set.HUD.Layouts.Docked = &tree
	}
	if bad != len(st.files) {
		t.Fatalf("%d of %d files refused", bad, len(st.files))
	}
}

func TestPresetsListIsCappedAndRefreshesOnOpen(t *testing.T) {
	s, su, st := setupShell(t)
	st.files = map[string][]byte{}
	for i := 0; i < maxKept+30; i++ {
		st.files["p"+strings.Repeat("a", 3)+string(rune('a'+i%26))+string(rune('a'+i/26))] = []byte("x")
	}
	s.frame()
	p := su.presets
	p.Declare(testArea)
	if got := len(p.entries) - len(sheet.BuiltIns()); got != maxKept {
		t.Fatalf("%d kept looks listed", got)
	}
	// a file dropped in while the tab stays open is seen only after it was away
	st.files = map[string][]byte{"dropped": mustEncode(t, sheet.Capture(config.HUD{}, "Dropped", sheet.SaveFlags{Text: true}))}
	s.frame()
	p.Declare(testArea)
	if len(p.entries) != len(sheet.BuiltIns())+maxKept {
		t.Fatal("the list changed under the cursor")
	}
	s.n += 10 // the tab was elsewhere
	s.frame()
	p.Declare(testArea)
	if len(p.entries) != len(sheet.BuiltIns())+1 || p.entries[len(p.entries)-1].label != "Dropped" {
		t.Fatalf("not refreshed: %d", len(p.entries))
	}
}

func mustEncode(t *testing.T, p sheet.Preset) []byte {
	b, err := sheet.EncodePreset(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPresetsFolderOpenFailureDisablesTheButton(t *testing.T) {
	s, su, _ := setupShell(t)
	su.open = func(string) error { return errors.New("no file browser") }
	s.frame()
	p := su.presets
	p.Declare(tall)
	p.showFolder()
	if p.openErr == "" || !strings.Contains(lastHint(s), "no file browser") {
		t.Fatalf("open error %q hint %q", p.openErr, lastHint(s))
	}
	s.frame()
	p.Declare(tall)
	w, ok := s.kit.Find("pre.folder")
	if !ok || !w.Disabled || w.Reason == "" {
		t.Fatalf("button %+v", w)
	}
}
