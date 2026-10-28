package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The colony HUD's dispatchers (PLAY-018). Every region lives in its own
// file and owner; this file only calls them in order and is frozen (lead):
//
//	hud_layout.go      the layout (computeHUDLayout), shared
//	hud_resources.go   resource list, top-left                (HUD-TOP)
//	hud_colonistbar.go colonist bar, top-centre               (HUD-TOP)
//	hud_right.go       alerts, letters, messages, right column (HUD-RIGHT)
//	hud_global.go      speed, date, weather, toggles, lower-right (GLOBAL)
//	hud_cellinfo.go    hovered-cell info                       (GLOBAL)
//	hud_inspect.go     the inspect pane and its tabs           (CORE-PANE)
//	hud_commands.go    command buttons, merge, hotkeys         (CORE-PANE)
//	cmds_*.go          each selection kind's commands and pane (per kind)
//	hud_tabs.go        the bottom tab strip                    (WINDOWS)
//	hud_window.go      main-tab windows                        (WINDOWS)
//	learn_ui.go        the learning helper                     (LEARN)

// hudStates is the HUD regions' presentation state, one struct per owner,
// each declared in its owner's file (App.hst).
type hudStates struct {
	res   resourcesState // hud_resources.go
	bar   barState       // hud_colonistbar.go
	right rightState     // hud_right.go
	pane  paneState      // hud_inspect.go
	cmds  cmdsState      // hud_commands.go
	win   windowState    // hud_window.go
	learn learnState     // learn_ui.go
	arch  archState      // cmds_arch.go
	lay   hudMemo        // hud(): the last layout for the same inputs
}

// hud is this frame's HUD layout. The state comes from each region's
// owner; none of those accessors may call hud().
func (a *App) hud() hudLayout {
	sw, sh := a.size()
	st := hudState{
		Colonists:  a.barCount(),
		PaneShown:  a.paneShown(),
		ArchOpen:   a.archOpen,
		Commands:   a.commandCount(),
		GlobalH:    a.globalHeight(),
		MinimapOn:  a.minimapOn(),
		LearnH:     a.learnHeight(),
		Alerts:     a.alertCount(),
		Letters:    a.letterCount(),
		ResourcesH: a.resourcesHeight(),
		Messages:   a.messageCount(),
		ArchDescH:  a.archDescHeight(),
	}
	st.ExpandedW, st.ExpandedH = a.expandedWant()
	st.WindowW, st.WindowH = a.windowWant()
	st.PerfW, st.PerfH = a.perfBox()
	if m := &a.hst.lay; m.ok && m.sw == sw && m.sh == sh && m.st == st {
		return m.lay
	}
	lay := computeHUDLayout(sw, sh, st)
	a.hst.lay = hudMemo{sw: sw, sh: sh, st: st, lay: lay, ok: true}
	return lay
}

// hudMemo is the last layout hud() computed and the inputs it came from:
// the same inputs give the same layout, so the regions that ask for it
// several times per frame pay for one computation (ARCH-002).
type hudMemo struct {
	sw, sh int
	st     hudState
	lay    hudLayout
	ok     bool
}

func (a *App) colonyButtons(sw, sh int) []button {
	if a.hideUI {
		return nil // F11: only the map
	}
	out := a.floatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.devButtons()...)
	out = append(out, a.searchButtons(sw)...)
	out = append(out, a.learnButtons()...)    // learn_ui.go
	out = append(out, a.globalButtons(sw)...) // hud_global.go
	out = append(out, a.groupButtons()...)
	out = append(out, a.hudTabButtons(sw, sh)...)      // hud_tabs.go
	out = append(out, a.colonistBarButtons(sw, sh)...) // hud_colonistbar.go
	out = append(out, a.commandGridButtons()...)       // hud_commands.go
	out = append(out, a.paneButtons()...)              // hud_inspect.go
	out = append(out, a.rightColumnButtons(sw)...)     // hud_right.go
	out = append(out, a.resourceButtons()...)          // hud_resources.go
	out = append(out, a.architectButtons()...)
	out = append(out, a.workButtons()...)
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
	a.drawRoomOverlay(dst) // room_stats.go
	a.drawGroupMarks(dst)  // multiselect.go
	a.drawFormation(dst)   // formation.go
	a.drawBoxSelect(dst)
	a.drawArchitectGhost(dst)
	a.drawPenOutline(dst)  // pen_draw.go: selected pen, or all while the Pen marker tool is held
	a.drawMoveMarkers(dst) // move_mode.go: pending moves
	a.drawMoveMode(dst)
	a.drawSelectionMarks(dst) // each kind's file: the map marker
}

func (a *App) drawColonyHUD(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.drawGlobalControls(screen, sw, sh) // hud_global.go
	a.drawResources(screen, sw, sh)      // hud_resources.go
	a.drawColonistBar(screen, sw, sh)    // hud_colonistbar.go
	a.drawTabs(screen, sw, sh)           // hud_tabs.go
	a.drawPane(screen)                   // hud_inspect.go
	a.drawCommandGrid(screen)            // hud_commands.go
	a.drawArchitect(screen)
	a.drawWindows(screen, sw, sh)     // hud_window.go
	a.drawRightColumn(screen, sw, sh) // hud_right.go
	a.drawLearn(screen)               // learn_ui.go
	a.drawFloat(screen)
	a.drawCellInfo(screen, sh) // hud_cellinfo.go
	a.drawRoomCard(screen)
	a.drawDevPanels(screen, sw, sh) // devmode.go
	a.drawSearch(screen, sw)        // search_ui.go
}

// colonyChromeHit reports the point (logical) is on the HUD, so a click
// there never reaches the map.
func (a *App) colonyChromeHit(mx, my, sw, sh int) bool {
	if a.hideUI {
		return false
	}
	if mx < 0 || my < 0 || mx >= sw || my >= sh {
		return true // off the window: never the map
	}
	if a.search.open && a.searchRect(sw).contains(mx, my) {
		return true
	}
	if a.dev.menu && a.devMenuRect().contains(mx, my) {
		return true
	}
	if a.float.open && a.floatRect().contains(mx, my) {
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
func (a *App) chromeRects(sw, sh int) []rect {
	return append(a.hud().Chrome(), a.learnChrome()...) // learn_ui.go
}

// drawSelectionMarks marks the selected thing on the map (each kind's
// file); the pane and the command grid show the rest.
func (a *App) drawSelectionMarks(screen *ebiten.Image) {
	a.drawBuildingMark(screen)  // building_inspector.go
	a.drawAnimalMark(screen)    // animal_inspector.go
	a.drawPlantMark(screen)     // plant_inspector.go
	a.drawItemMark(screen)      // item_inspector.go
	a.drawFrameMark(screen)     // frame_inspector.go
	a.drawZoneSelection(screen) // zone_select.go
}

// hudRightClick lets a HUD region answer a right-click (dismiss a letter,
// cycle an alert back, open a command's options). True when it took it;
// it only ever takes clicks over the HUD.
func (a *App) hudRightClick(mx, my int) bool {
	return a.rightClickRightColumn(mx, my) || a.rightClickCommand(mx, my) || a.rightClickResources(mx, my)
}
