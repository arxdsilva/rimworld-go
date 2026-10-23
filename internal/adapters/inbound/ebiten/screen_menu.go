package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Main menu metrics (NG-1 §2.1): a right rail of 300x40 buttons, 10 px
// apart, from y = 140.
const (
	menuBtnH   = 40
	menuBtnGap = 10
	menuRailY  = 140
)

func (a *App) mainMenuButtons(sw, sh int) []button {
	cx := menuRailX(sw)
	cont := buttonSpec{id: "menu.continue", label: "Continue", disabled: !a.hasSaves(), reason: "no saves yet"}
	specs := []buttonSpec{
		{id: "menu.newgame", label: "New Colony"},
		{id: "menu.load", label: "Load Game"},
		{id: "menu.plugins", label: "Plugins", disabled: true, reason: "not implemented"},
		{id: "menu.options", label: "Options"},
		{id: "menu.tutorial", label: "Tutorial / Help", disabled: true, reason: "not implemented"},
		{id: "menu.credits", label: "Credits"},
		{id: "menu.quit", label: "Quit"},
	}
	if cont.disabled { // Continue leads the list only when it can be used
		specs = append([]buttonSpec{specs[0], cont}, specs[1:]...)
	} else {
		specs = append([]buttonSpec{cont}, specs...)
	}
	return stackButtons(cx, menuRailY, menuRailW, menuBtnH, menuBtnGap, specs)
}

func (a *App) drawMainMenu(screen *ebiten.Image, sw, sh int) {
	if !a.drawMenuDiorama(screen, sw, sh, 0) {
		drawMenuBackdrop(screen, sw, sh)
	}
	a.drawMenuSplash(screen, sw, sh)
	title := tsHuge
	shadow := title
	shadow.Color = color.RGBA{A: 0xC0}
	drawText(screen, "rimworld-go", 58, 30, shadow)
	title.Color = uiHeading
	drawText(screen, "rimworld-go", 56, 28, title)
	ver := dimText()
	drawText(screen, formatVersion(a.cfg.Version), 58, 28+lineH(tsHuge)+2, ver)
	mx, my := a.currentCursor()
	btns := a.mainMenuButtons(sw, sh)
	for i, b := range btns {
		drawMenuButton(screen, b, i == 0 && !b.disabled, b.contains(mx, my))
	}
}

// drawMenuButton is a main-menu rail button in the real font; the first
// enabled one is the gold primary.
func drawMenuButton(dst *ebiten.Image, b button, primary, hover bool) {
	r := rect{b.x, b.y, b.w, b.h}
	style := btnNormal
	if primary {
		style = btnPrimary
	}
	fill, border, text := style.colors(hover && !b.disabled, false, false)
	if b.disabled {
		fill, border, text = uiBtnDisabled, uiBtnBorder, uiTextDisabled
	}
	fillRect(dst, r, fill)
	strokeRect(dst, r, border)
	st := tsLarge
	st.Color, st.Align = text, alignCenter
	cx, cy := r.center()
	drawText(dst, b.label, float64(cx), float64(cy)-lineH(st)/2, st)
}

func (a *App) creditsButtons(sw, sh int) []button {
	return []button{{
		id:    "credits.back",
		label: "Back",
		x:     40,
		y:     footerY(sh),
		w:     140,
		h:     36,
	}}
}

// creditsLines is the Credits screen body.
func creditsLines() []string {
	return []string{
		"Created by arxdsilva",
		"",
		"rimworld-go is original work. It is not affiliated with Ludeon",
		"Studios or RimWorld.",
		"",
		"Vendor sprites (first playable stand-in): Raphael Goncalves",
		"(Rgsdev), CC0.",
		"Music and sound effects: original, made in code (cmd/audiogen).",
		"Noto Sans: SIL Open Font License 1.1.",
		"Ambience: generated with ElevenLabs (elevenlabs.io), free tier,",
		"non-commercial use.",
	}
}

func (a *App) drawCredits(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Credits", creditsLines(), a.creditsButtons(sw, sh))
}

// hasSaves reports a loadable save exists (for Continue).
func (a *App) hasSaves() bool {
	if !a.canLoad() {
		return false
	}
	if !a.slotsRead {
		a.slotsRead = true
		a.refreshSlots()
	}
	for _, sl := range a.slots {
		if sl.Err == "" {
			return true
		}
	}
	return false
}

// newestSlot is the index of the loadable save written last (-1 when none),
// commitment saves included; ties keep the list order.
func (a *App) newestSlot() int {
	best := -1
	for i, sl := range a.slots {
		if sl.Err != "" {
			continue
		}
		if best < 0 || sl.Modified.After(a.slots[best].Modified) {
			best = i
		}
	}
	return best
}
