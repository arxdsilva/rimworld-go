package ebiten

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/options/saves"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
)

// Options journeys (QA-001, OPT-*): the General and Interface tabs, the
// save list, and the pause modes, driven through the pause menu's Options
// screen with the real mouse.

// optOpen opens Options › tab from the pause menu of the running colony.
func optOpen(j *journey, tab string) {
	j.t.Helper()
	j.key("Escape", 0)
	j.clickID("pause.options")
	j.clickID("options.tab." + tab)
	j.draw(1) // the tab paints without a panic
}

// optBtn is the screen's button or widget id as the player sees it.
func optBtn(j *journey, id string) (label string, ok bool) {
	j.t.Helper()
	j.update(1)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return b.Label, true
		}
	}
	if w, found := j.a.uiSt().Find(id); found {
		return w.Label, true
	}
	return "", false
}

// optLabel is the text on an Options row's button.
func optLabel(j *journey, id string) string {
	j.t.Helper()
	l, ok := optBtn(j, id)
	if !ok {
		j.t.Fatalf("no button %q on the %s tab", id, j.a.opt.CurrentTab())
	}
	return l
}

// TestJourneyOptionsAutoPauseCycles: the Automatic pause row steps through
// its modes in order and wraps.
func TestJourneyOptionsAutoPauseCycles(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "general")
	var seen []string
	for range config.AutoPauseModes {
		j.clickID("options.autopause")
		seen = append(seen, j.a.settings.General.AutoPause)
	}
	distinct := map[string]bool{}
	for _, m := range seen {
		distinct[m] = true
	}
	if len(distinct) != len(config.AutoPauseModes) {
		t.Fatalf("a full turn visited %v, want every one of %v", seen, config.AutoPauseModes)
	}
	// One more click wraps round to the mode the first click left.
	j.clickID("options.autopause")
	if j.a.settings.General.AutoPause != seen[0] {
		t.Fatalf("the lap ended on %q, want %q again", j.a.settings.General.AutoPause, seen[0])
	}
	if got := optLabel(j, "options.autopause"); got == "" {
		t.Fatal("the row shows no mode")
	}
}

// TestJourneyOptionsRestoreDefaultsConfirms: Restore defaults asks first;
// Cancel keeps every setting, OK resets them and keeps the key bindings.
func TestJourneyOptionsRestoreDefaultsConfirms(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.a.settings.Controls.Bindings = map[string][]string{"pan_left": {"KeyJ"}}
	j.a.settings.Gameplay.DevMode = true
	optOpen(j, "general")
	j.clickID("options.pauseonload")
	j.clickID("options.autosaveskept.inc")
	pol, kept := j.a.settings.General.PauseOnLoad, j.a.settings.General.AutosavesKept
	def := config.Defaults().General
	if pol == def.PauseOnLoad || kept == def.AutosavesKept {
		t.Fatalf("the rows did not change: pause on load %v, kept %d", pol, kept)
	}

	j.clickID("options.restore")
	if j.a.uiModalTop() != "options.restore" {
		t.Fatalf("top modal %q, want the restore confirmation", j.a.uiModalTop())
	}
	if j.a.settings.General.PauseOnLoad != pol {
		t.Fatal("the settings reset before the player confirmed")
	}
	j.clickID("modal.options.restore.cancel")
	if j.a.settings.General.PauseOnLoad != pol || j.a.settings.General.AutosavesKept != kept {
		t.Fatalf("Cancel changed the settings: %+v", j.a.settings.General)
	}
	if j.a.uiModalTop() == "options.restore" {
		t.Fatal("Cancel left the dialog open")
	}

	j.clickID("options.restore")
	j.clickID("modal.options.restore.ok")
	g := j.a.settings.General
	if g.PauseOnLoad != def.PauseOnLoad || g.AutosavesKept != def.AutosavesKept {
		t.Fatalf("OK did not reset: %+v, want %+v", g, def)
	}
	if got := j.a.settings.Controls.Bindings["pan_left"]; len(got) != 1 || got[0] != "KeyJ" {
		t.Fatalf("restore defaults lost the key bindings: %v", got)
	}
}

// TestJourneyOptionsPreferredNames: the dialog stores the typed list; Cancel
// stores nothing.
func TestJourneyOptionsPreferredNames(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "general")
	j.clickID("options.names")
	if j.a.uiModalTop() != "options.names" {
		t.Fatalf("top modal %q, want the names dialog", j.a.uiModalTop())
	}
	j.clickID("options.names.field")
	j.typeText("Ada, Bo, ada")
	j.clickID("modal.options.names.cancel")
	if n := j.a.settings.General.PreferredNames; len(n) != 0 {
		t.Fatalf("Cancel stored %v", n)
	}

	j.clickID("options.names")
	j.clickID("options.names.field")
	j.typeText("Ada, Bo, ada")
	j.clickID("modal.options.names.ok")
	got := j.a.settings.General.PreferredNames
	if len(got) != 2 || got[0] != "Ada" || got[1] != "Bo" {
		t.Fatalf("preferred names %v, want [Ada Bo] (the repeat dropped)", got)
	}
	if l := optLabel(j, "options.names"); l != "2 names" {
		t.Fatalf("the row says %q", l)
	}
}

// TestJourneyOptionsDeveloperTabNeedsDevMode: the Developer tab is there
// only while developer mode is on.
func TestJourneyOptionsDeveloperTabNeedsDevMode(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "gameplay")
	if _, ok := optBtn(j, "options.tab.developer"); ok {
		t.Fatal("a Developer tab without developer mode")
	}
	j.clickID("options.devmode")
	if !j.a.settings.Gameplay.DevMode {
		t.Fatal("the Developer mode row did not turn it on")
	}
	j.update(1)
	j.clickID("options.tab.developer")
	if _, ok := optBtn(j, "options.verboselog"); !ok {
		t.Fatal("the Developer tab has no rows")
	}
	j.clickID("options.tab.gameplay")
	j.clickID("options.devmode")
	j.update(1)
	if _, ok := optBtn(j, "options.tab.developer"); ok {
		t.Fatal("the Developer tab stayed after developer mode went off")
	}
}

// TestJourneyOptionsLanguageListsEnglish: the Language row shows English
// and, with no other catalog, stays on it.
func TestJourneyOptionsLanguageListsEnglish(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "general")
	if l := optLabel(j, "options.language"); !strings.Contains(l, "English") {
		t.Fatalf("the Language row says %q, want English", l)
	}
	j.clickID("options.language")
	if j.a.settings.General.Language != "en" {
		t.Fatalf("language %q after a click with only English on offer", j.a.settings.General.Language)
	}
}

// TestJourneyOptionsKelvin: the Interface tab's unit row reaches Kelvin
// after Celsius and Fahrenheit, and the colony's temperature readout follows.
func TestJourneyOptionsKelvin(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "interface")
	var seen []string
	for range 3 {
		j.clickID("options.tempunit")
		seen = append(seen, j.a.settings.Gameplay.TempUnit)
	}
	if !contains(seen, units.UnitK) || !contains(seen, units.UnitF) {
		t.Fatalf("three clicks visited %v, want Fahrenheit and Kelvin among them", seen)
	}
	for j.a.settings.Gameplay.TempUnit != units.UnitK {
		j.clickID("options.tempunit")
	}
	if l := optLabel(j, "options.tempunit"); !strings.Contains(l, "Kelvin") {
		t.Fatalf("the unit row says %q", l)
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	if j.a.nav.screen() != ports.ScreenColony {
		t.Fatalf("Escape twice left %s", j.a.nav.screen())
	}
	j.update(2)
	if u := j.a.renderer.overlays().TempDraw.Units; !u.K || u.Symbol() != "K" {
		t.Fatalf("the map's temperature overlay reads in %+v, want Kelvin", u)
	}
}

// ---- saves -----------------------------------------------------------------

// fakeSaves is an in-memory save store behind the colony's save hooks; every
// write gets a later modified time than the one before.
type fakeSaves struct {
	at    map[string]time.Time
	order []string // write order, oldest first
	n     int
}

func newFakeSaves() *fakeSaves { return &fakeSaves{at: map[string]time.Time{}} }

func (f *fakeSaves) put(name string) {
	f.n++
	f.at[name] = time.Unix(1_700_000_000, 0).Add(time.Duration(f.n) * time.Minute)
	f.order = append(f.order, name)
}

func (f *fakeSaves) hooks() SaveHooks {
	return SaveHooks{
		Save: func(_ Simulation, m ports.SaveMeta) error { f.put(m.Name); return nil },
		List: func() ([]ports.SaveSlot, error) {
			var out []ports.SaveSlot
			for n, at := range f.at {
				out = append(out, ports.SaveSlot{Name: n, Path: n, Date: "day 1", Colonists: 1, Modified: at})
			}
			return out, nil
		},
		Load: func(string) (Simulation, ports.SaveMeta, error) { return nil, ports.SaveMeta{}, errors.New("not in this journey") },
	}
}

// savesJourney is a colony whose saves go to a fake store.
func savesJourney(t *testing.T) (*journey, *fakeSaves) {
	t.Helper()
	j := newJourney(t)
	f := newFakeSaves()
	j.a.cfg.Saves = f.hooks()
	j.a.opt.Saves.SaveSlot = "col"
	j.a.settings.General.AutoPause = config.AutoPauseNever // a long step brings no letter that pauses the clock
	return j, f
}

// TestJourneyOptionsAutosaveRotation: with three autosaves kept the morning
// saves fill -autosave-1…3 in turn and then overwrite the oldest.
func TestJourneyOptionsAutosaveRotation(t *testing.T) {
	t.Parallel()
	j, f := savesJourney(t)
	optOpen(j, "general")
	j.clickID("options.autosaveskept.inc")
	j.clickID("options.autosaveskept.inc")
	if got := j.a.settings.General.AutosavesKept; got != 3 {
		t.Fatalf("autosaves kept %d, want 3", got)
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	j.a.settings.Gameplay.AutosaveDays = 1
	var wrote []string
	for day := 1; day <= 4; day++ {
		morning := uint64(day)*clock.TicksPerDay + saves.AutosaveHour*clock.TicksPerHour
		if d := morning - 5 - uint64(j.g.Tick()); d > 0 {
			j.g.Step(int(d))
		}
		j.tick(10)
		if len(f.order) != len(wrote)+1 {
			t.Fatalf("day %d: %d saves written, want %d (%v)", day, len(f.order), len(wrote)+1, f.order)
		}
		wrote = append(wrote, f.order[len(f.order)-1])
	}
	want := []string{"col-autosave-1", "col-autosave-2", "col-autosave-3", "col-autosave-1"}
	if fmt.Sprint(wrote) != fmt.Sprint(want) {
		t.Fatalf("autosaves went to %v, want %v (oldest first)", wrote, want)
	}
}

// TestJourneyOptionsLoadListPages: the Load list shows 8 saves and a More
// saves row, which turns the page.
func TestJourneyOptionsLoadListPages(t *testing.T) {
	t.Parallel()
	j, f := savesJourney(t)
	for i := 1; i <= 11; i++ {
		f.put(fmt.Sprintf("save-%02d", i))
	}
	j.key("Escape", 0)
	j.clickID("pause.load")
	if j.a.nav.screen() != ports.ScreenLoad {
		t.Fatalf("Load opened %s", j.a.nav.screen())
	}
	slotRows := func() (n int, more string) {
		j.update(1)
		for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
			switch {
			case b.ID == "load.slot.-1":
				more = b.Label
			case strings.HasPrefix(b.ID, "load.slot."):
				n++
			}
		}
		return
	}
	n, more := slotRows()
	if n != 8 || !strings.Contains(more, "More saves") || !strings.Contains(more, "page 1 of 2") {
		t.Fatalf("page 1: %d saves, more row %q; want 8 and a More saves row", n, more)
	}
	j.clickID("load.slot.-1")
	n, more = slotRows()
	if n != 3 || !strings.Contains(more, "page 2 of 2") {
		t.Fatalf("page 2: %d saves, more row %q; want the last 3", n, more)
	}
}

// ---- pause and speed --------------------------------------------------------

// optSetList clicks a list row until its setting reads want, then leaves
// Options for the colony.
func optSetList[T comparable](j *journey, tab, id string, get func() T, want T) {
	j.t.Helper()
	optOpen(j, tab)
	for i := 0; get() != want; i++ {
		if i > 24 {
			j.t.Fatalf("%s never reached %v (at %v)", id, want, get())
		}
		j.clickID(id)
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() != overlayNone {
		j.t.Fatalf("Escape twice left %s (overlay %v)", j.a.nav.screen(), j.a.nav.overlay())
	}
}

// TestJourneyOptionsAutoPauseModesOnARaid: a raid brings a threat letter and
// a danger message. Never keeps the clock running; every other mode stops it.
func TestJourneyOptionsAutoPauseModesOnARaid(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode   string
		paused bool
	}{
		{config.AutoPauseNever, false},
		{config.AutoPauseMajor, true},
		{config.AutoPauseThreat, true},
		{config.AutoPauseLetter, true},
	} {
		t.Run(c.mode, func(t *testing.T) {
			t.Parallel()
			j := newJourney(t)
			optSetList(j, "general", "options.autopause", func() string { return j.a.settings.General.AutoPause }, c.mode)
			r := j.a.renderer
			j.key("Digit1", 0)
			j.update(2) // what is there at the start is only noted
			if r.speed != speedNormal {
				t.Fatalf("the colony is at %v before the raid", r.speed)
			}
			j.g.Raid(1)
			j.update(20)
			if len(j.world().Letters) == 0 {
				t.Fatal("the raid sent no letter")
			}
			if got := r.speed == speedPaused; got != c.paused {
				t.Fatalf("mode %s: paused %v after a raid, want %v", c.mode, got, c.paused)
			}
		})
	}
}

// TestJourneyOptionsSpaceResumesAtTheLastSpeed: with "Speed after pause" on
// last, Space returns to the speed the colony was paused from; on normal it
// returns to normal.
func TestJourneyOptionsSpaceResumesAtTheLastSpeed(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	j.key("Digit2", 0)
	j.key("Space", 0)
	j.key("Space", 0)
	if r.speed != speedNormal {
		t.Fatalf("default: Space resumed at %v, want normal", r.speed)
	}
	optSetList(j, "interface", "options.unpausespeed", func() string { return j.a.settings.Interface.UnpauseSpeed }, config.UnpauseLast)
	j.key("Digit3", 0)
	j.key("Space", 0)
	if r.speed != speedPaused {
		t.Fatalf("Space left the colony at %v, want paused", r.speed)
	}
	j.key("Space", 0)
	if r.speed != speedVeryFast {
		t.Fatalf("Space resumed at %v, want the superfast it was paused from", r.speed)
	}
}

// TestJourneyOptionsPauseOnLoad: a loaded game starts paused with the option
// on (the default) and running with it off.
func TestJourneyOptionsPauseOnLoad(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{true, false} {
		t.Run(fmt.Sprintf("on=%v", on), func(t *testing.T) {
			t.Parallel()
			j, f := savesJourney(t)
			f.put("old")
			saved := game.New(game.Config{Seed: 2, Width: 24, Height: 24})
			if _, err := saved.SpawnPawn(8, 4); err != nil {
				t.Fatal(err)
			}
			saved.Step(120)
			hooks := j.a.cfg.Saves
			hooks.Load = func(string) (Simulation, ports.SaveMeta, error) { return saved, ports.SaveMeta{Name: "old"}, nil }
			j.a.cfg.Saves = hooks
			j.a.settings.General.PauseOnLoad = on
			j.key("Escape", 0)
			j.clickID("pause.load")
			j.clickID("load.slot.0")
			j.clickID("modal.saves.unsaved.leave") // the journey's colony was never saved (SAV-013)
			j.update(3)
			if j.a.nav.screen() != ports.ScreenColony || j.a.renderer == nil {
				t.Fatalf("after the load: screen %s", j.a.nav.screen())
			}
			if got := j.a.renderer.speed == speedPaused; got != on {
				t.Fatalf("pause on load %v: loaded game paused %v (speed %v)", on, got, j.a.renderer.speed)
			}
		})
	}
}
