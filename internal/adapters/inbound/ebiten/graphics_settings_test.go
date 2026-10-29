package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/perf"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/sysinfo"
)

// PLAY-019: hardware-aware graphics defaults at first launch and the
// Options > Graphics rows that change them.

// hwApp is an App on store whose machine has total bytes of memory and
// cores cores, on a screen with device scale dev. Nothing is loaded yet:
// tests call a.loadSettings() themselves, as the real start does.
func hwApp(store *fakeSettings, total uint64, cores int, dev float64) *App {
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Settings = store.hooks()
	a.cfg.Hardware = func() sysinfo.Info { return sysinfo.Info{Total: total, Cores: cores} }
	a.devFn = func() float64 { return dev }
	return a
}

// TestFirstLaunchAppliesTierOfTheMachine: with no settings file yet the
// hardware picks the preset, and it is saved once.
func TestFirstLaunchAppliesTierOfTheMachine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		total uint64
		cores int
		tier  string
		want  func(g config.Graphics) bool
	}{
		{"4 GiB machine is low", 4 * sysinfo.GiB, 4, config.QualityLow, func(g config.Graphics) bool {
			return !g.Shadows && !g.Lighting && g.Water == config.WaterSimple && g.Grass == config.GrassSparse && g.Effects == config.EffectsMinimal
		}},
		{"8 GiB machine is medium", 8 * sysinfo.GiB, 4, config.QualityMedium, func(g config.Graphics) bool {
			return g.Shadows && g.Lighting && g.Water == config.WaterSimple && g.Grass == config.GrassReduced && g.Effects == config.EffectsReduced
		}},
		{"16 GiB, 8 cores is high", 16 * sysinfo.GiB, 8, config.QualityHigh, func(g config.Graphics) bool {
			return g.Shadows && g.Lighting && g.Water == config.WaterAnimated && g.Grass == config.GrassFull && g.Effects == config.EffectsFull
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeSettings{s: config.Defaults()}
			a := hwApp(store, c.total, c.cores, 1)
			a.loadSettings()
			g := a.settings.Graphics
			if g.Detected != c.tier || g.Quality != c.tier || !c.want(g) {
				t.Fatalf("graphics = %+v, want the %s preset", g, c.tier)
			}
			if store.saves != 1 || store.s.Graphics.Detected != c.tier {
				t.Fatalf("saved %d times, stored detected %q; want one save of %s", store.saves, store.s.Graphics.Detected, c.tier)
			}
		})
	}
}

// TestFirstLaunchDropsHiDPIOnMediumAboveTwoX: the medium preset keeps HiDPI
// only up to a device scale of 2.
func TestFirstLaunchDropsHiDPIOnMediumAboveTwoX(t *testing.T) {
	t.Parallel()
	for dev, want := range map[float64]bool{1: true, 2: true, 3: false} {
		store := &fakeSettings{s: config.Defaults()}
		a := hwApp(store, 8*sysinfo.GiB, 4, dev)
		a.loadSettings()
		if a.settings.Graphics.HiDPI != want {
			t.Fatalf("device scale %v: HiDPI %v, want %v", dev, a.settings.Graphics.HiDPI, want)
		}
	}
}

// TestSecondStartDoesNotDetectAgain: Detected is remembered, so the next
// start keeps the stored rows and does not save.
func TestSecondStartDoesNotDetectAgain(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	hwApp(store, 4*sysinfo.GiB, 4, 1).loadSettings()
	if store.saves != 1 {
		t.Fatalf("first start saved %d times", store.saves)
	}
	b := hwApp(store, 32*sysinfo.GiB, 16, 1) // a bigger machine: the stored choice still stands
	b.loadSettings()
	if store.saves != 1 || b.settings.Graphics.Quality != config.QualityLow || b.settings.Graphics.Detected != config.QualityLow {
		t.Fatalf("second start: %d saves, graphics %+v", store.saves, b.settings.Graphics)
	}
}

// TestUnknownHardwareLeavesDetectionForLater: without a hardware reader the
// rows keep today's look, nothing is saved and Detected stays empty so a
// later start detects.
func TestUnknownHardwareLeavesDetectionForLater(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := hwApp(store, 4*sysinfo.GiB, 4, 1)
	a.cfg.Hardware = nil
	a.loadSettings()
	if a.settings.Graphics.Detected != "" || store.saves != 0 || a.settings.Graphics.Quality != config.QualityHigh {
		t.Fatalf("detected %q after %d saves, quality %q", a.settings.Graphics.Detected, store.saves, a.settings.Graphics.Quality)
	}
}

// TestPreV5FileKeepsHandChangedValues: a settings file from before PLAY-019
// that changed Effects and turned HiDPI off keeps both as the player's
// choice; the rest takes the detected tier. config.Decode has already
// stamped such a file with the current version, so both are covered.
// (VSync off is the default since PERF-002, so it no longer reads as a
// choice on an old file.)
func TestPreV5FileKeepsHandChangedValues(t *testing.T) {
	t.Parallel()
	for _, version := range []int{4, config.SettingsVersion} {
		old := config.Defaults()
		old.Version = version
		old.Graphics.Effects = config.EffectsMinimal // the player turned effects down by hand
		old.Graphics.HiDPI = false
		store := &fakeSettings{s: old}
		a := hwApp(store, 16*sysinfo.GiB, 8, 1)
		a.loadSettings()
		g := a.settings.Graphics
		if g.Effects != config.EffectsMinimal || g.HiDPI || g.VSync {
			t.Fatalf("v%d: hand-changed rows overwritten: effects %q hidpi %v vsync %v", version, g.Effects, g.HiDPI, g.VSync)
		}
		if !slices.Contains(g.Chosen, "effects") || !slices.Contains(g.Chosen, "hidpi") || slices.Contains(g.Chosen, "vsync") {
			t.Fatalf("v%d: chosen = %v, want effects and hidpi only", version, g.Chosen)
		}
		if g.Detected != config.QualityHigh || g.Quality != config.QualityCustom || !g.Shadows || g.Grass != config.GrassFull {
			t.Fatalf("v%d: graphics = %+v, want the high tier with the chosen rows kept (custom)", version, g)
		}
		if store.saves != 1 || store.s.Version != config.SettingsVersion {
			t.Fatalf("v%d: saved %d times at version %d", version, store.saves, store.s.Version)
		}
	}
}

// TestStoreThatCannotSaveKeepsTheRows: the read-only store of the -hidpi
// developer override is not detected into (that would overwrite the flag).
func TestStoreThatCannotSaveKeepsTheRows(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := hwApp(store, 4*sysinfo.GiB, 4, 1)
	a.cfg.Settings.Save = nil
	a.loadSettings()
	if a.settings.Graphics.Detected != "" || a.settings.Graphics.Quality != config.QualityHigh {
		t.Fatalf("graphics = %+v, want them untouched", a.settings.Graphics)
	}
}

// TestPreV5FileWithOldDefaultsTakesTheTier: an untouched old file follows
// the hardware like a first launch.
func TestPreV5FileWithOldDefaultsTakesTheTier(t *testing.T) {
	t.Parallel()
	old := config.Defaults()
	old.Version = 4
	store := &fakeSettings{s: old}
	a := hwApp(store, 4*sysinfo.GiB, 4, 1)
	a.loadSettings()
	g := a.settings.Graphics
	if len(g.Chosen) != 0 || g.Quality != config.QualityLow || g.Effects != config.EffectsMinimal {
		t.Fatalf("graphics = %+v, want the low preset", g)
	}
}

// TestCapturesAndProbeNeverDetect: a screenshot session and the fps probe
// keep their deterministic defaults and never touch the stored settings.
func TestCapturesAndProbeNeverDetect(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(a *App){
		"capture": func(a *App) { a.cfg.Capture = &Capture{} },
		"probe":   func(a *App) { a.cfg.FPSProbe = &perf.FPSProbe{} },
	} {
		store := &fakeSettings{s: config.Defaults()}
		a := hwApp(store, 4*sysinfo.GiB, 2, 1)
		set(a)
		a.loadSettings()
		g := a.settings.Graphics
		if g.Detected != "" || g.Quality != config.QualityHigh || !g.Shadows || store.saves != 0 {
			t.Fatalf("%s: graphics %+v, %d saves; want the untouched defaults", name, g, store.saves)
		}
	}
}

// graphicsStore is a store already detected as high.
func graphicsStore() *fakeSettings {
	s := config.Defaults()
	s.Graphics.Detected = config.QualityHigh
	return &fakeSettings{s: s}
}

// TestOptionsGraphicsRowsRecordTheChoice: each detail row changes its
// setting, is recorded in Chosen and turns the preset into Custom.
func TestOptionsGraphicsRowsRecordTheChoice(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		row, key string
		changed  func(g config.Graphics) bool
	}{
		{"effects", "effects", func(g config.Graphics) bool { return g.Effects == config.EffectsReduced }},
		{"vsync", "vsync", func(g config.Graphics) bool { return g.VSync }}, // off by default since PERF-002
		{"hidpi", "hidpi", func(g config.Graphics) bool { return !g.HiDPI }},
		{"shadows", "shadows", func(g config.Graphics) bool { return !g.Shadows }},
		{"lighting", "lighting", func(g config.Graphics) bool { return !g.Lighting }},
		{"water", "water", func(g config.Graphics) bool { return g.Water == config.WaterSimple }},
		{"grass", "grass", func(g config.Graphics) bool { return g.Grass == config.GrassReduced }},
		{"renderscale", "renderscale", func(g config.Graphics) bool { return g.RenderScale == 0.75 }},
		{"framecap", "framecap", func(g config.Graphics) bool { return g.FrameCap == 144 }},
	} {
		store := graphicsStore()
		a := optionsApp(store)
		a.opt.Tab = "graphics"
		a.opt.ApplyOption("options." + c.row)
		g := store.s.Graphics
		if !c.changed(g) {
			t.Fatalf("%s: not changed in the stored settings: %+v", c.row, g)
		}
		if !slices.Contains(g.Chosen, c.key) || g.Quality != config.QualityCustom || store.saves != 1 {
			t.Fatalf("%s: chosen %v quality %q saves %d; want recorded, custom, one save", c.row, g.Chosen, g.Quality, store.saves)
		}
		if got := optionBtn(t, a, "options.quality").Label; got != "Custom" {
			t.Fatalf("%s: quality row reads %q, want Custom", c.row, got)
		}
	}
}

// TestOptionsFullscreenIsNotAPresetRow: Fullscreen is the window's, not a
// quality row, so changing it leaves the preset alone.
func TestOptionsFullscreenIsNotAPresetRow(t *testing.T) {
	t.Parallel()
	store := graphicsStore()
	a := optionsApp(store)
	a.opt.Tab = "graphics"
	a.opt.ApplyOption("options.fullscreen")
	g := store.s.Graphics
	if !g.Fullscreen || g.Quality != config.QualityHigh || len(g.Chosen) != 0 {
		t.Fatalf("graphics = %+v", g)
	}
}

// TestOptionsQualityAppliesPresetAndClearsChoices: picking a preset sets
// every row, drops the hand-picked marks and reads High/Medium/Low.
func TestOptionsQualityAppliesPresetAndClearsChoices(t *testing.T) {
	t.Parallel()
	store := graphicsStore()
	a := optionsApp(store)
	a.devFn = func() float64 { return 2 }
	a.opt.Tab = "graphics"
	a.opt.ApplyOption("options.shadows") // custom, shadows chosen
	if g := store.s.Graphics; g.Quality != config.QualityCustom || g.Shadows {
		t.Fatalf("setup: %+v", g)
	}
	steps := []struct {
		label string
		check func(g config.Graphics) bool
	}{
		{"High", func(g config.Graphics) bool { // custom -> High
			return g.Quality == config.QualityHigh && g.Shadows && g.Lighting && g.Water == config.WaterAnimated && g.Grass == config.GrassFull && g.HiDPI
		}},
		{"Medium", func(g config.Graphics) bool {
			return g.Quality == config.QualityMedium && g.Effects == config.EffectsReduced && g.Water == config.WaterSimple && g.Grass == config.GrassReduced && g.HiDPI
		}},
		{"Low", func(g config.Graphics) bool {
			return g.Quality == config.QualityLow && !g.Shadows && !g.Lighting && g.Effects == config.EffectsMinimal && g.Grass == config.GrassSparse && !g.HiDPI
		}},
		{"High", func(g config.Graphics) bool { return g.Quality == config.QualityHigh && g.Shadows && g.Lighting }},
	}
	for i, s := range steps {
		a.opt.ApplyOption("options.quality")
		g := store.s.Graphics
		if !s.check(g) || len(g.Chosen) != 0 {
			t.Fatalf("step %d: graphics %+v, want the %s preset with no chosen rows", i, g, s.label)
		}
		if got := optionBtn(t, a, "options.quality").Label; got != s.label {
			t.Fatalf("step %d: quality row reads %q, want %s", i, got, s.label)
		}
		if g.Detected != config.QualityHigh {
			t.Fatalf("step %d: a preset pick changed Detected to %q", i, g.Detected)
		}
	}
}

// TestOptionsDetectAgain: re-reads the hardware, applies its tier and
// forgets the hand-picked rows.
func TestOptionsDetectAgain(t *testing.T) {
	t.Parallel()
	store := graphicsStore()
	a := optionsApp(store)
	a.cfg.Hardware = func() sysinfo.Info { return sysinfo.Info{Total: 4 * sysinfo.GiB, Cores: 2} }
	a.devFn = func() float64 { return 1 }
	a.opt.Tab = "graphics"
	a.opt.ApplyOption("options.lighting") // a hand-picked row
	if b := optionBtn(t, a, "options.quality.detect"); b.Disabled || b.Tip == "" {
		t.Fatalf("detect button %+v", b)
	}
	saves := store.saves
	a.opt.ApplyOption("options.quality.detect")
	g := store.s.Graphics
	if g.Detected != config.QualityLow || g.Quality != config.QualityLow || len(g.Chosen) != 0 || g.VSync || g.FrameCap != config.FrameCapDefault {
		t.Fatalf("graphics = %+v, want the low tier applied and nothing chosen", g)
	}
	if g.Lighting || g.Shadows || g.Grass != config.GrassSparse {
		t.Fatalf("low rows not applied: %+v", g)
	}
	if store.saves != saves+1 {
		t.Fatalf("saves %d -> %d, want one", saves, store.saves)
	}
}

// TestOptionsDetectAgainNeedsHardware: with no hardware reader the button
// is off and says why.
func TestOptionsDetectAgainNeedsHardware(t *testing.T) {
	t.Parallel()
	a := optionsApp(graphicsStore())
	a.opt.Tab = "graphics"
	b := optionBtn(t, a, "options.quality.detect")
	if !b.Disabled || b.Reason == "" {
		t.Fatalf("detect button %+v, want it disabled with a reason", b)
	}
}

// TestOptionsGraphicsRowsHaveCostTips: every detail row says what it costs.
func TestOptionsGraphicsRowsHaveCostTips(t *testing.T) {
	t.Parallel()
	a := optionsApp(graphicsStore())
	a.opt.Tab = "graphics"
	for _, id := range []string{"quality", "effects", "shadows", "lighting", "water", "grass", "hidpi", "vsync", "renderscale"} {
		if b := optionBtn(t, a, "options."+id); len(b.Tip) < 20 {
			t.Fatalf("%s tip = %q, want a sentence", id, b.Tip)
		}
	}
}

// TestOptionsGraphicsLabelsFitTheirColumn: with two columns every label
// ends before its control (the Gameplay tab sets the one-column limit).
func TestOptionsGraphicsLabelsFitTheirColumn(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	rows := options.OptionRows("graphics")
	if !slices.ContainsFunc(rows, func(r options.OptionRow) bool { return r.Key == "renderscale" && r.Col == 1 }) {
		t.Fatal("no World detail row in the detail column")
	}
	for i, r := range rows {
		lx, cx, y := options.OptPos(rows, i)
		if w, _ := gfx.Measure(r.Label, gfx.Body); lx+float32(w) > cx-8 {
			t.Fatalf("label %q (%v px at x=%v) runs into its control at x=%v", r.Label, w, lx, cx)
		}
		if y+options.OptBtnH > float32(540-footerYOff) {
			t.Fatalf("row %q at y=%v reaches the footer of a 540 px window", r.Key, y)
		}
	}
	a.opt.Tab = "graphics"
	for _, b := range a.opt.Buttons(960, 540) {
		if b.X+b.W > 960-40 { // inside the panel
			t.Fatalf("%s ends at x=%v, outside the 960 px panel", b.ID, b.X+b.W)
		}
	}
}

// TestSyncRendererReadsGraphics: the renderer takes the knobs live from
// the settings.
func TestSyncRendererReadsGraphics(t *testing.T) {
	t.Parallel()
	a := optionsApp(graphicsStore())
	a.startGame()
	r := a.renderer
	if r.env.Shadow.Off || r.lit.Off || r.water.Simple || r.lod.Grass.Level != "" && r.lod.Grass.Level != config.GrassFull {
		t.Fatalf("high defaults should leave every knob at today's look")
	}
	s := a.settings
	s.Graphics.Shadows, s.Graphics.Lighting = false, false
	s.Graphics.Water, s.Graphics.Grass = config.WaterSimple, config.GrassSparse
	a.setSettings(s)
	if !r.env.Shadow.Off || !r.lit.Off || !r.water.Simple || r.lod.Grass.Level != config.GrassSparse {
		t.Fatalf("knobs not synced: shadows off %v lighting off %v simple water %v grass %q",
			r.env.Shadow.Off, r.lit.Off, r.water.Simple, r.lod.Grass.Level)
	}
	s.Graphics.Shadows, s.Graphics.Lighting = true, true
	s.Graphics.Water, s.Graphics.Grass = config.WaterAnimated, config.GrassFull
	a.setSettings(s)
	if r.env.Shadow.Off || r.lit.Off || r.water.Simple {
		t.Fatal("knobs not restored")
	}
}
