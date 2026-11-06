package gear

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The panel the tab asks for (the layout clamps it to 400..560 wide).
const (
	WantW, WantH = 480, 400
)

// Layout metrics (logical px).
const (
	ScrollID = "gear.rows" // the tab's scroll view
	iconSz   = 24          // an item's sprite
	btnSz    = 22          // a square icon button
	btnGap   = 2
	unloadW  = 56 // the Unload / Cancel text button
	barH     = 10
)

// Widget ids of an item row's buttons. what is "weapon", "spare", "worn"
// or "pack" and n the garment index or pack entry ID (as MarkUnload).
func InfoID(what string, n int) string   { return fmt.Sprintf("gear.info.%s.%d", what, n) }
func DropID(what string, n int) string   { return fmt.Sprintf("gear.drop.%s.%d", what, n) }
func UnloadID(what string, n int) string { return fmt.Sprintf("gear.unload.%s.%d", what, n) }
func LockID(n int) string                { return fmt.Sprintf("gear.lock.%d", n) }
func EatID(n int) string                 { return fmt.Sprintf("gear.eat.%d", n) }

// Tab is the Gear tab over its host.
type Tab struct {
	h Host
	f Facts
}

// New is the tab over h and f.
func New(h Host, f Facts) Tab { return Tab{h: h, f: f} }

// access is what the player may do with p's gear from the tab.
type access struct {
	readOnly bool   // a guest, visitor, ally or hostile: Info only
	lock     bool   // a colonist: the lock toggle shows
	unload   bool   // the manual-unload habit applies
	broken   string // a mental break: every action is refused with this
}

func accessOf(p game.PawnView, unloading bool) access {
	a := access{readOnly: p.Hostile || p.Ally || p.Visitor && !p.Prisoner}
	a.lock = !a.readOnly && !p.Prisoner
	a.unload = unloading && !a.readOnly && !p.Prisoner && p.Status == "alive"
	if p.MentalBreak != "" {
		a.broken = "In a mental break (" + p.MentalBreak + "): no gear orders until it passes."
	}
	return a
}

// ReadOnly reports the tab offers p no actions (a guest or visitor).
func ReadOnly(p game.PawnView) bool { return accessOf(p, false).readOnly }

func rowH(r Row) float32 {
	switch {
	case r.kind == rowHead:
		return gfx.LineStep(gfx.Label) + 6
	case r.kind == rowItem:
		return max(iconSz+6, 2*gfx.LineStep(gfx.Small)+4)
	case r.Bar > 0:
		return max(gfx.LineStep(gfx.Small), barH) + 4
	}
	return gfx.LineStep(gfx.Small) + 2
}

// ContentH is the height of rows stacked.
func ContentH(rows []Row) float32 {
	var h float32
	for _, r := range rows {
		h += rowH(r)
	}
	return h
}

// Declare declares the tab of p inside r, the expanded panel's inner
// rectangle: the rows in a scroll view, each item's buttons and tooltips.
func (t Tab) Declare(p game.PawnView, r gfx.Rect) {
	rows := Rows(p, t.f)
	k := t.h.Kit()
	area := k.ScrollArea(ScrollID, r, ContentH(rows), uikit.ScrollOpts{Step: 3 * gfx.LineStep(gfx.Small)})
	view := area.View
	type shown struct {
		row  Row
		rect gfx.Rect
		img  *ebiten.Image
	}
	var vis []shown
	y := view.Y - area.Off
	for _, row := range rows {
		h := rowH(row)
		rr := gfx.Rect{X: view.X, Y: y, W: view.W, H: h}
		y += h
		if rr.Bottom() <= view.Y || rr.Y >= view.Bottom() {
			continue
		}
		var img *ebiten.Image
		if row.Item() && row.Sprite != "" {
			img = t.h.ItemImage(row.Sprite)
		}
		vis = append(vis, shown{row, rr, img})
	}
	ac := accessOf(p, t.h.Unloading())
	warn := k.Access.Palette().Warning
	textW := func(r Row) float32 {
		if r.kind != rowItem {
			return view.W - 4
		}
		return view.W - iconSz - 6 - buttonsW(r, ac)
	}
	k.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, view)
		for _, s := range vis {
			paintRow(sub, s.row, s.rect, s.img, textW(s.row), warn)
		}
	})
	for i, s := range vis {
		if s.row.Tip != "" {
			hit := gfx.Rect{X: s.rect.X, Y: s.rect.Y, W: textW(s.row) + iconSz + 6, H: s.rect.H}.Intersect(view)
			if !hit.Empty() {
				k.Tip(fmt.Sprintf("gear.row.%d", i), hit, s.row.Tip)
			}
		}
		if s.row.Item() && s.rect.Y >= view.Y && s.rect.Bottom() <= view.Bottom()+0.01 {
			t.buttons(p, s.row, s.rect, ac)
		}
	}
}

// buttonsW is the width an item row's buttons take at its right.
func buttonsW(r Row, ac access) float32 {
	n := 1 // Info
	if !ac.readOnly {
		n++ // Drop
		if r.garment && ac.lock {
			n++
		}
		if r.Use != gearview.UseNone {
			n++
		}
	}
	w := float32(n) * (btnSz + btnGap)
	if ac.unload {
		w += unloadW + btnGap
	}
	return w
}

// buttons declares an item row's buttons, right to left: Info, Drop, the
// lock or Eat, and Unload while that habit is on.
func (t Tab) buttons(p game.PawnView, r Row, rr gfx.Rect, ac access) {
	x := rr.Right()
	by := rr.Y + (rr.H-btnSz)/2
	next := func(w float32) gfx.Rect {
		x -= w + btnGap
		return gfx.Rect{X: x + btnGap, Y: by, W: w, H: btnSz}
	}
	id := p.ID
	if t.iconButton(InfoID(r.What, r.N), next(btnSz), "ui_icon/gear_info", "i",
		uikit.BtnOpts{Tip: "Info\nWhat this is, in full.", NoFocus: true}) {
		t.h.Do(ports.OpenInfoCard{Pawn: id, Mode: r.Mode, Index: r.N})
	}
	if ac.readOnly {
		return
	}
	drop := uikit.BtnOpts{Tip: "Drop\nPut it down here and now. Shift-click to do it after the queued orders.", NoFocus: true}
	switch {
	case ac.broken != "":
		drop.Disabled, drop.Reason = true, ac.broken
	case p.Gear.NoDrop != "":
		drop.Disabled, drop.Reason = true, p.Gear.NoDrop
	}
	if t.iconButton(DropID(r.What, r.N), next(btnSz), "ui_icon/gear_drop", "D", drop) {
		t.h.Enqueue(command.Command{Kind: command.DropGear, Pawn: uint64(id), Mode: r.Mode, Index: r.N, Queue: t.h.Shift()})
	}
	if r.garment && ac.lock {
		t.lockButton(id, r, next(btnSz), ac)
	}
	if r.Use != gearview.UseNone {
		t.eatButton(id, r, next(btnSz), ac)
	}
	if ac.unload {
		t.unloadButton(p, r, next(unloadW), ac)
	}
}

func (t Tab) lockButton(id entity.EntityID, r Row, br gfx.Rect, ac access) {
	o := uikit.BtnOpts{Selected: r.Forced, NoFocus: true,
		Tip: "Lock\nKeep it on: the apparel policy will not take it off."}
	key, fb := "ui_icon/gear_unlock", "L"
	if r.Forced {
		key, o.Tip = "ui_icon/gear_lock", "Unlock\nLet the apparel policy decide about it again."
	}
	if ac.broken != "" {
		o.Disabled, o.Reason = true, ac.broken
	}
	if t.iconButton(LockID(r.N), br, key, fb, o) {
		t.h.Enqueue(command.Command{Kind: command.SetApparelForced, Pawn: uint64(id), Index: r.N, Enabled: !r.Forced})
	}
}

func (t Tab) eatButton(id entity.EntityID, r Row, br gfx.Rect, ac access) {
	o := uikit.BtnOpts{NoFocus: true, Tip: "Eat\nEat it now, where they stand."}
	if r.Use == gearview.UseDrug {
		o.Tip = "Take\nTake one dose now. The drug policy still applies."
	}
	switch {
	case ac.broken != "":
		o.Disabled, o.Reason = true, ac.broken
	case r.EatWhy != "":
		o.Disabled, o.Reason = true, r.EatWhy
	}
	if t.iconButton(EatID(r.N), br, "ui_icon/gear_eat", "E", o) {
		t.h.Enqueue(command.Command{Kind: command.ConsumeFromPack, Pawn: uint64(id), Index: r.N})
	}
}

// unloadButton is CSE-038's Unload (Cancel on a marked pack entry).
func (t Tab) unloadButton(p game.PawnView, r Row, br gfx.Rect, ac access) {
	label, tip := "Unload", "Unload\nPut this away: in a stockpile if the colony has room, else on the ground beside "+p.Name+"."
	if r.Marked {
		label, tip = "Cancel", "Cancel\nLeave this in the pack."
	}
	o := uikit.BtnOpts{Tip: tip, NoFocus: true}
	if ac.broken != "" {
		o.Disabled, o.Reason = true, ac.broken
	}
	if t.h.Kit().Button(UnloadID(r.What, r.N), br, label, o) {
		t.h.Enqueue(command.Command{Kind: command.MarkUnload, Pawn: uint64(p.ID), Mode: r.Mode, Index: r.N, Enabled: !r.Marked})
	}
}

// iconButton is a square kit button with ui icon key over it (the
// fallback letter when the art is missing); greyed while disabled.
func (t Tab) iconButton(id string, br gfx.Rect, key, fallback string, o uikit.BtnOpts) bool {
	img := t.h.Image(key)
	label := ""
	if img == nil {
		label = fallback
	}
	clicked := t.h.Kit().Button(id, br, label, o)
	if img != nil {
		dis := o.Disabled
		t.h.Kit().Queue(func(dst *ebiten.Image) {
			var op ebiten.DrawImageOptions
			op.Filter = ebiten.FilterLinear
			if dis {
				op.ColorScale.Scale(0.45, 0.45, 0.45, 1)
			}
			s := float64(br.W - 4)
			b := img.Bounds()
			gfx.Image(dst, img, float64(br.X+2), float64(br.Y+2), s, s*float64(b.Dy())/float64(max(1, b.Dx())), &op)
		})
	}
	return clicked
}
