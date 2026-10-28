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
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The pawn inspector's Health tab (MAIN_UI_SCREENS §14, ENG-003 M1): the
// capacities with their percent, pain, the hurt, missing, scarred and
// artificial body parts, drug lines, and the operations queue with an
// "Add operation" picker. It reads only Simulation.Health and sends
// command.AddOperation / command.RemoveOperation; the simulation checks
// every operation again before it queues or performs it.
//
// MED-03: a care row heads the tab: five care levels (command
// SetMedicalCare) and a "Self-tend" checkbox (SetSelfTend, colonists
// only); each queued operation has up and down arrows (ReorderOperation).
//
// MED-06: each queued operation shows its success chance ("72 %") with
// the factor breakdown on hover; the picker also offers organ harvest and
// transplant, euthanasia and anaesthesia (a carcinoma excision once the
// condition exists). Prisoners and guests get the same tab.

const (
	healthRowH      = 20
	healthOpRowH    = 24
	healthPartsID   = "health.parts"
	healthStateID   = "health.state" // MED-01: the headline line
	healthBloodID   = "health.blood" // MED-01: the blood loss row
	healthBloodBarH = 3
	healthCondBarH  = 3  // MED-02: a condition's severity and immunity bars
	healthCondRowH  = 34 // one condition row: its line and two bars
	healthAddID     = "health.op.add"
	healthPickID    = "health.addop"
	healthPickList  = "health.addop.list"
	healthNoInjury  = "No injuries."
	healthNoOps     = "No operations queued."
	healthPickW     = 420
	healthPickH     = 380
	healthOpsShare  = 0.32 // of the tab's height, for the operations block
	healthCapsShare = 0.44 // of the tab's width, for the capacities column
	healthCareH     = 24   // MED-03: the care row
	healthSelfID    = "health.selftend"
	healthSelfW     = 96
	healthArrowW    = 22
)

// healthCareID is the care button of level p (MED-03).
func healthCareID(p medcare.Policy) string { return fmt.Sprintf("health.care.%d", p) }

// healthUpID and healthDownID are queued operation slot i's reorder
// arrows (MED-03).
func healthUpID(i int) string   { return fmt.Sprintf("health.op.up.%d", i) }
func healthDownID(i int) string { return fmt.Sprintf("health.op.down.%d", i) }

// careShort is a care level's button label (the full label is its tip).
func careShort(p medcare.Policy) string {
	switch p {
	case medcare.NoCare:
		return "None"
	case medcare.NoMeds:
		return "No meds"
	case medcare.HerbalMax:
		return "Herbal"
	case medcare.IndustrialMax:
		return "Industrial"
	}
	return "Best"
}

// careIcon is a care level's glyph (ui_icon/care_*, ART-026).
func careIcon(p medcare.Policy) string {
	switch p {
	case medcare.NoCare:
		return "ui_icon/care_none"
	case medcare.NoMeds:
		return "ui_icon/care_nomeds"
	case medcare.HerbalMax:
		return "ui_icon/care_herbal"
	case medcare.IndustrialMax:
		return "ui_icon/care_industrial"
	}
	return "ui_icon/care_best"
}

// careTip is a care level's hover card.
func careTip(p medcare.Policy) string {
	var body string
	switch p {
	case medcare.NoCare:
		body = "No doctor tends this patient. A downed patient is still carried to bed."
	case medcare.NoMeds:
		body = "Tended, but never with medicine."
	case medcare.HerbalMax:
		body = "Tended with herbal medicine at most."
	case medcare.IndustrialMax:
		body = "Tended with industrial or herbal medicine."
	default:
		body = "Tended with the best medicine there is."
	}
	return p.Label() + "\n" + body
}

// healthRemoveID is the cancel button of queued operation slot i.
func healthRemoveID(i int) string { return fmt.Sprintf("health.op.rm.%d", i) }

// healthChanceID is queued operation slot i's success chance (MED-06).
func healthChanceID(i int) string { return fmt.Sprintf("health.op.chance.%d", i) }

// healthChanceW is the width of a queued operation's chance column.
const healthChanceW = 44

// healthChanceText is a queued operation's chance cell ("72 %").
func healthChanceText(op game.OperationView) string { return fmt.Sprintf("%d %%", op.ChancePct) }

// healthChanceColor tints the chance: red under 50 %, amber under 80 %.
func healthChanceColor(pct int) color.RGBA {
	switch {
	case pct < 50:
		return uiDanger
	case pct < 80:
		return uiWarning
	}
	return uiText
}

// healthCapID is capacity row i (the breakdown tooltip).
func healthCapID(i int) string { return fmt.Sprintf("health.cap.%d", i) }

// healthCondID is condition row i (MED-02).
func healthCondID(i int) string { return fmt.Sprintf("health.cond.%d", i) }

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
	a.healthCare(p, rect{in.X, top, in.W, healthCareH}, h) // MED-03
	top += healthCareH + 6
	body := rect{in.X, top, in.W, in.bottom() - top}
	upper, ops := body.splitV(body.H * (1 - healthOpsShare))
	left, right := upper.splitH(upper.W * healthCapsShare)
	a.healthCaps(left.insetXY(0, 0), h)
	partsR := rect{right.X + 8, right.Y, right.W - 8, right.H - 6}
	if n := len(h.Conditions); n > 0 { // MED-02: the conditions sit under the part rows
		condH := min(float32(lineStep(tsLabel))+2+float32(n)*healthCondRowH, partsR.H/2)
		partsR.H -= condH
		a.healthConds(rect{partsR.X, partsR.bottom(), partsR.W, condH}, h.Conditions)
	}
	a.healthParts(partsR, h.Parts)
	a.healthOps(p, ops, h)
}

// healthCare is the care row (MED-03): "Care", a button per care level
// (the current one selected) and, for a colonist, the Self-tend
// checkbox. The dead get it disabled with the reason.
func (a *App) healthCare(p game.PawnView, r rect, h game.HealthView) {
	dead := p.Status == "dead"
	reason := ""
	if dead {
		reason = "The dead need no care."
	}
	st := tsLabel
	st.Color = uiTextDim
	a.uiLabel(r.X, r.Y+(r.H-float32(lineH(st)))/2, "Care", st)
	x := r.X + 40
	w := r.right() - x
	if !p.Prisoner && !p.Visitor {
		w -= healthSelfW + 6
	}
	bw := max(w/float32(medcare.NumPolicies), 24)
	u := a.uiSt()
	for c := medcare.Policy(0); c < medcare.NumPolicies; c++ {
		cur := medcare.Policy(h.Care) == c
		if a.uiButton(healthCareID(c), rect{x, r.Y, bw - 2, r.H}, careShort(c),
			btnOpts{Selected: cur, Tip: careTip(c), Disabled: dead, Reason: reason}) && !cur {
			a.shell.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(p.ID), Mode: uint8(c)})
		}
		if a.renderer != nil && a.renderer.art != nil && bw >= 2*r.H {
			if img := a.renderer.art.image(careIcon(c), color.NRGBA{}); img != nil {
				ix, iy, is := float64(x+3), float64(r.Y+3), float64(r.H-6)
				u.queue(func(dst *ebiten.Image) { drawIconCS(dst, img, ix, iy, is, noCS) })
			}
		}
		x += bw
	}
	if p.Prisoner || p.Visitor {
		return // self-tend is for colonists
	}
	on := h.SelfTend
	if a.uiCheck(healthSelfID, rect{r.right() - healthSelfW, r.Y, healthSelfW, r.H}, "Self-tend", &on,
		checkOpts{Disabled: dead, Reason: reason,
			Tip: "Self-tend\nWith no one else to help, this colonist tends its own wounds (at lower quality)."}) {
		a.shell.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(p.ID), Enabled: on})
	}
}

// healthArrow is a reorder arrow button: the button, then a triangle
// pointing up or down drawn over it.
func (a *App) healthArrow(id string, r rect, up bool, o btnOpts) bool {
	clicked := a.uiButton(id, r, "", o)
	c := uiHeading
	if o.Disabled {
		c = uiTextDisabled
	}
	cx, cy := r.center()
	a.uiSt().queue(func(dst *ebiten.Image) {
		if up {
			uiFillPoly(dst, [][2]float32{{cx - 5, cy + 3}, {cx + 5, cy + 3}, {cx, cy - 4}}, c, true)
			return
		}
		uiFillPoly(dst, [][2]float32{{cx - 5, cy - 3}, {cx + 5, cy - 3}, {cx, cy + 4}}, c, true)
	})
	return clicked
}

// healthStateColor tints the Health tab's headline: red while bleeding
// or losing to an infection, amber while downed, injured or fighting one.
func healthStateColor(h game.HealthView) color.RGBA {
	switch {
	case h.BleedPerDay > 0, strings.HasSuffix(h.State, ": losing"):
		return uiDanger
	case h.State == "Downed" || h.State == "Injured" || strings.HasSuffix(h.State, ": winning"):
		return uiWarning
	}
	return uiText
}

// healthStateTip is the hover card of the headline.
func healthStateTip(h game.HealthView) string {
	switch {
	case h.BleedPerDay > 0:
		return fmt.Sprintf("%s\nOpen wounds lose %d%% of the blood a day. Untended, the blood runs out in about %d hours. A tend stops the bleeding.",
			h.State, h.BleedPerDay, h.DeathInHours)
	case strings.HasSuffix(h.State, ": losing") && h.DeathInHours >= 0:
		return fmt.Sprintf("%s\nAt this rate it kills in about %d hours, before immunity builds. A better tend slows it.", h.State, h.DeathInHours)
	case strings.HasSuffix(h.State, ": winning"):
		return h.State + "\nImmunity should build before it gets worse. Keep it tended."
	}
	return h.State + "\nNothing is bleeding."
}

// healthCondDue is a condition row's tend marker: "needs tend now",
// "needs tend in Nh", or "" for one that takes no tending.
func healthCondDue(c game.ConditionView) string {
	switch {
	case c.Due:
		return "needs tend now"
	case c.DueInHours > 0:
		return fmt.Sprintf("needs tend in %dh", c.DueInHours)
	}
	return ""
}

// healthConds is the conditions section under the part rows (MED-02):
// each condition's name and tend marker, then its severity bar (danger)
// over its immunity bar (good), with the tooltip from the view.
func (a *App) healthConds(r rect, conds []game.ConditionView) {
	a.uiLabel(r.X, r.Y, "Conditions", tsLabel)
	y := r.Y + float32(lineStep(tsLabel)) + 2
	step := float32(lineStep(tsBody))
	u := a.uiSt()
	for i, c := range conds {
		if y+healthCondRowH > r.bottom()+1 {
			break
		}
		st := tsBody
		st.Color = uiWarning
		if c.Losing {
			st.Color = uiDanger
		}
		due := healthCondDue(c)
		a.uiLabel(r.X, y, fitText(c.Label, st, r.W-120), st)
		if due != "" {
			ds := tsSmall
			ds.Color, ds.Align = uiTextDim, alignRight
			if c.Due {
				ds.Color = uiDanger
			}
			a.uiLabel(r.right()-4, y+2, due, ds)
		}
		sev := rect{r.X, y + step, r.W - 4, healthCondBarH}
		imm := rect{r.X, sev.bottom() + 2, r.W - 4, healthCondBarH}
		sevW := sev.W * float32(min(max(c.SevPct, 0), 100)) / 100
		immW := imm.W * float32(min(max(c.ImmPct, 0), 100)) / 100
		u.queue(func(dst *ebiten.Image) {
			fillRect(dst, sev, uiSliderTrack)
			fillRect(dst, imm, uiSliderTrack)
			if sevW > 0 {
				fillRect(dst, rect{sev.X, sev.Y, sevW, sev.H}, uiDanger)
			}
			if immW > 0 {
				fillRect(dst, rect{imm.X, imm.Y, immW, imm.H}, uiGood)
			}
		})
		label := fmt.Sprintf("%s: severity %d%%, immunity %d%%", c.Label, c.SevPct, c.ImmPct)
		if due != "" {
			label += ", " + due
		}
		u.add(uiWidget{id: healthCondID(i), r: rect{r.X, y, r.W, healthCondRowH - 2}, label: label, tip: c.Tip})
		y += healthCondRowH
	}
}

// healthBloodTip is the hover card of the blood loss row.
func healthBloodTip(h game.HealthView) string {
	t := fmt.Sprintf("Blood loss %d%%\nAt 15%% it starts to cloud the mind; at 60%% the pawn collapses; at 100%% it dies.", h.BloodLossPct)
	if h.BleedPerDay == 0 {
		t += " The body makes about a third of its blood back a day while nothing bleeds."
	}
	return t
}

// healthCaps is the capacities column: the state headline and blood loss
// (MED-01), pain, each capacity, then drugs.
func (a *App) healthCaps(r rect, h game.HealthView) {
	y := r.Y
	step := float32(lineStep(tsBody))
	u := a.uiSt()
	if h.State != "" && y+step <= r.bottom() {
		st := tsBody
		st.Color = healthStateColor(h)
		a.uiLabel(r.X, y, fitText(h.State, st, r.W-4), st)
		u.add(uiWidget{id: healthStateID, r: rect{r.X, y, r.W, step}, label: h.State, tip: healthStateTip(h)})
		y += step + 2
	}
	if (h.BloodLossPct > 0 || h.BleedPerDay > 0) && y+step+healthBloodBarH+2 <= r.bottom() {
		c := uiWarning
		if h.BleedPerDay > 0 || h.BloodLossPct >= 60 {
			c = uiDanger
		}
		label := fmt.Sprintf("Blood loss: %d%%", h.BloodLossPct)
		a.uiLabel(r.X, y, fitText("Blood loss", tsBody, r.W-48), tsBody)
		st := tsBody
		st.Color, st.Align = c, alignRight
		a.uiLabel(r.right()-4, y, fmt.Sprintf("%d%%", h.BloodLossPct), st)
		bar := rect{r.X, y + step, r.W - 4, healthBloodBarH}
		fill := bar
		fill.W = bar.W * float32(min(max(h.BloodLossPct, 0), 100)) / 100
		u.queue(func(dst *ebiten.Image) {
			fillRect(dst, bar, uiSliderTrack)
			if fill.W > 0 {
				fillRect(dst, fill, c)
			}
		})
		u.add(uiWidget{id: healthBloodID, r: rect{r.X, y, r.W, step + healthBloodBarH}, label: label, tip: healthBloodTip(h)})
		y += step + healthBloodBarH + 4
	}
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
	for i, c := range h.Capacities {
		y0 := y
		line(c.Label, c.Pct, healthCapColor(c.Pct))
		if y > y0 && c.Tip != "" {
			u.add(uiWidget{id: healthCapID(i), r: rect{r.X, y0, r.W, step}, label: fmt.Sprintf("%s: %d%%", c.Label, c.Pct), tip: c.Tip})
		}
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
		if pt.Bleed > 0 { // MED-01
			tip += fmt.Sprintf("\nBleeding: %d%%/day", pt.Bleed)
		}
		if pt.TendTip != "" { // MED-02: each wound's tend quality
			tip += "\n" + pt.TendTip
		}
		u.add(uiWidget{id: fmt.Sprintf("health.part.%d", rows[v.i]), r: hit, label: healthPartLine(pt), tip: tip})
	}
}

// paintPartRow draws one body part row: the part, then its state (red for
// a missing or bleeding part, amber for wounds, dim for scars and artificial parts).
func paintPartRow(dst *ebiten.Image, row rect, p game.PartView, stripe bool) {
	if stripe {
		fillRect(dst, row, uiStripe)
	}
	const labelW = 96
	ty := float64(row.Y) + (float64(row.H)-lineH(tsBody))/2
	drawText(dst, fitText(p.Label, tsBody, labelW-6), float64(row.X+4), ty, tsBody)
	st := tsSmall
	switch {
	case p.Bleed > 0, strings.HasPrefix(p.Status, game.PartMissing):
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
	case len(pickerOptions(h)) == 0:
		reason = "No operation fits this body."
	}
	if a.uiButton(healthAddID, addR, "Add operation", btnOpts{Disabled: reason != "", Reason: reason,
		Tip: "Add operation\nQueue surgery for a doctor: fit an implant or remove one, amputate, harvest or transplant an organ, anesthetize or euthanize. Each uses medicine."}) {
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
		textW := r.W - 96 - 2*healthArrowW - healthChanceW
		if op.Status != "" {
			a.uiLabel(r.X, y, fitText(op.Label, tsBody, textW), tsBody)
			st := tsSmall
			st.Color = opStatusColor(op)
			a.uiLabel(r.X, y+float32(lineStep(tsBody))-2, fitText(op.Status, st, textW), st)
		} else {
			a.uiLabel(r.X, y+4, fitText(op.Label, tsBody, textW), tsBody)
		}
		// MED-03: move it one place up or down the queue
		upR := rect{r.right() - 84 - 2*healthArrowW, y, healthArrowW, healthOpRowH - 2}
		// MED-06: the success chance, its factors on hover
		if op.ChanceTip != "" {
			cr := rect{upR.X - healthChanceW - 2, y, healthChanceW, healthOpRowH - 2}
			cs := tsBody
			cs.Color, cs.Align = healthChanceColor(op.ChancePct), alignRight
			a.uiLabel(cr.right()-2, y+4, healthChanceText(op), cs)
			a.uiSt().add(uiWidget{id: healthChanceID(op.Slot), r: cr, label: healthChanceText(op), tip: op.ChanceTip})
		}
		downR := rect{upR.right() + 2, y, healthArrowW, healthOpRowH - 2}
		upWhy, downWhy := "", ""
		switch {
		case dead:
			upWhy, downWhy = reason, reason
		case len(h.Operations) == 1:
			upWhy, downWhy = "The only operation queued.", "The only operation queued."
		case op.Slot == 0:
			upWhy = "Already first."
		case op.Slot == len(h.Operations)-1:
			downWhy = "Already last."
		}
		if a.healthArrow(healthUpID(op.Slot), upR, true, btnOpts{Disabled: upWhy != "", Reason: upWhy,
			Tip: "Move up\nDo " + op.Label + " sooner."}) {
			a.shell.Enqueue(command.Command{Kind: command.ReorderOperation, Pawn: uint64(p.ID), Index: op.Slot, Count: -1})
		}
		if a.healthArrow(healthDownID(op.Slot), downR, false, btnOpts{Disabled: downWhy != "", Reason: downWhy,
			Tip: "Move down\nDo " + op.Label + " later."}) {
			a.shell.Enqueue(command.Command{Kind: command.ReorderOperation, Pawn: uint64(p.ID), Index: op.Slot, Count: 1})
		}
		if a.uiButton(healthRemoveID(op.Slot), rect{r.right() - 80, y, 80, healthOpRowH - 2}, "Cancel",
			btnOpts{Style: btnDanger, Tip: "Cancel\nTake " + op.Label + " off the queue."}) {
			a.shell.Enqueue(command.Command{Kind: command.RemoveOperation, Pawn: uint64(p.ID), Index: op.Slot})
		}
		y += healthOpRowH
	}
}

// opStatusColor is a queued operation's status colour: a warning when the
// wait needs the player (stock, research, reach, a second doctor), dim
// while it is only a matter of time.
func opStatusColor(op game.OperationView) color.Color {
	if op.Blocked {
		return uiWarning
	}
	return uiTextDim
}

// healthOption is one operation the picker offers.
type healthOption struct {
	label string
	op    body.OpKind
	part  body.Part
	item  item.Kind
	// reason is why it can't happen yet ("" when it can); it can still be queued.
	reason string
}

// pickerOptions are the operations the picker lists: the view's own
// Options (with their reasons), else the local derivation for views
// that carry none.
func pickerOptions(h game.HealthView) []healthOption {
	if len(h.Options) == 0 {
		return healthOptions(h)
	}
	out := make([]healthOption, 0, len(h.Options))
	for _, o := range h.Options {
		r := ""
		if o.Disabled {
			r = o.Reason
			if r == "" {
				r = "Not possible yet."
			}
		}
		out = append(out, healthOption{label: o.Label, op: body.OpKind(o.Kind), part: body.Part(o.Part), item: item.Kind(o.Item), reason: r})
	}
	return out
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
					add(healthOption{label: fmt.Sprintf("Install %s (%s)", k, pv.Label), op: body.InstallImplant, part: part, item: k})
				}
			}
		case strings.HasPrefix(pv.Status, game.PartArtificial):
			add(healthOption{label: fmt.Sprintf("Remove implant (%s)", pv.Label), op: body.RemoveImplant, part: part})
		case pv.Status == game.PartReplaced:
		case !body.DefOf(part).Vital && !body.Inner(part):
			add(healthOption{label: "Amputate " + pv.Label, op: body.Amputate, part: part})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].op < out[j].op })
	return out
}

// healthOptionTip is a picker row's hover card: what the operation does
// (MED-06), then why it can't happen yet or what it needs.
func healthOptionTip(o healthOption) string {
	t := o.label
	switch o.op {
	case body.HarvestOrgan:
		t += "\nTakes the organ out whole for storage or transplant. Taking a heart, the liver or the last kidney or lung kills. The patient and the colony think badly of it; a guest's or prisoner's faction loses goodwill."
	case body.InstallOrgan:
		t += "\nTransplants a natural organ into the missing or damaged one."
	case body.Euthanize:
		t += "\nPuts the patient down painlessly. It cannot fail."
	case body.Anesthetize:
		t += "\nPuts the patient under for six hours, woozy for six more."
	case body.ExciseCarcinoma:
		t += "\nCuts the carcinoma out. Needs Medicine 10 and industrial medicine; a failure can kill."
	}
	switch {
	case o.reason != "":
		return t + "\n" + o.reason + "\nYou can queue it anyway; it waits until it can happen."
	case o.item != 0:
		return t + "\nNeeds a " + o.item.String() + " in stock, medicine and a doctor."
	}
	return t + "\nNeeds medicine and a doctor; the risk falls with Medicine skill, a clean, lit room, a hospital bed and a vitals monitor."
}

// openHealthPicker opens the Add operation window for patient id.
func (a *App) openHealthPicker(id entity.EntityID) {
	sel := -1
	options := func() []healthOption {
		h, ok := a.renderer.game.Health(id)
		if !ok {
			return nil
		}
		return pickerOptions(h)
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
				RowH:  36,
				Label: func(i int) string { return opts[i].label },
				Draw: func(dst *ebiten.Image, i int, row rect, hover, selected bool) {
					a.paintOptionRow(dst, row, opts[i], hover, selected)
				},
				Tip: func(i int) string { return healthOptionTip(opts[i]) },
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
					a.shell.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(id),
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
	if o.reason != "" {
		st.Color = uiTextDim
	}
	if selected {
		st.Color = uiAccent
	}
	if o.reason == "" {
		drawText(dst, fitText(o.label, st, row.right()-textX-4), float64(textX), float64(row.Y)+(float64(row.H)-lineH(st))/2, st)
		return
	}
	sm := tsSmall
	sm.Color = uiWarning
	drawText(dst, fitText(o.label, st, row.right()-textX-4), float64(textX), float64(row.Y)+1, st)
	drawText(dst, fitText(o.reason, sm, row.right()-textX-4), float64(textX), float64(row.Y)+lineH(st)+1, sm)
}
