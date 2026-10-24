package ebiten

// Captures of the social colony's tabs and windows (-scenario social):
//
//	social    Maren's Social tab (married to Tobias), first row hovered
//	health    Cyrus's Health tab (a jaw scar: disfigured)
//	prisoner  Garr's Prisoner tab
//	history   the History window
//	world     the World window
//	assign    the Assign window
//
// Pawns are found by the scenario's names; a capture of another colony
// falls back to the first colonist.

// captureSocialUI opens the state's tab or window.
func (a *App) captureSocialUI(ui string) {
	sw, sh := a.size()
	a.uiSt().lastScreen = a.nav.screen() // a window opened now must survive the kit's screen-change reset
	pick := func(name string) {
		snap := a.renderer.snapshot()
		for _, p := range snap {
			if p.Name == name || (name == "" && p.Name != "") {
				a.renderer.Select(p.ID)
				a.renderer.centerOn(float64(p.X), float64(p.Y), sw, sh)
				return
			}
		}
		if len(snap) > 0 {
			a.renderer.Select(snap[0].ID)
		}
	}
	a.cursor = &[2]int{-100, -100}
	switch ui {
	case "social":
		pick("Maren")
		a.inspectTab = "social"
		a.cursor = &[2]int{int(inspectorRect().x) + 120, int(inspectorRect().y) + 48} // the first row's tooltip
	case "health":
		pick("Cyrus")
		a.inspectTab = "health"
	case "prisoner":
		pick("Garr")
		a.inspectTab = "prisoner"
	case "history":
		a.openHistory()
	case "world":
		a.toggleWorld()
	case "assign":
		a.openAssign()
	}
}
