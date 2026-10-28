package ebiten

// The allowed-area overlay's tool hook (the overlay itself is
// render/overlay, area_overlay.go).

// syncAreaOverlay points the overlay at the area the armed Architect tool
// paints (none when no area tool is armed); the pickers set hover.
func (a *App) syncAreaOverlay() {
	if a.renderer == nil {
		return
	}
	id := 0
	if _, ok := areaToolPaints(a.archTool); ok && a.archOpen {
		id = a.areaPaintIndex()
	}
	a.renderer.overlays().Area.Armed = id
}
