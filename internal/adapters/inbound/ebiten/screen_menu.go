package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
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
	cont := buttonSpec{id: ids.MenuContinue, label: "Continue", disabled: !a.hasSaves(), reason: "no saves yet"}
	specs := []buttonSpec{
		{id: ids.MenuNewGame, label: "New Colony"},
		{id: ids.MenuLoad, label: "Load Game"},
		{id: "menu.plugins", label: "Plugins", disabled: true, reason: "not implemented"},
		{id: ids.MenuOptions, label: "Options"},
		{id: "menu.tutorial", label: "Tutorial / Help", disabled: true, reason: "not implemented"},
		{id: ids.MenuCredits, label: "Credits"},
		{id: ids.MenuQuit, label: "Quit"},
	}
	if cont.disabled { // Continue leads the list only when it can be used
		specs = append([]buttonSpec{specs[0], cont}, specs[1:]...)
	} else {
		specs = append([]buttonSpec{cont}, specs...)
	}
	return stackButtons(cx, menuRailY, menuRailW, menuBtnH, menuBtnGap, specs)
}

// Low-RAM notice (PLAY-019): a card under the right rail, in the warning
// colour, with a Got it button. It warns only; nothing is blocked.
const (
	ramDismissID  = "menu.ram.dismiss"
	ramCardPad    = 12
	ramCardStripe = 3
	ramCardGap    = 12 // from the last rail button, and from the screen's bottom
	ramBtnW       = 96
	ramBtnH       = 28
)

// ramNoticeStyle is the notice's text: the warning token.
func ramNoticeStyle() textStyle { return withColor(tsBody, uiWarning) }

// lowRAMNotice is the notice to show now and the amount of memory it is
// for: no text when the hardware is unknown, big enough, or the player
// already dismissed the notice for this amount (Gameplay.RAMNoticeSeen).
func (a *App) lowRAMNotice() (text string, total uint64) {
	total = a.hardwareTotal()
	if total == 0 || total >= minimumRAM() || a.settings.Gameplay.RAMNoticeSeen == total {
		return "", 0
	}
	return lowRAMText(total), total
}

// dismissRAMNotice remembers the notice was seen for total bytes of RAM.
func (a *App) dismissRAMNotice(total uint64) {
	s := a.settings
	s.Gameplay.RAMNoticeSeen = total
	a.setSettings(s)
}

// ramNoticeCard is the notice's card: the rail's width, below the last
// menu button and above the screen's bottom edge.
func (a *App) ramNoticeCard(sw, sh int, text string) rect {
	x, w := menuRailX(sw), float32(menuRailW)
	lines := wrapPx(text, ramNoticeStyle(), float64(w-2*ramCardPad-ramCardStripe))
	h := float32(2*ramCardPad+ramBtnH+8) + float32(len(lines))*float32(lineH(ramNoticeStyle()))
	top := float32(0)
	for _, b := range a.mainMenuButtons(sw, sh) {
		top = max(top, b.y+b.h)
	}
	y := max(top+ramCardGap, float32(sh)-ramCardGap-h)
	if y+h > float32(sh) { // a window under the minimum: stay on screen
		y = float32(sh) - h
	}
	return rect{X: x, Y: y, W: w, H: h}
}

// mainMenuUI declares the main menu's widgets: the low-RAM notice.
func (a *App) mainMenuUI(sw, sh int) {
	text, total := a.lowRAMNotice()
	if text == "" {
		return
	}
	card := a.ramNoticeCard(sw, sh, text)
	st := ramNoticeStyle()
	lines := wrapPx(text, st, float64(card.W-2*ramCardPad-ramCardStripe))
	a.uiPanel(card)
	a.uiSt().queue(func(dst *ebiten.Image) {
		fillRect(dst, rect{X: card.X, Y: card.Y, W: ramCardStripe, H: card.H}, uiWarning)
		tx, ty := float64(card.X+ramCardPad+ramCardStripe), float64(card.Y+ramCardPad)
		for i, l := range lines {
			drawText(dst, l, tx, ty+float64(i)*lineH(st), st)
		}
	})
	btn := rect{X: card.Right() - ramCardPad - ramBtnW, Y: card.Bottom() - ramCardPad - ramBtnH, W: ramBtnW, H: ramBtnH}
	a.ngButton(ramDismissID, btn, "Got it", btnOpts{Tip: "Got it\nHide this notice. It comes back only if the computer's memory changes."},
		func() { a.dismissRAMNotice(total) })
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
	r := rect{X: b.x, Y: b.y, W: b.w, H: b.h}
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
	cx, cy := r.Center()
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
		"Music and sound effects: original, made in code (cmd/audiogen).",
		"Noto Sans: SIL Open Font License 1.1.",
		"Ambience and animal voices: generated with ElevenLabs",
		"(elevenlabs.io), free tier,",
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
