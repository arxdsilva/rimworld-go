package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/domain/scenario"
)

// Choose Scenario (NG-1 §2.2): a searchable list of the built-in and the
// custom scenarios on the left, the read-only details of the selected one
// on the right. Next stamps a copy of the scenario into the draft.

// Row heights of the list.
const (
	scenRowHeadH  = 28
	scenRowEntryH = 72
	scenRowNoteH  = 46
	scenSearchH   = 30
	scenButtonsH  = 34
	scenIconSize  = 40
)

// List row kinds.
const (
	rowHeader = iota
	rowEntry
	rowNote
)

// scenRow is one row of the scenario list.
type scenRow struct {
	Kind  int
	Entry int // index into entries (rowEntry)
	Text  string
	y, h  float32
}

// matches reports the entry passes the search text.
func (e scenEntry) matches(q string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(strings.ToLower(e.Summary), q)
}

// Rows lays the list out: the "Built-in" group, then "Your scenarios".
func (ng *newGameState) Rows(storeOK bool) ([]scenRow, float32) {
	q := strings.ToLower(strings.TrimSpace(ng.Search))
	var out []scenRow
	y := float32(0)
	add := func(r scenRow, h float32) {
		r.y, r.h = y, h
		out = append(out, r)
		y += h
	}
	add(scenRow{Kind: rowHeader, Text: "Built-in"}, scenRowHeadH)
	for i, e := range ng.Entries {
		if e.builtin && e.matches(q) {
			add(scenRow{Kind: rowEntry, Entry: i}, scenRowEntryH)
		}
	}
	add(scenRow{Kind: rowHeader, Text: "Your scenarios"}, scenRowHeadH)
	n := 0
	for i, e := range ng.Entries {
		if !e.builtin && e.matches(q) {
			add(scenRow{Kind: rowEntry, Entry: i}, scenRowEntryH)
			n++
		}
	}
	if n == 0 {
		note := "No saved scenarios yet — use New scenario"
		switch {
		case q != "":
			note = "No scenario matches the search"
		case !storeOK:
			note = "Saving scenarios is unavailable here; new scenarios last until you quit"
		}
		add(scenRow{Kind: rowNote, Text: note}, scenRowNoteH)
	}
	return out, y
}

// scenarioLayout is the columns of Choose Scenario.
type scenarioLayout struct {
	Left, Right, search, List, Buttons rect
}

// scenarioRects places the screen's parts in the dialog's content.
func scenarioRects(content rect) scenarioLayout {
	leftW := float32(int(content.W * 0.38))
	L := scenarioLayout{
		Left:  rect{X: content.X, Y: content.Y, W: leftW - 8, H: content.H},
		Right: rect{X: content.X + leftW + 8, Y: content.Y, W: content.W - leftW - 8, H: content.H},
	}
	L.search = rect{X: L.Left.X, Y: L.Left.Y, W: L.Left.W, H: scenSearchH}
	L.Buttons = rect{X: L.Left.X, Y: L.Left.Bottom() - scenButtonsH, W: L.Left.W, H: scenButtonsH}
	L.List = rect{X: L.Left.X, Y: L.search.Bottom() + 8, W: L.Left.W, H: L.Buttons.Y - 8 - (L.search.Bottom() + 8)}
	return L
}

func (a *App) drawScenario(screen *ebiten.Image, sw, sh int) {
	a.drawNGFrame(screen, sw, sh, "Choose Scenario")
}

// scenarioButtons: the screen's controls are kit widgets (layoutHits).
func (a *App) scenarioButtons(sw, sh int) []button { return nil }

func (a *App) scenarioUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	_, _, content, footer := dialogFrame(sw, sh)
	L := scenarioRects(content)
	u := a.uiSt()

	if a.in.Ctrl() && a.in.Key("F") {
		u.TabTo = "scen.search"
	}
	a.uiTextField("scen.search", L.search, &ng.Search, textFieldSpec{MaxLen: 40, Placeholder: "Search scenarios (Ctrl+F)"})
	a.scenarioList(ng, L.List)

	third := (L.Buttons.W - 12) / 3
	b := func(i int) rect {
		return rect{X: L.Buttons.X + float32(i)*(third+6), Y: L.Buttons.Y, W: third, H: L.Buttons.H}
	}
	sel := ng.Selected()
	a.ngButton("scen.new", b(0), "New scenario", btnOpts{Tip: "New scenario\nStart a fresh scenario in the editor."}, func() {
		a.openEditor(newScenarioDef(), "")
	})
	editOpts := btnOpts{Tip: "Edit a copy\nOpen a copy of the selected scenario in the editor. The original is never changed."}
	if sel == nil || sel.Err != "" {
		editOpts.Disabled, editOpts.Reason = true, "Select a readable scenario first."
	}
	a.ngButton("scen.edit", b(1), "Edit a copy", editOpts, func() {
		if s := ng.Selected(); s != nil {
			a.openEditor(copyForEditing(s.Def), s.Key)
		}
	})
	a.ngButton("scen.import", b(2), "Load file...", btnOpts{Disabled: a.cfg.Scenarios == nil, Reason: "Scenario files are unavailable in this build.", Tip: "Load file\nPick a scenario file from the scenarios folder."}, a.openScenarioFiles)

	a.uiPanel(L.Right)
	if sel != nil {
		if sel.Err != "" {
			a.scenarioUnreadable(L.Right, sel)
		} else {
			a.detailView("scen.details", L.Right, sel.Def, sel.lines)
		}
	}

	next := btnOpts{}
	switch {
	case sel == nil:
		next.Disabled, next.Reason = true, "Select a scenario first."
	case sel.Err != "":
		next.Disabled, next.Reason = true, "Can't read this file ("+sel.Err+")"
	case sel.errCount() > 0:
		next.Disabled = true
		next.Reason = fmt.Sprintf("Fix %d problem%s in the editor", sel.errCount(), plural(sel.errCount()))
	}
	a.ngFooter(footer, "Next", ids.WizardNext, next, func() {
		if s := ng.Selected(); s != nil && s.Err == "" {
			ng.Scenario = cloneDef(s.Def)
			a.forward()
		}
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// cloneDef copies a scenario so the draft never aliases a list entry.
func cloneDef(d scenario.Def) scenario.Def {
	d.Parts = append([]scenario.Part(nil), d.Parts...)
	return d
}

// scenarioUnreadable is the right column for a file that can't be read.
func (a *App) scenarioUnreadable(r rect, e *scenEntry) {
	in := r.Inset(14)
	hs := tsTitle
	hs.Color = uiDanger
	a.uiLabel(in.X, in.Y, "Can't read this file", hs)
	y := in.Y + float32(lineH(hs)) + 8
	a.textLines(in.X, y, in.W, e.File+".json: "+e.Err, dimText())
}

// scenarioList declares the list (groups, rows, delete buttons) and the
// keyboard selection.
func (a *App) scenarioList(ng *newGameState, r rect) {
	u := a.uiSt()
	rows, total := ng.Rows(a.cfg.Scenarios != nil)
	a.uiPanel(r)
	if u.Focus == "scen.list" && !a.uiModalOpen() {
		step := 0
		switch {
		case a.keyRepeat("ArrowUp"):
			step = -1
		case a.keyRepeat("ArrowDown"):
			step = 1
		}
		if step != 0 {
			a.moveScenarioSel(ng, rows, step, r.H)
		}
	}
	area := a.uiScrollArea("scen.list", r, total, scrollOpts{Step: 72, Focusable: true})

	type vis struct {
		row scenRow
		y   float32
	}
	var shown []vis
	for _, row := range rows {
		y := area.View.Y + row.y - area.Off
		if y+row.h < area.View.Y || y > area.View.Bottom() {
			continue
		}
		shown = append(shown, vis{row, y})
	}
	selKey := ng.selectedKey()
	entries := ng.Entries
	u.Queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for _, v := range shown {
			a.paintScenarioRow(sub, area.View, v.row, v.y, entries, selKey)
		}
	})
	for _, v := range shown {
		if v.row.Kind != rowEntry {
			continue
		}
		e := entries[v.row.Entry]
		row := rect{X: area.View.X, Y: v.y, W: area.View.W, H: v.row.h}
		if e.deletable() {
			del := rect{X: row.Right() - 30, Y: row.Bottom() - 28, W: 22, H: 22}
			if del.Y >= area.View.Y && del.Bottom() <= area.View.Bottom() {
				key := e.Key
				a.ngButton("scen.del."+key, del, "x", btnOpts{Style: btnDanger, Tip: "Delete\nRemove this scenario."}, func() { a.confirmDeleteScenario(key) })
			}
		}
		idx := v.row.Entry
		hit := row.Intersect(area.View)
		w := uiWidget{ID: "scen.row." + e.Key, R: hit, Label: e.Name}
		pick := func() {
			u.Focus = "scen.list"
			if entries[idx].Err != "" {
				a.uiDeny("Can't read this file (" + entries[idx].Err + ")")
				return
			}
			ng.Sel = idx
			a.uiOK()
		}
		w.Click = pick
		if e.Err != "" {
			w.Tip = "Can't read this file\n" + e.Err
		}
		if u.Add(w) && a.uiPress(hit) {
			pick()
		}
	}
}

// moveScenarioSel moves the highlight to the next selectable row.
func (a *App) moveScenarioSel(ng *newGameState, rows []scenRow, step int, viewH float32) {
	cur := -1
	for i, r := range rows {
		if r.Kind == rowEntry && r.Entry == ng.Sel {
			cur = i
		}
	}
	for i := cur + step; i >= 0 && i < len(rows); i += step {
		if rows[i].Kind == rowEntry && ng.Entries[rows[i].Entry].Err == "" {
			ng.Sel = rows[i].Entry
			a.uiScrollReveal("scen.list", viewH, rows[i].y, rows[i].y+rows[i].h)
			return
		}
	}
}

// paintScenarioRow draws one row of the list into dst (clipped to view).
func (a *App) paintScenarioRow(dst *ebiten.Image, view rect, row scenRow, y float32, entries []scenEntry, selKey string) {
	u := a.uiSt()
	r := rect{X: view.X, Y: y, W: view.W, H: row.h}
	switch row.Kind {
	case rowHeader:
		st := tsLabel
		st.Color = uiHeading
		drawText(dst, row.Text, float64(r.X+10), float64(r.Y)+(float64(r.H)-lineH(st))/2+2, st)
		fillRect(dst, rect{X: r.X + 8, Y: r.Bottom() - 3, W: r.W - 16, H: 1}, uiPanelBorder)
	case rowNote:
		st := dimText()
		lines := wrapPx(row.Text, st, float64(r.W-24))
		for i, l := range lines {
			drawText(dst, l, float64(r.X+12), float64(r.Y)+8+float64(i)*lineH(st), st)
		}
	case rowEntry:
		e := entries[row.Entry]
		hover := u.Hot == "scen.row."+e.Key
		switch {
		case e.Key == selKey:
			fillRect(dst, r.Inset(2), uiBtnSelected)
			strokeRect(dst, r.Inset(2), uiAccent)
		case hover:
			fillRect(dst, r.Inset(2), uiBtnHover)
		}
		ix, iy := r.X+10, r.Y+(r.H-scenIconSize)/2
		iconKey := "ui_icon/scenario_custom"
		if e.builtin {
			iconKey = "ui_icon/scenario_" + e.Def.Key
		}
		if img := a.art.Image(iconKey, noTint); img != nil {
			drawIcon(dst, img, float64(ix), float64(iy), scenIconSize)
		} else {
			fillRect(dst, rect{X: ix, Y: iy, W: scenIconSize, H: scenIconSize}, uiField)
			strokeRect(dst, rect{X: ix, Y: iy, W: scenIconSize, H: scenIconSize}, uiPanelBorder)
		}
		tx := r.X + scenIconSize + 22
		pipsW := float32(4*10 + 3*3)
		nameW := r.W - (tx - r.X) - pipsW - 18
		name := tsLabel
		if e.Err != "" {
			name.Color = uiTextDisabled
		}
		drawText(dst, fitText(e.Name, name, nameW), float64(tx), float64(r.Y)+8, name)
		sum := tsSmall
		sum.Color = uiTextDim
		if e.Err != "" {
			sum.Color = uiTextDisabled
		}
		if errs := e.errCount(); errs > 0 && e.Err == "" {
			sum.Color = uiWarning
			drawText(dst, fmt.Sprintf("%d problem%s to fix", errs, plural(errs)), float64(tx), float64(r.Y)+30, sum)
		} else {
			lines := wrapPx(e.Summary, sum, float64(r.W-(tx-r.X)-14))
			for i := 0; i < len(lines) && i < 2; i++ {
				l := lines[i]
				if i == 1 && len(lines) > 2 {
					l = ellipsize(l+" "+lines[2], sum, float64(r.W-(tx-r.X)-14))
				}
				drawText(dst, l, float64(tx), float64(r.Y)+30+float64(i)*lineH(sum), sum)
			}
		}
		if e.Err == "" {
			px := r.Right() - 10 - pipsW
			for i := 0; i < 4; i++ {
				c := uiSliderTrack
				if i < e.hardness {
					c = uiAccent
				}
				uiFillRect(dst, px+float32(i)*13, r.Y+10, 10, 10, c, false)
			}
		}
	}
}

// ---- delete, load ------------------------------------------------------

// confirmDeleteScenario asks before removing a custom scenario.
func (a *App) confirmDeleteScenario(key string) {
	ng := a.draft()
	name := key
	for _, e := range ng.Entries {
		if e.Key == key {
			name = e.Name
		}
	}
	a.uiConfirm("scen.delete", "Delete scenario?", fmt.Sprintf("Delete \"%s\"? This can't be undone.", name), "Delete", true, func() { a.deleteScenario(key) })
}

// deleteScenario removes a custom scenario: its file, or the unsaved copy.
func (a *App) deleteScenario(key string) {
	ng := a.draft()
	for _, e := range ng.Entries {
		if e.Key != key {
			continue
		}
		switch {
		case e.Unsaved:
			var idx int
			if _, err := fmt.Sscanf(e.Key, "unsaved.%d", &idx); err == nil && idx >= 0 && idx < len(a.ngUnsaved) {
				a.ngUnsaved = append(a.ngUnsaved[:idx], a.ngUnsaved[idx+1:]...)
			}
		case e.File != "" && a.cfg.Scenarios != nil:
			if err := a.cfg.Scenarios.Delete(e.File); err != nil {
				a.hint = "delete failed: " + err.Error()
			}
		}
	}
	ng.RefreshScenarios(a)
}

// openScenarioFiles lists the files of the scenarios folder; picking one
// selects it in the list.
func (a *App) openScenarioFiles() {
	if a.cfg.Scenarios == nil {
		return
	}
	files, err := a.cfg.Scenarios.List()
	msg := ""
	if err != nil {
		msg = "Can't read the scenarios folder: " + err.Error()
	}
	cols := []tableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return files[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(files[x].Name) < strings.ToLower(files[y].Name) }},
		{ID: "mod", Title: "Modified", W: 150, Cell: func(i int) string { return files[i].Modified },
			Less: func(x, y int) bool { return files[x].Modified < files[y].Modified }},
	}
	a.uiPushModal(modal{
		ID: "scen.load", Title: "Load scenario file", W: 560, H: 420,
		Draw: func(a *App, body rect) {
			tb := body
			if msg != "" || len(files) == 0 {
				note := msg
				if note == "" {
					note = "No scenario files yet. Save one from the editor."
				}
				a.textLines(body.X, body.Y, body.W, note, dimText())
				return
			}
			row := -1
			if a.uiTable("modal.scen.load.table", tb, cols, len(files), &row, 28) && row >= 0 {
				name := files[row].Name
				a.uiCloseModal("scen.load")
				ng := a.draft()
				ng.Search = ""
				ng.RefreshScenarios(a)
				ng.selectKey("file." + name)
			}
		},
		Buttons: []modalButton{{ID: "cancel", Label: "Cancel", Closes: true}},
	})
}
