package saves

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The Load screen: the list of saves, a page at a time.

const (
	slotRowH = 40
	slotRows = 9
	// footerYOff is how far above the screen's bottom the Back button sits
	// (the same footer as every menu screen's).
	footerYOff = 64
)

// commitTag marks a commitment save in the load list; EndingTag a save
// whose ship has left (AX-8: the story is complete).
const (
	commitTag = "Commitment"
	EndingTag = "Complete: escaped by ship"
)

// slotNextPage is the index the "more saves" row sends (load.slot.-1):
// Click hands every load.slot.N click to LoadSlot(N).
const slotNextPage = -1

// slotWindow is the saves the Load screen shows now: slots start..start+n
// (all of them up to slotRows; a longer list gives slotRows-1 per page and
// a row to turn the page), and the number of pages.
func (b *Book) slotWindow() (start, n, pages int) {
	if len(b.Slots) <= slotRows {
		return 0, len(b.Slots), 1
	}
	per := slotRows - 1
	pages = (len(b.Slots) + per - 1) / per
	page := max(0, min(b.SlotPage, pages-1))
	start = page * per
	return start, min(per, len(b.Slots)-start), pages
}

// LoadButtons are the Load screen's buttons: Back, then one per slot shown,
// then, when the list is longer than a page, the row that turns the page.
func (b *Book) LoadButtons(sw, sh int) []uikit.Button {
	out := []uikit.Button{{ID: "load.back", Label: b.sh.T("Back"), X: 40, Y: float32(sh - footerYOff), W: 140, H: 36}}
	x := float32(sw)/2 - 300
	start, n, pages := b.slotWindow()
	for j := 0; j < n; j++ {
		i, sl := start+j, b.Slots[start+j]
		// the row's text is drawn by DrawLoad (proportional columns)
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(i), Quiet: true,
			X: x, Y: float32(120 + j*(slotRowH+6)), W: 600, H: slotRowH,
			Disabled: sl.Err != "", Reason: sl.Err})
	}
	if pages > 1 {
		page := start/(slotRows-1) + 1
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(slotNextPage),
			Label: b.sh.Tf("More saves (page %d of %d)", page, pages),
			X:     x, Y: float32(120 + n*(slotRowH+6)), W: 600, H: slotRowH})
	}
	return out
}

// DrawLoad paints the Load screen.
func (b *Book) DrawLoad(screen *ebiten.Image, sw, sh int) {
	body := []string{}
	if len(b.Slots) == 0 {
		body = append(body, b.sh.T("No saves yet."), "", b.sh.T("Save from the pause menu (Esc) while playing; the game also"), b.sh.T("autosaves every morning at 06:00."))
	}
	buttons := b.LoadButtons(sw, sh)
	b.sh.PaintMenu(screen, sw, sh, b.sh.T("Load Game"), body, buttons)
	start, n, _ := b.slotWindow()
	for j := 0; j < n; j++ {
		sl := b.Slots[start+j]
		bt := buttons[j+1]
		bx, by := float64(bt.X), float64(bt.Y)
		right := float64(bt.X+bt.W) - 12
		if !sl.Modified.IsZero() {
			when := sl.Modified.Format("Jan 2 15:04")
			gfx.Text(screen, when, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(when, gfx.Small)) + 16
		}
		if sl.Ending != "" {
			gfx.Text(screen, b.sh.T(EndingTag), right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.Good, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(b.sh.T(EndingTag), gfx.Small)) + 16
		}
		if sl.Commitment {
			gfx.Text(screen, b.sh.T(commitTag), right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.Accent, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(b.sh.T(commitTag), gfx.Small)) + 16
		}
		gfx.Text(screen, gfx.Ellipsize(sl.Name, gfx.Label, right-bx-12), bx+12, by+3, gfx.Label)
		sub, c := b.sh.Tf("%s  %d colonists", sl.Date, sl.Colonists), color.Color(uikit.TextDim)
		if sl.Err != "" {
			sub, c = b.sh.Tf("cannot load: %s", sl.Err), uikit.Danger
		}
		gfx.Text(screen, gfx.Ellipsize(sub, gfx.Small, float64(bt.W)-24), bx+12, by+float64(bt.H)-gfx.LineH(gfx.Small)-2, gfx.WithColor(gfx.Small, c))
	}
}
