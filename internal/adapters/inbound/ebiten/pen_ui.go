package ebiten

import (
	"fmt"
	"sort"
	"strings"

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
func (a *App) penOf(b game.BuildingView) (game.PenView, bool) {
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
func penFoodLines(pv game.PenView) []string {
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
func penStatusLine(pv game.PenView) string {
	if !pv.Enclosed {
		return fmt.Sprintf("%s   Not enclosed: it holds nothing", pv.Name)
	}
	return fmt.Sprintf("%s   %d animals", pv.Name, pv.Animals)
}

// ---- the Pen tab ---------------------------------------------------------

const penTabW = 400 // the Pen tab's panel

// penTabBody lays out the pen's settings, viewW wide: which penned species
// and sexes the pen takes (with All / None), and the auto-cut checklist.
// The status and food lines are the pane's.
func penTabBody(w game.WorldView, pv game.PenView, viewW float32) tabBody {
	var body tabBody
	y := float32(0)
	body.labels = append(body.labels, tabLabel{text: "Animals", y: y, st: tsLabel})
	body.buttons = append(body.buttons,
		button{id: "bldg.pen.all", label: "All", x: viewW - 118, y: y, w: 56, h: penBtnH},
		button{id: "bldg.pen.none", label: "None", x: viewW - 56, y: y, w: 56, h: penBtnH})
	y += 24
	rows := penSpeciesRows(w, pv)
	for _, row := range rows {
		body.buttons = append(body.buttons,
			button{id: "bldg.pen.sp." + row.key, label: row.label, x: 0, y: y, w: 190, h: penBtnH, selected: row.set.Allow},
			button{id: "bldg.pen.sp." + row.key + ".m", label: "M", x: 200, y: y, w: 44, h: penBtnH, selected: row.set.Male,
				tip: "Males\nTake male " + row.label + " animals."},
			button{id: "bldg.pen.sp." + row.key + ".f", label: "F", x: 250, y: y, w: 44, h: penBtnH, selected: row.set.Female,
				tip: "Females\nTake female " + row.label + " animals."})
		y += penRowH
	}
	if len(rows) == 0 {
		body.labels = append(body.labels, tabLabel{text: "No penned animals in the colony yet.", x: 4, y: y + 2, w: viewW - 4, st: tsBody})
		y += penRowH
	}
	y += 10
	body.labels = append(body.labels, tabLabel{text: "Auto-cut (every 2 hours, inside an enclosed pen)", y: y, w: viewW, st: tsLabel})
	y += 24
	cut := map[string]bool{}
	for _, k := range pv.Settings.AutoCut {
		cut[k] = true
	}
	keys := penCutKeys(w, pv)
	for i, k := range keys {
		body.buttons = append(body.buttons, button{id: "bldg.pen.cut." + k, label: penPlantLabel(k),
			x: float32(i%penCutCols) * 176, y: y + float32(i/penCutCols*penRowH), w: 170, h: penBtnH, selected: cut[k]})
	}
	body.h = y + float32((len(keys)+penCutCols-1)/penCutCols*penRowH)
	return body
}

// penTab is the Pen tab of pen marker b.
func (a *App) penTab(b game.BuildingView) paneTab {
	id := b.ID
	h := float32(2 * paneInset)
	if pv, ok := a.penOf(b); ok {
		h += penTabBody(a.shell.World(), pv, penTabW-2*paneInset-scrollBarW).h
	}
	return paneTab{ID: "inspector.pen", Label: "Pen", WantW: penTabW, WantH: h,
		Declare: func(r rect) { a.penTabUI(id, r) }}
}

// penTabUI declares the Pen tab of marker id in r, the expanded panel's
// inner rectangle.
func (a *App) penTabUI(id entity.EntityID, r rect) {
	b, ok := a.buildingView(id)
	if !ok || !isPenMarker(b) {
		return
	}
	pv, ok := a.penOf(b)
	if !ok {
		return
	}
	a.declareTabBody("bldg.pen.body", r, penTabBody(a.shell.World(), pv, r.W-scrollBarW), a.applyBuildingClick)
}

// penPlantLabel is a plant species key as words ("jungle_tree").
func penPlantLabel(k string) string { return strings.ReplaceAll(k, "_", " ") }

// applyPenClick handles the pen marker inspector's bldg.pen.* ids.
func (a *App) applyPenClick(id string, b game.BuildingView) bool {
	rest, ok := strings.CutPrefix(id, "bldg.pen.")
	if !ok || !isPenMarker(b) {
		return false
	}
	pv, found := a.penOf(b)
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
func (a *App) emitPenSettings(pv game.PenView, rows []penSpeciesRow, autoCut []string) {
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
