package ebiten

import (
	"log"

	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	wizardpkg "rimworld-go/internal/adapters/inbound/ebiten/wizard"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// options.Screen owns the Options screen, the settings store's UI and the
// save slots (ARCH-005); optionsHost, optionsSaver and optionsStory are the
// root's side of its Host, Saver and Story and newOptions builds the
// feature for NewApp. The rest of this file is the root-side half of the
// settings (applying a change to the renderer, window and sound), the save
// and window-close wiring, and the aliases and one-line wrappers that keep
// the root's other call sites unchanged until the sweep (SOLID_GO.md §11).

type (
	// SettingsHooks connect the Options screen to storage (zero value: the
	// defaults, nothing saved).
	SettingsHooks = options.Hooks
	// SaveSlot is one save as the load screen lists it (ports.SaveSlot).
	SaveSlot = ports.SaveSlot
)

// ---- the feature's shell

type optionsHost struct{ a *App }

var _ options.Host = optionsHost{}

// T and Tf serve the catalog (ports.Texts) as the Host's own.
func (h optionsHost) T(src string) string { return h.a.shell.T(src) }

func (h optionsHost) Tf(src string, args ...any) string { return h.a.shell.Tf(src, args...) }

// optionsShell is the feature's view of the App: the ports roles plus its
// Host, Saver and Story.
type optionsShell struct {
	shell
	optionsHost
	optionsSaver
	optionsStory
}

func newOptions(a *App) *options.Screen {
	return options.New(optionsShell{a.shell, optionsHost{a}, optionsSaver{a}, optionsStory{a}})
}

// renderLimit is the physical size the render cap (ui_scale.go, PERF-003)
// holds the frame to, and whether it is holding it (the screen asks for
// more than the cap allows).
func (h optionsHost) renderLimit() (w, ht int, capped bool) {
	a := h.a
	if a.cfg.Capture != nil || !a.settings.Graphics.HiDPI {
		return a.pw, a.ph, false
	}
	return a.pw, a.ph, a.scale.Dev < a.devRaw-1e-9
}

// Env is the program as of now.
func (h optionsHost) Env() options.Env {
	a := h.a
	return options.Env{
		Running: a.renderer != nil, Commitment: a.commitment(), Capture: a.cfg.Capture != nil,
		CanSave: a.canSave(), CanLoad: a.canLoad(), Hardware: a.cfg.Hardware,
		DeviceScale: a.deviceScale, RenderLimit: h.renderLimit, UIScale: stepInUse(a.scale, a.devRaw), Bindings: a.bindings(),
		Pressed: a.in.Raw.Pressed, Mods: a.in.Raw.Mods, LangDir: a.shell.shellTexts.dir,
	}
}

// SetSettings applies changed settings everywhere and stores them.
func (h optionsHost) SetSettings(s config.Settings) { h.a.setSettings(s) }

// Cue plays a UI sound.
func (h optionsHost) Cue(id string) { h.a.cfg.Sound.Play(id, 1, 0) }

// Toast shows a short message.
func (h optionsHost) Toast(text string) { h.a.chrome.Toast(text) }

// Send enqueues a command through the renderer and applies it at once.
func (h optionsHost) Send(c command.Command) {
	r := h.a.renderer
	if r == nil {
		return
	}
	h.a.shell.Enqueue(c) // the renderer's queue: applies at once while paused, and the lessons see it
	if p, ok := r.game.(pendingApplier); ok {
		p.ApplyPending()
	}
}

// PaintMenu draws the menu backdrop, the title, a body and the buttons.
func (h optionsHost) PaintMenu(dst *ebiten.Image, sw, sh int, title string, body []string, buttons []uikit.Button) {
	h.a.paintMenu(dst, sw, sh, title, body, buttons)
}

// Rebind is the Controls tab's capture state (it stays on the App while
// other root files read it).
func (h optionsHost) Rebind() *options.RebindState { return &h.a.rebind }

// optionsSaver is the saved-game storage as the screen uses it.
type optionsSaver struct{ a *App }

var _ options.Saver = optionsSaver{}

// Save writes the running colony to slot.
func (s optionsSaver) Save(slot string) error {
	if s.a.renderer == nil {
		return nil
	}
	if err := s.a.saveHook()(s.a.renderer.game, slot); err != nil {
		return err
	}
	if h := s.a.cfg.Saves.SaveView; h != nil { // VIEW-001: every map's camera rides beside the save
		v := s.a.renderer.scenes.views(s.a.mapViewport())
		if err := h(slot, v); err != nil {
			log.Printf("save camera view: %v", err)
		}
	}
	return nil
}

// List reads the save list.
func (s optionsSaver) List() ([]ports.SaveSlot, error) { return s.a.cfg.Saves.List() }

// Load replaces the session with the save at path.
func (s optionsSaver) Load(path string) (string, clock.Tick, error) {
	sim, slot, err := s.a.cfg.Saves.Load(path)
	if err != nil {
		return "", 0, err
	}
	s.a.dropSession()
	s.a.startSession(sim)
	reportColony(s.a.cfg.Usage, "load", sim)
	if h := s.a.cfg.Saves.LoadView; h != nil && s.a.renderer != nil {
		if v, ok := h(path); ok {
			s.a.renderer.scenes.resumeViews(v)
		}
	}
	return slot, sim.Tick(), nil
}

// optionsStory is what the Story and Habits tabs borrow from the wizard and
// the colony.
type optionsStory struct{ a *App }

var _ options.Story = optionsStory{}

// Rules is the colony's habit rules, from the read model.
func (h optionsStory) Rules() gameplay.Settings {
	if h.a.renderer == nil {
		return gameplay.Settings{}
	}
	return h.a.renderer.game.World().Gameplay
}

// StoryView is the colony's storyteller, difficulty and save mode.
func (s optionsStory) StoryView() game.StoryView {
	if s.a.renderer == nil {
		return game.StoryView{}
	}
	return s.a.renderer.game.Story()
}

// PaintCard draws a storyteller's card.
func (s optionsStory) PaintCard(dst *ebiten.Image, r gfx.Rect, p storyteller.Profile, selected, hot bool) {
	s.a.wiz.PaintStoryCard(dst, r, p, selected, hot)
}

// CardTip is a storyteller card's hover text.
func (s optionsStory) CardTip(p storyteller.Profile) string {
	return wizardpkg.ModeCaptionIn(&s.a.shell.shellTexts, p.Mode) + ". " + wizardpkg.PacingTipIn(&s.a.shell.shellTexts, p)
}

// PresetTip is a difficulty row's hover text.
func (s optionsStory) PresetTip(n difficulty.Named) string { return wizardpkg.PresetTip(n) }

// SaveMode names a save mode and says what it does.
func (s optionsStory) SaveMode(commitment bool) (label, text string) {
	m := wizardpkg.SaveModes[0]
	if commitment {
		m = wizardpkg.SaveModes[1]
	}
	return m.Label, m.Text
}

// OpenCustom opens the dialog on the colony's current values.
func (s optionsStory) OpenCustom(v game.StoryView, accept func(difficulty.Settings)) {
	a := s.a
	from, ok := difficulty.Parse(v.Preset)
	if !ok || from == difficulty.Custom {
		from = difficulty.Seasoned
	}
	cd := wizardpkg.CustomDiff{S: v.Difficulty, From: from}
	formH := new(float32)
	a.uiPushModal(modal{
		ID: options.StCustomMod, Title: a.shell.T("Custom difficulty"), W: wizardpkg.DiffModalW, H: wizardpkg.DiffModalH,
		Draw: func(a *App, body gfx.Rect) { a.storyCustomBody(body, &cd, formH) },
		Buttons: []uikit.ModalButton{
			{ID: "accept", Label: a.shell.T("Accept"), Style: uikit.BtnPrimary, Closes: true, OnClick: func() { accept(cd.S) }},
			{ID: "reset", Label: a.shell.T("Reset"), OnClick: func() { cd.S = difficulty.Of(cd.From) }},
			{ID: "cancel", Label: a.shell.T("Cancel"), Closes: true},
		},
	})
}

// storyCustomBody is the wizard's custom-difficulty form over the dialog's
// own working copy instead of the wizard's draft.
func (a *App) storyCustomBody(body gfx.Rect, cd *wizardpkg.CustomDiff, formH *float32) {
	top, rest := body.SplitV(wizardpkg.DiffTopRowH)
	lab := uikit.DimText()
	a.uiSt().Label(top.X, float32(gfx.CenterTop(float64(top.Y), float64(top.H), lab)), a.shell.T("Start from:"), lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	a.ngChoice("diff.from", gfx.Rect{X: top.X + 90, Y: top.Y + 2, W: 170, H: top.H - 4}, a.shell.T("Start from"), names, int(cd.From), "", func(i int) { cd.From = difficulty.Preset(i) })
	a.ngButton("diff.copy", gfx.Rect{X: top.X + 272, Y: top.Y + 2, W: 150, H: top.H - 4}, a.shell.T("Copy"), uikit.BtnOpts{Tip: a.shell.T("Copy\nCopy every value of the preset into Custom.")},
		func() { cd.S = difficulty.Of(cd.From) })

	area := a.uiSt().ScrollArea(options.StFormID, rest, *formH, uikit.ScrollOpts{Step: 40})
	colW := (area.View.W - 8 - wizardpkg.DiffColGap) / 2
	left := &uikit.Form{View: area.View, X: area.View.X + 4, W: colW, Off: area.Off}
	right := &uikit.Form{View: area.View, X: area.View.X + 4 + colW + wizardpkg.DiffColGap, W: colW, Off: area.Off}
	s := &cd.S

	left.SectionTitle(a, a.shell.T("Threats"))
	a.diffSlider(left, "ts", a.shell.T("Threat scale"), &s.ThreatScale, "ts", wizardpkg.PctText, "", a.shell.T("Threat scale\nHow big raids and packs are, as a share of the normal size."))
	a.diffCheck(left, "bt", a.shell.T("Big threats"), &s.BigThreats, "", a.shell.T("Big threats\nRaids and packs. Off leaves only small events."))
	a.diffStepper(left, "pd", a.shell.T("Peaceful start (days)"), &s.PeacefulDays, 0, 30, a.shell.T("Peaceful start\nDays at the start with no big threat."))
	a.diffCheck(left, "ph", a.shell.T("Predators hunt colonists"), &s.PredatorsHuntColonists, "", a.shell.T("Predators\nWild predators may attack colonists, not only prey."))
	a.diffSlider(left, "ar", a.shell.T("Hostile wildlife revenge"), &s.AnimalRevenge, "ar", wizardpkg.PctText, "", a.shell.T("Revenge\nHow fiercely wild animals fight back when hurt."))
	a.diffCheck(left, "qt", a.shell.T("Quest threats"), &s.QuestThreats, "", a.shell.T("Quest threats\nQuests may bring raids and other dangers. Off offers only quests without fighting."))
	left.Gap(8)

	left.SectionTitle(a, a.shell.T("Escalation"))
	a.diffCheck(left, "ad", a.shell.T("Adaptation"), &s.Adaptation, "", a.shell.T("Adaptation\nThreats grow while the colony loses nobody, and ease after a death."))
	adaptOff := ""
	if !s.Adaptation {
		adaptOff = a.shell.T("Only used with adaptation")
	}
	a.diffSlider(left, "as", a.shell.T("Adaptation strength"), &s.AdaptStrength, "as", wizardpkg.PctText, adaptOff, a.shell.T("Adaptation strength\nHow far adaptation can move the threat size."))
	a.diffCheck(left, "te", a.shell.T("Time-based escalation"), &s.TimeEscalation, "", a.shell.T("Time-based escalation\nThreats also grow with the years that pass."))
	if !s.TimeEscalation {
		s.IgnoreWealth = false
	}
	timeOff := ""
	if !s.TimeEscalation {
		timeOff = a.shell.T("Only used with time-based escalation")
	}
	a.diffSlider(left, "ep", a.shell.T("Escalation per year"), &s.EscPerYear, "ep", wizardpkg.PctText, timeOff, a.shell.T("Escalation per year\nHow much the threat size grows each year."))
	a.diffCheck(left, "iw", a.shell.T("Ignore wealth"), &s.IgnoreWealth, timeOff, a.shell.T("Ignore wealth\nThreat size follows time and people only, not what you own."))

	right.SectionTitle(a, a.shell.T("Economy"))
	a.diffSlider(right, "cy", a.shell.T("Crop yield"), &s.CropYield, "cy", wizardpkg.PctText, "", "")
	a.diffSlider(right, "my", a.shell.T("Mining yield"), &s.MineYield, "my", wizardpkg.PctText, "", "")
	a.diffSlider(right, "by", a.shell.T("Butchering yield"), &s.ButcherYield, "by", wizardpkg.PctText, "", "")
	a.diffSlider(right, "rs", a.shell.T("Research speed"), &s.ResearchSpeed, "rs", wizardpkg.PctText, "", "")
	a.diffSlider(right, "qr", a.shell.T("Quest rewards"), &s.QuestRewardPct, "qr", wizardpkg.PctText, "", a.shell.T("Quest rewards\nHow much quests pay, as a share of the normal amount."))
	right.Gap(8)

	right.SectionTitle(a, a.shell.T("Health and mind"))
	a.diffSlider(right, "mo", a.shell.T("Mood offset"), &s.MoodOffset, "mo", wizardpkg.SignedText, "", a.shell.T("Mood offset\nAdded to every colonist's mood."))
	a.diffSlider(right, "if", a.shell.T("Infection chance"), &s.Infection, "if", wizardpkg.PctText, "", "")
	a.diffSlider(right, "df", a.shell.T("Disease frequency"), &s.DiseaseFreq, "df", wizardpkg.PctText, "", a.shell.T("Disease frequency\nHow often flu, plague, malaria, sleeping sickness, gut worms and muscle parasites strike, from day 15 (malaria in warm lands, sleeping sickness in the tropics). 100% is the normal rate; 0% turns diseases off."))
	if row, ok := right.Row(18); ok {
		ns := gfx.Small
		ns.Color = uikit.TextDim
		a.uiSt().Label(row.X, row.Y, a.shell.T("How often diseases strike; 0% turns them off."), ns)
	}
	a.diffSlider(right, "pt", a.shell.T("Pain tolerance offset"), &s.PainTolerance, "pt", wizardpkg.SignedText, "", "")
	right.Gap(8)

	right.SectionTitle(a, a.shell.T("Nature"))
	a.diffCheck(right, "ew", a.shell.T("Extreme weather"), &s.ExtremeWeather, "", a.shell.T("Extreme weather\nHeat waves and cold snaps."))

	*formH = max(left.Content(), right.Content())
}

// ---- storage hooks

// SaveHooks connects the UI to saved-game storage. Any nil hook disables
// that feature.
type SaveHooks struct {
	Save func(sim Simulation, slot string) error
	List func() ([]ports.SaveSlot, error)
	Load func(path string) (Simulation, string, error) // sim, slot name
	// SaveCommit saves a commitment-mode colony to its one slot (NG-1 §2.13);
	// nil falls back to Save.
	SaveCommit func(sim Simulation, slot string) error
	// SaveView and LoadView keep each map's camera beside a save
	// (VIEW-001, MAP-1); nil leaves the cameras to centre as before.
	SaveView func(slot string, v map[game.MapID]camerapkg.View) error
	LoadView func(path string) (map[game.MapID]camerapkg.View, bool)
}

// commitment reports a commitment colony is running (read through the
// simulation's story read model).
func (a *App) commitment() bool { return a.renderer != nil && a.renderer.game.Story().Commitment }

// saveHook is how the running colony saves: SaveCommit for a commitment
// colony when wired, else Save.
func (a *App) saveHook() func(Simulation, string) error {
	if a.cfg.Saves.SaveCommit != nil && a.commitment() {
		return a.cfg.Saves.SaveCommit
	}
	return a.cfg.Saves.Save
}

func (a *App) canSave() bool { return a.renderer != nil && a.saveHook() != nil }
func (a *App) canLoad() bool { return a.cfg.Saves.List != nil && a.cfg.Saves.Load != nil }

// windowHooks is the OS window's close handling (faked in tests).
type windowHooks interface {
	SetClosingHandled(bool)
	BeingClosed() bool
}

type ebitenWindow struct{}

func (ebitenWindow) SetClosingHandled(v bool) { ebiten.SetWindowClosingHandled(v) }
func (ebitenWindow) BeingClosed() bool        { return ebiten.IsWindowBeingClosed() }

// handleWindowClose takes over the window's close button while a
// commitment colony runs, and reports when the game should end: the
// window is being closed, the colony has been saved. It quits even when
// that save fails, so a broken disk never traps the player in the window.
func (a *App) handleWindowClose() bool {
	want := a.commitment() && a.cfg.Capture == nil
	if want != a.closeHandled {
		a.cfg.WindowHooks.SetClosingHandled(want)
		a.closeHandled = want
	}
	if !a.closeHandled || !a.cfg.WindowHooks.BeingClosed() {
		return false
	}
	a.opt.SaveBeforeLeaving()
	return true
}

// hardwareTotal is the computer's memory in bytes: 0 when the hardware is
// unknown (no reader, or it could not tell). compose/game reads the hardware
// once at startup, so asking every frame costs nothing.
func (a *App) hardwareTotal() uint64 {
	if a.cfg.Hardware == nil {
		return 0
	}
	return a.cfg.Hardware().Total
}

// ---- settings: the root's half (what a change does to the program)

// loadSettings reads the stored settings once at startup
// (options.LoadSettings detects the graphics tier on a first launch).
func (a *App) loadSettings() {
	a.binds = nil // rebuilt from the loaded controls
	s, hint := options.LoadSettings(a.cfg.Settings, options.Detect{
		Skip:     a.cfg.Capture != nil || a.cfg.FPSProbe != nil,
		Hardware: a.cfg.Hardware, DeviceScale: a.deviceScale,
	})
	a.settings = s
	if hint != "" {
		a.hint = hint
	}
}

// setSettings applies a change everywhere and stores it.
func (a *App) setSettings(s config.Settings) {
	before := a.settings.Gameplay.Colonies()
	a.settings = s.Normalize()
	a.applySettings()
	if n := a.settings.Gameplay.Colonies(); n != before && a.renderer != nil {
		a.renderer.enqueue(command.Command{Kind: command.SetColonyCap, Count: n}) // MAP-06: the one setting the rules read
	}
	if hint := options.SaveSettings(a.cfg.Settings, a.settings); hint != "" {
		a.hint = hint
	}
}

// applySettings pushes the settings into the UI scale, the window, the
// colony renderer and the sound.
func (a *App) applySettings() {
	a.binds = input.FromSettings(a.settings.Controls.Bindings)
	a.updateScale() // UI scale and HiDPI apply live (ui_scale.go)
	if !a.settings.Gameplay.DevMode {
		*a.tools.State = devtools.State{}
	}
	if a.cfg.Window != nil {
		a.cfg.Window(a.settings.Graphics.Fullscreen, a.settings.Graphics.VSync)
	}
	if a.cfg.Display != nil { // OPT-029: window size preset, borderless
		a.cfg.Display(a.settings.Camera.Window, a.settings.Camera.Borderless)
	}
	if a.cfg.Cursor != nil && !a.settings.Camera.CustomCursor { // OPT-028: off gives the OS cursor back
		a.cfg.Cursor(true)
	}
	a.shell.use(a.settings.General.Language) // OPT-001
	a.syncRenderer()
	a.cfg.Sound.SetVolumes(a.settings.Audio)
}

// syncRenderer copies the presentation settings into the colony renderer.
func (a *App) syncRenderer() {
	r := a.renderer
	if r == nil {
		return
	}
	s := a.settings
	started := r.smooth.Prefs.Started
	r.fx = s.Graphics.FX()
	applyGraphicsKnobs(r, s.Graphics)
	r.edgeScroll = s.Gameplay.EdgeScroll && a.cfg.Capture == nil
	r.smooth.Speed = float64(s.Gameplay.PanSpeed) / 100
	r.reducedFlashing = s.Accessibility.ReducedFlashing
	r.reducedMotion = s.Accessibility.ReducedMotion
	r.view.Prefs = render.Prefs{AnimalNames: s.Interface.AnimalNames, FarPawns: s.Interface.FarPawns, FarStyle: s.Interface.FarStyle,
		StillPlants: !s.Camera.PlantSway, HatsOnlyOnMap: s.Interface.HatsOnlyOnMap}
	r.smooth.Instant = s.Accessibility.ReducedMotion // no eased pan or zoom
	still := s.Accessibility.ReducedMotion || a.cfg.Capture != nil
	pf := &r.smooth.Prefs
	pf.CenterZoom, pf.Glide, pf.Drag = !s.Camera.ZoomToMouse, s.Camera.SmoothJumps && !still, s.Camera.DragPct
	pf.WorldOut, pf.Shake, pf.Mode = s.Camera.ZoomOutToWorld, shakeStrength(s.Camera.ShakePct, still), s.General.AutoPause
	pf.UnpauseLast, pf.Started = s.Interface.UnpauseSpeed == config.UnpauseLast, true
	if still {
		r.smooth.Fx.Clear()
	}
	if !started && s.General.PauseOnLoad && a.cfg.Capture == nil && r.game.Tick() > 0 { // OPT-004: a loaded game waits for the player
		r.smooth.Prefs.LastSpeed = int(r.speed)
		r.speed = speedPaused
	}
	r.overlays().TempDraw.Units = units.FromUnit(s.Gameplay.TempUnit)
	if !s.Gameplay.DevMode && r.speed == speedUltra {
		r.speed = speedVeryFast // ultrafast is a developer tier
	}
}

// applyGraphicsKnobs sets the renderer's detail knobs from the graphics
// settings (PLAY-019): the colonist shadows, the normal-map lighting, the
// water ripple and the grass tufts per tile. Each is read live, so an
// Options change shows on the next frame.
func applyGraphicsKnobs(r *Renderer, g config.Graphics) {
	r.env.Shadow.SetOff(!g.Shadows)
	r.lit.Off = !g.Lighting
	r.water.Simple = g.Water == config.WaterSimple
	r.pipe().LOD.Grass.Level = g.Grass
}

// shakeStrength is the shake multiplier for a percentage; reduced motion and
// captures get none.
func shakeStrength(pct int, still bool) float64 {
	if still || pct <= 0 {
		return 0
	}
	return float64(pct) / 100
}
