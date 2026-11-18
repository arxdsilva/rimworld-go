package ebiten

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// pluginsJourney is a journey back on the main menu with two installed
// plugins (beta loaded, alpha off), the run started with beta enabled, and
// settings saved into *saved.
func pluginsJourney(t *testing.T, v plugins.View) (*journey, *[]config.Settings) {
	t.Helper()
	j := newJourney(t)
	saved := &[]config.Settings{}
	j.a.cfg.Plugins = v
	j.a.cfg.Settings = options.Hooks{Save: func(s config.Settings) error { *saved = append(*saved, s); return nil }}
	s := j.a.settings
	s.Plugins.Enabled = slices.Clone(v.Enabled)
	j.a.settings = s
	j.key("Escape", 0)
	j.clickID("pause.menu")
	j.update(1)
	if j.a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("on %s, want the main menu", j.a.nav.screen())
	}
	return j, saved
}

func journeyPlugins() plugins.View {
	return plugins.View{
		Rows: []plugins.Row{
			{ID: "beta", Name: "Beta Pack", Version: "1.0.0", Loaded: true, Position: 1, Reason: "alphabetical"},
			{ID: "alpha", Name: "Alpha Pack", Version: "0.2.0"},
		},
		Enabled: []string{"beta"},
	}
}

// TestJourneyPluginsToggle: Plugins on the main menu lists the installed
// plugins; ticking one saves the enabled set (sorted) and says the change
// waits for the next start, without starting anything; unticking it again
// takes the banner away; Escape returns to the menu.
func TestJourneyPluginsToggle(t *testing.T) {
	j, saved := pluginsJourney(t, journeyPlugins())
	j.clickID(ids.MenuPlugins)
	if j.a.nav.screen() != ports.ScreenPlugins {
		t.Fatalf("Plugins opened %s", j.a.nav.screen())
	}
	j.update(1)
	text := shownText(t, j.a)
	for _, want := range []string{"Beta Pack", "Alpha Pack", "loaded", "off", "Position 1: alphabetical"} {
		if !strings.Contains(text, want) {
			t.Errorf("the Plugins screen doesn't show %q:\n%s", want, text)
		}
	}
	const banner = "Changes take effect the next time the game starts."
	if strings.Contains(text, banner) {
		t.Fatal("the restart banner shows before anything changed")
	}
	sims := j.a.simsStarted
	j.clickID(uikit.OptID(plugins.OnID, 1)) // Alpha Pack
	if n := len(*saved); n == 0 || !slices.Equal((*saved)[n-1].Plugins.Enabled, []string{"alpha", "beta"}) {
		t.Fatalf("saved settings after ticking Alpha Pack: %+v", *saved)
	}
	j.update(1)
	if text := shownText(t, j.a); !strings.Contains(text, banner) || !strings.Contains(text, "on after restart") {
		t.Fatalf("no restart banner after the change:\n%s", text)
	}
	if j.a.simsStarted != sims || j.a.nav.screen() != ports.ScreenPlugins {
		t.Fatalf("a toggle started a game or left the screen: sims %d -> %d, on %s", sims, j.a.simsStarted, j.a.nav.screen())
	}
	j.clickID(uikit.OptID(plugins.OnID, 1))
	j.update(1)
	if got := (*saved)[len(*saved)-1].Plugins.Enabled; !slices.Equal(got, []string{"beta"}) {
		t.Fatalf("after unticking: %v", got)
	}
	if strings.Contains(shownText(t, j.a), banner) {
		t.Fatal("the banner stays once the set is back to the running one")
	}
	j.key("Escape", 0)
	if j.a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("Escape left %s", j.a.nav.screen())
	}
}

// TestJourneyPluginsProblemsNotice: plugin errors put a notice on the main
// menu (warnings alone don't); View opens the problems view with each
// problem's provenance, and Disable turns the failing plugin off.
func TestJourneyPluginsProblemsNotice(t *testing.T) {
	warnOnly := journeyPlugins()
	warnOnly.Problems = []plugins.Problem{{Plugin: "beta", Field: "game", Msg: "declares no supported game version", Warn: true}}
	j, _ := pluginsJourney(t, warnOnly)
	if hasWidget(j.a, mainmenu.PluginProblemsID) {
		t.Fatal("a warning raised the problems notice")
	}

	v := journeyPlugins()
	v.Problems = []plugins.Problem{{Plugin: "beta", File: "defs/backstories.json", Def: "backstory/beta.smith", Field: "skills.Mining", Msg: "must be 0..20"}}
	j, saved := pluginsJourney(t, v)
	if !hasWidget(j.a, mainmenu.PluginProblemsID) || !strings.Contains(shownText(t, j.a), "1 plugin problem") {
		t.Fatalf("no notice for a plugin error:\n%s", shownText(t, j.a))
	}
	j.clickID(mainmenu.PluginProblemsID)
	if j.a.nav.screen() != ports.ScreenPluginProblems {
		t.Fatalf("View opened %s", j.a.nav.screen())
	}
	j.update(1)
	text := shownText(t, j.a)
	for _, want := range []string{"beta › defs/backstories.json › backstory/beta.smith › skills.Mining", "must be 0..20", "Disable Beta Pack"} {
		if !strings.Contains(text, want) {
			t.Errorf("the problems view doesn't show %q:\n%s", want, text)
		}
	}
	j.clickID(plugins.DisableID)
	if n := len(*saved); n == 0 || len((*saved)[n-1].Plugins.Enabled) != 0 {
		t.Fatalf("Disable saved %+v", *saved)
	}
	j.clickID(plugins.ProblemBackID)
	if j.a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("Back left %s", j.a.nav.screen())
	}
}

// TestJourneyPluginsEmpty: with no plugin installed the screen says where
// to put one.
func TestJourneyPluginsEmpty(t *testing.T) {
	j, _ := pluginsJourney(t, plugins.View{Folders: []string{"/game/mods", "/home/player/.config/rimworld-go/mods"}})
	j.clickID(ids.MenuPlugins)
	j.update(1)
	text := shownText(t, j.a)
	for _, want := range []string{"No plugins are installed.", "/game/mods", "/home/player/.config/rimworld-go/mods"} {
		if !strings.Contains(text, want) {
			t.Errorf("the empty Plugins screen doesn't show %q:\n%s", want, text)
		}
	}
}
