package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
)

// fakeSettings is an in-memory settings store that counts saves.
type fakeSettings struct {
	s     config.Settings
	saves int
}

func (f *fakeSettings) hooks() SettingsHooks {
	return SettingsHooks{
		Load: func() (config.Settings, error) { return f.s, nil },
		Save: func(s config.Settings) error { f.s = s; f.saves++; return nil },
	}
}

func optionsApp(store *fakeSettings) *App {
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Settings = store.hooks()
	a.loadSettings()
	a.finishBoot()
	return a
}

func TestOptionTabsLayout(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	for _, size := range [][2]int{{1280, 720}, {960, 540}} {
		for _, tab := range optionTabs() {
			a.optionsTab = tab
			bs := a.optionsButtons(size[0], size[1])
			seen := map[string]bool{}
			for i, b := range bs {
				if seen[b.id] {
					t.Fatalf("%s %v: duplicate id %s", tab, size, b.id)
				}
				seen[b.id] = true
				if b.x+b.w > float32(size[0]) || b.y+b.h > float32(size[1]) {
					t.Fatalf("%s %v: %s off screen", tab, size, b.id)
				}
				for _, o := range bs[i+1:] {
					if b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
						t.Fatalf("%s %v: %s overlaps %s", tab, size, b.id, o.id)
					}
				}
			}
			if !seen["options.back"] || !seen["options.tab."+tab] {
				t.Fatalf("%s: missing back or tab button", tab)
			}
		}
	}
}

func TestOptionsVolumeStepsClampAndSave(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	click(t, a, "menu.options")
	for i := 0; i < 5; i++ {
		click(t, a, "options.master.inc")
	}
	if a.settings.Audio.Master != 100 || store.s.Audio.Master != 100 {
		t.Fatalf("master = %d (stored %d), want clamped at 100", a.settings.Audio.Master, store.s.Audio.Master)
	}
	for i := 0; i < 12; i++ {
		click(t, a, "options.music.dec")
	}
	if a.settings.Audio.Music != 0 {
		t.Fatalf("music = %d, want 0", a.settings.Audio.Music)
	}
	if store.saves != 17 {
		t.Fatalf("saves = %d, want one per click", store.saves)
	}
}

func TestOptionsEffectsCyclesAndReachesRenderer(t *testing.T) {
	t.Parallel()
	a := optionsApp(&fakeSettings{s: config.Defaults()})
	a.startGame()
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.options")
	click(t, a, "options.tab.graphics")
	want := []string{config.EffectsReduced, config.EffectsMinimal, config.EffectsFull}
	fx := []float64{0.5, 0.2, 1}
	for i, w := range want {
		click(t, a, "options.effects")
		if a.settings.Graphics.Effects != w || a.renderer.fx != fx[i] {
			t.Fatalf("click %d: effects %q fx %v, want %q %v", i, a.settings.Graphics.Effects, a.renderer.fx, w, fx[i])
		}
	}
}

func TestOptionsTabSwitchChangesRows(t *testing.T) {
	t.Parallel()
	a := optionsApp(&fakeSettings{s: config.Defaults()})
	click(t, a, "menu.options")
	if a.currentOptionsTab() != "audio" {
		t.Fatalf("opens on %q, want audio", a.currentOptionsTab())
	}
	click(t, a, "options.tab.access")
	click(t, a, "options.reducedmotion")
	if !a.settings.Accessibility.ReducedMotion {
		t.Fatal("reduced motion not set")
	}
}

func TestSettingsSurviveRestart(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	click(t, a, "menu.options")
	click(t, a, "options.tab.gameplay")
	click(t, a, "options.edgescroll")
	click(t, a, "options.autosave")
	b := optionsApp(store)
	if !b.settings.Gameplay.EdgeScroll || b.settings.Gameplay.AutosaveDays != 3 {
		t.Fatalf("after restart: %+v", b.settings.Gameplay)
	}
}

func TestEdgePan(t *testing.T) {
	t.Parallel()
	v := viewport{x0: 0, y0: 40, x1: 1280, y1: 680}
	cases := []struct {
		mx, my int
		dx, dy float64
	}{
		{640, 360, 0, 0},
		{2, 360, -1, 0},
		{1278, 360, 1, 0},
		{640, 42, 0, -1},
		{640, 678, 0, 1},
		{1, 41, -1, -1},
		{640, 20, 0, 0}, // over the top bar: outside the view
		{-5, 360, 0, 0}, // off the window
	}
	for _, c := range cases {
		if dx, dy := edgePan(c.mx, c.my, v); dx != c.dx || dy != c.dy {
			t.Fatalf("edgePan(%d,%d) = %v,%v want %v,%v", c.mx, c.my, dx, dy, c.dx, c.dy)
		}
	}
}

func TestMenuParticlesFollowSettings(t *testing.T) {
	t.Parallel()
	s := config.Defaults()
	if e, m := menuParticles(s); e != menuEmbers || m != menuMotes {
		t.Fatalf("full effects = %d embers %d motes", e, m)
	}
	s.Graphics.Effects = config.EffectsMinimal
	if e, m := menuParticles(s); e >= menuEmbers || m >= menuMotes || e == 0 {
		t.Fatalf("minimal effects = %d embers %d motes", e, m)
	}
	s.Accessibility.ReducedMotion = true
	if e, m := menuParticles(s); e != 0 || m != 0 {
		t.Fatalf("reduced motion = %d embers %d motes, want none", e, m)
	}
}

func TestAutosaveInterval(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 4, h: 4}
	a := testApp(sim)
	saves := 0
	a.cfg.Saves = SaveHooks{Save: func(Simulation, string) error { saves++; return nil }}
	a.finishBoot()
	a.startGame()
	a.settings.Gameplay.AutosaveDays = 3
	for t0 := clock.Tick(0); t0 < 10*60000; t0 += 100 {
		sim.tick = t0
		a.tickAutosave()
	}
	if saves != 3 { // days 3, 6 and 9
		t.Fatalf("autosaves in 10 days every 3 = %d, want 3", saves)
	}
}
