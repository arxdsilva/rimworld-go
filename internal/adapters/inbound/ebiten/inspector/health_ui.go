package inspector

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	HealthStateID   = "health.state" // MED-01: the headline line
	HealthBloodID   = "health.blood" // MED-01: the blood loss row
	healthBloodBarH = 3
	healthCondBarH  = 3  // MED-02: a condition's severity and immunity bars
	healthCondRowH  = 34 // one condition row: its line and two bars
	HealthAddID     = "health.op.add"
	HealthPickID    = "health.addop"
	HealthPickList  = "health.addop.list"
	healthNoInjury  = "No injuries."
	healthNoOps     = "No operations queued."
	healthPickW     = 420
	healthPickH     = 380
	healthOpsShare  = 0.32 // of the tab's height, for the operations block
	healthCapsShare = 0.44 // of the tab's width, for the capacities column
	healthCareH     = 24   // MED-03: the care row
	HealthSelfID    = "health.selftend"
	healthSelfW     = 96
	healthArrowW    = 22
)

// healthCareID is the care button of level p (MED-03).
func HealthCareID(p medcare.Policy) string { return fmt.Sprintf("health.care.%d", p) }

// healthUpID and healthDownID are queued operation slot i's reorder
// arrows (MED-03).
func HealthUpID(i int) string   { return fmt.Sprintf("health.op.up.%d", i) }
func HealthDownID(i int) string { return fmt.Sprintf("health.op.down.%d", i) }

// careShort is a care level's button label (the full label is its tip).
func CareShort(p medcare.Policy) string {
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
func HealthRemoveID(i int) string { return fmt.Sprintf("health.op.rm.%d", i) }

// healthChanceID is queued operation slot i's success chance (MED-06).
func HealthChanceID(i int) string { return fmt.Sprintf("health.op.chance.%d", i) }

// healthChanceW is the width of a queued operation's chance column.
const healthChanceW = 44

// healthChanceText is a queued operation's chance cell ("72 %").
func healthChanceText(op game.OperationView) string { return fmt.Sprintf("%d %%", op.ChancePct) }

// healthChanceColor tints the chance: red under 50 %, amber under 80 %.
func HealthChanceColor(pct int) color.RGBA {
	switch {
	case pct < 50:
		return uikit.Danger
	case pct < 80:
		return uikit.Warning
	}
	return uikit.Text
}

// healthCapID is capacity row i (the breakdown tooltip).
func HealthCapID(i int) string { return fmt.Sprintf("health.cap.%d", i) }

// healthCondID is condition row i (MED-02).
func HealthCondID(i int) string { return fmt.Sprintf("health.cond.%d", i) }

// healthCapColor tints a capacity: red when crippled, amber when impaired.
func healthCapColor(pct int) color.RGBA {
	switch {
	case pct < 50:
		return uikit.Danger
	case pct < 100:
		return uikit.Warning
	}
	return uikit.Text
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
func (a Tabs) healthTab(p game.PawnView, in gfx.Rect) {
	h, ok := a.sim().Health(p.ID)
	top := in.Y + float32(gfx.LineStep(gfx.Label)) + 8
	if !ok {
		a.kit().Label(in.X, top, noDataYet, uikit.DimText())
		return
	}
	a.healthCare(p, gfx.Rect{X: in.X, Y: top, W: in.W, H: healthCareH}, h) // MED-03
	top += healthCareH + 6
	body := gfx.Rect{X: in.X, Y: top, W: in.W, H: in.Bottom() - top}
	upper, ops := body.SplitV(body.H * (1 - healthOpsShare))
	left, right := upper.SplitH(upper.W * healthCapsShare)
	a.healthCaps(left.InsetXY(0, 0), h)
	partsR := gfx.Rect{X: right.X + 8, Y: right.Y, W: right.W - 8, H: right.H - 6}
	if n := len(h.Conditions); n > 0 { // MED-02: the conditions sit under the part rows
		condH := min(float32(gfx.LineStep(gfx.Label))+2+float32(n)*healthCondRowH, partsR.H/2)
		partsR.H -= condH
		a.healthConds(gfx.Rect{X: partsR.X, Y: partsR.Bottom(), W: partsR.W, H: condH}, h.Conditions)
	}
	a.healthParts(partsR, h.Parts)
	a.healthOps(p, ops, h)
}

// healthCare is the care row (MED-03): "Care", a button per care level
// (the current one selected) and, for a colonist, the Self-tend
// checkbox. The dead get it disabled with the reason.
func (a Tabs) healthCare(p game.PawnView, r gfx.Rect, h game.HealthView) {
	dead := p.Status == "dead"
	reason := ""
	if dead {
		reason = "The dead need no care."
	}
	st := gfx.Label
	st.Color = uikit.TextDim
	a.kit().Label(r.X, r.Y+(r.H-float32(gfx.LineH(st)))/2, "Care", st)
	x := r.X + 40
	w := r.Right() - x
	if !p.Prisoner && !p.Visitor {
		w -= healthSelfW + 6
	}
	bw := max(w/float32(medcare.NumPolicies), 24)
	u := a.kit()
	for c := medcare.Policy(0); c < medcare.NumPolicies; c++ {
		cur := medcare.Policy(h.Care) == c
		if a.kit().Button(HealthCareID(c), gfx.Rect{X: x, Y: r.Y, W: bw - 2, H: r.H}, CareShort(c),
			uikit.BtnOpts{Selected: cur, Tip: careTip(c), Disabled: dead, Reason: reason}) && !cur {
			a.shell.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(p.ID), Mode: uint8(c)})
		}
		if a.sim() != nil && a.env().MapArt != nil && bw >= 2*r.H {
			if img := a.env().MapArt.Image(careIcon(c), color.NRGBA{}); img != nil {
				ix, iy, is := float64(x+3), float64(r.Y+3), float64(r.H-6)
				u.Queue(func(dst *ebiten.Image) { drawIconCS(dst, img, ix, iy, is, sprite.NoCS) })
			}
		}
		x += bw
	}
	if p.Prisoner || p.Visitor {
		return // self-tend is for colonists
	}
	on := h.SelfTend
	if a.kit().Check(HealthSelfID, gfx.Rect{X: r.Right() - healthSelfW, Y: r.Y, W: healthSelfW, H: r.H}, "Self-tend", &on,
		uikit.CheckOpts{Disabled: dead, Reason: reason,
			Tip: "Self-tend\nWith no one else to help, this colonist tends its own wounds (at lower quality)."}) {
		a.shell.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(p.ID), Enabled: on})
	}
}

// healthArrow is a reorder arrow button: the button, then a triangle
// pointing up or down drawn over it.
func (a Tabs) healthArrow(id string, r gfx.Rect, up bool, o uikit.BtnOpts) bool {
	clicked := a.kit().Button(id, r, "", o)
	c := uikit.Heading
	if o.Disabled {
		c = uikit.TextDisabled
	}
	cx, cy := r.Center()
	a.kit().Queue(func(dst *ebiten.Image) {
		if up {
			gfx.FillPoly(dst, [][2]float32{{cx - 5, cy + 3}, {cx + 5, cy + 3}, {cx, cy - 4}}, c, true)
			return
		}
		gfx.FillPoly(dst, [][2]float32{{cx - 5, cy - 3}, {cx + 5, cy - 3}, {cx, cy + 4}}, c, true)
	})
	return clicked
}

// healthStateColor tints the Health tab's headline: red while bleeding
// or losing to an infection, amber while downed, injured or fighting one.
func HealthStateColor(h game.HealthView) color.RGBA {
	switch {
	case h.BleedPerDay > 0, strings.HasSuffix(h.State, ": losing"):
		return uikit.Danger
	case h.State == "Downed" || h.State == "Injured" || strings.HasSuffix(h.State, ": winning"):
		return uikit.Warning
	}
	return uikit.Text
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
func HealthCondDue(c game.ConditionView) string {
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
func (a Tabs) healthConds(r gfx.Rect, conds []game.ConditionView) {
	a.kit().Label(r.X, r.Y, "Conditions", gfx.Label)
	y := r.Y + float32(gfx.LineStep(gfx.Label)) + 2
	step := float32(gfx.LineStep(gfx.Body))
	u := a.kit()
	for i, c := range conds {
		if y+healthCondRowH > r.Bottom()+1 {
			break
		}
		st := gfx.Body
		st.Color = uikit.Warning
		if c.Losing {
			st.Color = uikit.Danger
		}
		due := HealthCondDue(c)
		a.kit().Label(r.X, y, uikit.FitText(c.Label, st, r.W-120), st)
		if due != "" {
			ds := gfx.Small
			ds.Color, ds.Align = uikit.TextDim, gfx.AlignRight
			if c.Due {
				ds.Color = uikit.Danger
			}
			a.kit().Label(r.Right()-4, y+2, due, ds)
		}
		sev := gfx.Rect{X: r.X, Y: y + step, W: r.W - 4, H: healthCondBarH}
		imm := gfx.Rect{X: r.X, Y: sev.Bottom() + 2, W: r.W - 4, H: healthCondBarH}
		sevW := sev.W * float32(min(max(c.SevPct, 0), 100)) / 100
		immW := imm.W * float32(min(max(c.ImmPct, 0), 100)) / 100
		u.Queue(func(dst *ebiten.Image) {
			uikit.FillRect(dst, sev, uikit.SliderTrack)
			uikit.FillRect(dst, imm, uikit.SliderTrack)
			if sevW > 0 {
				uikit.FillRect(dst, gfx.Rect{X: sev.X, Y: sev.Y, W: sevW, H: sev.H}, uikit.Danger)
			}
			if immW > 0 {
				uikit.FillRect(dst, gfx.Rect{X: imm.X, Y: imm.Y, W: immW, H: imm.H}, uikit.Good)
			}
		})
		label := fmt.Sprintf("%s: severity %d%%, immunity %d%%", c.Label, c.SevPct, c.ImmPct)
		if due != "" {
			label += ", " + due
		}
		u.Add(uikit.Widget{ID: HealthCondID(i), R: gfx.Rect{X: r.X, Y: y, W: r.W, H: healthCondRowH - 2}, Label: label, Tip: c.Tip})
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
func (a Tabs) healthCaps(r gfx.Rect, h game.HealthView) {
	y := r.Y
	step := float32(gfx.LineStep(gfx.Body))
	u := a.kit()
	if h.State != "" && y+step <= r.Bottom() {
		st := gfx.Body
		st.Color = HealthStateColor(h)
		a.kit().Label(r.X, y, uikit.FitText(h.State, st, r.W-4), st)
		u.Add(uikit.Widget{ID: HealthStateID, R: gfx.Rect{X: r.X, Y: y, W: r.W, H: step}, Label: h.State, Tip: healthStateTip(h)})
		y += step + 2
	}
	if (h.BloodLossPct > 0 || h.BleedPerDay > 0) && y+step+healthBloodBarH+2 <= r.Bottom() {
		c := uikit.Warning
		if h.BleedPerDay > 0 || h.BloodLossPct >= 60 {
			c = uikit.Danger
		}
		label := fmt.Sprintf("Blood loss: %d%%", h.BloodLossPct)
		a.kit().Label(r.X, y, uikit.FitText("Blood loss", gfx.Body, r.W-48), gfx.Body)
		st := gfx.Body
		st.Color, st.Align = c, gfx.AlignRight
		a.kit().Label(r.Right()-4, y, fmt.Sprintf("%d%%", h.BloodLossPct), st)
		bar := gfx.Rect{X: r.X, Y: y + step, W: r.W - 4, H: healthBloodBarH}
		fill := bar
		fill.W = bar.W * float32(min(max(h.BloodLossPct, 0), 100)) / 100
		u.Queue(func(dst *ebiten.Image) {
			uikit.FillRect(dst, bar, uikit.SliderTrack)
			if fill.W > 0 {
				uikit.FillRect(dst, fill, c)
			}
		})
		u.Add(uikit.Widget{ID: HealthBloodID, R: gfx.Rect{X: r.X, Y: y, W: r.W, H: step + healthBloodBarH}, Label: label, Tip: healthBloodTip(h)})
		y += step + healthBloodBarH + 4
	}
	line := func(label string, pct int, c color.RGBA) {
		if y+step > r.Bottom() {
			return
		}
		a.kit().Label(r.X, y, uikit.FitText(label, gfx.Body, r.W-48), gfx.Body)
		st := gfx.Body
		st.Color, st.Align = c, gfx.AlignRight
		a.kit().Label(r.Right()-4, y, fmt.Sprintf("%d%%", pct), st)
		y += step
	}
	painC := uikit.Text
	if h.Pain >= 40 {
		painC = uikit.Warning
	}
	line("Pain", h.Pain, painC)
	for i, c := range h.Capacities {
		y0 := y
		line(c.Label, c.Pct, healthCapColor(c.Pct))
		if y > y0 && c.Tip != "" {
			u.Add(uikit.Widget{ID: HealthCapID(i), R: gfx.Rect{X: r.X, Y: y0, W: r.W, H: step}, Label: fmt.Sprintf("%s: %d%%", c.Label, c.Pct), Tip: c.Tip})
		}
	}
	if len(h.Drugs) > 0 && y+2*step <= r.Bottom() {
		y += 4
		a.kit().Label(r.X, y, "Drugs", gfx.Label)
		y += float32(gfx.LineStep(gfx.Label))
		for _, d := range h.Drugs {
			line(d.Label, d.Pct, uikit.Text)
		}
	}
}

// healthPartLine is a part row's plain text (its accessible label).
func healthPartLine(p game.PartView) string {
	return p.Label + ": " + p.Status
}

// healthParts is the scrollable list of hurt, missing and artificial parts.
func (a Tabs) healthParts(r gfx.Rect, parts []game.PartView) {
	a.kit().Label(r.X, r.Y, "Body", gfx.Label)
	r.Y += float32(gfx.LineStep(gfx.Label)) + 2
	r.H -= float32(gfx.LineStep(gfx.Label)) + 2
	rows := hurtParts(parts)
	if len(rows) == 0 {
		a.kit().Label(r.X, r.Y, healthNoInjury, uikit.DimText())
		return
	}
	u := a.kit()
	area := a.kit().ScrollArea(healthPartsID, r, float32(len(rows))*healthRowH, uikit.ScrollOpts{Step: healthRowH * 2})
	type vis struct {
		i int
		y float32
	}
	var shown []vis
	for i := range rows {
		y := area.View.Y + float32(i)*healthRowH - area.Off
		if y+healthRowH < area.View.Y || y > area.View.Bottom() {
			continue
		}
		shown = append(shown, vis{i, y})
	}
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for _, v := range shown {
			paintPartRow(sub, gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: healthRowH}, parts[rows[v.i]], v.i%2 == 1)
		}
	})
	for _, v := range shown {
		pt := parts[rows[v.i]]
		hit := gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: healthRowH}.Intersect(area.View)
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
		u.Add(uikit.Widget{ID: fmt.Sprintf("health.part.%d", rows[v.i]), R: hit, Label: healthPartLine(pt), Tip: tip})
	}
}

// paintPartRow draws one body part row: the part, then its state (red for
// a missing or bleeding part, amber for wounds, dim for scars and artificial parts).
func paintPartRow(dst *ebiten.Image, row gfx.Rect, p game.PartView, stripe bool) {
	if stripe {
		uikit.FillRect(dst, row, uiStripe)
	}
	const labelW = 96
	ty := float64(row.Y) + (float64(row.H)-gfx.LineH(gfx.Body))/2
	gfx.Text(dst, uikit.FitText(p.Label, gfx.Body, labelW-6), float64(row.X+4), ty, gfx.Body)
	st := gfx.Small
	switch {
	case p.Bleed > 0, strings.HasPrefix(p.Status, game.PartMissing):
		st.Color = uikit.Danger
	case strings.HasPrefix(p.Status, game.PartWounded):
		st.Color = uikit.Warning
	default:
		st.Color = uikit.TextDim
	}
	sy := float64(row.Y) + (float64(row.H)-gfx.LineH(st))/2
	gfx.Text(dst, uikit.FitText(p.Status, st, row.W-labelW-4), float64(row.X+labelW), sy, st)
}

// healthOps is the operations block: the queue with a cancel button per
// row and the Add operation button.
func (a Tabs) healthOps(p game.PawnView, r gfx.Rect, h game.HealthView) {
	dead := p.Status == "dead"
	a.kit().Label(r.X, r.Y+4, "Operations", gfx.Label)
	addR := gfx.Rect{X: r.Right() - 132, Y: r.Y, W: 132, H: 24}
	reason := ""
	switch {
	case dead:
		reason = "The dead need no surgery."
	case len(PickerOptions(h)) == 0:
		reason = "No operation fits this body."
	}
	if a.kit().Button(HealthAddID, addR, "Add operation", uikit.BtnOpts{Disabled: reason != "", Reason: reason,
		Tip: "Add operation\nQueue surgery for a doctor: fit an implant or remove one, amputate, harvest or transplant an organ, anesthetize or euthanize. Each uses medicine."}) {
		a.openHealthPicker(p.ID)
	}
	y := r.Y + 30
	if len(h.Operations) == 0 {
		a.kit().Label(r.X, y, healthNoOps, uikit.DimText())
		return
	}
	for _, op := range h.Operations {
		if y+healthOpRowH > r.Bottom() {
			break
		}
		textW := r.W - 96 - 2*healthArrowW - healthChanceW
		if op.Status != "" {
			a.kit().Label(r.X, y, uikit.FitText(op.Label, gfx.Body, textW), gfx.Body)
			st := gfx.Small
			st.Color = OpStatusColor(op)
			a.kit().Label(r.X, y+float32(gfx.LineStep(gfx.Body))-2, uikit.FitText(op.Status, st, textW), st)
		} else {
			a.kit().Label(r.X, y+4, uikit.FitText(op.Label, gfx.Body, textW), gfx.Body)
		}
		// MED-03: move it one place up or down the queue
		upR := gfx.Rect{X: r.Right() - 84 - 2*healthArrowW, Y: y, W: healthArrowW, H: healthOpRowH - 2}
		// MED-06: the success chance, its factors on hover
		if op.ChanceTip != "" {
			cr := gfx.Rect{X: upR.X - healthChanceW - 2, Y: y, W: healthChanceW, H: healthOpRowH - 2}
			cs := gfx.Body
			cs.Color, cs.Align = HealthChanceColor(op.ChancePct), gfx.AlignRight
			a.kit().Label(cr.Right()-2, y+4, healthChanceText(op), cs)
			a.kit().Add(uikit.Widget{ID: HealthChanceID(op.Slot), R: cr, Label: healthChanceText(op), Tip: op.ChanceTip})
		}
		downR := gfx.Rect{X: upR.Right() + 2, Y: y, W: healthArrowW, H: healthOpRowH - 2}
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
		if a.healthArrow(HealthUpID(op.Slot), upR, true, uikit.BtnOpts{Disabled: upWhy != "", Reason: upWhy,
			Tip: "Move up\nDo " + op.Label + " sooner."}) {
			a.shell.Enqueue(command.Command{Kind: command.ReorderOperation, Pawn: uint64(p.ID), Index: op.Slot, Count: -1})
		}
		if a.healthArrow(HealthDownID(op.Slot), downR, false, uikit.BtnOpts{Disabled: downWhy != "", Reason: downWhy,
			Tip: "Move down\nDo " + op.Label + " later."}) {
			a.shell.Enqueue(command.Command{Kind: command.ReorderOperation, Pawn: uint64(p.ID), Index: op.Slot, Count: 1})
		}
		if a.kit().Button(HealthRemoveID(op.Slot), gfx.Rect{X: r.Right() - 80, Y: y, W: 80, H: healthOpRowH - 2}, "Cancel",
			uikit.BtnOpts{Style: uikit.BtnDanger, Tip: "Cancel\nTake " + op.Label + " off the queue."}) {
			a.shell.Enqueue(command.Command{Kind: command.RemoveOperation, Pawn: uint64(p.ID), Index: op.Slot})
		}
		y += healthOpRowH
	}
}

// opStatusColor is a queued operation's status colour: a warning when the
// wait needs the player (stock, research, reach, a second doctor), dim
// while it is only a matter of time.
func OpStatusColor(op game.OperationView) color.Color {
	if op.Blocked {
		return uikit.Warning
	}
	return uikit.TextDim
}

// healthOption is one operation the picker offers.
type HealthOption struct {
	Label string
	Op    body.OpKind
	Part  body.Part
	Item  item.Kind
	// Reason is why it can't happen yet ("" when it can); it can still be queued.
	Reason string
}

// pickerOptions are the operations the picker lists: the view's own
// Options (with their reasons), else the local derivation for views
// that carry none.
func PickerOptions(h game.HealthView) []HealthOption {
	if len(h.Options) == 0 {
		return HealthOptions(h)
	}
	out := make([]HealthOption, 0, len(h.Options))
	for _, o := range h.Options {
		r := ""
		if o.Disabled {
			r = o.Reason
			if r == "" {
				r = "Not possible yet."
			}
		}
		out = append(out, HealthOption{Label: o.Label, Op: body.OpKind(o.Kind), Part: body.Part(o.Part), Item: item.Kind(o.Item), Reason: r})
	}
	return out
}

// healthOptions lists the operations that fit the body as the view shows
// it (Parts[i] is body.Part(i)): an implant for each missing part it fits,
// then removing each implant, then amputating each part still there that
// is not vital. Queued ones are left out.
func HealthOptions(h game.HealthView) []HealthOption {
	queued := map[string]bool{}
	for _, o := range h.Operations {
		queued[o.Label] = true
	}
	var out []HealthOption
	add := func(o HealthOption) {
		if !queued[o.Label] {
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
					add(HealthOption{Label: fmt.Sprintf("Install %s (%s)", k, pv.Label), Op: body.InstallImplant, Part: part, Item: k})
				}
			}
		case strings.HasPrefix(pv.Status, game.PartArtificial):
			add(HealthOption{Label: fmt.Sprintf("Remove implant (%s)", pv.Label), Op: body.RemoveImplant, Part: part})
		case pv.Status == game.PartReplaced:
		case !body.DefOf(part).Vital && !body.Inner(part):
			add(HealthOption{Label: "Amputate " + pv.Label, Op: body.Amputate, Part: part})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Op < out[j].Op })
	return out
}

// healthOptionTip is a picker row's hover card: what the operation does
// (MED-06), then why it can't happen yet or what it needs.
func HealthOptionTip(o HealthOption) string {
	t := o.Label
	switch o.Op {
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
	case o.Reason != "":
		return t + "\n" + o.Reason + "\nYou can queue it anyway; it waits until it can happen."
	case o.Item != 0:
		return t + "\nNeeds a " + o.Item.String() + " in stock, medicine and a doctor."
	}
	return t + "\nNeeds medicine and a doctor; the risk falls with Medicine skill, a clean, lit room, a hospital bed and a vitals monitor."
}

// openHealthPicker opens the Add operation window for patient id.
func (a Tabs) openHealthPicker(id entity.EntityID) {
	sel := -1
	options := func() []HealthOption {
		h, ok := a.sim().Health(id)
		if !ok {
			return nil
		}
		return PickerOptions(h)
	}
	a.kit().Push(uikit.Modal{
		ID: HealthPickID, Title: "Add operation", W: healthPickW, H: healthPickH,
		Draw: func(r gfx.Rect) {
			opts := options()
			if sel >= len(opts) {
				sel = -1
			}
			if len(opts) == 0 {
				a.kit().Label(r.X, r.Y, "No operation fits this body.", uikit.DimText())
				return
			}
			a.kit().List(HealthPickList, r, len(opts), &sel, uikit.ListSpec{
				RowH:  36,
				Label: func(i int) string { return opts[i].Label },
				Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
					a.paintOptionRow(dst, row, opts[i], hover, selected)
				},
				Tip: func(i int) string { return HealthOptionTip(opts[i]) },
			})
		},
		Buttons: []uikit.ModalButton{
			{ID: "health.addop.ok", Label: "Add", Style: uikit.BtnPrimary, Closes: true,
				Disabled: func() string {
					if sel < 0 || sel >= len(options()) {
						return "Pick an operation first."
					}
					return ""
				},
				OnClick: func() {
					opts := options()
					if sel < 0 || sel >= len(opts) {
						return
					}
					o := opts[sel]
					a.shell.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(id),
						Index: int(o.Op), Build: uint8(o.Part), Stuff: uint8(o.Item)})
				}},
			{ID: "health.addop.cancel", Label: "Cancel", Closes: true},
		},
	})
}

// paintOptionRow draws a picker row with the implant's icon when its art
// exists (the label alone otherwise).
func (a Tabs) paintOptionRow(dst *ebiten.Image, row gfx.Rect, o HealthOption, hover, selected bool) {
	if selected || hover {
		uikit.FillRect(dst, row, uiStripe)
	}
	textX := row.X + 6
	if o.Item != 0 && a.env().Art != nil {
		if key := world.ItemSprite(o.Item.String()); key != "" && a.env().Art.Has(key) {
			gfx.Icon(dst, a.env().Art.Image(key, color.NRGBA{}), float64(row.X+4), float64(row.Y+2), float64(row.H-4))
			textX = row.X + row.H + 4
		}
	}
	st := gfx.Body
	if o.Reason != "" {
		st.Color = uikit.TextDim
	}
	if selected {
		st.Color = uikit.Accent
	}
	if o.Reason == "" {
		gfx.Text(dst, uikit.FitText(o.Label, st, row.Right()-textX-4), float64(textX), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
		return
	}
	sm := gfx.Small
	sm.Color = uikit.Warning
	gfx.Text(dst, uikit.FitText(o.Label, st, row.Right()-textX-4), float64(textX), float64(row.Y)+1, st)
	gfx.Text(dst, uikit.FitText(o.Reason, sm, row.Right()-textX-4), float64(textX), float64(row.Y)+gfx.LineH(st)+1, sm)
}
