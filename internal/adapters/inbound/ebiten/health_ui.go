package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The pawn inspector's Health tab (MAIN_UI_SCREENS §14, ENG-003 M1): the
// capacities with their percent, pain, the hurt, missing, scarred and
// artificial body parts, drug lines, and the operations queue with an
// "Add operation" picker. It reads only Simulation.Health and sends
// command.AddOperation / command.RemoveOperation; the simulation checks
// every operation again before it queues or performs it.

const (
	healthRowH      = 20
	healthOpRowH    = 24
	healthPartsID   = "health.parts"
	healthAddID     = "health.op.add"
	healthPickID    = "health.addop"
	healthPickList  = "health.addop.list"
	healthNoInjury  = "No injuries."
	healthNoOps     = "No operations queued."
	healthPickW     = 420
	healthPickH     = 380
	healthOpsShare  = 0.32 // of the tab's height, for the operations block
	healthCapsShare = 0.44 // of the tab's width, for the capacities column
)

// healthRemoveID is the cancel button of queued operation slot i.
func healthRemoveID(i int) string { return fmt.Sprintf("health.op.rm.%d", i) }

// healthCapColor tints a capacity: red when crippled, amber when impaired.
func healthCapColor(pct int) color.RGBA {
	switch {
	case pct < 50:
		return uiDanger
	case pct < 100:
		return uiWarning
	}
	return uiText
}

// hurtParts are the rows worth listing: every part that is not healthy.
func hurtParts(ps []game.PartView) []int {
	var out []int
	for i, p := range ps {
		if p.Status != game.PartHealthy {
			out = append(out, i)
		}
	}
	return out
}

// healthTab declares and paints the Health tab of colonist p in the
// inspector's content rectangle in, below the colonist's name.
func (a *App) healthTab(p game.PawnView, in rect) {
	h, ok := a.renderer.game.Health(p.ID)
	top := in.Y + float32(lineStep(tsLabel)) + 8
	if !ok {
		a.uiLabel(in.X, top, noDataYet, dimText())
		return
	}
	body := rect{in.X, top, in.W, in.bottom() - top}
	upper, ops := body.splitV(body.H * (1 - healthOpsShare))
	left, right := upper.splitH(upper.W * healthCapsShare)
	a.healthCaps(left.insetXY(0, 0), h)
	a.healthParts(rect{right.X + 8, right.Y, right.W - 8, right.H - 6}, h.Parts)
	a.healthOps(p, ops, h)
}

// healthCaps is the capacities column: pain, each capacity, then drugs.
func (a *App) healthCaps(r rect, h game.HealthView) {
	y := r.Y
	step := float32(lineStep(tsBody))
	line := func(label string, pct int, c color.RGBA) {
		if y+step > r.bottom() {
			return
		}
		a.uiLabel(r.X, y, fitText(label, tsBody, r.W-48), tsBody)
		st := tsBody
		st.Color, st.Align = c, alignRight
		a.uiLabel(r.right()-4, y, fmt.Sprintf("%d%%", pct), st)
		y += step
	}
	painC := uiText
	if h.Pain >= 40 {
		painC = uiWarning
	}
	line("Pain", h.Pain, painC)
	for _, c := range h.Capacities {
		line(c.Label, c.Pct, healthCapColor(c.Pct))
	}
	if len(h.Drugs) > 0 && y+2*step <= r.bottom() {
		y += 4
		a.uiLabel(r.X, y, "Drugs", tsLabel)
		y += float32(lineStep(tsLabel))
		for _, d := range h.Drugs {
			line(d.Label, d.Pct, uiText)
		}
	}
}

// healthPartLine is a part row's plain text (its accessible label).
func healthPartLine(p game.PartView) string {
	return p.Label + ": " + p.Status
}

// healthParts is the scrollable list of hurt, missing and artificial parts.
func (a *App) healthParts(r rect, parts []game.PartView) {
	a.uiLabel(r.X, r.Y, "Body", tsLabel)
	r.Y += float32(lineStep(tsLabel)) + 2
	r.H -= float32(lineStep(tsLabel)) + 2
	rows := hurtParts(parts)
	if len(rows) == 0 {
		a.uiLabel(r.X, r.Y, healthNoInjury, dimText())
		return
	}
	u := a.uiSt()
	area := a.uiScrollArea(healthPartsID, r, float32(len(rows))*healthRowH, scrollOpts{Step: healthRowH * 2})
	type vis struct {
		i int
		y float32
	}
	var shown []vis
	for i := range rows {
		y := area.View.Y + float32(i)*healthRowH - area.Off
		if y+healthRowH < area.View.Y || y > area.View.bottom() {
			continue
		}
		shown = append(shown, vis{i, y})
	}
	u.queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for _, v := range shown {
			paintPartRow(sub, rect{area.View.X, v.y, area.View.W, healthRowH}, parts[rows[v.i]], v.i%2 == 1)
		}
	})
	for _, v := range shown {
		pt := parts[rows[v.i]]
		hit := rect{area.View.X, v.y, area.View.W, healthRowH}.intersect(area.View)
		tip := pt.Label + "\n" + pt.Status
		if !strings.HasPrefix(pt.Status, game.PartMissing) && !strings.HasPrefix(pt.Status, game.PartReplaced) {
			tip += fmt.Sprintf("\nHealth left: %d", pt.HP)
		}
		u.add(uiWidget{id: fmt.Sprintf("health.part.%d", rows[v.i]), r: hit, label: healthPartLine(pt), tip: tip})
	}
}

// paintPartRow draws one body part row: the part, then its state (red for
// a missing part, amber for wounds, dim for scars and artificial parts).
func paintPartRow(dst *ebiten.Image, row rect, p game.PartView, stripe bool) {
	if stripe {
		fillRect(dst, row, uiStripe)
	}
	const labelW = 96
	ty := float64(row.Y) + (float64(row.H)-lineH(tsBody))/2
	drawText(dst, fitText(p.Label, tsBody, labelW-6), float64(row.X+4), ty, tsBody)
	st := tsSmall
	switch {
	case strings.HasPrefix(p.Status, game.PartMissing):
		st.Color = uiDanger
	case strings.HasPrefix(p.Status, game.PartWounded):
		st.Color = uiWarning
	default:
		st.Color = uiTextDim
	}
	sy := float64(row.Y) + (float64(row.H)-lineH(st))/2
	drawText(dst, fitText(p.Status, st, row.W-labelW-4), float64(row.X+labelW), sy, st)
}

// healthOps is the operations block: the queue with a cancel button per
// row and the Add operation button.
func (a *App) healthOps(p game.PawnView, r rect, h game.HealthView) {
	dead := p.Status == "dead"
	a.uiLabel(r.X, r.Y+4, "Operations", tsLabel)
	addR := rect{r.right() - 132, r.Y, 132, 24}
	reason := ""
	switch {
	case dead:
		reason = "The dead need no surgery."
	case len(healthOptions(h)) == 0:
		reason = "No operation fits this body."
	}
	if a.uiButton(healthAddID, addR, "Add operation", btnOpts{Disabled: reason != "", Reason: reason,
		Tip: "Add operation\nQueue surgery for a doctor: fit an implant, remove one, or amputate."}) {
		a.openHealthPicker(p.ID)
	}
	y := r.Y + 30
	if len(h.Operations) == 0 {
		a.uiLabel(r.X, y, healthNoOps, dimText())
		return
	}
	for _, op := range h.Operations {
		if y+healthOpRowH > r.bottom() {
			break
		}
		a.uiLabel(r.X, y+4, fitText(op.Label, tsBody, r.W-96), tsBody)
		if a.uiButton(healthRemoveID(op.Slot), rect{r.right() - 80, y, 80, healthOpRowH - 2}, "Cancel",
			btnOpts{Style: btnDanger, Tip: "Cancel\nTake " + op.Label + " off the queue."}) {
			a.renderer.game.Enqueue(command.Command{Kind: command.RemoveOperation, Pawn: uint64(p.ID), Index: op.Slot})
		}
		y += healthOpRowH
	}
}

// healthOption is one operation the picker offers.
type healthOption struct {
	label string
	op    body.OpKind
	part  body.Part
	item  item.Kind
}

// healthOptions lists the operations that fit the body as the view shows
// it (Parts[i] is body.Part(i)): an implant for each missing part it fits,
// then removing each implant, then amputating each part still there that
// is not vital. Queued ones are left out.
func healthOptions(h game.HealthView) []healthOption {
	queued := map[string]bool{}
	for _, o := range h.Operations {
		queued[o.Label] = true
	}
	var out []healthOption
	add := func(o healthOption) {
		if !queued[o.label] {
			out = append(out, o)
		}
	}
	for i, pv := range h.Parts {
		if i >= int(body.NumParts) {
			break
		}
		part := body.Part(i)
		switch {
		case pv.Status == game.PartMissing:
			for k := item.Kind(1); k != 0; k++ {
				if k.IsImplant() && body.Fits(k, part) {
					add(healthOption{fmt.Sprintf("Install %s (%s)", k, pv.Label), body.InstallImplant, part, k})
				}
			}
		case strings.HasPrefix(pv.Status, game.PartArtificial):
			add(healthOption{fmt.Sprintf("Remove implant (%s)", pv.Label), body.RemoveImplant, part, 0})
		case pv.Status == game.PartReplaced:
		case !body.DefOf(part).Vital:
			add(healthOption{"Amputate " + pv.Label, body.Amputate, part, 0})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].op < out[j].op })
	return out
}

// openHealthPicker opens the Add operation window for patient id.
func (a *App) openHealthPicker(id entity.EntityID) {
	sel := -1
	options := func() []healthOption {
		h, ok := a.renderer.game.Health(id)
		if !ok {
			return nil
		}
		return healthOptions(h)
	}
	a.uiPushModal(modal{
		id: healthPickID, title: "Add operation", w: healthPickW, h: healthPickH,
		draw: func(a *App, r rect) {
			opts := options()
			if sel >= len(opts) {
				sel = -1
			}
			if len(opts) == 0 {
				a.uiLabel(r.X, r.Y, "No operation fits this body.", dimText())
				return
			}
			a.uiList(healthPickList, r, len(opts), &sel, listSpec{
				RowH:  28,
				Label: func(i int) string { return opts[i].label },
				Draw: func(dst *ebiten.Image, i int, row rect, hover, selected bool) {
					a.paintOptionRow(dst, row, opts[i], hover, selected)
				},
				Tip: func(i int) string {
					if opts[i].item != 0 {
						return opts[i].label + "\nNeeds a " + opts[i].item.String() + " in stock and a doctor."
					}
					return opts[i].label + "\nNeeds a doctor; the risk falls with Medicine skill and a hospital bed."
				},
			})
		},
		buttons: []modalButton{
			{id: "health.addop.ok", label: "Add", style: btnPrimary, closes: true,
				disabled: func() string {
					if sel < 0 || sel >= len(options()) {
						return "Pick an operation first."
					}
					return ""
				},
				onClick: func() {
					opts := options()
					if sel < 0 || sel >= len(opts) {
						return
					}
					o := opts[sel]
					a.renderer.game.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(id),
						Index: int(o.op), Build: uint8(o.part), Stuff: uint8(o.item)})
				}},
			{id: "health.addop.cancel", label: "Cancel", closes: true},
		},
	})
}

// paintOptionRow draws a picker row with the implant's icon when its art
// exists (the label alone otherwise).
func (a *App) paintOptionRow(dst *ebiten.Image, row rect, o healthOption, hover, selected bool) {
	if selected || hover {
		fillRect(dst, row, uiStripe)
	}
	textX := row.X + 6
	if o.item != 0 && a.art != nil {
		if key := itemSprite(o.item.String()); key != "" && a.art.has(key) {
			drawIcon(dst, a.art.image(key, color.NRGBA{}), float64(row.X+4), float64(row.Y+2), float64(row.H-4))
			textX = row.X + row.H + 4
		}
	}
	st := tsBody
	if selected {
		st.Color = uiAccent
	}
	drawText(dst, fitText(o.label, st, row.right()-textX-4), float64(textX), float64(row.Y)+(float64(row.H)-lineH(st))/2, st)
}
