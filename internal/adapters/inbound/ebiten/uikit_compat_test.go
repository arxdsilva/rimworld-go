package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	uikitpkg "rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

func (a *App) uiModalOpen() bool { return a.uiSt().ModalOpen() }

func (a *App) uiSlider(id string, r gfx.Rect, val *float64, sp uikitpkg.SliderSpec) bool {
	return a.uiSt().Slider(id, r, val, a.withTick(sp))
}

func (a *App) uiSliderInt(id string, r gfx.Rect, val *int, sp uikitpkg.SliderSpec) bool {
	return a.uiSt().SliderInt(id, r, val, a.withTick(sp))
}
