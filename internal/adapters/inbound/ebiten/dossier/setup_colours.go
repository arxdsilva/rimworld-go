package dossier

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The Colours tab: the colours of the card in three groups, a colour editor
// (sample, hex entry, hue, saturation, lightness and alpha) and a preview of
// the four bar styles. Only differences from the theme are stored
// (HUD.Colors); the tree_* colours of the layout editor are not editable.

const (
	colListW  = 210
	colRowH   = 24
	colHeadH  = 24
	hexFieldW = 110
)

// colourEntry is one editable colour: its key and its name.
type colourEntry struct{ key, name string }

// colourGroup is a titled run of colours.
type colourGroup struct {
	title   string
	entries []colourEntry
}

func colourGroups() []colourGroup {
	return []colourGroup{
		{"The card", []colourEntry{
			{"text", "Main text"}, {"text_off", "Dimmed text"}, {"rule", "Lines"},
			{"tone_critical", "Critical"}, {"tone_warning", "Warning"}, {"tone_neutral", "Neutral"},
			{"tone_good", "Good"}, {"tone_top", "Excellent"},
			{"bar_back", "Bar background"}, {"bar_main", "Bar fill"}, {"bar_low", "Bar fill when low"}, {"bar_tick", "Bar threshold mark"},
			{"picker_text", "Selector text"}, {"picker_back", "Selector background"},
		}},
		{"Skills", []colourEntry{
			{"skill_like", "Minor passion"}, {"skill_love", "Major passion"},
			{"skill_full", "Learning is full"}, {"skill_busy", "Skill in use"},
		}},
		{"Sides", []colourEntry{
			{"side_own", "Your colony"}, {"side_ally", "Allied"}, {"side_free", "Independent"},
			{"side_foe", "Hostile"}, {"side_wild", "Wild"}, {"side_held", "Prisoner"},
		}},
	}
}

// colourName is the name of a key, "" for one the player cannot edit.
func colourName(key string) string {
	for _, g := range colourGroups() {
		for _, e := range g.entries {
			if e.key == key {
				return e.name
			}
		}
	}
	return ""
}

// colourTab is the Colours tab's own state: the colour being edited, and its
// hue, saturation and lightness, which an RGB colour cannot hold at grey or at
// the ends.
type colourTab struct {
	sel   string
	hsl   [3]float64
	made  color.RGBA // the colour hsl was last matched to
	listH float32
	editH float32
}

// colourOf is the colour a key has now: the player's, else the theme's.
func colourOf(h config.HUD, key string) color.RGBA {
	c, _ := uikit.DossierColor(key)
	if v, ok := h.Colors[key]; ok {
		if o, ok := sheet.ParseColor(v); ok {
			c = o
		}
	}
	return c
}

// shown is a straight-alpha colour as the screen takes it.
func shown(c color.RGBA) color.RGBA {
	return uikit.Fade(color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xFF}, c.A)
}

// setColour stores c for key; the theme's own colour deletes the entry, so only
// differences are kept.
func (s *Setup) setColour(key string, c color.RGBA) {
	def, _ := uikit.DossierColor(key)
	s.Edit(func(h *config.HUD) {
		if c == def {
			delete(h.Colors, key)
			return
		}
		if h.Colors == nil {
			h.Colors = map[string]string{}
		}
		h.Colors[key] = sheet.FormatColor(c)
	})
}

// drawColours declares the Colours tab in c.
func (s *Setup) drawColours(c gfx.Rect) {
	if s.col.sel == "" {
		s.col.sel = colourGroups()[0].entries[0].key
	}
	h := s.HUD()
	left := gfx.Rect{X: c.X, Y: c.Y, W: colListW, H: c.H}
	right := gfx.Rect{X: c.X + colListW + 12, Y: c.Y, W: c.W - colListW - 12, H: c.H}
	s.colourList(left, h)
	s.colourEditor(right, h)
}

// colourList is the grouped list of colours.
func (s *Setup) colourList(r gfx.Rect, h config.HUD) {
	k := s.k
	area := k.ScrollArea("dossier.setup.collist", r, s.col.listH, uikit.ScrollOpts{Step: 48, Focusable: true})
	f := uikit.NewForm(area)
	for _, g := range colourGroups() {
		if row, ok := f.Row(colHeadH); ok {
			st := gfx.WithColor(gfx.Label, uikit.Heading)
			k.Label(row.X, row.Y+4, g.title, st)
		}
		for _, e := range g.entries {
			row, ok := f.Row(colRowH)
			if !ok {
				continue
			}
			id := "dossier.setup.col." + e.key
			sel := func() { s.col.sel = e.key }
			if k.Add(uikit.Widget{ID: id, Label: e.name, R: row, Click: sel, Tip: e.name}) && k.Press(row) {
				sel()
			}
			cur, changed, selected, hot := colourOf(h, e.key), h.Colors[e.key] != "", s.col.sel == e.key, k.Hot == id
			k.Queue(func(dst *ebiten.Image) { drawColourRow(dst, row, e.name, cur, changed, selected, hot) })
		}
		f.Gap(6)
	}
	s.col.listH = f.Content()
}

func drawColourRow(dst *ebiten.Image, row gfx.Rect, name string, cur color.RGBA, changed, selected, hot bool) {
	switch {
	case selected:
		uikit.FillRect(dst, row, uikit.BtnSelected)
	case hot:
		uikit.FillRect(dst, row, uikit.Fade(uikit.PanelBorder, 90))
	}
	sw := gfx.Rect{X: row.X + 4, Y: row.Y + 4, W: 22, H: row.H - 8}
	uikit.FillRect(dst, sw, uikit.SliderTrack)
	uikit.FillRect(dst, sw, shown(cur))
	uikit.StrokeRect(dst, sw, uikit.PanelBorder)
	st := gfx.WithColor(gfx.Body, uikit.Text)
	if changed {
		st = gfx.WithColor(gfx.Body, uikit.Heading)
	}
	gfx.Text(dst, uikit.FitText(name, st, row.W-sw.W-24), float64(sw.Right()+8), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
}

// colourEditor is the editor for the selected colour and the bar styles.
func (s *Setup) colourEditor(r gfx.Rect, h config.HUD) {
	k := s.k
	key := s.col.sel
	area := k.ScrollArea("dossier.setup.coledit", r, s.col.editH, uikit.ScrollOpts{Step: 48, Focusable: true})
	f := uikit.NewForm(area)
	f.SectionTitle(s, colourName(key))
	cur := colourOf(h, key)
	if row, ok := f.Row(48); ok {
		k.Queue(func(dst *ebiten.Image) { drawSample(dst, row, cur) })
	}
	s.hexRow(f, key, cur)
	hh, ss, ll := s.colourHSL(cur)
	if v, ch := s.num(f, numSpec{id: "hue", label: "Hue", v: int(hh + 0.5), lo: 0, hi: 359, format: func(v int) string { return strconv.Itoa(v) + "°" }}); ch {
		s.col.hsl[0] = float64(v)
		s.pushHSL(key, cur.A)
	}
	if v, ch := s.num(f, numSpec{id: "sat", label: "Saturation", v: int(ss*100 + 0.5), lo: 0, hi: 100, format: pctText}); ch {
		s.col.hsl[1] = float64(v) / 100
		s.pushHSL(key, cur.A)
	}
	if v, ch := s.num(f, numSpec{id: "light", label: "Lightness", v: int(ll*100 + 0.5), lo: 0, hi: 100, format: pctText}); ch {
		s.col.hsl[2] = float64(v) / 100
		s.pushHSL(key, cur.A)
	}
	if v, ch := s.num(f, numSpec{id: "alpha", label: "Opacity", v: int(cur.A), lo: 0, hi: 255}); ch {
		cur.A = uint8(v)
		s.setColour(key, cur)
	}
	s.resetRow(f, h, key)
	s.barStyles(f, h)
	s.col.editH = f.Content()
}

func pctText(v int) string { return strconv.Itoa(v) + "%" }

// colourHSL is the hue, saturation and lightness of cur; they are kept while cur
// is what they made, so grey does not lose its hue.
func (s *Setup) colourHSL(cur color.RGBA) (h, sat, l float64) {
	if s.col.made != cur {
		s.col.hsl[0], s.col.hsl[1], s.col.hsl[2] = sheet.RGBToHSL(cur)
		s.col.made = cur
	}
	return s.col.hsl[0], s.col.hsl[1], s.col.hsl[2]
}

// pushHSL stores the colour the edited hue, saturation and lightness make.
func (s *Setup) pushHSL(key string, alpha uint8) {
	c := sheet.HSLToRGB(s.col.hsl[0], s.col.hsl[1], s.col.hsl[2], alpha)
	s.col.made = c
	s.setColour(key, c)
}

func hexDigit(r rune) bool {
	return r == '#' || r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}

// hexRow is the typed #RRGGBBAA entry.
func (s *Setup) hexRow(f *uikit.Form, key string, cur color.RGBA) {
	row, ok := f.Row(32)
	if !ok {
		return
	}
	k := s.k
	k.Label(row.X, row.Y+5, "Colour code", gfx.Body)
	id := "dossier.setup.t.hex"
	text := s.buf(id, sheet.FormatColor(cur))
	_, committed := k.TextField(id, gfx.Rect{X: row.Right() - hexFieldW, Y: row.Y + 2, W: hexFieldW, H: 24}, text, uikit.TextFieldSpec{
		MaxLen: 9, Valid: hexDigit,
		Tip: "Colour code\nSix or eight hex digits, #RRGGBB or #RRGGBBAA; the last two are the opacity. Press Enter to keep it."})
	if !committed {
		return
	}
	if c, ok := sheet.ParseColor(*text); ok {
		s.setColour(key, c)
		return
	}
	*text = sheet.FormatColor(cur)
	s.a.Hint("A colour code is # followed by six or eight hex digits, such as #3FA7E0FF.")
}

// resetRow has the two reset buttons.
func (s *Setup) resetRow(f *uikit.Form, h config.HUD, key string) {
	row, ok := f.Row(36)
	if !ok {
		return
	}
	w := (row.W - 8) / 2
	one, all := "", ""
	if _, ok := h.Colors[key]; !ok {
		one = "This colour is already the theme's."
	}
	if len(h.Colors) == 0 {
		all = "Every colour is already the theme's."
	}
	uikit.StyledButton(s.k, "dossier.setup.col.reset1", gfx.Rect{X: row.X, Y: row.Y + 4, W: w, H: 28}, "Reset this colour",
		uikit.BtnOpts{Disabled: one != "", Reason: one, Tip: "Reset this colour\nGo back to the theme's colour for it."},
		func() { s.Edit(func(h *config.HUD) { delete(h.Colors, key) }) })
	uikit.StyledButton(s.k, "dossier.setup.col.resetall", gfx.Rect{X: row.X + w + 8, Y: row.Y + 4, W: w, H: 28}, "Reset all colours",
		uikit.BtnOpts{Disabled: all != "", Reason: all, Tip: "Reset all colours\nGo back to the theme's colours for everything."},
		func() { s.Edit(func(h *config.HUD) { h.Colors = nil }) })
}

// drawSample paints the colour over a dark backing with a line of text in it.
func drawSample(dst *ebiten.Image, row gfx.Rect, c color.RGBA) {
	back := gfx.Rect{X: row.X, Y: row.Y + 2, W: row.W, H: row.H - 4}
	uikit.FillRect(dst, back, uikit.SliderTrack)
	uikit.FillRect(dst, gfx.Rect{X: back.X, Y: back.Y, W: back.W / 3, H: back.H}, shown(c))
	uikit.StrokeRect(dst, back, uikit.PanelBorder)
	st := gfx.WithColor(gfx.Body, shown(c))
	gfx.Text(dst, "Sample text", float64(back.X+back.W/3+14), float64(back.Y)+(float64(back.H)-gfx.LineH(st))/2, st)
}

// barStyles previews the four bar colour styles with the bar colours now.
func (s *Setup) barStyles(f *uikit.Form, h config.HUD) {
	f.Gap(6)
	f.SectionTitle(s, "Bar styles")
	low, main, back := colourOf(h, "bar_low"), colourOf(h, "bar_main"), colourOf(h, "bar_back")
	for _, st := range sheet.BarStyles() {
		row, ok := f.Row(24)
		if !ok {
			continue
		}
		st := st
		s.k.Queue(func(dst *ebiten.Image) { drawStyleSample(dst, row, st, low, main, back) })
	}
	if row, ok := f.Row(34); ok {
		ds := uikit.DimText()
		ds.Size = gfx.Small.Size
		uikit.TextLines(s.k, row.X, row.Y+2, row.W, "Each bar of a layout picks its style in the Sections tab. These bars are at 15, 40, 70 and 100 percent.", ds)
	}
}

func drawStyleSample(dst *ebiten.Image, row gfx.Rect, style string, low, main, back color.RGBA) {
	st := gfx.WithColor(gfx.Body, uikit.Text)
	gfx.Text(dst, uikit.FitText(sheet.BarStyleLabel(style), st, row.W-4*46-8), float64(row.X), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
	for i, frac := range []float64{0.15, 0.4, 0.7, 1} {
		bar := gfx.Rect{X: row.Right() - float32(4-i)*46 + 4, Y: row.Y + 7, W: 40, H: 10}
		uikit.FillRect(dst, bar, shown(back))
		uikit.FillRect(dst, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(frac), H: bar.H}, shown(sheet.BarColor(style, frac, low, main)))
	}
}
