package mainmenu

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Update notice: a card under the version line, on the left because the
// low-RAM card can fill the right rail's foot. It offers the download and
// blocks nothing. Later hides it until the next launch.
const (
	UpdateDownloadID = "menu.update.download"
	UpdateLaterID    = "menu.update.later"
	updateCardX      = 56
	updateCardW      = 360
	updateRailGap    = 12
	updateDownloadW  = 110
	updateLaterW     = 72
	updateBtnGap     = 8
)

// Updates is what the menu needs to tell the player about a newer release.
type Updates interface {
	// UpdateAvailable is the newer release and the address to get it from,
	// as text to show ("" when none is known).
	UpdateAvailable() (version, page string)
	// DownloadUpdate opens the download page in the player's browser.
	DownloadUpdate()
}

// VersionLine is the running version, with the newer release when one is known.
func VersionLine(current, newer string) string {
	if newer == "" {
		return current
	}
	return fmt.Sprintf("%s (%s available)", current, newer)
}

// updateHeadline is the card's first line.
func updateHeadline(version string) string {
	return fmt.Sprintf("Version %s is available", version)
}

// UpdateCard is the notice's card: under the title and the version line, and
// clear of the rail on a narrow window.
func UpdateCard(sw int, page string) gfx.Rect {
	w := min(float32(updateCardW), RailX(sw)-updateCardX-updateRailGap)
	lines := gfx.WrapPx(page, gfx.Body, float64(w-2*ramCardPad-ramCardStripe))
	y := float32(28+gfx.LineH(gfx.Huge)+2+gfx.LineH(gfx.Body)) + 10
	h := float32(2*ramCardPad+ramBtnH+8) + float32(len(lines)+1)*float32(gfx.LineH(gfx.Body))
	return gfx.Rect{X: updateCardX, Y: y, W: w, H: h}
}

// declareUpdateNotice declares the card when a newer release is known and the
// player hasn't hidden it.
func (s *Screen) declareUpdateNotice(sw int) {
	version, page := s.sh.UpdateAvailable()
	if version == "" || s.updateHidden {
		return
	}
	card := UpdateCard(sw, page)
	lines := gfx.WrapPx(page, gfx.Body, float64(card.W-2*ramCardPad-ramCardStripe))
	headline := updateHeadline(version)
	k := s.sh.Kit()
	k.Panel(card)
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, gfx.Rect{X: card.X, Y: card.Y, W: ramCardStripe, H: card.H}, uikit.Accent)
		tx, ty := float64(card.X+ramCardPad+ramCardStripe), float64(card.Y+ramCardPad)
		gfx.Text(dst, headline, tx, ty, gfx.WithColor(gfx.Body, uikit.Heading))
		dim := gfx.WithColor(gfx.Body, uikit.TextDim)
		for i, l := range lines {
			gfx.Text(dst, l, tx, ty+float64(i+1)*gfx.LineH(dim), dim)
		}
	})
	y := card.Bottom() - ramCardPad - ramBtnH
	download := gfx.Rect{X: card.Right() - ramCardPad - updateDownloadW, Y: y, W: updateDownloadW, H: ramBtnH}
	later := gfx.Rect{X: download.X - updateBtnGap - updateLaterW, Y: y, W: updateLaterW, H: ramBtnH}
	s.noticeButton(UpdateLaterID, later, "Later",
		uikit.BtnOpts{Tip: "Later\nHide this notice until the next time you start the game."},
		func() { s.updateHidden = true })
	s.noticeButton(UpdateDownloadID, download, "Download",
		uikit.BtnOpts{Style: uikit.BtnPrimary, Tip: "Download\nOpen the download page in your browser."},
		s.sh.DownloadUpdate)
}

// noticeButton declares a notice card's button. Its action runs at once, also
// when the click comes from the id-driven click path of tests and captures,
// and once per click on either path.
func (s *Screen) noticeButton(id string, r gfx.Rect, label string, o uikit.BtnOpts, act func()) {
	k := s.sh.Kit()
	pressed := k.Button(id, r, label, o)
	if n := len(k.Cur); n > 0 && k.Cur[n-1].ID == id {
		k.Cur[n-1].Click = func() {
			k.OK()
			act()
		}
	}
	if pressed {
		act()
	}
}
