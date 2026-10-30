package windows

import (
	"fmt"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// The Storage settings window (PRD-4, MAIN_UI_SCREENS §17-§18 zone
// settings): a stockpile or shelf's name, Copy and Paste settings, special
// filters and the allowed-items tree (categories that open into item
// kinds, each with an allow checkbox). It reads ZoneView.Filter and
// Special and sends SetStockpileFilter, SetStockpileSpecial,
// PasteStockpile and RenameZone. The copy clipboard is this window's
// state (the source zone id), never the simulation's.

const (
	StorageModalID = "colony.storage"
	storageRowH    = 24
	storageIndent  = 22
	storageHeadH   = 30
	storageW       = 440
	storageH       = 520
)

// storageState is the window's own state.
type storageState struct {
	zone uint16          // the zone being edited
	clip uint16          // the copied zone (0: none)
	open map[string]bool // category paths shown expanded
	name string          // the rename field's text
}

// Range filters (SetStockpileSpecial Index 2 and 3): X..X2.
const (
	specialHP      = 2
	specialQuality = 3
	hpStep         = 10
	hpMax          = 100
	qualityMax     = 6
)

// storageHas reports the zone's allowed list holds name.
func storageHas(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

// StorageClipboard is the zone id Copy holds (0: nothing copied).
func (w Storage) StorageClipboard() uint16 { return w.storage.clip }

// Open shows the settings of storage zone id.
func (w Storage) Open(id uint16) {
	if !w.live() {
		return
	}
	w.storage.zone = id
	w.storage.name = ""
	if z, ok := w.storageZone(); ok {
		w.storage.name = z.Name
	}
	w.OpenKit(uikit.Modal{
		ID: StorageModalID, Title: "Storage settings", W: storageW, H: storageH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// storageZone is the zone being edited, while it exists.
func (w Storage) storageZone() (game.ZoneView, bool) {
	for _, z := range w.sh.World().Zones {
		if z.ID == w.storage.zone {
			return z, true
		}
	}
	return game.ZoneView{}, false
}

func (w Storage) zoneExists(id uint16) bool {
	for _, z := range w.sh.World().Zones {
		if z.ID == id {
			return true
		}
	}
	return false
}

// body declares the header (name, Copy, Paste) and the scrolling tree.
func (w Storage) body(body gfx.Rect) {
	z, ok := w.storageZone()
	if !ok {
		w.kit().Label(body.X, body.Y, "This storage zone is gone.", uikit.DimText())
		return
	}
	st := w.storage
	if st.open == nil {
		st.open = map[string]bool{}
	}
	zid := z.ID
	field := gfx.Rect{X: body.X, Y: body.Y, W: body.W - 150, H: 26}
	if _, committed := w.kit().TextField("storage.name", field, &st.name, uikit.TextFieldSpec{
		MaxLen: game.ZoneNameMax, Placeholder: "Name", Normalize: strings.TrimSpace}); committed {
		w.sh.Enqueue(command.Command{Kind: command.RenameZone, Target: uint64(zid), Name: st.name})
	}
	copyR := gfx.Rect{X: field.Right() + 6, Y: body.Y, W: 66, H: 26}
	pasteR := gfx.Rect{X: copyR.Right() + 6, Y: body.Y, W: 66, H: 26}
	if w.kit().Button("storage.copy", copyR, "Copy", uikit.BtnOpts{Tip: "Copy settings\nRemember this zone's allowed items, special filters and priority."}) {
		st.clip = zid
	}
	paste := uikit.BtnOpts{Tip: "Paste settings\nApply the copied zone's allowed items, special filters and priority here."}
	switch {
	case st.clip == 0:
		paste.Disabled, paste.Reason = true, "nothing copied yet"
	case st.clip == zid:
		paste.Disabled, paste.Reason = true, "this is the copied zone"
	case !w.zoneExists(st.clip):
		paste.Disabled, paste.Reason = true, "the copied zone is gone"
	}
	if w.kit().Button("storage.paste", pasteR, "Paste", paste) {
		w.sh.Enqueue(command.Command{Kind: command.PasteStockpile, Target: uint64(zid), Zone: uint64(st.clip)})
	}
	list := gfx.Rect{X: body.X, Y: body.Y + storageHeadH, W: body.W, H: body.H - storageHeadH}
	w.tree(list, z, st.open)
}

// treeRow is one visible line of the tree.
type treeRow struct {
	id, label string
	path      string // the filter row's Path
	special   int    // >= 0: a special toggle index
	rng       int    // specialHP or specialQuality: a range stepper line
	lo, hi    int    // the range's current ends
	depth     int
	leaf      bool
	on        bool
	partial   bool
	head      bool // a heading line
}

// rangeOf reads "hp A-B%" or "quality lo-hi" from the zone's active
// specials (the full range while it is not narrowed).
func rangeOf(special []string, name string, max int) (int, int) {
	for _, s := range special {
		if rest, ok := strings.CutPrefix(s, name+" "); ok {
			var lo, hi int
			if n, _ := fmt.Sscanf(strings.TrimSuffix(rest, "%"), "%d-%d", &lo, &hi); n == 2 {
				return lo, hi
			}
		}
	}
	return 0, max
}

// treeRows lists the visible lines: headings, special toggles, the two
// range steppers and the filter tree (game.StockpileFilterRows, pre-order)
// with the children of a collapsed category left out.
func treeRows(z game.ZoneView, open map[string]bool) []treeRow {
	rows := []treeRow{{id: "storage.h.special", label: "Special filters", head: true, special: -1}}
	for i, s := range game.StockpileSpecials() {
		rows = append(rows, treeRow{id: "storage.special." + s, label: s, special: i, on: storageHas(z.Special, s)})
	}
	hlo, hhi := rangeOf(z.Special, "hp", hpMax)
	qlo, qhi := rangeOf(z.Special, "quality", qualityMax)
	rows = append(rows,
		treeRow{id: "storage.special.hp", label: "Hit points %", special: -1, rng: specialHP, lo: hlo, hi: hhi},
		treeRow{id: "storage.special.quality", label: "Quality", special: -1, rng: specialQuality, lo: qlo, hi: qhi},
		treeRow{id: "storage.h.items", label: "Allowed items", head: true, special: -1})
	hide := -1 // depth of the collapsed category whose children are skipped
	for _, r := range game.StockpileFilterRows(z.Filter) {
		if hide >= 0 {
			if r.Depth > hide {
				continue
			}
			hide = -1
		}
		rows = append(rows, treeRow{id: "storage.filter." + r.Path, label: r.Label, path: r.Path, special: -1,
			depth: r.Depth, leaf: r.Leaf, on: r.On, partial: r.Partial})
		if !r.Leaf && !open[r.Path] {
			hide = r.Depth
		}
	}
	return rows
}

// tree declares the scrolling tree. Only rows wholly inside the view are
// declared, so a checkbox never draws over the edge.
func (w Storage) tree(r gfx.Rect, z game.ZoneView, open map[string]bool) {
	rows := treeRows(z, open)
	area := w.kit().ScrollArea("storage.list", r, float32(len(rows))*storageRowH, uikit.ScrollOpts{Step: storageRowH * 3, Focusable: true})
	for i, row := range rows {
		y := area.View.Y + float32(i)*storageRowH - area.Off
		if y < area.View.Y || y+storageRowH > area.View.Bottom() {
			continue
		}
		x := area.View.X + float32(row.depth*storageIndent)
		if row.head {
			w.kit().Label(x+4, y+4, row.label, uikit.DimText())
			continue
		}
		if row.rng != 0 {
			w.rangeRow(x, y, area.View.W, z, row)
			continue
		}
		if !row.leaf && row.special < 0 {
			tw := gfx.Rect{X: x, Y: y, W: 22, H: storageRowH}
			mark := "+"
			if open[row.path] {
				mark = "-"
			}
			if w.kit().Button("storage.expand."+row.path, tw, mark, uikit.BtnOpts{Tip: "Show or hide the items in " + row.label}) {
				open[row.path] = !open[row.path]
			}
			x += 26
		}
		on, label := row.on, row.label
		if row.partial {
			label += "  (some)"
		}
		cr := gfx.Rect{X: x, Y: y, W: area.View.Right() - x, H: storageRowH}
		if w.kit().Check(row.id, cr, label, &on, uikit.CheckOpts{}) {
			w.sendNode(z.ID, row, !row.on) // the view still holds the old state this frame
		}
	}
}

// sendNode sends the allow or forbid for one tree line.
func (w Storage) sendNode(zid uint16, row treeRow, on bool) {
	if row.special >= 0 {
		w.sh.Enqueue(command.Command{Kind: command.SetStockpileSpecial, Target: uint64(zid), Index: row.special, Enabled: on})
		return
	}
	w.sh.Enqueue(command.Command{Kind: command.SetStockpileFilter, Target: uint64(zid), Name: row.path, Enabled: on})
}

// rangeRow declares a range line: the label and a - / + pair for each end.
// A click sends SetStockpileSpecial with the new X..X2.
func (w Storage) rangeRow(x, y, width float32, z game.ZoneView, row treeRow) {
	w.kit().Label(x+4, y+4, fmt.Sprintf("%s  %d-%d", row.label, row.lo, row.hi), uikit.DimText())
	step, max := 1, qualityMax
	if row.rng == specialHP {
		step, max = hpStep, hpMax
	}
	bx := x + width - 4*24
	for i, d := range []struct {
		id, mark string
		end, by  int
	}{{"min.dec", "-", 0, -step}, {"min.inc", "+", 0, step}, {"max.dec", "-", 1, -step}, {"max.inc", "+", 1, step}} {
		lo, hi := row.lo, row.hi
		if d.end == 0 {
			lo += d.by
		} else {
			hi += d.by
		}
		ok := lo >= 0 && hi <= max && lo <= hi
		r := gfx.Rect{X: bx + float32(i)*24, Y: y, W: 22, H: storageRowH}
		if w.kit().Button(row.id+"."+d.id, r, d.mark, uikit.BtnOpts{Disabled: !ok, Reason: "out of range"}) {
			w.sh.Enqueue(command.Command{Kind: command.SetStockpileSpecial, Target: uint64(z.ID), Index: row.rng, X: lo, Y: 0, X2: hi})
		}
	}
}
