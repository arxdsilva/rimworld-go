package mainmenu

import (
	"errors"
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Main menu metrics (NG-1 §2.1): a right rail of 300x40 buttons, 10 px
// apart, from y = 140.
const (
	BtnH    = 40
	BtnGap  = 10
	railTop = 140
)

// Low-RAM notice (PLAY-019): a card under the right rail, in the warning
// colour, with a Got it button. It warns only; nothing is blocked.
const (
	RAMDismissID  = "menu.ram.dismiss"
	ramCardPad    = 12
	ramCardStripe = 3
	ramCardGap    = 12 // from the last rail button, and from the screen's bottom
	ramBtnW       = 96
	ramBtnH       = 28
)

// Screen is the main menu: its background scene, its rail and its notice.
type Screen struct {
	sh      Shell
	splash  *ebiten.Image
	diorama *Diorama
	tried   bool
}

// New is the menu screen over its shell.
func New(sh Shell) *Screen { return &Screen{sh: sh} }

// SetDiorama replaces the background scene (tests).
func (s *Screen) SetDiorama(d *Diorama) { s.diorama = d }

// LoadScene reads the diorama once; a failure leaves the menu on the
// gradient backdrop.
func (s *Screen) LoadScene() {
	if s.tried {
		return
	}
	d, err := s.sh.LoadMenuScene()
	if errors.Is(err, ErrNoLoader) {
		return
	}
	s.tried = true
	if err == nil {
		s.diorama = d
	}
}

// Buttons are the rail's buttons: Continue leads the list only when it
// can be used.
func (s *Screen) Buttons(sw, sh int) []uikit.Button {
	cx := RailX(sw)
	cont := uikit.ButtonSpec{ID: ids.MenuContinue, Label: "Continue", Disabled: !s.sh.HasSaves(), Reason: "no saves yet"}
	specs := []uikit.ButtonSpec{
		{ID: ids.MenuNewGame, Label: "New Colony"},
		{ID: ids.MenuLoad, Label: "Load Game"},
		{ID: "menu.plugins", Label: "Plugins", Disabled: true, Reason: "not implemented"},
		{ID: ids.MenuOptions, Label: "Options"},
		{ID: "menu.tutorial", Label: "Tutorial / Help", Disabled: true, Reason: "not implemented"},
		{ID: ids.MenuCredits, Label: "Credits"},
		{ID: ids.MenuQuit, Label: "Quit"},
	}
	if cont.Disabled {
		specs = append([]uikit.ButtonSpec{specs[0], cont}, specs[1:]...)
	} else {
		specs = append([]uikit.ButtonSpec{cont}, specs...)
	}
	return uikit.StackButtons(cx, railTop, RailW, BtnH, BtnGap, specs)
}

// RAMNoticeStyle is the notice's text: the warning token.
func RAMNoticeStyle() gfx.TextStyle { return gfx.WithColor(gfx.Body, uikit.Warning) }

// RAMNoticeCard is the notice's card: the rail's width, below the last
// menu button and above the screen's bottom edge.
func (s *Screen) RAMNoticeCard(sw, sh int, text string) gfx.Rect {
	x, w := RailX(sw), float32(RailW)
	lines := gfx.WrapPx(text, RAMNoticeStyle(), float64(w-2*ramCardPad-ramCardStripe))
	h := float32(2*ramCardPad+ramBtnH+8) + float32(len(lines))*float32(gfx.LineH(RAMNoticeStyle()))
	top := float32(0)
	for _, b := range s.Buttons(sw, sh) {
		top = max(top, b.Y+b.H)
	}
	y := max(top+ramCardGap, float32(sh)-ramCardGap-h)
	if y+h > float32(sh) { // a window under the minimum: stay on screen
		y = float32(sh) - h
	}
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}

// Declare declares the main menu's widgets: the low-RAM notice.
func (s *Screen) Declare(sw, sh int) {
	text, total := s.sh.LowRAMNotice()
	if text == "" {
		return
	}
	card := s.RAMNoticeCard(sw, sh, text)
	st := RAMNoticeStyle()
	lines := gfx.WrapPx(text, st, float64(card.W-2*ramCardPad-ramCardStripe))
	k := s.sh.Kit()
	k.Panel(card)
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, gfx.Rect{X: card.X, Y: card.Y, W: ramCardStripe, H: card.H}, uikit.Warning)
		tx, ty := float64(card.X+ramCardPad+ramCardStripe), float64(card.Y+ramCardPad)
		for i, l := range lines {
			gfx.Text(dst, l, tx, ty+float64(i)*gfx.LineH(st), st)
		}
	})
	btn := gfx.Rect{X: card.Right() - ramCardPad - ramBtnW, Y: card.Bottom() - ramCardPad - ramBtnH, W: ramBtnW, H: ramBtnH}
	opts := uikit.BtnOpts{Tip: "Got it\nHide this notice. It comes back only if the computer's memory changes."}
	dismiss := func() { s.sh.DismissRAMNotice(total) }
	// The click runs at once, also when it comes from the id-driven click
	// path of tests and captures.
	pressed := k.Button(RAMDismissID, btn, "Got it", opts)
	if n := len(k.Cur); n > 0 && k.Cur[n-1].ID == RAMDismissID {
		k.Cur[n-1].Click = func() {
			k.OK()
			dismiss()
		}
	}
	if pressed {
		dismiss()
	}
}

// Draw paints the main menu: the diorama (or the gradient), the splash,
// the title with the version and the rail.
func (s *Screen) Draw(screen *ebiten.Image, sw, sh int) {
	if !s.DrawDiorama(screen, sw, sh, 0) {
		uikit.DrawMenuBackdrop(screen, sw, sh)
	}
	s.drawSplash(screen, sw, sh)
	title := gfx.Huge
	shadow := title
	shadow.Color = color.RGBA{A: 0xC0}
	gfx.Text(screen, "rimworld-go", 58, 30, shadow)
	title.Color = uikit.Heading
	gfx.Text(screen, "rimworld-go", 56, 28, title)
	ver := gfx.Body
	ver.Color = uikit.TextDim
	gfx.Text(screen, fmt.Sprintf("rimworld-go  %s", s.sh.Version()), 58, 28+gfx.LineH(gfx.Huge)+2, ver)
	mx, my := s.sh.Cursor()
	for i, b := range s.Buttons(sw, sh) {
		drawButton(screen, b, i == 0 && !b.Disabled, b.Contains(mx, my))
	}
}

// drawButton is a main-menu rail button in the real font; the first
// enabled one is the gold primary.
func drawButton(dst *ebiten.Image, b uikit.Button, primary, hover bool) {
	r := gfx.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
	style := uikit.BtnNormal
	if primary {
		style = uikit.BtnPrimary
	}
	fill, border, text := style.Colors(hover && !b.Disabled, false, false)
	if b.Disabled {
		fill, border, text = uikit.BtnDisabled, uikit.BtnBorder, uikit.TextDisabled
	}
	uikit.FillRect(dst, r, fill)
	uikit.StrokeRect(dst, r, border)
	st := gfx.Large
	st.Color, st.Align = text, gfx.AlignCenter
	cx, cy := r.Center()
	gfx.Text(dst, b.Label, float64(cx), float64(cy)-gfx.LineH(st)/2, st)
}

// CreditsLines is the Credits screen body.
func CreditsLines() []string {
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

// NewestSlot is the index of the loadable save written last (-1 when
// none), commitment saves included; ties keep the list order.
func NewestSlot(slots []ports.SaveSlot) int {
	best := -1
	for i, sl := range slots {
		if sl.Err != "" {
			continue
		}
		if best < 0 || sl.Modified.After(slots[best].Modified) {
			best = i
		}
	}
	return best
}
