package devtools

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/body"
)

// The Lose a part picker's modal and list ids.
const (
	LosePickID   = "dev.lose"
	LosePickList = "dev.lose.list"
)

// LoseParts are the part labels the Lose a part picker offers.
func LoseParts() []string {
	out := make([]string, 0, body.NumParts)
	for pt := body.Part(0); pt < body.NumParts; pt++ {
		out = append(out, pt.Label())
	}
	return out
}

// openLosePicker lists the body parts; the pick is lost by pw through d.
func (p *Panel) openLosePicker(pw game.PawnView, d Tools) {
	parts := LoseParts()
	sel := -1
	p.sh.Kit().Push(uikit.Modal{
		ID: LosePickID, Title: "Lose a part", W: 300, H: 420,
		Draw: func(body gfx.Rect) {
			p.sh.Kit().List(LosePickList, body, len(parts), &sel, uikit.ListSpec{RowH: 24, Label: func(i int) string { return parts[i] }})
		},
		Buttons: []uikit.ModalButton{
			{ID: "dev.lose.ok", Label: "Lose it", Style: uikit.BtnDanger, Closes: true,
				Disabled: func() string {
					if sel < 0 {
						return "Pick a part first."
					}
					return ""
				},
				OnClick: func() {
					if sel < 0 || sel >= len(parts) {
						return
					}
					if d.DevLosePart(pw.ID, parts[sel]) {
						p.sh.Toast(pawnLabel(pw) + " lost the " + parts[sel])
					} else {
						p.sh.Toast("Could not remove " + parts[sel])
					}
				}},
			{ID: "dev.lose.cancel", Label: "Cancel", Closes: true},
		},
	})
}
