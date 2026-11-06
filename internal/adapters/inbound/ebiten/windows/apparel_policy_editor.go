package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/command"
)

// The Apparel policies window: the policy list with New, Copy and Delete, a
// name field, the "default for new colonists" toggle, a tree of apparel
// kinds and the materials, hit point and quality ranges, and the tainted
// toggle. The ranges are drafts that go out as one EditApparelPolicy when
// the knob is let go.

const (
	ApparelPolicyModalID = "colony.apparelpolicy"
	apparelPolicyW       = 760
	apparelPolicyH       = 520
	apparelListW         = 170
	apparelRowH          = 24
	apparelCtlH          = 24
	apparelNameMax       = 24
	apparelRoot          = "Apparel"
	apparelNewName       = "New outfit"
	apparelPending       = 90 // frames a sent range is kept before the view is trusted again
)

// Widget ids.
const (
	ApparelNewID       = "apparelpolicy.new"
	ApparelCopyID      = "apparelpolicy.copy"
	ApparelDeleteID    = "apparelpolicy.delete"
	ApparelNameID      = "apparelpolicy.name"
	ApparelBackID      = "apparelpolicy.back"
	ApparelDefaultID   = "apparelpolicy.default"
	ApparelAllowAllID  = "apparelpolicy.allow_all"
	ApparelClearAllID  = "apparelpolicy.clear_all"
	ApparelHPMinID     = "apparelpolicy.hp.min"
	ApparelHPMaxID     = "apparelpolicy.hp.max"
	ApparelQualMinID   = "apparelpolicy.quality.min"
	ApparelQualMaxID   = "apparelpolicy.quality.max"
	ApparelTaintedID   = "apparelpolicy.tainted"
	apparelSelPrefix   = "apparelpolicy.sel."
	apparelFilterPfx   = "apparelpolicy.filter."
	apparelExpandPfx   = "apparelpolicy.expand."
	apparelStuffPrefix = "apparelpolicy.stuff."
)

// ApparelSelectID is policy id's row in the editor.
func ApparelSelectID(id uint32) string { return apparelSelPrefix + strconv.FormatUint(uint64(id), 10) }

// apparelRange is a range slider pair's draft. Pending counts down the
// frames a sent value is held, so the view catching up does not snap the
// knob back.
type apparelRange struct {
	lo, hi  int
	dirty   bool
	pending int
}

// outfitState is the editor's own state.
type outfitState struct {
	sel     uint32          // the policy being edited
	last    uint32          // the policy whose name and ranges the drafts hold
	name    string          // the rename field
	open    map[string]bool // expanded category paths
	hp, qlt apparelRange
}

// OpenApparelPolicies opens the Apparel policies window.
func (w Assign) OpenApparelPolicies() {
	if !w.live() {
		return
	}
	w.outfit.open = map[string]bool{}
	w.OpenKit(uikit.Modal{
		ID: ApparelPolicyModalID, Title: w.sh.T("Apparel policies"), W: apparelPolicyW, H: apparelPolicyH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.apparelBody(body) },
	})
}

// apparelBody declares the policy list, its buttons, the name field and
// the controls of the selected policy.
func (w Assign) apparelBody(body gfx.Rect) {
	st := w.outfit
	ps := w.sh.World().Gear.Policies
	if len(ps) == 0 {
		w.kit().Label(body.X, body.Y, w.sh.T("Apparel policies are not available in this game."), uikit.DimText())
		return
	}
	cur := ps[0]
	if i := outfitPolicyIndex(ps, int(st.sel)); i >= 0 {
		cur = ps[i]
	}
	st.sel = cur.ID
	if st.last != cur.ID { // a draft may be empty while it is edited: only a new selection reloads it
		st.name, st.last = cur.Name, cur.ID
		st.hp, st.qlt = apparelRange{}, apparelRange{}
	}
	if w.kit().Button(ApparelBackID, gfx.Rect{X: body.Right() - 80, Y: body.Y - 2, W: 80, H: apparelCtlH}, w.sh.T("Assign"), uikit.BtnOpts{Tip: w.sh.T("Back to the Assign window.")}) {
		w.Launch()
		return
	}
	w.apparelList(body, ps, cur)
	right := gfx.Rect{X: body.X + apparelListW + 12, Y: body.Y, W: body.W - apparelListW - 12 - 88, H: body.H}
	field := gfx.Rect{X: right.X, Y: right.Y, W: right.W, H: apparelCtlH}
	if _, committed := w.kit().TextField(ApparelNameID, field, &st.name, uikit.TextFieldSpec{MaxLen: apparelNameMax, Placeholder: w.sh.T("Name"), Normalize: trimName}); committed {
		if st.name == "" { // an empty name is refused on commit: the old one comes back
			st.name = cur.Name
		} else {
			w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyRename, Index: int(cur.ID), Name: st.name})
		}
	}
	full := gfx.Rect{X: right.X, Y: right.Y + apparelCtlH + 4, W: body.Right() - right.X, H: body.H - apparelCtlH - 4}
	w.apparelDefault(full, cur)
	full.Y, full.H = full.Y+apparelCtlH+4, full.H-apparelCtlH-4
	w.apparelRanges(gfx.Rect{X: full.X, Y: full.Bottom() - 3*apparelCtlH - 8, W: full.W, H: 3*apparelCtlH + 8}, cur)
	w.apparelTree(gfx.Rect{X: full.X, Y: full.Y, W: full.W, H: full.H - 3*apparelCtlH - 16}, cur)
}

// apparelList declares the policy list and the New, Copy and Delete buttons.
func (w Assign) apparelList(body gfx.Rect, ps []gearview.Policy, cur gearview.Policy) {
	st := w.outfit
	by := body.Bottom() - 2*apparelCtlH - 4
	list := gfx.Rect{X: body.X, Y: body.Y + apparelCtlH + 4, W: apparelListW, H: by - body.Y - apparelCtlH - 8}
	area := w.kit().ScrollArea("apparelpolicy.list", list, float32(len(ps))*(apparelRowH+2), uikit.ScrollOpts{Step: apparelRowH * 3})
	for i, p := range ps {
		r := gfx.Rect{X: area.View.X, Y: area.View.Y + float32(i)*(apparelRowH+2) - area.Off, W: area.View.W, H: apparelRowH}
		if r.Y < area.View.Y || r.Bottom() > area.View.Bottom() {
			continue
		}
		label := p.Name
		if p.Default {
			label += " *"
		}
		if w.kit().Button(ApparelSelectID(p.ID), r, gfx.Ellipsize(label, gfx.Body, float64(r.W)-12), uikit.BtnOpts{Selected: p.ID == cur.ID, Tip: lang.Tf(w.sh, "Edit %s", p.Name)}) {
			st.sel = p.ID
		}
	}
	w.kit().Label(body.X, body.Y+4, w.sh.T("Policies (* default)"), uikit.DimText())
	half := float32(apparelListW-6) / 2
	if w.kit().Button(ApparelNewID, gfx.Rect{X: body.X, Y: by, W: half, H: apparelCtlH}, w.sh.T("New"), uikit.BtnOpts{Tip: w.sh.T("New policy\nMake a policy that allows every garment.")}) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: -1, Name: apparelNewName})
	}
	if w.kit().Button(ApparelCopyID, gfx.Rect{X: body.X + half + 6, Y: by, W: half, H: apparelCtlH}, w.sh.T("Copy"), uikit.BtnOpts{Tip: w.sh.T("Copy policy\nMake a policy as a copy of the selected one.")}) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: int(cur.ID), Name: cur.Name})
	}
	del := uikit.BtnOpts{Tip: w.sh.T("Delete policy\nIts wearers fall back to the default policy.")}
	switch {
	case cur.Default:
		del.Disabled, del.Reason = true, w.sh.T("The default policy stays. Make another one the default first.")
	case len(ps) <= 1:
		del.Disabled, del.Reason = true, w.sh.T("The last policy stays.")
	}
	if w.kit().Button(ApparelDeleteID, gfx.Rect{X: body.X, Y: by + apparelCtlH + 4, W: apparelListW, H: apparelCtlH}, w.sh.T("Delete"), del) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDelete, Index: int(cur.ID)})
		st.sel = ps[0].ID
	}
}

// apparelDefault is the "default for new colonists" toggle.
func (w Assign) apparelDefault(r gfx.Rect, cur gearview.Policy) {
	on := cur.Default
	o := uikit.CheckOpts{Tip: w.sh.T("Default policy\nColonists who join later start on the default policy.")}
	if cur.Default {
		o.Disabled, o.Reason = true, w.sh.T("This is the default. Make another policy the default to change it.")
	}
	if w.kit().Check(ApparelDefaultID, gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: apparelCtlH}, w.sh.T("Default for new colonists"), &on, o) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDefault, Index: int(cur.ID)})
	}
}

// apparelRanges declares the hit point and quality sliders and the tainted
// toggle in r (three rows).
func (w Assign) apparelRanges(r gfx.Rect, cur gearview.Policy) {
	st := w.outfit
	w.rangeRow(gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: apparelCtlH}, w.sh.T("Hit points"), &st.hp, [2]int{cur.HPMin, cur.HPMax},
		[2]string{ApparelHPMinID, ApparelHPMaxID}, 0, 100, func(v float64) string { return strconv.Itoa(int(v)) + "%" },
		func(lo, hi int) {
			w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyHP, Index: int(cur.ID), X: lo, Y: hi})
		})
	w.rangeRow(gfx.Rect{X: r.X, Y: r.Y + apparelCtlH + 4, W: r.W, H: apparelCtlH}, w.sh.T("Quality"), &st.qlt, [2]int{int(cur.QualityMin), int(cur.QualityMax)},
		[2]string{ApparelQualMinID, ApparelQualMaxID}, 0, int(quality.Legendary), func(v float64) string { return quality.Level(v).String() },
		func(lo, hi int) {
			w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyQuality, Index: int(cur.ID), X: lo, Y: hi})
		})
	allowed := !cur.NoTainted
	if w.kit().Check(ApparelTaintedID, gfx.Rect{X: r.X, Y: r.Y + 2*(apparelCtlH+4), W: r.W, H: apparelCtlH}, w.sh.T("Allow tainted garments (worn by the dead)"), &allowed,
		uikit.CheckOpts{Tip: w.sh.T("Tainted garments\nClothes taken from a corpse make the wearer uneasy. Off: they are never put on.")}) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyTainted, Index: int(cur.ID), Enabled: cur.NoTainted})
	}
}

// rangeRow declares a label and a low and a high slider. The pair keeps
// lo <= hi and goes out as one command once no knob is held.
func (w Assign) rangeRow(r gfx.Rect, label string, d *apparelRange, view [2]int, ids [2]string, lo, hi int, format func(float64) string, send func(lo, hi int)) {
	held := w.kit().Active == ids[0] || w.kit().Active == ids[1]
	if d.pending > 0 && !held {
		d.pending--
		if d.lo == view[0] && d.hi == view[1] {
			d.pending = 0
		}
	}
	if !held && !d.dirty && d.pending == 0 {
		d.lo, d.hi = view[0], view[1]
	}
	w.kit().Label(r.X, r.Y+4, label, uikit.DimText())
	sx := r.X + 96
	sw := (r.Right() - sx - 8) / 2
	spec := func(title string) uikit.SliderSpec {
		return uikit.SliderSpec{Min: float64(lo), Max: float64(hi), Step: 1, Format: format, Title: title}
	}
	if w.kit().SliderInt(ids[0], gfx.Rect{X: sx, Y: r.Y, W: sw, H: r.H}, &d.lo, spec(lang.Tf(w.sh, "Lowest %s", strings.ToLower(label)))) {
		d.hi, d.dirty = max(d.hi, d.lo), true
	}
	if w.kit().SliderInt(ids[1], gfx.Rect{X: sx + sw + 8, Y: r.Y, W: sw, H: r.H}, &d.hi, spec(lang.Tf(w.sh, "Highest %s", strings.ToLower(label)))) {
		d.lo, d.dirty = min(d.lo, d.hi), true
	}
	if d.dirty && !held {
		send(d.lo, d.hi)
		d.dirty, d.pending = false, apparelPending
	}
}

// apparelRow is a line of the tree: a filter-tree node, a heading or a
// material.
type apparelRow struct {
	node  game.FilterRow
	stuff string // a material's name ("" for a node)
	head  string // a heading's text
}

// apparelRows are the Apparel branch of the filter tree for policy p, with
// the children of a collapsed category left out, then the materials.
func apparelRows(p gearview.Policy, open map[string]bool) []apparelRow {
	var out []apparelRow
	hide := -1
	for _, r := range game.StockpileFilterRows(p.Allowed) {
		if r.Path != apparelRoot && !strings.HasPrefix(r.Path, apparelRoot+"/") {
			continue
		}
		if hide >= 0 {
			if r.Depth > hide {
				continue
			}
			hide = -1
		}
		out = append(out, apparelRow{node: r})
		if !r.Leaf && !open[r.Path] {
			hide = r.Depth
		}
	}
	out = append(out, apparelRow{head: "Materials"})
	for _, m := range apparelMaterials() {
		out = append(out, apparelRow{stuff: m})
	}
	return out
}

// apparelMaterials are the materials the garments of the tree are made
// from, in kind order.
func apparelMaterials() []string {
	apparel, _ := zone.Resolve(apparelRoot)
	var cats item.StuffCategory
	for _, n := range zone.Tree() {
		if n.Leaf && apparel.Has(n.Kind) {
			cats |= item.MadeFrom(n.Kind)
		}
	}
	var out []string
	for _, k := range item.Stuffs(cats) {
		out = append(out, k.String())
	}
	return out
}

// apparelTree declares the scrolling tree of allowed garments and
// materials of policy p.
func (w Assign) apparelTree(r gfx.Rect, p gearview.Policy) {
	st := w.outfit
	w.kit().Label(r.X, r.Y+2, w.sh.T("Allowed garments"), uikit.DimText())
	allow := gfx.Rect{X: r.Right() - 170, Y: r.Y, W: 82, H: apparelCtlH}
	clear := gfx.Rect{X: r.Right() - 84, Y: r.Y, W: 82, H: apparelCtlH}
	if w.kit().Button(ApparelAllowAllID, allow, w.sh.T("Allow all"), uikit.BtnOpts{Tip: w.sh.T("Allow every garment.")}) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyAllow, Index: int(p.ID), Name: apparelRoot, Enabled: true})
	}
	if w.kit().Button(ApparelClearAllID, clear, w.sh.T("Clear all"), uikit.BtnOpts{Tip: w.sh.T("Forbid every garment.")}) {
		w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyAllow, Index: int(p.ID), Name: apparelRoot, Enabled: false})
	}
	list := gfx.Rect{X: r.X, Y: r.Y + apparelCtlH + 4, W: r.W, H: max(0, r.H-apparelCtlH-4)}
	rows := apparelRows(p, st.open)
	area := w.kit().ScrollArea("apparelpolicy.tree", list, float32(len(rows))*apparelRowH, uikit.ScrollOpts{Step: apparelRowH * 3, Focusable: true})
	for i, row := range rows {
		y := area.View.Y + float32(i)*apparelRowH - area.Off
		if y < area.View.Y || y+apparelRowH > area.View.Bottom() {
			continue
		}
		w.apparelLine(row, p, st, gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: apparelRowH})
	}
}

// apparelLine declares one line of the tree.
func (w Assign) apparelLine(row apparelRow, p gearview.Policy, st *outfitState, r gfx.Rect) {
	switch {
	case row.head != "":
		w.kit().Label(r.X, r.Y+4, w.sh.T("Materials"), gfx.WithColor(gfx.Label, uikit.Heading))
	case row.stuff != "":
		was := hasName(p.Stuffs, row.stuff)
		on := was
		if w.kit().Check(apparelStuffPrefix+row.stuff, r, titleCase(row.stuff), &on, uikit.CheckOpts{Tip: lang.Tf(w.sh, "Material\nAllow garments made of %s.", row.stuff)}) {
			w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyStuff, Index: int(p.ID), Name: row.stuff, Enabled: !was})
		}
	default:
		n := row.node
		x := r.X + float32(n.Depth*22)
		if !n.Leaf {
			mark := "+"
			if st.open[n.Path] {
				mark = "-"
			}
			if w.kit().Button(apparelExpandPfx+n.Path, gfx.Rect{X: x, Y: r.Y, W: 22, H: apparelRowH}, mark, uikit.BtnOpts{Tip: lang.Tf(w.sh, "Show or hide the garments in %s", n.Label)}) {
				st.open[n.Path] = !st.open[n.Path]
			}
			x += 26
		}
		on, label := n.On, n.Label
		if n.Partial {
			label = lang.Tf(w.sh, "%s  (some)", label)
		}
		if w.kit().Check(apparelFilterPfx+n.Path, gfx.Rect{X: x, Y: r.Y, W: r.Right() - x, H: apparelRowH}, label, &on, uikit.CheckOpts{}) {
			w.sh.Enqueue(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyAllow, Index: int(p.ID), Name: n.Path, Enabled: !n.On})
		}
	}
}

// titleCase is s with its first letter in capitals.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
