package ebiten

import (
	architectpkg "rimworld-go/internal/adapters/inbound/ebiten/architect"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	portspkg "rimworld-go/internal/adapters/inbound/ebiten/ports"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The shell (ARCH-002) is what the features see of the App: one small
// type per ports role, composed into App.sh, plus the two tables the
// composition root fills (the screens' widget declarations and the click
// routes). Feature code reaches the renderer, the hint, the settings and
// the other features' state through these roles and nothing else, so a
// feature that moves into its own package keeps calling the same names on
// its Shell; the root stays the only place that knows the concrete types.
//
// Every role is safe on a menu screen (no renderer): reads return the
// zero value, writes do nothing.
type shell struct {
	shellHint
	shellSize
	shellCmd
	shellWorld
	shellView
	shellSel
	shellOverlay
	shellSpeed
	shellSettings
	shellPanels
	shellWidgets
	shellSaves
	shellDispatch

	screens map[portspkg.Screen]func(a *App, sw, sh int) // uiBegin: each screen's widgets
	routes  []clickRoute                                 // applyClick: by id or prefix, in order
}

func newShell(a *App) shell {
	return shell{
		shellHint{a}, shellSize{a}, shellCmd{a}, shellWorld{a}, shellView{a}, shellSel{a},
		shellOverlay{a}, shellSpeed{a}, shellSettings{a}, shellPanels{a}, shellWidgets{a}, shellSaves{a}, shellDispatch{a},
		screenTable(), clickRoutes(),
	}
}

var (
	_ ports.Hinter     = shellHint{}
	_ ports.Sizer      = shellSize{}
	_ ports.Commands   = shellCmd{}
	_ ports.World      = shellWorld{}
	_ ports.View       = shellView{}
	_ ports.PawnSel    = shellSel{}
	_ ports.Overlays   = shellOverlay{}
	_ ports.Speed      = shellSpeed{}
	_ ports.Settings   = shellSettings{}
	_ ports.Panels     = shellPanels{}
	_ ports.Widgets    = shellWidgets{}
	_ ports.Saves      = shellSaves{}
	_ ports.Dispatcher = shellDispatch{}
)

// ---- Hinter

type shellHint struct{ a *App }

// Hint shows s in the HUD's hint line.
func (s shellHint) Hint(text string) { s.a.hint = text }

// ---- Sizer

type shellSize struct{ a *App }

// Size is the logical screen.
func (s shellSize) Size() (sw, sh int) { return s.a.size() }

// Cursor is the pointer in logical pixels.
func (s shellSize) Cursor() (int, int) { return s.a.currentCursor() }

// MapCursor is the pointer in map pixels.
func (s shellSize) MapCursor() (int, int) { return s.a.mapCursor() }

// MapScale is the logical-to-map pixel factor.
func (s shellSize) MapScale() float64 { return s.a.mapScale() }

// ---- Commands

type shellCmd struct{ a *App }

// Enqueue sends a command through the renderer's queue.
func (s shellCmd) Enqueue(c command.Command) {
	if r := s.a.renderer; r != nil {
		r.enqueue(c)
	}
}

// ---- World

type shellWorld struct{ a *App }

// World is this frame's world view.
func (s shellWorld) World() game.WorldView {
	if r := s.a.renderer; r != nil {
		return r.world()
	}
	return game.WorldView{}
}

// Pawns is this frame's pawn views.
func (s shellWorld) Pawns() []game.PawnView {
	if r := s.a.renderer; r != nil {
		return r.snapshot()
	}
	return nil
}

// MapSize is the map in tiles.
func (s shellWorld) MapSize() (w, h int) {
	if r := s.a.renderer; r != nil {
		return r.game.Width(), r.game.Height()
	}
	return 0, 0
}

// Tick is the simulation tick.
func (s shellWorld) Tick() clock.Tick {
	if r := s.a.renderer; r != nil {
		return r.game.Tick()
	}
	return 0
}

// Date is the calendar date.
func (s shellWorld) Date() clock.Date {
	if r := s.a.renderer; r != nil {
		return r.game.Date()
	}
	return clock.Date{}
}

// ---- View

type shellView struct{ a *App }

// TileAt is the tile under a map-pixel point (the caller derives it from
// MapCursor; world_scale_test checks the callers).
func (s shellView) TileAt(mx, my int) (int, int) {
	if r := s.a.renderer; r != nil {
		return r.TileAt(mx, my)
	}
	return 0, 0
}

// TilePx is the tile size in map pixels.
func (s shellView) TilePx() float64 {
	if r := s.a.renderer; r != nil {
		return r.cam.Tile()
	}
	return camerapkg.TileSize
}

// WorldToScreen is a tile's top-left on the map in map pixels.
func (s shellView) WorldToScreen(tx, ty int) (float32, float32) {
	if r := s.a.renderer; r != nil {
		return r.worldToScreen(tx, ty)
	}
	return 0, 0
}

// WorldToScreenF is a world point on the map in map pixels.
func (s shellView) WorldToScreenF(wx, wy float64) (float32, float32) {
	if r := s.a.renderer; r != nil {
		return r.worldToScreenF(wx, wy)
	}
	return 0, 0
}

// CenterOn pans the camera to a tile's centre.
func (s shellView) CenterOn(tx, ty int) {
	if r := s.a.renderer; r != nil {
		sw, sh := s.a.size()
		r.centerOn(float64(tx)+0.5, float64(ty)+0.5, sw, sh)
	}
}

// Present is the presentation frame counter.
func (s shellView) Present() uint64 {
	if r := s.a.renderer; r != nil {
		return r.present
	}
	return 0
}

// ---- PawnSel

type shellSel struct{ a *App }

// Select makes id the renderer's selected pawn.
func (s shellSel) Select(id entity.EntityID) {
	if r := s.a.renderer; r != nil {
		r.Select(id)
	}
}

// Selected is the renderer's selected pawn, if any.
func (s shellSel) Selected() (entity.EntityID, bool) {
	if r := s.a.renderer; r != nil && r.HasSelect() {
		return r.Selected(), true
	}
	return 0, false
}

// ClearSelect drops the renderer's selected pawn.
func (s shellSel) ClearSelect() {
	if r := s.a.renderer; r != nil {
		r.ClearSelect()
	}
}

// ---- Overlays

type shellOverlay struct{ a *App }

func (s shellOverlay) RoofShown() bool {
	return s.a.renderer != nil && s.a.renderer.overlays().Roof.Shown()
}
func (s shellOverlay) HomeShown() bool { return s.a.renderer != nil && s.a.renderer.homeShown() }
func (s shellOverlay) PlanShown() bool { return s.a.renderer != nil && s.a.renderer.planOverlayOn() }
func (s shellOverlay) PowerShown() bool {
	return s.a.renderer != nil && s.a.renderer.overlays().Power.Shown()
}

func (s shellOverlay) ToggleRoof() {
	if r := s.a.renderer; r != nil {
		r.toggleRoofOverlayIfAny()
	}
}

func (s shellOverlay) ToggleHome() {
	if r := s.a.renderer; r != nil {
		r.toggleHomeOverlayIfAny()
	}
}

func (s shellOverlay) TogglePlan() {
	if r := s.a.renderer; r != nil {
		r.togglePlanOverlayIfAny()
	}
}

func (s shellOverlay) TogglePower() {
	if r := s.a.renderer; r != nil {
		r.togglePowerOverlayIfAny()
	}
}

// ---- Speed

type shellSpeed struct{ a *App }

// Paused reports the game clock is stopped.
func (s shellSpeed) Paused() bool { return s.a.renderer == nil || s.a.renderer.speed == speedPaused }

// TogglePause pauses, or resumes at normal speed.
func (s shellSpeed) TogglePause() {
	if r := s.a.renderer; r != nil {
		r.togglePause()
	}
}

// SetSpeed picks a tier (0 paused … 3 superfast, 4 ultrafast).
func (s shellSpeed) SetSpeed(tier int) {
	if r := s.a.renderer; r != nil {
		r.setSpeed(speed(tier))
	}
}

// SpeedTier is the current tier.
func (s shellSpeed) SpeedTier() int {
	if r := s.a.renderer; r != nil {
		return int(r.speed)
	}
	return int(speedPaused)
}

// PauseReason says why the game paused itself ("" when the player did).
func (s shellSpeed) PauseReason() string { return s.a.speedReason() }

// ---- Settings

type shellSettings struct{ a *App }

// Settings is the player's options.
func (s shellSettings) Settings() config.Settings { return s.a.settings }

// KeyName is the live key for an action ("Space").
func (s shellSettings) KeyName(act input.Action) string { return s.a.keyName(act) }

// KeyHint is the live key for an action, bracketed for a tooltip ("[Space]").
func (s shellSettings) KeyHint(act input.Action) string { return s.a.keyHint(act) }

// ---- Panels

type shellPanels struct{ a *App }

// ArchitectOpen reports the Architect panel is up.
func (s shellPanels) ArchitectOpen() bool { return s.a.arch.Open }

// ArchitectTool is the Architect's tool in hand ("" when none).
func (s shellPanels) ArchitectTool() string { return s.a.arch.ToolID }

// WindowMode is the open main-tab window ("work", "schedule", …) and
// whether one is open.
func (s shellPanels) WindowMode() (mode string, open bool) { return s.a.win.Mode, s.a.win.Open }

// UIHidden reports F11 hid the interface.
func (s shellPanels) UIHidden() bool { return s.a.uiHidden() }

// ---- Widgets

type shellWidgets struct{ a *App }

// Kit is the immediate-mode widget kit.
func (s shellWidgets) Kit() *uikit.Kit { return s.a.uiSt() }

// ---- Saves

type shellSaves struct{ a *App }

// Slots is the load screen's list (RefreshSlots reads it from disk).
func (s shellSaves) Slots() []ports.SaveSlot { return s.a.opt.Slots }
func (s shellSaves) RefreshSlots()           { s.a.opt.RefreshSlots() }
func (s shellSaves) LoadSlot(i int)          { s.a.opt.LoadSlot(i) }
func (s shellSaves) NewestSlot() int         { return s.a.newestSlot() }

// ---- Dispatcher

type shellDispatch struct{ a *App }

// Do handles a cross-feature intent: the root is the one place that knows
// how the panels exclude each other and how selection and camera meet.
func (s shellDispatch) Do(it ports.Intent) {
	a := s.a
	switch it := it.(type) {
	case ports.OpenArchitect:
		a.arch.OpenPanel()
	case ports.CloseArchitect:
		a.arch.ClosePanel()
	case ports.PresetBuildTool:
		a.arch.Preset(it.Kind, it.Stuff, it.Facing)
	case ports.AimMortar:
		a.ord.Aim.Start(it.ID, it.X, it.Y)
	case ports.AimGrenade:
		a.ord.Aim.StartGrenade(it.Pawn)
	case ports.OpenStorage:
		a.win.Storage().Open(it.Zone)
	case ports.ShowWindow:
		a.win.Show(it.Mode)
	case ports.CloseWindows:
		a.win.Open = false
	case ports.SelectPawn:
		a.sel.Select(selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: it.ID})
	case ports.TogglePawnInSet:
		a.sel.ToggleInSet(selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: it.ID})
	case ports.ClearSelection:
		a.sel.Clear(selectionpkg.None)
	case ports.CenterOnPawn:
		a.centreOnPawn(it.ID)
	case ports.Push:
		a.apply(intent{kind: intentPush, screen: it.Screen})
	case ports.Back:
		a.apply(intent{kind: intentBack})
	case ports.StartGame:
		a.apply(intent{kind: intentStartGame})
	case ports.Quit:
		a.apply(intent{kind: intentQuit})
	case ports.ReturnToMenu:
		a.apply(intent{kind: intentReturnToMenu})
	case ports.Resume:
		a.apply(intent{kind: intentResume})
	case ports.Pause:
		a.apply(intent{kind: intentPause})
	}
}

// ---- the screen table

// screenTable is each screen's widget declaration (uiBegin): it runs once
// per Update, right after the input is polled, while the screen is on
// top with no overlay. A new screen adds a row here (SOLID_GO.md §4:
// registration at the composition root, not init()).
func screenTable() map[portspkg.Screen]func(a *App, sw, sh int) {
	return map[portspkg.Screen]func(a *App, sw, sh int){
		portspkg.ScreenMainMenu:       func(a *App, sw, sh int) { a.menuScreen.Declare(sw, sh) },
		portspkg.ScreenScenario:       func(a *App, sw, sh int) { a.scenarioUI(sw, sh) },
		portspkg.ScreenScenarioEditor: func(a *App, sw, sh int) { a.editorUI(sw, sh) },
		portspkg.ScreenStoryteller:    func(a *App, sw, sh int) { a.storytellerUI(sw, sh) },
		portspkg.ScreenWorldGen:       func(a *App, sw, sh int) { a.worldGenUI(sw, sh) },
		portspkg.ScreenLanding:        func(a *App, sw, sh int) { a.landingUI(sw, sh) },
		portspkg.ScreenPawns:          func(a *App, sw, sh int) { a.pawnsUI(sw, sh) },
		portspkg.ScreenOptions:        func(a *App, sw, sh int) { a.opt.UI(sw, sh) },
		portspkg.ScreenColony:         func(a *App, sw, sh int) { a.colonyUI(sw, sh) },
	}
}

// ---- the click routes

// clickRoute is one click handler: an exact id or a prefix, and what runs
// for it. run reports the click was handled; false falls through to the
// next route (an exact id that only narrows a prefix, a wizard id the
// wizard does not know).
type clickRoute struct {
	id     string
	prefix bool
	run    func(a *App, id string) bool
}

func (rt clickRoute) matches(id string) bool {
	if rt.prefix {
		return strings.HasPrefix(id, rt.id)
	}
	return id == rt.id
}

// exact is a route for one id; on is a route for a prefix. A handler
// that returns nothing always handles its id.
func exact(id string, run func(a *App, id string)) clickRoute {
	return clickRoute{id: id, run: func(a *App, id string) bool { run(a, id); return true }}
}

func on(prefix string, run func(a *App, id string)) clickRoute {
	return clickRoute{id: prefix, prefix: true, run: func(a *App, id string) bool { run(a, id); return true }}
}

// clickRoutes is every HUD and screen click the shell dispatches, in the
// order they are tried (applyClick). A feature that moves into its own
// package replaces its rows with one registration of its prefixes
// (SOLID_GO.md §4); nothing here is reached from init().
func clickRoutes() []clickRoute {
	return []clickRoute{
		exact(hud.RoofButtonID, func(a *App, _ string) { a.shell.ToggleRoof() }),
		exact(hud.HomeButtonID, func(a *App, _ string) { a.shell.ToggleHome() }),
		exact(architectpkg.HomeAutoButtonID, func(a *App, _ string) { a.arch.ToggleHomeAuto() }), // home_auto_ui.go
		exact(hud.PlanButtonID, func(a *App, _ string) { a.shell.TogglePlan() }),
		exact(hud.ZonesButtonID, func(a *App, _ string) { toggleZonesOverlay(a) }), // zones_overlay_ui.go
		exact(hud.RoomsButtonID, func(a *App, _ string) { a.chrome.Rooms = !a.chrome.Rooms }),
		on(ids.PrefixLetter, func(a *App, id string) { a.chrome.Right().LetterClick(id) }), // hud/letters.go
		on(ids.PrefixSpeed, func(a *App, id string) { a.chrome.Global().SpeedClick(id) }),  // hud/global.go
		on(ids.PrefixLearn, (*App).applyLearnClick),                                        // learn_ui.go
		on(ids.PrefixRes, func(a *App, id string) { a.chrome.Res().Click(id) }),            // hud/resources.go
		on(ids.PrefixCmd, (*App).applyCommandClick),                                        // hud_commands.go
		on(ids.PrefixWin, func(a *App, id string) { a.win.Click(id) }),                     // hud_window.go
		exact(ids.TabWildlife, func(a *App, _ string) { a.win.ToggleWildlife() }),          // hud_window.go
		on(ids.PrefixGroup, (*App).applyGroupClick),                                        // multiselect.go
		on(ids.PrefixSearch, (*App).applySearchClick),                                      // search_ui.go
		on(ids.PrefixDev, (*App).applyDevClick),                                            // devmode.go
		on(ids.PrefixBarPawn, clickBarPawn),                                                // hud/colonistbar.go
		on(ids.PrefixResearchPick, func(a *App, id string) { a.win.Research().Pick(id) }),  // research_ui.go
		on(ids.PrefixAnimals, func(a *App, id string) { a.win.Animals().Click(id) }),       // animals_ui.go
		{id: "tab.", prefix: true, run: clickMainTab},                                      // work, schedule, research, animals; others fall through
		on(ids.PrefixSchedBrush, func(a *App, id string) {
			if v, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixSchedBrush)); err == nil {
				a.win.Brush = schedule.Activity(v)
			}
		}),
		on(ids.PrefixSchedCell, func(a *App, id string) { a.win.Schedule().Paint(id) }), // windows
		exact(ids.TabArchitect, func(a *App, _ string) {
			if a.arch.Open {
				a.arch.ClosePanel()
			} else {
				a.arch.OpenPanel()
			}
		}),
		on(ids.PrefixArchCat, func(a *App, id string) { a.arch.SelectCategory(strings.TrimPrefix(id, ids.PrefixArchCat)) }),
		on(ids.PrefixArchStuff, func(a *App, id string) { a.arch.Stuff = strings.TrimPrefix(id, ids.PrefixArchStuff) }),
		on(ids.PrefixArchCrop, func(a *App, id string) { a.arch.Crop = strings.TrimPrefix(id, ids.PrefixArchCrop) }),
		on(ids.PrefixArchTool, func(a *App, id string) { a.arch.SelectTool(strings.TrimPrefix(id, ids.PrefixArchTool)) }),
		on(ids.PrefixFloat, (*App).chooseFloat), // orders.go
		on(ids.PrefixLoadSlot, func(a *App, id string) {
			if i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixLoadSlot)); err == nil {
				a.opt.LoadSlot(i)
			}
		}),
		exact(ids.PauseSaveQuit, func(a *App, _ string) { a.apply(intent{kind: intentReturnToMenu}) }), // commitment: Save & Quit to Menu (saves.go)
		exact(ids.PauseSave, func(a *App, _ string) {
			a.opt.SaveNow(a.opt.SaveSlot)
			a.apply(intent{kind: intentResume})
		}),
		exact(ids.PauseLoad, openLoadScreen),
		exact(ids.MenuLoad, openLoadScreen),
		exact(ids.MenuContinue, func(a *App, _ string) {
			a.opt.RefreshSlots()
			if i := a.newestSlot(); i >= 0 { // the most recent save, commitment ones too
				a.opt.LoadSlot(i)
			}
		}),
		exact(ids.PawnDraft, func(a *App, _ string) { a.ord.ToggleDraft() }),
		exact(ids.PawnClearOrders, func(a *App, _ string) {
			if p, ok := a.insp.SelectedPawn(); ok {
				a.shell.Enqueue(command.Command{Kind: command.ClearOrders, Pawn: uint64(p.ID)})
			}
		}),
		on(ids.PrefixAnimal, (*App).applyAnimalClick),
		on(ids.PrefixPlant, (*App).applyPlantClick),                                      // plant_inspector.go
		on(ids.PrefixItem, (*App).applyItemClick),                                        // item_inspector.go
		on(ids.PrefixFrame, (*App).applyFrameClick),                                      // frame_inspector.go
		on(ids.PrefixZone, (*App).applyZoneClick),                                        // zone_select.go
		on(ids.PrefixMsg, func(a *App, id string) { a.chrome.Right().FocusMessage(id) }), // hud/messages.go
		on(ids.PrefixAlert, func(a *App, id string) { a.chrome.Alerts().Focus(id) }),     // hud/alerts.go
		on(ids.PrefixBldg, (*App).applyBuildingClick),                                    // building_inspector.go
		on(ids.PrefixBill, (*App).applyBuildingClick),                                    // building_inspector.go
		on(ids.PrefixWorkCell, (*App).toggleWorkCell),                                    // app.go
		exact(ids.TabMenu, func(a *App, _ string) { a.apply(intent{kind: intentPause}) }),
		{id: ids.PrefixOptions, prefix: true, run: func(a *App, id string) bool { // screen_options.go
			if id == ids.OptionsBack { // the screen intent, not an option
				return false
			}
			a.opt.ApplyOption(id)
			return true
		}},
		exact(ids.MenuOptions, openOptions),
		exact(ids.PauseOptions, openOptions),
		exact(ids.InspectorClose, func(a *App, _ string) { a.shell.ClearSelect() }),
		on(ids.PrefixInspector, (*App).selectPaneTab),        // hud_inspect.go: open (or collapse) a tab
		{id: "", prefix: true, run: (*App).applyWizardClick}, // the wizard's own ids (screen_newgame.go)
	}
}

// clickBarPawn is a colonist-bar portrait: select, Shift adds or removes,
// a double-click centres.
func clickBarPawn(a *App, id string) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, ids.PrefixBarPawn), 10, 64)
	if err != nil || a.renderer == nil {
		return
	}
	pid := entity.EntityID(n)
	if a.in.Shift() {
		a.sel.ToggleInSet(selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: pid})
		return
	}
	a.sel.Select(selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: pid}) // clears every other selection
	if a.chrome.Bar().DoubleClick(pid) {
		a.centreOnPawn(pid)
	}
}

// clickMainTab opens a main-tab window, or closes the one that is open.
// It handles only the four window tabs; every other "tab." id falls
// through to its own route (architect, menu) or the screen intent.
func clickMainTab(a *App, id string) bool {
	switch id {
	case ids.TabWork, ids.TabSchedule, ids.TabResearch, ids.TabAnimals:
	default:
		return false
	}
	mode := strings.TrimPrefix(id, "tab.")
	if a.win.Open && a.win.Mode == mode {
		a.win.Open = false
		return true
	}
	a.win.Open, a.win.Mode = true, mode
	a.arch.ClosePanel()
	a.ord.CloseFloat()
	return true
}

// openLoadScreen refreshes the slot list and pushes the Load screen.
func openLoadScreen(a *App, _ string) {
	a.opt.RefreshSlots()
	a.apply(intent{kind: intentPush, screen: portspkg.ScreenLoad})
}

// openOptions opens Options on its first tab with no key capture pending.
func openOptions(a *App, id string) {
	a.opt.Tab = ""
	a.rebind = options.RebindState{}
	a.apply(a.clickIntent(id))
}
