package ebiten

import (
	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/command"
)

// The Home auto-expand toggle (PLAY-015): a house-with-arrows button in
// the top strip, left of the overlay toggles. Selected while Home grows
// by itself; a click emits SetHomeAuto with the opposite state.

const (
	homeAutoButtonID = "hud.homeauto"
	homeAutoButtonW  = 48
)

// homeAutoButtons is the toggle, left of the leftmost overlay button.
func (a *App) homeAutoButtons(sw int) []button {
	hb := a.homeButtons(sw)
	if len(hb) == 0 {
		return nil
	}
	left := hb[0].x
	for _, bs := range [][]button{a.planButtons(sw), a.roomsButtons(sw)} {
		if len(bs) > 0 && bs[0].x < left {
			left = bs[0].x
		}
	}
	on := a.renderer.world().HomeAuto
	tip := "Home auto-expand: off (click to turn on)"
	if on {
		tip = "Home auto-expand: on (click to turn off)"
	}
	return []button{{id: homeAutoButtonID, label: "Auto", x: left - homeAutoButtonW - hudPad/2, y: 4, w: homeAutoButtonW, h: hudTopH - 8, selected: on, tip: tip}}
}

func (a *App) drawHomeAutoButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.homeAutoButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("home_auto"); img != nil {
			uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
	}
}

// toggleHomeAuto asks the sim to flip automatic Home expansion.
func (a *App) toggleHomeAuto() {
	if a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: !a.renderer.world().HomeAuto})
}
