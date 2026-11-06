package dossier

import (
	"fmt"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The Look tab: every size, place and text setting of the card, in groups. A
// control that has no effect now is greyed with the reason beside it and in its
// tooltip (the reason line is plain text, not only a hover).

const (
	numRowH   = 46 // label and typed entry above, slider below
	anchorRow = 3*28 + 4
	fieldW    = 64
)

// reasonX is where the reason of a greyed control starts, from the row's left.
const reasonX = 210

// Reasons a Look control is off.
func noRestyleWhy(tr lang.Tr) string {
	return tr.T("Needs \"Restyle the pane\": a plain pane cannot hold the card.")
}
func dockedWhy(tr lang.Tr) string {
	return tr.T("The card is docked in the pane, so it has no place of its own.")
}
func floatingWhy(tr lang.Tr) string { return tr.T("Only a docked card sets the pane's height.") }
func logBodyWhy(tr lang.Tr) string {
	return tr.T("Only a floating card puts the log in the pane's body.")
}
func noCompactWhy(tr lang.Tr) string {
	return tr.T("Needs \"Compress notifications\" and a floating card.")
}

// anchorName is the label of nine-position button i.
func anchorName(tr lang.Tr, i int) string {
	switch i {
	case 0:
		return tr.T("Top left")
	case 1:
		return tr.T("Top")
	case 2:
		return tr.T("Top right")
	case 3:
		return tr.T("Left")
	case 4:
		return tr.T("Centre")
	case 5:
		return tr.T("Right")
	case 6:
		return tr.T("Bottom left")
	case 7:
		return tr.T("Bottom")
	}
	return tr.T("Bottom right")
}

// numSpec is one number row: a label, a typed entry and a slider.
type numSpec struct {
	id, label, tip, off string
	v, lo, hi           int
	format              func(int) string
}

func plainNum(v int) string { return strconv.Itoa(v) }

func signedDigits(r rune) bool { return r >= '0' && r <= '9' || r == '-' }

// buf is the text typed into field id. While the field is not being edited it
// follows the model; the frame an edit ends keeps the typed text, so the commit
// that frame reports reads what was typed.
func (s *Setup) buf(id string, follow string) *string {
	b := s.bufs[id]
	if b == nil {
		b = new(string)
		s.bufs[id] = b
	}
	switch {
	case s.k.Edit.ID == id:
		s.typing[id] = true
	case s.typing[id]:
		s.typing[id] = false
	default:
		*b = follow
	}
	return b
}

// num declares a number row and returns the value the player chose this frame
// (the slider live, the typed entry when committed).
func (s *Setup) num(f *uikit.Form, sp numSpec) (int, bool) {
	row, ok := f.Row(numRowH)
	if !ok {
		return sp.v, false
	}
	k := s.k
	if sp.format == nil {
		sp.format = plainNum
	}
	ls := gfx.Body
	if sp.off != "" {
		ls.Color = uikit.TextDisabled
	}
	k.Label(row.X, row.Y+2, sp.label, ls)
	s.reason(row, sp.off)
	if sp.off == "" && sp.tip != "" {
		k.Tip("dossier.setup.tip."+sp.id, gfx.Rect{X: row.X, Y: row.Y, W: reasonX, H: 20}, sp.label+"\n"+sp.tip)
	}
	fid := "dossier.setup.t." + sp.id
	text := s.buf(fid, strconv.Itoa(sp.v))
	_, committed := k.TextField(fid, gfx.Rect{X: row.Right() - fieldW, Y: row.Y, W: fieldW, H: 22}, text,
		uikit.TextFieldSpec{MaxLen: 4, Valid: signedDigits, Disabled: sp.off != "", Reason: sp.off,
			Tip: s.a.Tf("%s\nType a number from %d to %d, then press Enter.", sp.label, sp.lo, sp.hi)})
	nv, changed := sp.v, false
	if n, err := strconv.Atoi(*text); committed && err == nil {
		nv, changed = max(sp.lo, min(sp.hi, n)), true
		*text = strconv.Itoa(nv)
	} else if committed {
		*text = strconv.Itoa(sp.v)
	}
	sv := sp.v
	sr := gfx.Rect{X: row.X + 2, Y: row.Y + 25, W: row.W - 4, H: 18}
	if k.SliderInt("dossier.setup.f."+sp.id, sr, &sv, uikit.SliderSpec{Min: float64(sp.lo), Max: float64(sp.hi), Step: 1,
		Title: sp.label, Format: func(x float64) string { return sp.format(int(x)) }, Disabled: sp.off != "", Reason: sp.off}) {
		nv, changed = sv, true
	}
	return nv, changed && nv != sp.v
}

// reason writes why a control is off, dim and small, between its label and its
// entry.
func (s *Setup) reason(row gfx.Rect, why string) {
	if why == "" {
		return
	}
	st := gfx.WithColor(gfx.Small, uikit.TextDim)
	s.k.Label(row.X+reasonX, row.Y+5, uikit.FitText(why, st, row.W-reasonX-fieldW-8), st)
}

// check declares a switch row; off greys it with a reason.
func (s *Setup) check(f *uikit.Form, id, label, tip, off string, v bool) (bool, bool) {
	row, ok := f.Row(28)
	if !ok {
		return v, false
	}
	nv := v
	changed := s.k.Check("dossier.setup.f."+id, row, label, &nv, uikit.CheckOpts{Disabled: off != "", Reason: off, Tip: label + "\n" + tip})
	s.reason(gfx.Rect{X: row.X, Y: row.Y + 3, W: row.W + fieldW, H: row.H}, off)
	// A click by id toggles the kit's copy after this call and reports it on
	// the next one, when the copy is fresh: the new value is always the opposite.
	return !v, changed
}

// drawLook declares the Look tab in c.
func (s *Setup) drawLook(c gfx.Rect) {
	k := s.k
	area := k.ScrollArea("dossier.setup.look", c, s.lookH, uikit.ScrollOpts{Step: 40, Focusable: true})
	f := uikit.NewForm(area)
	h := s.HUD()
	s.lookCard(f, h)
	s.lookPane(f, h)
	s.lookFloat(f, h)
	s.lookText(f, h)
	s.lookH = f.Content()
}

// flag edits one switch of the settings.
func (s *Setup) flag(f *uikit.Form, id, label, tip, off string, get func(*config.HUD) *bool, h config.HUD) {
	cur := *get(&h)
	if nv, ch := s.check(f, id, label, tip, off, cur); ch {
		s.Edit(func(h *config.HUD) { *get(h) = nv })
	}
}

// number edits one number of the settings.
func (s *Setup) number(f *uikit.Form, sp numSpec, get func(*config.HUD) *int, h config.HUD) {
	sp.v = *get(&h)
	if nv, ch := s.num(f, sp); ch {
		s.Edit(func(h *config.HUD) { *get(h) = nv })
	}
}

func (s *Setup) lookCard(f *uikit.Form, h config.HUD) {
	f.SectionTitle(s, s.a.T("The card"))
	s.flag(f, "enabled", s.a.T("Show the colonist card"), s.a.T("Off: the pane and the map look as they always did."), "",
		func(h *config.HUD) *bool { return &h.Enabled }, h)
	s.number(f, numSpec{id: "refresh", label: s.a.T("Refresh every"), lo: 1, hi: 30,
		tip:    s.a.T("How often the card reads the colony again, in tenths of a second. Lower is livelier and costs more."),
		format: func(v int) string { return fmt.Sprintf("%.1f s", float64(v)/10) }},
		func(h *config.HUD) *int { return &h.RefreshTenths }, h)
	s.flag(f, "restyle", s.a.T("Restyle the pane"), s.a.T("Off: the pane keeps its plain look and the card floats over the map."), "",
		func(h *config.HUD) *bool { return &h.Pane.Restyle }, h)
	s.flag(f, "docked", s.a.T("Dock the card in the pane"), s.a.T("Off: the card floats over the map."), noRestyle(s.a, h),
		func(h *config.HUD) *bool { return &h.Docked }, h)
	f.Gap(6)
}

// paneShare reads a pane height as the share of the screen the pane and its
// tabs take (the height is set for a 720-tall screen).
func paneShare(v int) string {
	return fmt.Sprintf("%d (%d%% of the screen)", v, ((v+sheet.DockTabsH)*100+sheet.DockRefH/2)/sheet.DockRefH)
}

func noRestyle(tr lang.Tr, h config.HUD) string {
	if !h.Pane.Restyle {
		return noRestyleWhy(tr)
	}
	return ""
}

func (s *Setup) lookPane(f *uikit.Form, h config.HUD) {
	f.SectionTitle(s, s.a.T("The pane"))
	heightOff := noRestyle(s.a, h)
	if heightOff == "" && !h.Docked {
		heightOff = floatingWhy(s.a)
	}
	s.number(f, numSpec{id: "height", label: s.a.T("Pane height"), lo: config.HUDPaneMinH, hi: config.HUDPaneMaxH, off: heightOff,
		format: paneShare,
		tip:    s.a.T("How tall the pane is while it holds the card, on a 720-tall screen; a taller screen grows it in step. With its tabs it takes at most 30% of the screen.")},
		func(h *config.HUD) *int { return &h.Pane.Height }, h)
	s.number(f, numSpec{id: "mintabs", label: s.a.T("Tabs kept in view"), lo: 6, hi: 12, off: noRestyle(s.a, h),
		tip: s.a.T("The pane is at least this many tabs wide.")},
		func(h *config.HUD) *int { return &h.Pane.MinTabs }, h)
	s.number(f, numSpec{id: "tabw", label: s.a.T("Tab width"), lo: 72, hi: 150, off: noRestyle(s.a, h),
		tip: s.a.T("How wide each tab of the pane is.")},
		func(h *config.HUD) *int { return &h.Pane.TabW }, h)
	s.flag(f, "emblem", s.a.T("Faction emblem in the header"), s.a.T("Show who the selected creature belongs to."), noRestyle(s.a, h),
		func(h *config.HUD) *bool { return &h.Pane.FactionIcon }, h)
	logOff := noRestyle(s.a, h)
	if logOff == "" && h.Docked {
		logOff = logBodyWhy(s.a)
	}
	s.flag(f, "logbody", s.a.T("Log in the pane's body"), s.a.T("A floating card leaves the pane's body free for the log."), logOff,
		func(h *config.HUD) *bool { return &h.Pane.LogBody }, h)
	f.Gap(6)
}

func (s *Setup) lookFloat(f *uikit.Form, h config.HUD) {
	f.SectionTitle(s, s.a.T("The floating card"))
	off := ""
	if h.Docked {
		off = dockedWhy(s.a)
	}
	s.anchorPicker(f, h, off)
	sw, sh := s.sz.Size()
	minX, maxX, minY, maxY := hudlayout.CardOffsetBounds(sw, sh, h.Card.Anchor, h.Card.W, h.Card.H)
	s.number(f, numSpec{id: "offx", label: s.a.T("Nudge sideways"), lo: minX, hi: maxX, off: off,
		tip: s.a.T("Moves the card from its place, to the right when positive.")},
		func(h *config.HUD) *int { return &h.Card.OffsetX }, h)
	s.number(f, numSpec{id: "offy", label: s.a.T("Nudge up or down"), lo: minY, hi: maxY, off: off,
		tip: s.a.T("Moves the card from its place, down when positive.")},
		func(h *config.HUD) *int { return &h.Card.OffsetY }, h)
	s.number(f, numSpec{id: "w", label: s.a.T("Card width"), lo: 100, hi: 1000, off: off, tip: s.a.T("A screen too small for it shrinks the card.")},
		func(h *config.HUD) *int { return &h.Card.W }, h)
	s.number(f, numSpec{id: "h", label: s.a.T("Card height"), lo: 100, hi: 1000, off: off, tip: s.a.T("A screen too small for it shrinks the card.")},
		func(h *config.HUD) *int { return &h.Card.H }, h)
	s.flag(f, "compress", s.a.T("Compress notifications"), s.a.T("Letters beside the card pack closer together."), "",
		func(h *config.HUD) *bool { return &h.Card.CompressLetters }, h)
	gapOff := ""
	if h.Docked || !h.Card.CompressLetters {
		gapOff = noCompactWhy(s.a)
	}
	s.number(f, numSpec{id: "gap", label: s.a.T("Gap between notifications"), lo: 0, hi: 12, off: gapOff,
		tip: s.a.T("Space between the letters, in pixels.")},
		func(h *config.HUD) *int { return &h.Card.LetterGap }, h)
	f.Gap(6)
}

// anchorPicker is the nine-position picker of the floating card.
func (s *Setup) anchorPicker(f *uikit.Form, h config.HUD, off string) {
	row, ok := f.Row(anchorRow)
	if !ok {
		return
	}
	k := s.k
	ls := gfx.Body
	if off != "" {
		ls.Color = uikit.TextDisabled
	}
	k.Label(row.X, row.Y+2, s.a.T("Place on the screen"), ls)
	s.reason(gfx.Rect{X: row.X, Y: row.Y, W: row.W + 3*92, H: row.H}, off)
	const bw, bh = 88.0, 26.0
	x0 := row.Right() - 3*(bw+4) + 4
	for i := 0; i < 9; i++ {
		i, name := i, anchorName(s.a, i)
		r := gfx.Rect{X: x0 + float32(i%3)*(bw+4), Y: row.Y + float32(i/3)*(bh+2), W: bw, H: bh}
		uikit.StyledButton(k, fmt.Sprintf("dossier.setup.anchor.%d", i), r, name,
			uikit.BtnOpts{Selected: h.Card.Anchor == i, Disabled: off != "", Reason: off, Tip: s.a.Tf("Place on the screen\nStart from the %s of the screen.", lower(name))},
			func() { s.setAnchor(i) })
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// setAnchor moves the card to anchor a and keeps its nudge on the screen.
func (s *Setup) setAnchor(a int) {
	sw, sh := s.sz.Size()
	s.Edit(func(h *config.HUD) {
		h.Card.Anchor = a
		minX, maxX, minY, maxY := hudlayout.CardOffsetBounds(sw, sh, a, h.Card.W, h.Card.H)
		h.Card.OffsetX = max(minX, min(maxX, h.Card.OffsetX))
		h.Card.OffsetY = max(minY, min(maxY, h.Card.OffsetY))
	})
}

func (s *Setup) lookText(f *uikit.Form, h config.HUD) {
	f.SectionTitle(s, s.a.T("Text"))
	pct := func(v int) string { return strconv.Itoa(v) + "%" }
	size := func(v int) string { return fmt.Sprintf("%+d", v) }
	s.number(f, numSpec{id: "body", label: s.a.T("Text size"), lo: 7, hi: 20, tip: s.a.T("The size of the card's ordinary text.")},
		func(h *config.HUD) *int { return &h.Text.BodySize }, h)
	s.number(f, numSpec{id: "bodyline", label: s.a.T("Text line height"), lo: config.HUDLineMinPct, hi: 250, format: pct, tip: s.a.T("Space for each line of ordinary text.")},
		func(h *config.HUD) *int { return &h.Text.BodyLinePct }, h)
	s.number(f, numSpec{id: "large", label: s.a.T("Name size"), lo: 0, hi: 24, format: size, tip: s.a.T("How much bigger than the text the name is.")},
		func(h *config.HUD) *int { return &h.Text.LargeDelta }, h)
	s.number(f, numSpec{id: "largeline", label: s.a.T("Name line height"), lo: config.HUDLineMinPct, hi: 250, format: pct, tip: s.a.T("Space for the name's line.")},
		func(h *config.HUD) *int { return &h.Text.LargeLinePct }, h)
	s.number(f, numSpec{id: "small", label: s.a.T("Small text size"), lo: -5, hi: 0, format: size, tip: s.a.T("How much smaller than the text the small print is.")},
		func(h *config.HUD) *int { return &h.Text.SmallDelta }, h)
	s.number(f, numSpec{id: "smallline", label: s.a.T("Small text line height"), lo: config.HUDLineMinPct, hi: 250, format: pct, tip: s.a.T("Space for each line of small print.")},
		func(h *config.HUD) *int { return &h.Text.SmallLinePct }, h)
	s.number(f, numSpec{id: "labelw", label: s.a.T("Label column width"), lo: 30, hi: 150, tip: s.a.T("Width kept for the names of needs, skills and stats.")},
		func(h *config.HUD) *int { return &h.Text.LabelW }, h)
	s.number(f, numSpec{id: "valuew", label: s.a.T("Value column width"), lo: 30, hi: 150, tip: s.a.T("Width kept for the numbers beside them.")},
		func(h *config.HUD) *int { return &h.Text.ValueW }, h)
	s.flag(f, "decimals", s.a.T("Show decimals"), s.a.T("Skill levels and stats with their fractions."), "",
		func(h *config.HUD) *bool { return &h.Text.Decimals }, h)
}
