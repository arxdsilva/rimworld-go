package ebiten

// The colony's widget-kit hook (UI-COLONY). The HUD regions (PLAY-018)
// and the kit windows, tabs, tooltips and letters declare their widgets
// here, once per Update, with the same widget kit the setup screens use.
// Where a button of the same id exists (a tab), the widget answers its
// click through uiClick, so app.go's applyClick needs no case for it.

func init() {
	registerScreenUI(screenColony, func(a *App, sw, sh int) { a.colonyUI(sw, sh) })
}

func (a *App) colonyUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.renderer == nil {
		return
	}
	a.lettersUI() // may open a letter window, which takes the layer from here on
	if a.hideUI {
		return
	}
	a.factionsTabUI(sw, sh)
	a.assignTabUI(sw, sh) // assign_ui.go: Assign tab (ENG-003)
	a.gatheringUI(sw)     // gathering_ui.go: wedding/party banner (ENG-003)
	a.socialTabsUI()      // social_tabs_ui.go: the History tab (ENG-003)
	a.worldTabUI(sw, sh)  // world_ui.go: World tab, caravans (ENG-003)
	a.workTipsUI(sw, sh)
	a.researchUI(sw, sh) // research_ui.go: the project list's scroll view
	a.statusChipsUI(sw)
	a.animalsTabUI(sw, sh)    // animals_ui.go: Area / Pen dropdowns
	a.areaSelectorsUI(sw, sh) // area_select.go: Schedule column, inspector line, overlay sync
	// PLAY-018 HUD regions (each owner's file)
	a.resourcesUI(sw, sh)   // hud_resources.go
	a.colonistBarUI(sw, sh) // hud_colonistbar.go
	a.rightColumnUI(sw, sh) // hud_right.go
	a.globalUI(sw, sh)      // hud_global.go
	a.paneUI(sw, sh)        // hud_inspect.go
	a.commandGridUI(sw, sh) // hud_commands.go
	a.learnUI(sw, sh)       // learn_ui.go
	a.mainWindowUI(sw, sh)  // hud_window.go
}
