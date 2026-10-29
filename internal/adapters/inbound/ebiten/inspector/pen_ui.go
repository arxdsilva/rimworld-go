package inspector

import (
	"fmt"
	"sort"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Pen marker (PLAY-015 §2.3, PLAY-018): the pane shows the pen's status
// and Food (grazing against the herd's demand); the Pen tab above it holds
// Animals (which penned species and sexes the pen takes) and Auto-cut
// (plant species designated for cutting inside). Reads WorldView.Pens;
// every edit emits SetPenSettings with the whole settings.

const (
	penMarkerKind = "pen marker"
	penToolID     = "pen_marker"
	penRowH       = 22
	penBtnH       = 20
	penCutCols    = 2
)

// penAutoCutDefaults are the plant species offered even when none stands
// in the pen yet: trees, cacti and brambles.
var penAutoCutDefaults = []string{"tree", "pine", "jungle_tree", "cactus", "bramble"}

func isPenMarker(b game.BuildingView) bool { return b.Kind == penMarkerKind }

// penOf is the PenView of marker b.
func (a Building) PenOf(b game.BuildingView) (game.PenView, bool) {
	for _, p := range a.shell.World().Pens {
		if p.ID == b.ID {
			return p, true
		}
	}
	return game.PenView{}, false
}

// penSpeciesRow is one penned species present in the colony.
type penSpeciesRow struct {
	key, label string
	set        game.PenSpecies
}

// penSpeciesRows lists the penned species among the colony's tame animals
// (sorted by key) with their current acceptance; a species the pen has no
// entry for takes every animal.
func penSpeciesRows(w game.WorldView, pv game.PenView) []penSpeciesRow {
	byKey := map[string]*penSpeciesRow{}
	for _, an := range w.Animals {
		if !an.Tame || !an.Penned {
			continue
		}
		if _, ok := byKey[an.Species]; !ok {
			row := penSpeciesRow{key: an.Species, label: an.Label,
				set: game.PenSpecies{Key: an.Species, Allow: true, Male: true, Female: true}}
			for _, sp := range pv.Settings.Species {
				if sp.Key == an.Species {
					row.set = sp
				}
			}
			byKey[an.Species] = &row
		}
	}
	out := make([]penSpeciesRow, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// penCutKeys are the auto-cut checklist's plant species: the defaults,
// the wild plants standing inside the pen and the ones already ticked.
func penCutKeys(w game.WorldView, pv game.PenView) []string {
	seen := map[string]bool{}
	out := append([]string(nil), penAutoCutDefaults...)
	for _, k := range out {
		seen[k] = true
	}
	var extra []string
	add := func(k string) {
		if k != "" && !seen[k] {
			seen[k] = true
			extra = append(extra, k)
		}
	}
	if len(pv.Bits) > 0 && w.TerrainW > 0 {
		for _, p := range w.Plants {
			if p.Zone == "" && bitAt(pv.Bits, w.TerrainW, p.X, p.Y) {
				add(p.Species)
			}
		}
	}
	for _, k := range pv.Settings.AutoCut {
		add(k)
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// bitAt reports cell (x, y) is set in a row-major bitset w cells wide.
func bitAt(bits []uint64, w, x, y int) bool {
	if w <= 0 || x < 0 || y < 0 || x >= w {
		return false
	}
	n := y*w + x
	return n/64 < len(bits) && bits[n/64]&(1<<(n%64)) != 0
}

// penFoodLines are the Food section: grazing, demand, trough stock and the
// verdict.
func PenFoodLines(pv game.PenView) []string {
	verdict := "Enough grazing"
	if short := pv.DemandPerDay - pv.GrazePerDay; short > 0 {
		verdict = fmt.Sprintf("Short by %d per day: add hay or expand", short)
	}
	return []string{
		fmt.Sprintf("Grazing: %d per day", pv.GrazePerDay),
		fmt.Sprintf("Herd needs: %d per day", pv.DemandPerDay),
		fmt.Sprintf("Trough stock: %d", pv.TroughStock),
		verdict,
	}
}

// penStatusLine is the line under the title: the pen, its herd, and the
// "Not enclosed" warning for an open one.
func PenStatusLine(pv game.PenView) string {
	if !pv.Enclosed {
		return fmt.Sprintf("%s   Not enclosed: it holds nothing", pv.Name)
	}
	return fmt.Sprintf("%s   %d animals", pv.Name, pv.Animals)
}

// ---- the Pen tab ---------------------------------------------------------

const PenTabW = 400 // the Pen tab's panel

// penTabBody lays out the pen's settings, viewW wide: which penned species
// and sexes the pen takes (with All / None), and the auto-cut checklist.
// The status and food lines are the pane's.
func PenTabBody(w game.WorldView, pv game.PenView, viewW float32) TabBody {
	var body TabBody
	y := float32(0)
	body.Labels = append(body.Labels, tabLabel{Text: "Animals", y: y, st: gfx.Label})
	body.Buttons = append(body.Buttons,
		uikit.Button{ID: "bldg.pen.all", Label: "All", X: viewW - 118, Y: y, W: 56, H: penBtnH},
		uikit.Button{ID: "bldg.pen.none", Label: "None", X: viewW - 56, Y: y, W: 56, H: penBtnH})
	y += 24
	rows := penSpeciesRows(w, pv)
	for _, row := range rows {
		body.Buttons = append(body.Buttons,
			uikit.Button{ID: "bldg.pen.sp." + row.key, Label: row.label, X: 0, Y: y, W: 190, H: penBtnH, Selected: row.set.Allow},
			uikit.Button{ID: "bldg.pen.sp." + row.key + ".m", Label: "M", X: 200, Y: y, W: 44, H: penBtnH, Selected: row.set.Male,
				Tip: "Males\nTake male " + row.label + " animals."},
			uikit.Button{ID: "bldg.pen.sp." + row.key + ".f", Label: "F", X: 250, Y: y, W: 44, H: penBtnH, Selected: row.set.Female,
				Tip: "Females\nTake female " + row.label + " animals."})
		y += penRowH
	}
	if len(rows) == 0 {
		body.Labels = append(body.Labels, tabLabel{Text: "No penned animals in the colony yet.", x: 4, y: y + 2, w: viewW - 4, st: gfx.Body})
		y += penRowH
	}
	y += 10
	body.Labels = append(body.Labels, tabLabel{Text: "Auto-cut (every 2 hours, inside an enclosed pen)", y: y, w: viewW, st: gfx.Label})
	y += 24
	cut := map[string]bool{}
	for _, k := range pv.Settings.AutoCut {
		cut[k] = true
	}
	keys := penCutKeys(w, pv)
	for i, k := range keys {
		body.Buttons = append(body.Buttons, uikit.Button{ID: "bldg.pen.cut." + k, Label: penPlantLabel(k),
			X: float32(i%penCutCols) * 176, Y: y + float32(i/penCutCols*penRowH), W: 170, H: penBtnH, Selected: cut[k]})
	}
	body.H = y + float32((len(keys)+penCutCols-1)/penCutCols*penRowH)
	return body
}

// penTab is the Pen tab of pen marker b.
func (a Building) penTab(b game.BuildingView) Tab {
	id := b.ID
	h := float32(2 * PaneInset)
	if pv, ok := a.PenOf(b); ok {
		h += PenTabBody(a.shell.World(), pv, PenTabW-2*PaneInset-uikit.ScrollBarW).H
	}
	return Tab{ID: "inspector.pen", Label: "Pen", WantW: PenTabW, WantH: h,
		Declare: func(r gfx.Rect) { a.penTabUI(id, r) }}
}

// penTabUI declares the Pen tab of marker id in r, the expanded panel's
// inner rectangle.
func (a Building) penTabUI(id entity.EntityID, r gfx.Rect) {
	b, ok := a.BuildingView(id)
	if !ok || !isPenMarker(b) {
		return
	}
	pv, ok := a.PenOf(b)
	if !ok {
		return
	}
	a.declareTabBody("bldg.pen.body", r, PenTabBody(a.shell.World(), pv, r.W-uikit.ScrollBarW), a.ApplyBuildingClick)
}

// penPlantLabel is a plant species key as words ("jungle_tree").
func penPlantLabel(k string) string { return strings.ReplaceAll(k, "_", " ") }

// applyPenClick handles the pen marker inspector's bldg.pen.* ids.
func (a Building) applyPenClick(id string, b game.BuildingView) bool {
	rest, ok := strings.CutPrefix(id, "bldg.pen.")
	if !ok || !isPenMarker(b) {
		return false
	}
	pv, found := a.PenOf(b)
	if !found {
		return true
	}
	w := a.shell.World()
	rows := penSpeciesRows(w, pv)
	switch {
	case rest == "all", rest == "none":
		on := rest == "all"
		for i := range rows {
			rows[i].set.Allow, rows[i].set.Male, rows[i].set.Female = on, on, on
		}
	case strings.HasPrefix(rest, "cut."):
		a.emitPenSettings(pv, rows, toggleKey(pv.Settings.AutoCut, strings.TrimPrefix(rest, "cut.")))
		return true
	case strings.HasPrefix(rest, "sp."):
		key, sex, _ := strings.Cut(strings.TrimPrefix(rest, "sp."), ".")
		for i := range rows {
			if rows[i].key != key {
				continue
			}
			switch sex {
			case "":
				rows[i].set.Allow = !rows[i].set.Allow
			case "m":
				rows[i].set.Male = !rows[i].set.Male
			case "f":
				rows[i].set.Female = !rows[i].set.Female
			}
		}
	}
	a.emitPenSettings(pv, rows, pv.Settings.AutoCut)
	return true
}

// toggleKey returns keys without k when it is there, else with k last.
func toggleKey(keys []string, k string) []string {
	out := make([]string, 0, len(keys)+1)
	found := false
	for _, x := range keys {
		if x == k {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		out = append(out, k)
	}
	return out
}

// emitPenSettings sends the pen's whole settings: the entries it already
// had (kept in order), with the listed species rows written explicitly,
// and the auto-cut list.
func (a Building) emitPenSettings(pv game.PenView, rows []penSpeciesRow, autoCut []string) {
	ps := game.PenSettings{AutoCut: append([]string{}, autoCut...)}
	done := map[string]bool{}
	for _, sp := range pv.Settings.Species {
		for _, r := range rows {
			if r.key == sp.Key {
				sp = r.set
				done[r.key] = true
			}
		}
		ps.Species = append(ps.Species, sp)
	}
	for _, r := range rows {
		if !done[r.key] {
			ps.Species = append(ps.Species, r.set)
		}
	}
	a.shell.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(pv.ID), Name: game.EncodePenSettings(ps)})
}
