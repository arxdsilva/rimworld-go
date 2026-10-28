package wizard

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	RowHeader = iota
	RowEntry
	RowNote
)

// scenRow is one row of the scenario list.
type scenRow struct {
	Kind  int
	Entry int // index into entries (rowEntry)
	Text  string
	y, h  float32
}

// matches reports the entry passes the search text.
func (e ScenEntry) matches(q string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(strings.ToLower(e.Summary), q)
}

// Rows lays the list out: the "Built-in" group, then "Your scenarios".
func (ng *Draft) Rows(storeOK bool) ([]scenRow, float32) {
	q := strings.ToLower(strings.TrimSpace(ng.Search))
	var out []scenRow
	y := float32(0)
	add := func(r scenRow, h float32) {
		r.y, r.h = y, h
		out = append(out, r)
		y += h
	}
	add(scenRow{Kind: RowHeader, Text: "Built-in"}, scenRowHeadH)
	for i, e := range ng.Entries {
		if e.builtin && e.matches(q) {
			add(scenRow{Kind: RowEntry, Entry: i}, scenRowEntryH)
		}
	}
	add(scenRow{Kind: RowHeader, Text: "Your scenarios"}, scenRowHeadH)
	n := 0
	for i, e := range ng.Entries {
		if !e.builtin && e.matches(q) {
			add(scenRow{Kind: RowEntry, Entry: i}, scenRowEntryH)
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
		add(scenRow{Kind: RowNote, Text: note}, scenRowNoteH)
	}
	return out, y
}

// scenarioLayout is the columns of Choose Scenario.
type scenarioLayout struct {
	Left, Right, search, List, Buttons gfx.Rect
}

// ScenarioRects places the screen's parts in the dialog's content.
func ScenarioRects(content gfx.Rect) scenarioLayout {
	leftW := float32(int(content.W * 0.38))
	L := scenarioLayout{
		Left:  gfx.Rect{X: content.X, Y: content.Y, W: leftW - 8, H: content.H},
		Right: gfx.Rect{X: content.X + leftW + 8, Y: content.Y, W: content.W - leftW - 8, H: content.H},
	}
	L.search = gfx.Rect{X: L.Left.X, Y: L.Left.Y, W: L.Left.W, H: scenSearchH}
	L.Buttons = gfx.Rect{X: L.Left.X, Y: L.Left.Bottom() - scenButtonsH, W: L.Left.W, H: scenButtonsH}
	L.List = gfx.Rect{X: L.Left.X, Y: L.search.Bottom() + 8, W: L.Left.W, H: L.Buttons.Y - 8 - (L.search.Bottom() + 8)}
	return L
}

func (w scenarioPage) drawScenario(screen *ebiten.Image, sw, sh int) {
	w.drawNGFrame(screen, sw, sh, "Choose Scenario")
}

func (w scenarioPage) scenarioUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	_, _, content, footer := uikit.DialogFrame(sw, sh)
	L := ScenarioRects(content)
	u := w.kit()

	if w.in().Ctrl() && w.in().Key("F") {
		u.TabTo = "scen.search"
	}
	w.kit().TextField("scen.search", L.search, &ng.Search, uikit.TextFieldSpec{MaxLen: 40, Placeholder: "Search scenarios (Ctrl+F)"})
	w.scenarioList(ng, L.List)

	third := (L.Buttons.W - 12) / 3
	b := func(i int) gfx.Rect {
		return gfx.Rect{X: L.Buttons.X + float32(i)*(third+6), Y: L.Buttons.Y, W: third, H: L.Buttons.H}
	}
	sel := ng.Selected()
	uikit.StyledButton(w.kit(), "scen.new", b(0), "New scenario", uikit.BtnOpts{Tip: "New scenario\nStart a fresh scenario in the editor."}, func() {
		editorPage{w.Flow}.openEditor(newScenarioDef(), "")
	})
	editOpts := uikit.BtnOpts{Tip: "Edit a copy\nOpen a copy of the selected scenario in the editor. The original is never changed."}
	if sel == nil || sel.Err != "" {
		editOpts.Disabled, editOpts.Reason = true, "Select a readable scenario first."
	}
	uikit.StyledButton(w.kit(), "scen.edit", b(1), "Edit a copy", editOpts, func() {
		if s := ng.Selected(); s != nil {
			editorPage{w.Flow}.openEditor(CopyForEditing(s.Def), s.Key)
		}
	})
	uikit.StyledButton(w.kit(), "scen.import", b(2), "Load file...", uikit.BtnOpts{Disabled: w.store() == nil, Reason: "Scenario files are unavailable in this build.", Tip: "Load file\nPick a scenario file from the scenarios folder."}, w.openScenarioFiles)

	w.kit().Panel(L.Right)
	if sel != nil {
		if sel.Err != "" {
			w.scenarioUnreadable(L.Right, sel)
		} else {
			w.detailView("scen.details", L.Right, sel.Def, sel.lines)
		}
	}

	next := uikit.BtnOpts{}
	switch {
	case sel == nil:
		next.Disabled, next.Reason = true, "Select a scenario first."
	case sel.Err != "":
		next.Disabled, next.Reason = true, "Can't read this file ("+sel.Err+")"
	case sel.errCount() > 0:
		next.Disabled = true
		next.Reason = fmt.Sprintf("Fix %d problem%s in the editor", sel.errCount(), plural(sel.errCount()))
	}
	w.ngFooter(footer, "Next", ids.WizardNext, next, func() {
		if s := ng.Selected(); s != nil && s.Err == "" {
			ng.Scenario = CloneDef(s.Def)
			w.forward()
		}
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// CloneDef copies a scenario so the draft never aliases a list entry.
func CloneDef(d scenario.Def) scenario.Def {
	d.Parts = append([]scenario.Part(nil), d.Parts...)
	return d
}

// scenarioUnreadable is the right column for a file that can't be read.
func (w scenarioPage) scenarioUnreadable(r gfx.Rect, e *ScenEntry) {
	in := r.Inset(14)
	hs := gfx.Title
	hs.Color = uikit.Danger
	w.kit().Label(in.X, in.Y, "Can't read this file", hs)
	y := in.Y + float32(gfx.LineH(hs)) + 8
	uikit.TextLines(w.kit(), in.X, y, in.W, e.File+".json: "+e.Err, uikit.DimText())
}

// scenarioList declares the list (groups, rows, delete buttons) and the
// keyboard selection.
func (w scenarioPage) scenarioList(ng *Draft, r gfx.Rect) {
	u := w.kit()
	rows, total := ng.Rows(w.store() != nil)
	w.kit().Panel(r)
	if u.Focus == "scen.list" && !w.kit().ModalOpen() {
		step := 0
		switch {
		case w.kit().Host().KeyRepeat("ArrowUp"):
			step = -1
		case w.kit().Host().KeyRepeat("ArrowDown"):
			step = 1
		}
		if step != 0 {
			w.moveScenarioSel(ng, rows, step, r.H)
		}
	}
	area := w.kit().ScrollArea("scen.list", r, total, uikit.ScrollOpts{Step: 72, Focusable: true})

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
		sub := gfx.ClipTo(dst, area.View)
		for _, v := range shown {
			w.paintScenarioRow(sub, area.View, v.row, v.y, entries, selKey)
		}
	})
	for _, v := range shown {
		if v.row.Kind != RowEntry {
			continue
		}
		e := entries[v.row.Entry]
		row := gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: v.row.h}
		if e.deletable() {
			del := gfx.Rect{X: row.Right() - 30, Y: row.Bottom() - 28, W: 22, H: 22}
			if del.Y >= area.View.Y && del.Bottom() <= area.View.Bottom() {
				key := e.Key
				uikit.StyledButton(w.kit(), "scen.del."+key, del, "x", uikit.BtnOpts{Style: uikit.BtnDanger, Tip: "Delete\nRemove this scenario."}, func() { w.confirmDeleteScenario(key) })
			}
		}
		idx := v.row.Entry
		hit := row.Intersect(area.View)
		wd := uikit.Widget{ID: "scen.row." + e.Key, R: hit, Label: e.Name}
		pick := func() {
			u.Focus = "scen.list"
			if entries[idx].Err != "" {
				w.kit().Deny("Can't read this file (" + entries[idx].Err + ")")
				return
			}
			ng.Sel = idx
			w.kit().OK()
		}
		wd.Click = pick
		if e.Err != "" {
			wd.Tip = "Can't read this file\n" + e.Err
		}
		if u.Add(wd) && w.kit().Press(hit) {
			pick()
		}
	}
}

// moveScenarioSel moves the highlight to the next selectable row.
func (w scenarioPage) moveScenarioSel(ng *Draft, rows []scenRow, step int, viewH float32) {
	cur := -1
	for i, r := range rows {
		if r.Kind == RowEntry && r.Entry == ng.Sel {
			cur = i
		}
	}
	for i := cur + step; i >= 0 && i < len(rows); i += step {
		if rows[i].Kind == RowEntry && ng.Entries[rows[i].Entry].Err == "" {
			ng.Sel = rows[i].Entry
			w.kit().ScrollReveal("scen.list", viewH, rows[i].y, rows[i].y+rows[i].h)
			return
		}
	}
}

// paintScenarioRow draws one row of the list into dst (clipped to view).
func (w scenarioPage) paintScenarioRow(dst *ebiten.Image, view gfx.Rect, row scenRow, y float32, entries []ScenEntry, selKey string) {
	u := w.kit()
	r := gfx.Rect{X: view.X, Y: y, W: view.W, H: row.h}
	switch row.Kind {
	case RowHeader:
		st := gfx.Label
		st.Color = uikit.Heading
		gfx.Text(dst, row.Text, float64(r.X+10), float64(r.Y)+(float64(r.H)-gfx.LineH(st))/2+2, st)
		uikit.FillRect(dst, gfx.Rect{X: r.X + 8, Y: r.Bottom() - 3, W: r.W - 16, H: 1}, uikit.PanelBorder)
	case RowNote:
		st := uikit.DimText()
		lines := gfx.WrapPx(row.Text, st, float64(r.W-24))
		for i, l := range lines {
			gfx.Text(dst, l, float64(r.X+12), float64(r.Y)+8+float64(i)*gfx.LineH(st), st)
		}
	case RowEntry:
		e := entries[row.Entry]
		hover := u.Hot == "scen.row."+e.Key
		switch {
		case e.Key == selKey:
			uikit.FillRect(dst, r.Inset(2), uikit.BtnSelected)
			uikit.StrokeRect(dst, r.Inset(2), uikit.Accent)
		case hover:
			uikit.FillRect(dst, r.Inset(2), uikit.BtnHover)
		}
		ix, iy := r.X+10, r.Y+(r.H-scenIconSize)/2
		iconKey := "ui_icon/scenario_custom"
		if e.builtin {
			iconKey = "ui_icon/scenario_" + e.Def.Key
		}
		if img := w.art().Image(iconKey, noTint); img != nil {
			gfx.Icon(dst, img, float64(ix), float64(iy), scenIconSize)
		} else {
			uikit.FillRect(dst, gfx.Rect{X: ix, Y: iy, W: scenIconSize, H: scenIconSize}, uikit.Field)
			uikit.StrokeRect(dst, gfx.Rect{X: ix, Y: iy, W: scenIconSize, H: scenIconSize}, uikit.PanelBorder)
		}
		tx := r.X + scenIconSize + 22
		pipsW := float32(4*10 + 3*3)
		nameW := r.W - (tx - r.X) - pipsW - 18
		name := gfx.Label
		if e.Err != "" {
			name.Color = uikit.TextDisabled
		}
		gfx.Text(dst, uikit.FitText(e.Name, name, nameW), float64(tx), float64(r.Y)+8, name)
		sum := gfx.Small
		sum.Color = uikit.TextDim
		if e.Err != "" {
			sum.Color = uikit.TextDisabled
		}
		if errs := e.errCount(); errs > 0 && e.Err == "" {
			sum.Color = uikit.Warning
			gfx.Text(dst, fmt.Sprintf("%d problem%s to fix", errs, plural(errs)), float64(tx), float64(r.Y)+30, sum)
		} else {
			lines := gfx.WrapPx(e.Summary, sum, float64(r.W-(tx-r.X)-14))
			for i := 0; i < len(lines) && i < 2; i++ {
				l := lines[i]
				if i == 1 && len(lines) > 2 {
					l = gfx.Ellipsize(l+" "+lines[2], sum, float64(r.W-(tx-r.X)-14))
				}
				gfx.Text(dst, l, float64(tx), float64(r.Y)+30+float64(i)*gfx.LineH(sum), sum)
			}
		}
		if e.Err == "" {
			px := r.Right() - 10 - pipsW
			for i := 0; i < 4; i++ {
				c := uikit.SliderTrack
				if i < e.hardness {
					c = uikit.Accent
				}
				gfx.FillRect(dst, px+float32(i)*13, r.Y+10, 10, 10, c, false)
			}
		}
	}
}

// ---- delete, load ------------------------------------------------------

// confirmDeleteScenario asks before removing a custom scenario.
func (w scenarioPage) confirmDeleteScenario(key string) {
	ng := w.draft()
	name := key
	for _, e := range ng.Entries {
		if e.Key == key {
			name = e.Name
		}
	}
	w.kit().Confirm("scen.delete", "Delete scenario?", fmt.Sprintf("Delete \"%s\"? This can't be undone.", name), "Delete", true, func() { w.deleteScenario(key) })
}

// deleteScenario removes a custom scenario: its file, or the unsaved copy.
func (w scenarioPage) deleteScenario(key string) {
	ng := w.draft()
	for _, e := range ng.Entries {
		if e.Key != key {
			continue
		}
		switch {
		case e.Unsaved:
			var idx int
			if _, err := fmt.Sscanf(e.Key, "unsaved.%d", &idx); err == nil && idx >= 0 && idx < len(w.Unsaved) {
				w.Unsaved = append(w.Unsaved[:idx], w.Unsaved[idx+1:]...)
			}
		case e.File != "" && w.store() != nil:
			if err := w.store().Delete(e.File); err != nil {
				w.sh.Hint("delete failed: " + err.Error())
			}
		}
	}
	ng.RefreshScenarios(w.store(), w.Unsaved)
}

// openScenarioFiles lists the files of the scenarios folder; picking one
// selects it in the list.
func (w scenarioPage) openScenarioFiles() {
	if w.store() == nil {
		return
	}
	files, err := w.store().List()
	msg := ""
	if err != nil {
		msg = "Can't read the scenarios folder: " + err.Error()
	}
	cols := []uikit.TableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return files[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(files[x].Name) < strings.ToLower(files[y].Name) }},
		{ID: "mod", Title: "Modified", W: 150, Cell: func(i int) string { return files[i].Modified },
			Less: func(x, y int) bool { return files[x].Modified < files[y].Modified }},
	}
	w.kit().Push(uikit.Modal{
		ID: "scen.load", Title: "Load scenario file", W: 560, H: 420,
		Draw: func(body gfx.Rect) {
			tb := body
			if msg != "" || len(files) == 0 {
				note := msg
				if note == "" {
					note = "No scenario files yet. Save one from the editor."
				}
				uikit.TextLines(w.kit(), body.X, body.Y, body.W, note, uikit.DimText())
				return
			}
			row := -1
			if w.kit().Table("modal.scen.load.table", tb, cols, len(files), &row, 28) && row >= 0 {
				name := files[row].Name
				w.kit().CloseModal("scen.load")
				ng := w.draft()
				ng.Search = ""
				ng.RefreshScenarios(w.store(), w.Unsaved)
				ng.selectKey("file." + name)
			}
		},
		Buttons: []uikit.ModalButton{{ID: "cancel", Label: "Cancel", Closes: true}},
	})
}
