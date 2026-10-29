package ebiten

import (
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
func (a *App) hud() hudLayout {
	sw, sh := a.size()
	st := hudState{
		Colonists:  a.barCount(),
		PaneShown:  a.paneShown(),
		ArchOpen:   a.arch.Open,
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
	a.lettersUI() // may open a letter window, which takes the layer from here on
	if a.chrome.Hidden {
		return
	}
	a.factionsTabUI(sw, sh)
	a.assignTabUI(sw, sh) // assign_ui.go: Assign tab (ENG-003)
	a.gatheringUI(sw)     // hud/gathering.go: wedding/party banner (ENG-003)
	a.socialTabsUI()      // social_tabs_ui.go: the History tab (ENG-003)
	a.worldTabUI(sw, sh)  // world_ui.go: World tab, caravans (ENG-003)
	a.workTipsUI(sw, sh)
	a.researchUI(sw, sh) // research_ui.go: the project list's scroll view
	a.statusChipsUI(sw)
	a.animalsTabUI(sw, sh)    // animals_ui.go: Area / Pen dropdowns
	a.areaSelectorsUI(sw, sh) // area_select.go: Schedule column, inspector line, overlay sync
	// PLAY-018 HUD regions: resources, colonist bar, right column, global
	a.chrome.DeclareRegions(sw, sh)
	a.paneUI(sw, sh)        // hud_inspect.go
	a.commandGridUI(sw, sh) // hud_commands.go
	a.learnUI(sw, sh)       // learn_ui.go
	a.mainWindowUI(sw, sh)  // hud_window.go
}

func (a *App) colonyButtons(sw, sh int) []button {
	if a.chrome.Hidden {
		return nil // F11: only the map
	}
	out := a.floatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.devButtons()...)
	out = append(out, a.searchButtons(sw)...)
	out = append(out, a.learnButtons()...)
	out = append(out, a.globalButtons(sw)...) // hud/global.go
	out = append(out, a.groupButtons()...)
	out = append(out, a.hudTabButtons(sw, sh)...)      // hud/tabs.go
	out = append(out, a.colonistBarButtons(sw, sh)...) // hud/colonistbar.go
	out = append(out, a.commandGridButtons()...)       // hud_commands.go
	out = append(out, a.paneButtons()...)              // hud_inspect.go
	out = append(out, a.rightColumnButtons(sw)...)     // hud/right.go
	out = append(out, a.resourceButtons()...)          // hud/resources.go
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
	a.chrome.RoomStats().DrawOverlay(dst) // hud/rooms.go
	a.drawGroupMarks(dst)                 // multiselect.go
	a.drawFormation(dst)                  // formation.go
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
	a.drawGlobalControls(screen, sw, sh) // hud/global.go
	a.drawResources(screen, sw, sh)      // hud/resources.go
	a.drawColonistBar(screen, sw, sh)    // hud/colonistbar.go
	a.drawTabs(screen, sw, sh)           // hud/tabs.go
	a.drawPane(screen)                   // hud_inspect.go
	a.drawCommandGrid(screen)            // hud_commands.go
	a.drawArchitect(screen)
	a.drawWindows(screen, sw, sh)     // hud_window.go
	a.drawRightColumn(screen, sw, sh) // hud/right.go
	a.drawLearn(screen)               // learn_ui.go
	a.drawFloat(screen)
	a.drawCellInfo(screen, sh)            // hud/cellinfo.go
	a.chrome.RoomStats().DrawCard(screen) // hud/rooms.go
	a.drawDevPanels(screen, sw, sh)       // devmode.go
	a.drawSearch(screen, sw)              // search_ui.go
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
	if a.ord.Search.Open && a.searchRect(sw).Contains(mx, my) {
		return true
	}
	if a.tools.State.Menu && a.devMenuRect().Contains(mx, my) {
		return true
	}
	if a.ord.Float.Open && a.floatRect().Contains(mx, my) {
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
