package worldinfo

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/world/planet"
)

// Finder is what the SearchBox needs of the world: the ranked name search.
// *planet.Planet satisfies it.
type Finder interface {
	Find(q string) []planet.Hit
}

// Search layout (px).
const (
	SearchW       = 320
	searchPad     = 10
	searchFieldH  = 30
	searchRowH    = 28
	searchMaxRows = 8
)

// SearchBox is the world-feature search (WG-10): a text field and, below
// it, the ranked results. The caller keeps one per screen and calls Draw
// every frame while it is open.
type SearchBox struct {
	Query string
	hits  []planet.Hit
	of    string // the query hits were made for
	ready bool
}

// Reset clears the query and its results.
func (s *SearchBox) Reset() { *s = SearchBox{} }

// Hits are the current results (at most 50, ranked).
func (s *SearchBox) Hits() []planet.Hit { return s.hits }

// Height is the panel's height for n visible rows.
func (s *SearchBox) Height() float32 {
	rows := min(len(s.hits), searchMaxRows)
	h := float32(2*searchPad + searchFieldH)
	if rows > 0 || s.Query != "" {
		h += 6 + float32(max(rows, 1))*searchRowH
	}
	if len(s.hits) > searchMaxRows {
		h += 20
	}
	return h
}

// Draw declares the panel with its top-right corner at (right, y) and
// returns its rectangle (the caller keeps the globe from taking clicks
// there). Ids: prefix+".field", prefix+".result.<n>"; pick runs with the
// clicked hit. The field takes the keyboard when the panel opens (focus).
func (s *SearchBox) Draw(k *uikit.Kit, prefix string, right, y float32, f Finder, pick func(planet.Hit)) gfx.Rect {
	s.refresh(f)
	r := gfx.Rect{X: right - SearchW, Y: y, W: SearchW, H: s.Height()}
	k.Panel(r)
	in := r.Inset(searchPad)
	field := gfx.Rect{X: in.X, Y: in.Y, W: in.W, H: searchFieldH}
	tl := trOf(k)
	k.TextField(prefix+".field", field, &s.Query, uikit.TextFieldSpec{
		MaxLen: 40, Placeholder: tl.T("Find a region or settlement"),
		Tip: tl.T("Search\nType part of a name: oceans, seas, lakes, mountain ranges, deserts, forests, ice caps and settlements."),
	})
	s.refresh(f)
	y0 := field.Bottom() + 6
	if len(s.hits) == 0 {
		if s.Query != "" {
			k.Label(in.X+4, y0+5, tl.T("Nothing by that name."), uikit.DimText())
		}
		return r
	}
	for n, h := range s.hits {
		if n >= searchMaxRows {
			k.Label(in.X+4, y0+float32(n)*searchRowH+2, lang.Tf(tl, "%d more. Type more letters.", len(s.hits)-searchMaxRows), uikit.DimText())
			break
		}
		h := h
		row := gfx.Rect{X: in.X, Y: y0 + float32(n)*searchRowH, W: in.W, H: searchRowH - 2}
		uikit.StyledButton(k, fmt.Sprintf("%s.result.%d", prefix, n), row, "", uikit.BtnOpts{Tip: lang.Tf(tl, "%s\n%s. Click to look at it on the globe.", h.Name, h.Kind.Label())}, func() { pick(h) })
		name, kind := h.Name, h.Kind.Label()
		k.Queue(func(dst *ebiten.Image) {
			st := gfx.Body
			gfx.Text(dst, uikit.FitText(name, st, row.W-110), float64(row.X+8), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
			ks := uikit.DimText()
			ks.Align = gfx.AlignRight
			gfx.Text(dst, kind, float64(row.Right()-8), float64(row.Y)+(float64(row.H)-gfx.LineH(ks))/2, ks)
		})
	}
	return r
}

// refresh recomputes the hits when the query changed.
func (s *SearchBox) refresh(f Finder) {
	if s.ready && s.of == s.Query {
		return
	}
	s.of, s.ready = s.Query, true
	s.hits = nil
	if f != nil {
		s.hits = f.Find(s.Query)
	}
}
