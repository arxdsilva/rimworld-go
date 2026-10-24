package ebiten

// The colony's widget-kit hook (UI-COLONY). The colony HUD is still the
// legacy button/label chrome; the NG-1 additions (Factions window, Bio
// tab, tooltips on the Work tab and the status strip, letters) are
// declared here, once per Update, with the same widget kit the setup
// screens use. Where a legacy button of the same id exists (a tab), the
// widget answers its click through uiClick, so app.go's applyClick needs no
// case for it.

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
	a.bioUI()
	a.socialTabsUI() // social_tabs_ui.go: Health and Social tabs (ENG-003)
	a.workTipsUI(sw, sh)
	a.statusChipsUI(sw)
}
