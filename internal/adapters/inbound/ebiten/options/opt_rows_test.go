package options

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// kitFake is a fakeShell with a widget kit, for the rows that open a dialog.
type kitFake struct {
	*fakeShell
	k *uikit.Kit
}

func (f kitFake) Kit() *uikit.Kit { return f.k }

func newKitFake() (*fakeShell, *uikit.Kit, *Screen) {
	f := &fakeShell{settings: config.Defaults()}
	k := &uikit.Kit{}
	return f, k, New(kitFake{f, k})
}

// optRowCase is one OPT row: the tab it is on, the id of the control that
// changes it and the setting it writes.
type optRowCase struct {
	tab, id string
	read    func(config.Settings) any
}

func optRowCases() []optRowCase {
	g := func(f func(s config.Settings) any) func(config.Settings) any { return f }
	return []optRowCase{
		{"general", "pauseonload", g(func(s config.Settings) any { return s.General.PauseOnLoad })},
		{"general", "autopause", g(func(s config.Settings) any { return s.General.AutoPause })},
		{"general", "autosaveskept.inc", g(func(s config.Settings) any { return s.General.AutosavesKept })},
		{"general", "customcursor", g(func(s config.Settings) any { return s.Camera.CustomCursor })},
		{"interface", "clockrealtime", g(func(s config.Settings) any { return s.Interface.ClockRealtime })},
		{"interface", "clock12h", g(func(s config.Settings) any { return s.Interface.Clock12h })},
		{"interface", "animalnames", g(func(s config.Settings) any { return s.Interface.AnimalNames })},
		{"interface", "barweapons", g(func(s config.Settings) any { return s.Interface.BarWeapons })},
		{"interface", "barmood", g(func(s config.Settings) any { return s.Interface.BarMood })},
		{"interface", "colonistbar", g(func(s config.Settings) any { return s.Gameplay.ShowColonistBar })},
		{"interface", "categorized", g(func(s config.Settings) any { return s.Gameplay.CategorizedResources })},
		{"interface", "tempunit", g(func(s config.Settings) any { return s.Gameplay.TempUnit })},
		{"interface", "confirm", g(func(s config.Settings) any { return s.Interface.Confirm })},
		{"interface", "unpausespeed", g(func(s config.Settings) any { return s.Interface.UnpauseSpeed })},
		{"interface", "farpawns", g(func(s config.Settings) any { return s.Interface.FarPawns })},
		{"interface", "farstyle", g(func(s config.Settings) any { return s.Interface.FarStyle })},
		{"interface", "menubg", g(func(s config.Settings) any { return s.Interface.MenuBackground })},
		{"interface", "plantsway", g(func(s config.Settings) any { return s.Camera.PlantSway })},
		{"access", "tooltipdelay.inc", g(func(s config.Settings) any { return s.Interface.TooltipDelayMS })},
		{"access", "colorvision", g(func(s config.Settings) any { return s.Interface.ColorVision })},
		{"access", "largersmall", g(func(s config.Settings) any { return s.Interface.LargerSmall })},
		{"access", "hatsonmap", g(func(s config.Settings) any { return s.Interface.HatsOnlyOnMap })},
		{"gameplay", "zoomtomouse", g(func(s config.Settings) any { return s.Camera.ZoomToMouse })},
		{"gameplay", "smoothjumps", g(func(s config.Settings) any { return s.Camera.SmoothJumps })},
		{"gameplay", "dragpct", g(func(s config.Settings) any { return s.Camera.DragPct })},
		{"gameplay", "zoomoutworld", g(func(s config.Settings) any { return s.Camera.ZoomOutToWorld })},
		{"gameplay", "rememberstyles", g(func(s config.Settings) any { return s.Camera.RememberStyles })},
		{"gameplay", "shake", g(func(s config.Settings) any { return s.Camera.ShakePct })},
		{"graphics", "windowsize", g(func(s config.Settings) any { return s.Camera.Window })},
		{"graphics", "borderless", g(func(s config.Settings) any { return s.Camera.Borderless })},
		{"developer", "verboselog", g(func(s config.Settings) any { return s.Developer.VerboseLog })},
		{"developer", "logonwarn", g(func(s config.Settings) any { return s.Developer.OpenLogOnWarn })},
		{"developer", "logesc", g(func(s config.Settings) any { return s.Developer.LogEscCloses })},
		{"developer", "pauseonerror", g(func(s config.Settings) any { return s.Developer.PauseOnError })},
		{"developer", "testmaps", g(func(s config.Settings) any { return s.Developer.TestMapSizes })},
	}
}

// Every OPT row changes its setting through SetSettings, once, with the
// toggle sound, and the normalised result is still what was asked for.
func TestEveryOptRowGoesThroughSetSettings(t *testing.T) {
	t.Parallel()
	for _, c := range optRowCases() {
		t.Run(c.tab+"/"+c.id, func(t *testing.T) {
			f, s := newFake()
			f.settings.Gameplay.DevMode = true
			s.Tab = c.tab
			before := c.read(f.settings)
			s.ApplyOption("options." + c.id)
			if len(f.set) != 1 {
				t.Fatalf("SetSettings called %d times", len(f.set))
			}
			after := c.read(f.set[0])
			if reflect.DeepEqual(before, after) {
				t.Fatalf("%v did not change", before)
			}
			if got := c.read(f.set[0].Normalize()); !reflect.DeepEqual(got, after) {
				t.Fatalf("Normalize turns %v into %v", after, got)
			}
			if len(f.cues) != 1 || f.cues[0] != "ui.toggle" {
				t.Fatalf("cues %v", f.cues)
			}
		})
	}
}

// Cycling rows come back to where they started and never leave the values
// Normalize accepts.
func TestOptCyclesStayInTheirChoices(t *testing.T) {
	t.Parallel()
	for _, c := range optRowCases() {
		f, s := newFake()
		f.settings.Gameplay.DevMode = true
		s.Tab = c.tab
		seen := map[string]bool{}
		for i := 0; i < 40; i++ {
			s.ApplyOption("options." + c.id)
			f.settings = f.settings.Normalize()
			f.settings.Gameplay.DevMode = true
			seen[fmt.Sprint(c.read(f.settings))] = true
		}
		if len(seen) < 2 {
			t.Errorf("%s: only %v", c.id, seen)
		}
	}
}

func TestTemperatureUnitCyclesCelsiusFahrenheitKelvin(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	s.Tab = "interface"
	var got []string
	for i := 0; i < 3; i++ {
		s.ApplyOption("options.tempunit")
		got = append(got, f.settings.Gameplay.TempUnit)
	}
	if !reflect.DeepEqual(got, []string{"F", "K", ""}) {
		t.Fatalf("units %q", got)
	}
}

func TestAutoPauseCyclesEveryMode(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	s.Tab = "general"
	start := f.settings.General.AutoPause
	var got []string
	for range config.AutoPauseModes {
		s.ApplyOption("options.autopause")
		got = append(got, f.settings.General.AutoPause)
	}
	if got[len(got)-1] != start || len(map[string]bool{got[0]: true, got[1]: true, got[2]: true}) != 3 {
		t.Fatalf("modes %v from %q", got, start)
	}
	for _, r := range OptionRows("gameplay") {
		if r.Key == "pausethreat" {
			t.Fatal("Pause on threat is replaced by Automatic pause")
		}
	}
}

func TestStepRowsClampAtTheirEnds(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	s.Tab = "general"
	f.settings.General.AutosavesKept = 1
	s.ApplyOption("options.autosaveskept.dec")
	if f.settings.General.AutosavesKept != 1 {
		t.Fatal("below one")
	}
	for i := 0; i < 40; i++ {
		s.ApplyOption("options.autosaveskept.inc")
	}
	if f.settings.General.AutosavesKept != config.AutosavesKeptMax {
		t.Fatalf("kept %d", f.settings.General.AutosavesKept)
	}
	s.Tab = "access"
	f.settings.Interface.TooltipDelayMS = 0
	s.ApplyOption("options.tooltipdelay.dec")
	if f.settings.Interface.TooltipDelayMS != 0 {
		t.Fatal("below zero")
	}
	s.ApplyOption("options.tooltipdelay.inc")
	if f.settings.Interface.TooltipDelayMS != config.TooltipDelayStep {
		t.Fatalf("delay %d", f.settings.Interface.TooltipDelayMS)
	}
}

// The Developer tab exists only with developer mode.
func TestDeveloperTabNeedsDeveloperMode(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	has := func() bool {
		for _, tab := range s.TabList() {
			if tab == "developer" {
				return true
			}
		}
		return false
	}
	s.Tab = "developer"
	if has() || s.CurrentTab() == "developer" {
		t.Fatal("no developer mode: no Developer tab")
	}
	f.settings.Gameplay.DevMode = true
	if !has() || s.CurrentTab() != "developer" {
		t.Fatalf("developer mode: tabs %v current %q", s.TabList(), s.CurrentTab())
	}
}

// TestOptionsButtonsFitAtEveryUIScale: every tab's buttons, the tab row
// included, stay on the logical screens a UI step can leave (the smallest a
// step above 1 allows is 1280x768, UIS-003; step 1 on the setup minimum is
// 1280x720) and do not overlap.
func TestOptionsButtonsFitAtEveryUIScale(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.Bindings = input.Defaults()
	f.env.Running = true
	f.settings.Gameplay.DevMode = true
	for _, size := range [][2]int{{1280, 720}, {1280, 768}, {1536, 864}, {1920, 1080}} {
		sw, sh := size[0], size[1]
		for _, tab := range s.TabList() {
			s.Tab = tab
			bs := s.Buttons(sw, sh)
			for i, b := range bs {
				if b.X < 0 || b.Y < 0 || b.X+b.W > float32(sw) || b.Y+b.H > float32(sh) {
					t.Fatalf("%s: %s is off screen (%v,%v %vx%v in %dx%d)", tab, b.ID, b.X, b.Y, b.W, b.H, sw, sh)
				}
				for _, o := range bs[i+1:] {
					if b.X < o.X+o.W && o.X < b.X+b.W && b.Y < o.Y+o.H && o.Y < b.Y+b.H {
						t.Fatalf("%dx%d %s: %s overlaps %s", sw, sh, tab, b.ID, o.ID)
					}
				}
			}
		}
	}
}

// No id repeats within a tab, every row has a label and a value, and a tab
// with more rows than the 600 px window shows has the Up and Down buttons
// that reach the rest (UIS-075).
func TestOptionTabRowsAreUniqueNamedAndReachable(t *testing.T) {
	t.Parallel()
	_, s := newFake()
	for _, tab := range []string{"audio", "graphics", "gameplay", "access", "general", "interface", "developer"} {
		seen := map[string]bool{}
		rows := OptionRows(tab)
		for _, r := range rows {
			if seen[r.Key] {
				t.Errorf("%s: %s twice", tab, r.Key)
			}
			seen[r.Key] = true
			if r.Label == "" || (r.value == nil && r.live == nil) {
				t.Errorf("%s/%s has no label or value", tab, r.Key)
			}
		}
		s.Tab = tab
		per := rowsPerPage(winRect(1280, 720, tab), tab)
		has := false
		for _, b := range s.Buttons(1280, 720) {
			has = has || b.ID == "options.rows.down"
		}
		if has != (len(rows) > per) {
			t.Errorf("%s: %d rows, %d per page, page buttons %v", tab, len(rows), per, has)
		}
	}
}

// OPT-003: the folder rows open the saves and the log folder.
func TestFolderRowsOpenTheFolders(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	var opened []string
	f.env.SavesDir, f.env.LogDir = "/data/saves", "/data/logs"
	f.env.OpenFolder = func(p string) error { opened = append(opened, p); return nil }
	s.Tab = "general"
	s.ApplyOption("options.savesfolder")
	s.ApplyOption("options.logfolder")
	if !reflect.DeepEqual(opened, []string{"/data/saves", "/data/logs"}) {
		t.Fatalf("opened %v", opened)
	}
	if len(f.set) != 0 {
		t.Fatal("opening a folder changes no setting")
	}
	f.env.OpenFolder = func(string) error { return errors.New("no browser") }
	f.hints = nil
	s.ApplyOption("options.savesfolder")
	if len(f.hints) != 1 {
		t.Fatalf("a failure is said once: %v", f.hints)
	}
}

// OPT-002: Restore defaults asks first, then resets everything but the
// bindings, the install id and the progress.
func TestRestoreDefaultsAsksFirst(t *testing.T) {
	t.Parallel()
	f, k, s := newKitFake()
	f.settings.Audio.Master = 10
	f.settings.Interface.Clock12h = true
	f.settings.Gameplay.TempUnit = "K"
	f.settings.Gameplay.LearnedLessons = []string{"a"}
	f.settings.Telemetry.InstallID = "id"
	f.settings.Controls.Bindings = map[string][]string{"x": {"Y"}}
	s.Tab = "general"
	s.ApplyOption("options.restore")
	if len(f.set) != 0 || len(k.Modals) != 1 {
		t.Fatalf("settings saved %d, dialogs %d: nothing changes before OK", len(f.set), len(k.Modals))
	}
	if b := k.Modals[0].Buttons[1]; b.Label != "Cancel" || b.OnClick != nil || !b.Closes {
		t.Fatalf("Cancel only closes: %+v", b)
	}
	k.Modals[0].Buttons[0].OnClick() // Restore
	if len(f.set) != 1 {
		t.Fatalf("saved %d times", len(f.set))
	}
	got, d := f.set[0], config.Defaults()
	if got.Audio != d.Audio || got.Interface.Clock12h || got.Gameplay.TempUnit != "" {
		t.Fatalf("not restored: %+v", got)
	}
	if got.Telemetry.InstallID != "id" || len(got.Controls.Bindings) != 1 || len(got.Gameplay.LearnedLessons) != 1 {
		t.Fatalf("progress and bindings were kept: %+v", got)
	}
}

// OPT-032: the names dialog stores a clean list.
func TestPreferredNamesDialogStoresTheList(t *testing.T) {
	t.Parallel()
	f, k, s := newKitFake()
	f.settings.General.PreferredNames = []string{"Ada"}
	s.Tab = "general"
	s.ApplyOption("options.names")
	if len(k.Modals) != 1 || s.namesDraft != "Ada" {
		t.Fatalf("dialogs %d draft %q", len(k.Modals), s.namesDraft)
	}
	s.namesDraft = " Bo ,ada,, Chidi ,Bo,  "
	k.Modals[0].Buttons[0].OnClick()
	if got := f.settings.General.PreferredNames; !reflect.DeepEqual(got, []string{"Bo", "ada", "Chidi"}) {
		t.Fatalf("names %q", got)
	}
}

func TestParseNamesLimits(t *testing.T) {
	t.Parallel()
	if got := ParseNames("  ,\n "); got != nil {
		t.Fatalf("blank list: %q", got)
	}
	long := "Abcdefghijklmnopqrstuvwxyz"
	if got := ParseNames(long); len(got) != 1 || len([]rune(got[0])) != maxNameRunes {
		t.Fatalf("long name: %q", got)
	}
	var many string
	for i := 0; i < 30; i++ {
		many += fmt.Sprintf("N%c,", 'a'+i)
	}
	if got := ParseNames(many); len(got) != maxPreferredNames {
		t.Fatalf("%d names kept", len(got))
	}
}

// OPT-006: with more than one autosave kept, the saves go round and the
// oldest is written over.
func TestAutosavesRotate(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.CanLoad = true, true
	f.settings.Gameplay.AutosaveDays = 1
	f.settings.General.AutosavesKept = 3
	s.SaveSlot = "colony"
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var want []string
	for day := 1; day <= 7; day++ {
		f.date, f.tick = morning(day)
		s.TickAutosave()
		name := f.saved[len(f.saved)-1]
		want = append(want, name)
		// the store lists what was written, with growing times
		found := false
		for i := range f.slots {
			if f.slots[i].Name == name {
				f.slots[i].Modified, found = base.Add(time.Duration(day)*time.Hour), true
			}
		}
		if !found {
			f.slots = append(f.slots, ports.SaveSlot{Name: name, Modified: base.Add(time.Duration(day) * time.Hour)})
		}
	}
	exp := []string{"colony-autosave-1", "colony-autosave-2", "colony-autosave-3", "colony-autosave-1", "colony-autosave-2", "colony-autosave-3", "colony-autosave-1"}
	if !reflect.DeepEqual(want, exp) {
		t.Fatalf("slots %v, want %v", want, exp)
	}
	if len(f.slots) != 3 {
		t.Fatalf("%d autosaves on disk, want 3", len(f.slots))
	}
}

// One autosave kept is the file it always was; lowering the number later
// never goes above it.
func TestOneAutosaveKeepsTheOldName(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.CanLoad = true, true
	s.SaveSlot = "colony"
	f.slots = []ports.SaveSlot{{Name: "colony-autosave-1"}, {Name: "colony-autosave-2"}}
	if got := autosaveSlot(s, 5); got != "colony-autosave" {
		t.Fatalf("slot %q", got)
	}
	f.settings.General.AutosavesKept = 2
	f.slots[0].Modified = time.Unix(10, 0)
	f.slots[1].Modified = time.Unix(5, 0)
	if got := autosaveSlot(s, 5); got != "colony-autosave-2" {
		t.Fatalf("the older of two: %q", got)
	}
	f.listErr = errors.New("gone")
	if got := autosaveSlot(s, 5); got != "colony-autosave-2" { // 5%2+1
		t.Fatalf("unreadable list: %q", got)
	}
}

func TestLoadingARotatedAutosaveKeepsTheColonySlot(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"colony-autosave-3": "colony", "colony-autosave": "colony", "colony": "colony", "a-autosave-b": "a-autosave-b",
	} {
		if got := baseSlot(in); got != want {
			t.Errorf("baseSlot(%q) = %q, want %q", in, got, want)
		}
	}
}

// A long save list is shown a page at a time: every save is reachable, and
// the page row is the last button.
func TestLoadListPagesThroughEverySave(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanLoad = true
	for i := 0; i < 30; i++ {
		f.slots = append(f.slots, ports.SaveSlot{Name: fmt.Sprintf("s%d", i), Path: fmt.Sprintf("/s%d", i)})
	}
	s.RefreshSlots()
	reached := map[int]bool{}
	for page := 0; page < 8; page++ {
		bs := s.LoadButtons(1280, 720)
		if len(bs) > 1+slotRows {
			t.Fatalf("page %d: %d buttons", page, len(bs))
		}
		for _, b := range bs[1 : len(bs)-1] {
			var i int
			fmt.Sscanf(b.ID, "load.slot.%d", &i)
			reached[i] = true
		}
		if last := bs[len(bs)-1]; last.ID != "load.slot.-1" {
			t.Fatalf("no page row: %s", last.ID)
		}
		s.LoadSlot(-1)
	}
	if len(reached) != 30 {
		t.Fatalf("reached %d of 30 saves", len(reached))
	}
	s.LoadSlot(29)
	if len(f.loaded) != 1 || f.loaded[0] != "/s29" {
		t.Fatalf("loaded %v", f.loaded)
	}
}

// OPT-001: the Language row cycles the catalogs in the language folder.
func TestLanguageRowCyclesTheCatalogs(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	dir := t.TempDir()
	body := `{"_meta":{"name":"Test"}}`
	if err := os.WriteFile(filepath.Join(dir, "xx.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f.env.LangDir = dir
	s.Tab = "general"
	var got []string
	for i := 0; i < 3; i++ {
		s.ApplyOption("options.language")
		got = append(got, f.settings.General.Language)
	}
	if len(lang.Codes(dir)) != 2 || got[0] != "xx" || got[1] != "en" {
		t.Fatalf("languages %v from %v", got, lang.Codes(dir))
	}
	f.env.LangDir = ""
	s.ApplyOption("options.language")
	if f.settings.General.Language != "en" {
		t.Fatal("without catalogs only English is on offer")
	}
}
