package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The colony screen's composition (PLAY-018, ARCH-005 wave 5.5): the
// layout memo over every feature's sizes, and the order the features
// declare their widgets, list their buttons, draw and claim the screen.
// The HUD's regions live in package hud (a.chrome); the inspector pane,
// the command grid, the windows, the Architect, the learning helper and
// the developer tools each in their own place. This file only calls them
// in order.

// hudStates is the HUD's presentation state the root still keeps
// (App.hst): the layout memo (the regions moved into hud, the pane and
// the command grid into inspector).
type hudStates struct {
	lay hudMemo // hud(): the last layout for the same inputs
}

// hud is this frame's HUD layout. The state comes from each region's
// owner; none of those accessors may call hud().
func (a *App) hud() hudlayout.Layout {
	sw, sh := a.size()
	st := hudlayout.Inputs{
		Colonists:  a.chrome.Bar().Count(),
		PaneShown:  a.insp.PaneShown(),
		ArchOpen:   a.arch.Open,
		Commands:   a.insp.Commands().CommandCount(),
		GlobalH:    a.globalHeight(),
		MinimapOn:  a.minimapOn(),
		LearnH:     a.lrn.Height(),
		Alerts:     a.chrome.Alerts().Count(),
		Letters:    a.chrome.Right().LetterCount(),
		ResourcesH: a.chrome.Res().Height(),
		Messages:   a.chrome.Right().MessageCount(),
		ArchDescH:  a.arch.DescHeight(),
	}
	st.ExpandedW, st.ExpandedH = a.insp.ExpandedWant()
	st.WindowW, st.WindowH = a.win.Want()
	st.PerfW, st.PerfH = a.meter.Box()
	if m := &a.hst.lay; m.ok && m.sw == sw && m.sh == sh && m.st == st {
		return m.lay
	}
	lay := hudlayout.Compute(sw, sh, st)
	a.hst.lay = hudMemo{sw: sw, sh: sh, st: st, lay: lay, ok: true}
	return lay
}

// hudMemo is the last layout hud() computed and the inputs it came from:
// the same inputs give the same layout, so the regions that ask for it
// several times per frame pay for one computation (ARCH-002).
type hudMemo struct {
	sw, sh int
	st     hudlayout.Inputs
	lay    hudlayout.Layout
	ok     bool
}

// colonyUI is the colony's widget-kit hook (UI-COLONY): the HUD regions
// and the kit windows, tabs, tooltips and letters declare their widgets
// here, once per Update, with the same widget kit the setup screens use.
// Where a button of the same id exists (a tab), the widget answers its
// click through uiClick, so applyClick needs no case for it.
func (a *App) colonyUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.renderer == nil {
		return
	}
	a.chrome.Right().DeclareLetters() // may open a letter window, which takes the layer from here on
	if a.chrome.Hidden {
		return
	}
	a.factionsTabUI(sw, sh)
	a.assignTabUI(sw, sh)         // assign_ui.go: Assign tab (ENG-003)
	a.chrome.Banner().Declare(sw) // hud/gathering.go: wedding/party banner (ENG-003)
	a.socialTabsUI()              // social_tabs_ui.go: the History tab (ENG-003)
	a.worldTabUI(sw, sh)          // world_ui.go: World tab, caravans (ENG-003)
	a.win.Work().Tips(sw, sh)
	a.win.Research().Declare(sw, sh) // research_ui.go: the project list's scroll view
	a.chrome.Global().DeclareChips(sw)
	a.win.Animals().Declare(sw, sh) // animals_ui.go: Area / Pen dropdowns
	a.areaSelectorsUI(sw, sh)       // area_select.go: Schedule column, inspector line, overlay sync
	// PLAY-018 HUD regions: resources, colonist bar, right column, global
	a.chrome.DeclareRegions(sw, sh)
	a.insp.PaneUI(sw, sh)                   // hud_inspect.go
	a.insp.Commands().CommandGridUI(sw, sh) // hud_commands.go
	a.lrn.UI(sw, sh)                        // learn_ui.go
	a.mainWindowUI(sw, sh)                  // hud_window.go
}

func (a *App) colonyButtons(sw, sh int) []uikit.Button {
	if a.chrome.Hidden {
		return nil // F11: only the map
	}
	out := a.ord.FloatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.tools.Buttons()...)
	out = append(out, a.ord.SearchButtons(sw)...)
	out = append(out, a.lrn.Buttons()...)
	out = append(out, a.chrome.Global().Buttons(sw)...) // hud/global.go
	out = append(out, a.insp.Commands().GroupButtons()...)
	out = append(out, a.chrome.Tabs().Buttons(sw, sh)...)        // hud/tabs.go
	out = append(out, a.chrome.Bar().Buttons(sw, sh)...)         // hud/colonistbar.go
	out = append(out, a.insp.Commands().CommandGridButtons()...) // hud_commands.go
	out = append(out, a.insp.PaneButtons()...)                   // hud_inspect.go
	out = append(out, a.chrome.Right().Buttons(sw)...)           // hud/right.go
	out = append(out, a.chrome.Res().Buttons()...)               // hud/resources.go
	out = append(out, a.arch.Buttons()...)
	out = append(out, a.win.Buttons()...)
	return out
}

// drawMapOverlays draws what the player's tools and selection mark on the
// map: room tints, selection and group marks, ghosts, the drag box, move
// markers. It draws into the map's target (the screen, or the
// render-scaled offscreen, world_scale.go), in map pixels, before the HUD
// chrome, which floats over it; its labels go through the canvas and stay
// native.
func (a *App) drawMapOverlays(dst *ebiten.Image) {
	if a.renderer == nil {
		return
	}
	a.syncRoofForce()
	a.chrome.RoomStats().DrawOverlay(dst) // hud/rooms.go
	a.drawGroupMarks(dst)                 // multiselect.go
	a.ord.DrawFormation(dst)              // formation.go
	a.drawBoxSelect(dst)
	a.arch.DrawGhost(dst)
	a.arch.Marks.DrawPenOutline(dst) // pen_draw.go: selected pen, or all while the Pen marker tool is held
	a.arch.Mover.DrawMarkers(dst)    // move_mode.go: pending moves
	a.arch.Mover.Draw(dst)
	drawCombatOverlay(a, dst) // combat_overlay_host.go: range, hit chance, cover, projectiles
	a.drawSelectionMarks(dst) // each kind's file: the map marker
}

func (a *App) drawColonyHUD(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.chrome.Global().Draw(screen, sw, sh)    // hud/global.go
	a.chrome.Res().Draw(screen, sw, sh)       // hud/resources.go
	a.chrome.Bar().Draw(screen, sw, sh)       // hud/colonistbar.go
	a.chrome.Tabs().Draw(screen, sw, sh)      // hud/tabs.go
	a.insp.DrawPane(screen)                   // hud_inspect.go
	a.insp.Commands().DrawCommandGrid(screen) // hud_commands.go
	a.arch.Draw(screen)
	a.win.Draw(screen, sw, sh)            // hud_window.go
	a.chrome.Right().Draw(screen, sw, sh) // hud/right.go
	a.lrn.Draw(screen)                    // learn_ui.go
	a.ord.DrawFloat(screen)
	a.chrome.Cell().Draw(screen, sh)      // hud/cellinfo.go
	a.chrome.RoomStats().DrawCard(screen) // hud/rooms.go
	a.tools.Draw(screen, sw, sh)          // devmode.go
	a.ord.DrawSearch(screen, sw)          // search_ui.go
}

// colonyChromeHit reports the point (logical) is on the HUD, so a click
// there never reaches the map.
func (a *App) colonyChromeHit(mx, my, sw, sh int) bool {
	if a.chrome.Hidden {
		return false
	}
	if mx < 0 || my < 0 || mx >= sw || my >= sh {
		return true // off the window: never the map
	}
	if a.ord.Search.Open && a.ord.SearchRect(sw).Contains(mx, my) {
		return true
	}
	if a.tools.State.Menu && a.tools.MenuRect().Contains(mx, my) {
		return true
	}
	if a.ord.Float.Open && a.ord.FloatRect().Contains(mx, my) {
		return true
	}
	x, y := float32(mx), float32(my)
	for _, r := range a.chromeRects(sw, sh) {
		if r.Contains(x, y) {
			return true
		}
	}
	return false
}

// chromeRects are the HUD's areas: the layout's regions, plus the
// learning helper's lesson card, which opens beside its region.
func (a *App) chromeRects(sw, sh int) []gfx.Rect {
	return append(a.hud().Chrome(), a.lrn.Chrome()...) // learn_ui.go
}

// drawSelectionMarks marks the selected thing on the map (each kind's
// file); the pane and the command grid show the rest.
func (a *App) drawSelectionMarks(screen *ebiten.Image) {
	a.drawBuildingMark(screen)         // building_inspector.go
	a.drawAnimalMark(screen)           // animal_inspector.go
	a.drawPlantMark(screen)            // plant_inspector.go
	a.drawItemMark(screen)             // item_inspector.go
	a.drawFrameMark(screen)            // frame_inspector.go
	a.arch.Zones.DrawSelection(screen) // zone_select.go
}

// hudRightClick lets a HUD region answer a right-click (dismiss a letter,
// cycle an alert back, open a command's options). True when it took it;
// it only ever takes clicks over the HUD.
func (a *App) hudRightClick(mx, my int) bool {
	return a.chrome.Right().RightClick(mx, my) || a.insp.Commands().RightClickCommand(mx, my) || a.chrome.Res().RightClick(mx, my)
}
