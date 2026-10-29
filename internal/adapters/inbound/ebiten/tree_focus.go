package ebiten

// treeFocusMaxGhost caps how many Architect ghost tiles reveal canopies,
// so a huge drag stays cheap (treeCanopyTarget loops over the focus).
const treeFocusMaxGhost = 64

// treeFocusCells appends to dst the tiles whose canopies turn see-through
// besides creatures (ART §9.2): the hovered tile, the selected things'
// tiles and the Architect ghost (capped).
func treeFocusCells(dst [][2]int, hover [2]int, hovered bool, selected [][2]int, ghost []ghostCell) [][2]int {
	if hovered {
		dst = append(dst, hover)
	}
	dst = append(dst, selected...)
	for i, c := range ghost {
		if i == treeFocusMaxGhost {
			break
		}
		dst = append(dst, [2]int{c.x, c.y})
	}
	return dst
}

// syncTreeFocus hands the renderer this frame's tree focus: the tile under
// the cursor (off the HUD), the selected pawn or animal and the Architect
// ghost.
func (a *App) syncTreeFocus() {
	r := a.renderer
	if r == nil {
		return
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	var hover [2]int
	hovered := !a.chrome.Hidden && !a.colonyChromeHit(mx, my, sw, sh)
	if hovered {
		hover[0], hover[1] = r.TileAt(a.mapCursor()) // the map is physical
	}
	a.treeSel = a.treeSel[:0]
	if r.hasSelect {
		for _, p := range r.game.Snapshot() {
			if p.ID == r.selected {
				a.treeSel = append(a.treeSel, [2]int{p.X, p.Y})
			}
		}
		an := r.world().Animals
		for i := range an {
			if an[i].ID == r.selected {
				a.treeSel = append(a.treeSel, [2]int{an[i].X, an[i].Y})
			}
		}
	}
	var ghost []ghostCell
	if a.arch.ToolID != "" {
		_, ghost = a.previewCells()
	}
	a.treeFocus = treeFocusCells(a.treeFocus[:0], hover, hovered, a.treeSel, ghost)
	r.SetTreeFocus(a.treeFocus)
}
