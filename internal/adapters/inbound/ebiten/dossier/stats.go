package dossier

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/pawn/stat"
)

const statRowH = 22

// statSheet is the stat sheet modal.
type statSheet struct{ p *Panel }

// statRow is one stat of the sheet.
type statRow struct{ label, value string }

// statRows lists every stat the selection has (-1 is off), in stat order.
func (s statSheet) rows() []statRow {
	var out []statRow
	dec := s.p.pf.Settings().HUD.Text.Decimals
	for id := stat.ID(0); id < stat.NumStats; id++ {
		if v := s.p.d.card.Stats[id]; v != -1 {
			out = append(out, statRow{id.Label(), sheet.FormatStat(id, v, dec)})
		}
	}
	return out
}

// openStats opens the stat sheet: every number of the selection, read only.
func (s statSheet) open() {
	name := s.p.d.p.Name
	if s.p.d.animal {
		name = s.p.d.a.Name
		if name == "" {
			name = s.p.d.a.Label
		}
	}
	rows := s.rows()
	perCol := (len(rows) + 1) / 2
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + float32(perCol)*statRowH
	s.p.k.Push(uikit.Modal{
		ID: "dossier.stats", Title: s.p.a.Tf("%s: stats", name), W: 520, H: h,
		Buttons: []uikit.ModalButton{{ID: "dossier.stats.close", Label: s.p.a.T("Close"), Closes: true}},
		Draw: func(b gfx.Rect) {
			s.p.k.Queue(func(dst *ebiten.Image) { s.draw(dst, b, rows, perCol) })
		},
	})
}

func (s statSheet) draw(dst *ebiten.Image, b gfx.Rect, rows []statRow, perCol int) {
	label := gfx.TextStyle{Size: 13, Color: s.p.pal["text"]}
	value := label
	value.Align = gfx.AlignRight
	colW := b.W / 2
	for i, r := range rows {
		x := b.X + float32(i/max(perCol, 1))*colW
		y := b.Y + float32(i%max(perCol, 1))*statRowH
		gfx.Text(dst, r.label, float64(x), float64(y), label)
		gfx.Text(dst, r.value, float64(x+colW-12), float64(y), value)
	}
}
