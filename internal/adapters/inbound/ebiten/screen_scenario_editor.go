package ebiten

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
)

// Scenario Editor (NG-1 §2.3): build a custom scenario from the mandatory
// fields and any number of optional parts, with a live preview and a
// validation panel. It edits a copy; Done hands the copy back to Choose
// Scenario, Save writes it to the scenarios folder.

// editorState is the open editor.
type editorState struct {
	def, orig scenario.Def
	source    string // key of the list entry the copy came from ("" for a new scenario)
	saved     []byte // the file content of the last Save
	savedAs   string // its file name
	open      map[int]bool
	reveal    int             // a part to scroll to and show next frame (-1 none)
	cardY     map[int]float32 // content y of each part card, measured last layout
	cardEnd   map[int]float32 // and where each card ends
	loadErr   string
}

func newEditor(def scenario.Def, source string) *editorState {
	return &editorState{def: def, orig: cloneDef(def), source: source, open: map[int]bool{}, reveal: -1, cardY: map[int]float32{}, cardEnd: map[int]float32{}}
}

func encDef(d scenario.Def) []byte {
	b, err := scenario.Encode(d)
	if err != nil {
		return []byte(fmt.Sprintf("%#v", d))
	}
	return b
}

// dirty reports changes that are neither the state the editor opened with
// nor the last saved file.
func (ed *editorState) dirty() bool {
	cur := encDef(ed.def)
	if bytes.Equal(cur, encDef(ed.orig)) {
		return false
	}
	return ed.saved == nil || !bytes.Equal(cur, ed.saved)
}

// newScenarioDef is the blank scenario of New scenario: Broken Hull's
// mandatory fields with one food item in the kit.
func newScenarioDef() scenario.Def {
	d, _ := scenario.Builtin("broken_hull")
	d.Key, d.Builtin, d.Hardness = "", false, 0
	d.Name, d.Summary, d.Desc = "New scenario", "", ""
	d.Seed = rand.Uint64N(1<<32) + 1
	d.Parts = []scenario.Part{scenario.NewPart(scenario.PartStartItem)}
	return d
}

// copyForEditing is the copy the editor works on for a list entry.
func copyForEditing(d scenario.Def) scenario.Def {
	d = cloneDef(d)
	if d.Builtin {
		d.Name = strings.TrimSpace(d.Name) + " copy"
		if r := []rune(d.Name); len(r) > scenario.MaxName {
			d.Name = string(r[:scenario.MaxName])
		}
	}
	d.Key, d.Builtin = "", false
	return d
}

// openEditor opens the editor on def.
func (a *App) openEditor(def scenario.Def, source string) {
	ng := a.draft()
	ng.ed = newEditor(def, source)
	a.apply(intent{kind: intentPush, screen: screenScenarioEditor})
}

// slugOf is a scenario's file name: lower case letters and digits joined
// by dashes.
func slugOf(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		return "scenario"
	}
	return s
}

// ---- the choice lists ----------------------------------------------------

var (
	edTraits     = trait.All()
	edItems      = scenario.ItemChoices()
	edAnimals    = scenario.AnimalChoices()
	edBuildings  = construction.All()
	edIncidents  = scenario.Incidents()
	edConditions = scenario.Conditions()
	edPawnConds  = scenario.PawnConditions()
	edTraitNames = mapNames(len(edTraits), func(i int) string { return edTraits[i].Label })
	edItemNames  = mapNames(len(edItems), func(i int) string { return itemLabel(edItems[i]) })
	edAnimalName = mapNames(len(edAnimals), func(i int) string { return prettyKey(edAnimals[i].Label()) })
	edBuildNames = mapNames(len(edBuildings), func(i int) string { return prettyKey(edBuildings[i].Name) })
	edIncNames   = mapNames(len(edIncidents), func(i int) string { return edIncidents[i].Label })
	edCondNames  = mapNames(len(edConditions), func(i int) string { return edConditions[i].Label() + " — " + edConditions[i].Desc() })
	edPawnNames  = mapNames(len(edPawnConds), func(i int) string { return scenario.PawnConditionLabel(edPawnConds[i]) })
	edProjNames  = mapNames(int(research.NumProjects), func(i int) string { return research.DefOf(research.Project(i)).Label })
	edWhoNames   = []string{"Every colonist", "One random colonist", "Random colonists"}
)

func mapNames(n int, f func(int) string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

func indexWhere(n int, is func(int) bool) int {
	for i := 0; i < n; i++ {
		if is(i) {
			return i
		}
	}
	return -1
}

// ---- layout ----------------------------------------------------------------

const (
	edHeaderH = 32
	edRowH    = 30
	edCardH   = 32
)

// editorRects places the parts of the editor in the dialog's content.
type editorLayout struct {
	header, form, preview, problems rect
}

func editorRects(content rect) editorLayout {
	head, rest := content.SplitV(edHeaderH + 10)
	leftW := float32(int(rest.W * 0.42))
	L := editorLayout{header: rect{X: head.X, Y: head.Y, W: head.W, H: edHeaderH}}
	L.form = rect{X: rest.X, Y: rest.Y, W: leftW - 8, H: rest.H}
	right := rect{X: rest.X + leftW + 8, Y: rest.Y, W: rest.W - leftW - 8, H: rest.H}
	ph := max(120, float32(int(right.H*0.36)))
	L.preview = rect{X: right.X, Y: right.Y, W: right.W, H: right.H - ph - 8}
	L.problems = rect{X: right.X, Y: right.Bottom() - ph, W: right.W, H: ph}
	return L
}

func (a *App) drawScenarioEditor(screen *ebiten.Image, sw, sh int) {
	a.drawNGFrame(screen, sw, sh, "Scenario Editor")
}

func (a *App) editorUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	if ng.ed == nil { // reached without Choose Scenario: edit a fresh scenario
		ng.ed = newEditor(newScenarioDef(), "")
	}
	ed := ng.ed
	_, _, content, footer := dialogFrame(sw, sh)
	L := editorRects(content)
	probs := ed.def.Validate()
	errs := 0
	for _, p := range probs {
		if p.Severity == scenario.Error {
			errs++
		}
	}

	// Header: name and the file buttons.
	btnW := []float32{84, 84, 84, 150}
	x := L.header.Right()
	var rects [4]rect
	for i := 3; i >= 0; i-- {
		x -= btnW[i]
		rects[i] = rect{X: x, Y: L.header.Y, W: btnW[i], H: L.header.H}
		x -= 8
	}
	nameR := rect{X: L.header.X, Y: L.header.Y, W: max(160, min(360, x-L.header.X)), H: L.header.H}
	a.uiTextField("sced.name", nameR, &ed.def.Name, textFieldSpec{MaxLen: scenario.MaxName, Placeholder: "Scenario name",
		Normalize: strings.TrimSpace, Tip: "Name\nUp to 40 characters."})
	saveOpts := btnOpts{Tip: "Save\nWrite the scenario to the scenarios folder."}
	switch {
	case a.cfg.Scenarios == nil:
		saveOpts.Disabled, saveOpts.Reason = true, "Scenario files are unavailable in this build."
	case errs > 0:
		saveOpts.Disabled, saveOpts.Reason = true, fmt.Sprintf("Fix %d problem%s before saving.", errs, plural(errs))
	}
	a.ngButton("sced.save", rects[0], "Save", saveOpts, a.editorSave)
	a.ngButton("sced.load", rects[1], "Load", btnOpts{Disabled: a.cfg.Scenarios == nil, Reason: "Scenario files are unavailable in this build.", Tip: "Load\nReplace this scenario with one from the scenarios folder."}, a.editorLoad)
	a.ngButton("sced.reset", rects[2], "Reset", btnOpts{Tip: "Reset\nBack to how the scenario was when the editor opened."}, func() {
		ed.def = cloneDef(ed.orig)
		ed.open = map[int]bool{}
		ed.reveal = -1
	})
	a.ngButton("sced.seed", rects[3], "Randomize seed", btnOpts{Tip: fmt.Sprintf("Randomize seed\nThe scenario's own seed (now %d) only drives its random parts, such as a random pet. It is not the planet seed.", ed.def.Seed)}, func() {
		ed.def.Seed = rand.Uint64N(1<<32) + 1
	})

	a.editorForm(ng, ed, L.form, probs)

	// Right column: live preview and validation.
	a.uiPanel(L.preview)
	a.detailView("sced.preview", L.preview, ed.def, scenarioLines(ed.def))
	a.validationPanel(ed, L.problems, probs)

	done := btnOpts{}
	if errs > 0 {
		done.Disabled, done.Reason = true, fmt.Sprintf("Fix %d problem%s", errs, plural(errs))
	}
	a.ngFooter(footer, "Done", "sced.done", done, a.editorDone)
}

// validationPanel lists the problems; clicking one shows its part.
func (a *App) validationPanel(ed *editorState, r rect, probs []scenario.Problem) {
	u := a.uiSt()
	a.uiPanel(r)
	in := r.Inset(10)
	hs := tsLabel
	hs.Color = uiHeading
	a.uiLabel(in.X, in.Y, "Validation", hs)
	list := rect{X: in.X, Y: in.Y + float32(lineH(hs)) + 6, W: in.W, H: in.H - float32(lineH(hs)) - 6}
	if len(probs) == 0 {
		gs := tsBody
		gs.Color = uiGood
		a.uiLabel(list.X, list.Y, "No problems found.", gs)
		return
	}
	type prow struct {
		lines []string
		h     float32
		y     float32
	}
	lh := float32(lineH(tsBody))
	var rows []prow
	y := float32(0)
	for _, p := range probs {
		msg := p.Msg
		if p.Part >= 0 {
			msg = fmt.Sprintf("Part %d. %s", p.Part+1, msg)
		}
		lines := wrapPx(msg, tsBody, float64(list.W-scrollBarW-30))
		h := float32(len(lines))*lh + 8
		rows = append(rows, prow{lines, h, y})
		y += h
	}
	area := a.uiScrollArea("sced.problems", list, y, scrollOpts{Step: 40})
	u.queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for i, r := range rows {
			ry := area.View.Y + r.y - area.Off
			if ry+r.h < area.View.Y || ry > area.View.Bottom() {
				continue
			}
			c := uiDanger
			if probs[i].Severity == scenario.Warning {
				c = uiWarning
			}
			if u.hot == "sced.prob."+uiNum(i) {
				fillRect(sub, rect{X: area.View.X, Y: ry, W: area.View.W, H: r.h}, uiBtnHover)
			}
			fillRect(sub, rect{X: area.View.X + 4, Y: ry + 5, W: 10, H: 10}, c)
			for k, l := range r.lines {
				drawText(sub, l, float64(area.View.X+24), float64(ry)+4+float64(k)*float64(lh), tsBody)
			}
		}
	})
	for i, r := range rows {
		i := i
		ry := area.View.Y + r.y - area.Off
		hit := rect{X: area.View.X, Y: ry, W: area.View.W, H: r.h}.Intersect(area.View)
		if hit.Empty() {
			continue
		}
		show := func() {
			if p := probs[i].Part; p >= 0 && p < len(ed.def.Parts) {
				ed.open[p] = true
				ed.reveal = p
			}
			a.uiOK()
		}
		w := uiWidget{id: "sced.prob." + uiNum(i), r: hit, click: show}
		if probs[i].Part >= 0 {
			w.tip = "Show the part\nScrolls to the part and opens it."
		}
		if u.add(w) && a.uiPress(hit) {
			show()
		}
	}
}

// ---- the form ----------------------------------------------------------------

// editorForm is the left column: summary, description, the mandatory
// block and the optional parts, in one scroll view.
func (a *App) editorForm(ng *newGameState, ed *editorState, r rect, probs []scenario.Problem) {
	a.uiPanel(r)
	body, bar := r.SplitV(-(edRowH + 12))
	a.ngButton("sced.add", rect{X: bar.X + 8, Y: bar.Y + 4, W: 150, H: edRowH}, "Add part...", btnOpts{Style: btnPrimary, Tip: "Add part\nAdd another rule to the scenario."}, a.openAddPart)
	view := body.Inset(6)
	area := a.uiScrollArea("sced.form", view, ng.formHeight("sced.form"), scrollOpts{Step: 48})
	f := newForm(area)
	d := &ed.def

	// Summary and description.
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Summary", 92)
		a.uiTextField("sced.summary", c.Inset(2), &d.Summary, textFieldSpec{MaxLen: scenario.MaxSummary, Placeholder: "One line shown in the list"})
	}
	if row, ok := f.row(22); ok {
		a.rowLabel(row, "Description", 120)
	}
	if row, ok := f.row(68); ok {
		a.uiTextField("sced.desc", row.Inset(2), &d.Desc, textFieldSpec{Multiline: true, MaxLen: scenario.MaxDesc, Placeholder: "The story of the start"})
	}
	f.gap(6)

	// Mandatory block.
	f.sectionTitle(a, "Starting setup")
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Player faction", 120)
		sel := 0
		if d.Faction == faction.Clans {
			sel = 1
		}
		a.segmented("sced.faction", rect{X: c.X, Y: c.Y + 3, W: min(c.W, 240), H: c.H - 6}, []string{"Settlers", "Clan"}, &sel,
			[]string{"Settlers\nIndustrial colonists who ride down in pods.", "Clan\nA tribe that walks in with what it carries."})
		if sel == 1 {
			d.Faction = faction.Clans
		} else {
			d.Faction = faction.Settlers
		}
	}
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Starting people", 120)
		old := d.People
		a.ngStepper("sced.people", rect{X: c.X, Y: c.Y + 3, W: 112, H: c.H - 6}, &d.People, scenario.MinPeople, scenario.MaxPeople)
		if d.People != old {
			d.ChooseFrom += d.People - old
		}
		d.ChooseFrom = max(d.People, min(d.People+scenario.MaxExtraPool, d.ChooseFrom))
	}
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Choose from", 120)
		a.ngStepper("sced.choose", rect{X: c.X, Y: c.Y + 3, W: 112, H: c.H - 6}, &d.ChooseFrom, d.People, d.People+scenario.MaxExtraPool)
	}
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Arrival", 120)
		sel := int(d.Arrival)
		a.segmented("sced.arrival", rect{X: c.X, Y: c.Y + 3, W: min(c.W, 330), H: c.H - 6}, []string{"Drop pods", "Standing", "Walk in"}, &sel, nil)
		d.Arrival = scenario.Arrival(sel)
	}
	if row, ok := f.row(edRowH); ok {
		c := a.rowLabel(row, "Tech level", 120)
		sel := int(d.Tech)
		a.segmented("sced.tech", rect{X: c.X, Y: c.Y + 3, W: min(c.W, 330), H: c.H - 6}, []string{"Neolithic", "Industrial", "Spacer"}, &sel, nil)
		d.Tech = faction.Tech(sel)
	}
	f.gap(8)

	// Optional parts.
	f.sectionTitle(a, "Optional parts")
	partProbs := map[int]bool{}
	for _, p := range probs {
		if p.Part >= 0 && p.Severity == scenario.Error {
			partProbs[p.Part] = true
		}
	}
	for i := 0; i < len(d.Parts); i++ {
		if a.partCard(f, ng, ed, i, partProbs[i]) {
			break // the list changed: lay out again next frame
		}
	}
	ng.setFormHeight("sced.form", f.content())

	if ed.reveal >= 0 {
		if y, ok := ed.cardY[ed.reveal]; ok {
			end := ed.cardEnd[ed.reveal]
			a.uiScrollReveal("sced.form", view.H, y, min(max(end, y+edCardH), y+view.H))
		}
		ed.reveal = -1
	}
}

// sectionTitle is a heading row of the form.
func (f *formCtx) sectionTitle(a *App, s string) {
	if row, ok := f.row(26); ok {
		hs := tsLabel
		hs.Color = uiHeading
		a.uiLabel(row.X, row.Y+4, s, hs)
		a.uiSt().queue(func(dst *ebiten.Image) {
			fillRect(dst, rect{X: row.X, Y: row.Bottom() - 2, W: row.W, H: 1}, uiPanelBorder)
		})
	}
}

// partCard lays out part i. It reports the part list changed (a move or a
// delete), so the caller stops this frame's layout.
func (a *App) partCard(f *formCtx, ng *newGameState, ed *editorState, i int, bad bool) (changed bool) {
	u := a.uiSt()
	p := &ed.def.Parts[i]
	top := f.y
	ed.cardY[i] = top
	row, ok := f.row(edCardH)
	expanded := ed.open[i]
	if ok {
		btn := float32(28)
		hdr := rect{X: row.X, Y: row.Y + 2, W: row.W - 3*(btn+4), H: edCardH - 4}
		toggle := func() { ed.open[i] = !ed.open[i]; a.uiOK() }
		w := uiWidget{id: "sced.card." + uiNum(i), r: hdr, label: p.Kind.Label(), click: toggle, focusable: true, tip: p.Kind.Label() + "\n" + p.Kind.Desc()}
		if u.add(w) && (a.uiPress(hdr) || a.uiKeyAct(w.id)) {
			toggle()
		}
		sum := p.Summary()
		u.queue(func(dst *ebiten.Image) {
			fill := uiBtn
			if u.hot == w.id {
				fill = uiBtnHover
			}
			fillRect(dst, hdr, fill)
			bc := uiBtnBorder
			if bad {
				bc = uiDanger
			}
			strokeRect(dst, hdr, bc)
			ty := float64(hdr.Y) + (float64(hdr.H)-lineH(tsLabel))/2
			cx, cy := hdr.X+14, hdr.Y+hdr.H/2
			if expanded {
				fillTri(dst, cx-4, cy-2, cx+4, cy-2, cx, cy+3)
			} else {
				fillTri(dst, cx-2, cy-4, cx-2, cy+4, cx+3, cy)
			}
			ts := tsLabel
			if bad {
				ts.Color = uiDanger
			}
			title := fmt.Sprintf("%d. %s", i+1, p.Kind.Label())
			drawText(dst, title, float64(hdr.X+28), ty, ts)
			if !expanded {
				tw, _ := measure(title, ts)
				ss := dimText()
				drawText(dst, fitText(sum, ss, hdr.W-34-float32(tw)-10), float64(hdr.X+28)+tw+10, ty, ss)
			}
		})
		bx := hdr.Right() + 4
		a.ngButton("sced.up."+uiNum(i), rect{X: bx, Y: hdr.Y, W: btn, H: hdr.H}, "^", btnOpts{Disabled: i == 0, Reason: "Already first.", Tip: "Move up"}, func() {
			ed.def.Parts[i], ed.def.Parts[i-1] = ed.def.Parts[i-1], ed.def.Parts[i]
			ed.open[i], ed.open[i-1] = ed.open[i-1], ed.open[i]
			changed = true
		})
		a.ngButton("sced.down."+uiNum(i), rect{X: bx + btn + 4, Y: hdr.Y, W: btn, H: hdr.H}, "v", btnOpts{Disabled: i == len(ed.def.Parts)-1, Reason: "Already last.", Tip: "Move down"}, func() {
			ed.def.Parts[i], ed.def.Parts[i+1] = ed.def.Parts[i+1], ed.def.Parts[i]
			ed.open[i], ed.open[i+1] = ed.open[i+1], ed.open[i]
			changed = true
		})
		// The button ids are the spec's: sced.up.<i>, sced.down.<i>, sced.del.<i>.
		a.ngButton("sced.del."+uiNum(i), rect{X: bx + 2*(btn+4), Y: hdr.Y, W: btn, H: hdr.H}, "x", btnOpts{Style: btnDanger, Tip: "Remove this part"}, func() {
			ed.def.Parts = append(ed.def.Parts[:i], ed.def.Parts[i+1:]...)
			for k := i; k < len(ed.def.Parts)+1; k++ {
				ed.open[k] = ed.open[k+1]
			}
			delete(ed.open, len(ed.def.Parts))
			changed = true
		})
	}
	if expanded && !changed {
		a.partControls(f, ng, ed, i, p)
	}
	if expanded {
		f.gap(4)
	}
	f.gap(4)
	ed.cardEnd[i] = f.y
	if ok {
		barY, barH, barX := f.view.Y+top-f.off, f.y-top-4, f.x-3
		u.queue(func(dst *ebiten.Image) { fillRect(dst, rect{X: barX, Y: barY, W: 2, H: barH}, uiPanelBorder) })
	}
	return changed
}

// ctl is a labelled control row: it returns the control's rectangle.
func (a *App) ctl(f *formCtx, label string) (rect, bool) {
	row, ok := f.row(edRowH)
	if !ok {
		return rect{}, false
	}
	row.X += 14
	row.W -= 14
	c := a.rowLabel(row, label, 118)
	return rect{X: c.X, Y: c.Y + 3, W: c.W, H: c.H - 6}, true
}

// slider is a percent slider with its value at the right.
func (a *App) pctSlider(id string, c rect, v *int, lo, hi int, title string) {
	sr := rect{X: c.X, Y: c.Y - 1, W: max(60, c.W-54), H: 20}
	a.uiSliderInt(id, sr, v, sliderSpec{Min: float64(lo), Max: float64(hi), Step: 5, Title: title,
		Format: func(x float64) string { return fmt.Sprintf("%d%%", int(x)) }})
	val := *v
	a.uiLabel(sr.Right()+10, c.Y+(c.H-float32(lineH(tsBody)))/2, fmt.Sprintf("%d%%", val), tsBody)
}

// partControls declares the controls of an expanded part.
func (a *App) partControls(f *formCtx, ng *newGameState, ed *editorState, i int, p *scenario.Part) {
	id := func(s string) string { return "sced.p." + uiNum(i) + "." + s }
	d := &ed.def
	chance := func(label string) {
		if c, ok := a.ctl(f, label); ok {
			v := int(p.Chance)
			a.pctSlider(id("chance"), c, &v, scenario.MinChance, scenario.MaxChance, "Chance")
			p.Chance = uint8(v)
		}
	}
	itemRows := func() {
		if c, ok := a.ctl(f, "Item"); ok {
			a.ngChoice(id("item"), c, "Item", edItemNames, indexWhere(len(edItems), func(k int) bool { return edItems[k] == p.Item }), "",
				func(k int) { p.Item = edItems[k] })
		}
		if c, ok := a.ctl(f, "Quantity"); ok {
			q := int(p.Qty)
			hi := scenario.MaxQty
			if p.Item.IsWeapon() {
				hi = scenario.MaxWeaponQty
			}
			a.ngNum(id("qty"), rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &q, scenario.MinQty, hi)
			p.Qty = int32(min(q, 1<<30))
		}
	}
	incidentRow := func() {
		if c, ok := a.ctl(f, "Incident"); ok {
			a.ngChoice(id("incident"), c, "Incident", edIncNames, indexWhere(len(edIncidents), func(k int) bool { return edIncidents[k].Key == p.Incident }), "",
				func(k int) { p.Incident = edIncidents[k].Key })
		}
	}
	switch p.Kind {
	case scenario.PartForcedTrait:
		if c, ok := a.ctl(f, "Trait"); ok {
			a.ngChoice(id("trait"), c, "Trait", edTraitNames, indexWhere(len(edTraits), func(k int) bool { return edTraits[k].ID == p.Trait }), "",
				func(k int) { p.Trait = edTraits[k].ID })
		}
		if c, ok := a.ctl(f, "Who gets it"); ok {
			a.ngChoice(id("who"), c, "Who gets the trait", edWhoNames, int(p.Who), "", func(k int) { p.Who = scenario.Who(k) })
		}
		if p.Who == scenario.WhoNRandom {
			if c, ok := a.ctl(f, "How many"); ok {
				n := int(p.N)
				a.ngStepper(id("n"), rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, max(1, d.People))
				p.N = uint8(n)
			}
		}
		chance("Chance")
	case scenario.PartAgeRange:
		if c, ok := a.ctl(f, "Youngest"); ok {
			v := int(p.MinAge)
			a.ngStepper(id("min"), rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &v, scenario.MinAge, scenario.MaxAge)
			p.MinAge = uint8(v)
		}
		if c, ok := a.ctl(f, "Oldest"); ok {
			v := int(p.MaxAge)
			a.ngStepper(id("max"), rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &v, scenario.MinAge, scenario.MaxAge)
			p.MaxAge = uint8(v)
		}
	case scenario.PartForcedCondition:
		if c, ok := a.ctl(f, "Condition"); ok {
			a.ngChoice(id("cond"), c, "Condition", edPawnNames, indexWhere(len(edPawnConds), func(k int) bool { return edPawnConds[k] == p.PawnCond }), "",
				func(k int) { p.PawnCond = pawngen.Condition(edPawnConds[k]) })
		}
		chance("Chance")
	case scenario.PartNoClothes:
		if row, ok := f.row(edRowH); ok {
			a.uiLabel(row.X+14, row.Y+6, "Colonists arrive with nothing to wear. No options.", dimText())
		}
	case scenario.PartStartItem:
		itemRows()
	case scenario.PartScatterItem:
		itemRows()
		if c, ok := a.ctl(f, "Piles"); ok {
			n := int(p.N)
			a.ngStepper(id("piles"), rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, 20)
			p.N = uint8(n)
		}
		if c, ok := a.ctl(f, "Where"); ok {
			sel := 1
			if p.Near {
				sel = 0
			}
			a.segmented(id("near"), rect{X: c.X, Y: c.Y, W: min(c.W, 260), H: c.H}, []string{"Near the start", "Anywhere"}, &sel, nil)
			p.Near = sel == 0
		}
	case scenario.PartStartAnimal:
		if c, ok := a.ctl(f, "Random pet"); ok {
			a.uiCheck(id("random"), rect{X: c.X, Y: c.Y, W: 220, H: c.H}, "A cat, dog or guinea pig", &p.Random, checkOpts{})
		}
		if c, ok := a.ctl(f, "Species"); ok {
			dis := ""
			if p.Random {
				dis = "A random pet is used."
			}
			a.ngChoice(id("species"), c, "Species", edAnimalName, indexWhere(len(edAnimals), func(k int) bool { return edAnimals[k] == p.Species }), dis,
				func(k int) { p.Species = animal.Species(edAnimals[k]) })
		}
		if c, ok := a.ctl(f, "Count"); ok {
			n := int(p.N)
			a.ngStepper(id("count"), rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, scenario.MaxAnimals)
			p.N = uint8(n)
		}
	case scenario.PartCondition:
		if c, ok := a.ctl(f, "Condition"); ok {
			a.ngChoice(id("cond"), c, "Permanent condition", edCondNames, indexWhere(len(edConditions), func(k int) bool { return edConditions[k] == p.Cond }), "",
				func(k int) { p.Cond = edConditions[k] })
		}
	case scenario.PartScheduledIncident:
		incidentRow()
		if c, ok := a.ctl(f, "First day"); ok {
			v := int(p.FirstDay)
			a.ngNum(id("first"), rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &v, scenario.MinFirstDay, scenario.MaxFirstDay)
			p.FirstDay = uint16(min(v, 65535))
		}
		if c, ok := a.ctl(f, "Repeat every"); ok {
			v := int(p.EveryDays)
			a.ngNum(id("every"), rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &v, 0, scenario.MaxEveryDays)
			p.EveryDays = uint16(min(v, 65535))
			a.uiLabel(c.X+100, c.Y+(c.H-float32(lineH(tsBody)))/2, "days (0 = once)", dimText())
		}
	case scenario.PartDisabledIncident:
		incidentRow()
	case scenario.PartDisallowBuilding:
		if c, ok := a.ctl(f, "Building"); ok {
			a.ngChoice(id("building"), c, "Building", edBuildNames, indexWhere(len(edBuildings), func(k int) bool { return edBuildings[k].Kind == p.Building }), "",
				func(k int) { p.Building = edBuildings[k].Kind })
		}
	case scenario.PartResearch:
		if c, ok := a.ctl(f, "Project"); ok {
			a.ngChoice(id("project"), c, "Research project", edProjNames, int(p.Project), "", func(k int) { p.Project = research.Project(k) })
		}
	case scenario.PartResearchCost:
		if c, ok := a.ctl(f, "Cost"); ok {
			v := int(p.Percent)
			a.pctSlider(id("percent"), c, &v, scenario.MinCostPct, scenario.MaxCostPct, "Research cost")
			p.Percent = uint16(v)
		}
	case scenario.PartIntro:
		if row, ok := f.row(120); ok {
			row.X += 14
			row.W -= 14
			a.uiTextField(id("text"), row.Inset(2), &p.Text, textFieldSpec{Multiline: true, MaxLen: scenario.MaxIntro, Placeholder: "Shown once after landing"})
		}
	}
}

// ---- add part, save, load, done ------------------------------------------------

// openAddPart opens the categorised list of part kinds.
func (a *App) openAddPart() {
	cats := scenario.Catalog()
	const headH, kindH = 28, 52
	total := float32(0)
	for _, c := range cats {
		total += headH + float32(len(c.Kinds))*kindH
	}
	a.uiPushModal(modal{
		id: "sced.addpart", title: "Add part", w: 600, h: 520,
		draw: func(a *App, body rect) {
			u := a.uiSt()
			area := a.uiScrollArea("modal.sced.addpart.scroll", body, total, scrollOpts{Step: kindH})
			y := float32(0)
			type kindPos struct {
				info scenario.KindInfo
				y    float32
			}
			var kinds []kindPos
			type headPos struct {
				name string
				y    float32
			}
			var heads []headPos
			for _, c := range cats {
				heads = append(heads, headPos{c.Name, y})
				y += headH
				for _, k := range c.Kinds {
					kinds = append(kinds, kindPos{k, y})
					y += kindH
				}
			}
			u.queue(func(dst *ebiten.Image) {
				sub := clipTo(dst, area.View)
				for _, h := range heads {
					hs := tsLabel
					hs.Color = uiHeading
					yy := area.View.Y + h.y - area.Off
					drawText(sub, h.name, float64(area.View.X+4), float64(yy)+6, hs)
					fillRect(sub, rect{X: area.View.X, Y: yy + headH - 3, W: area.View.W, H: 1}, uiPanelBorder)
				}
				for _, k := range kinds {
					row := rect{X: area.View.X, Y: area.View.Y + k.y - area.Off, W: area.View.W, H: kindH}
					if u.hot == "sced.addkind."+k.info.Kind.Key() {
						fillRect(sub, row.Inset(1), uiBtnHover)
					}
					drawText(sub, k.info.Label, float64(row.X+12), float64(row.Y)+6, tsLabel)
					ds := tsSmall
					ds.Color = uiTextDim
					for n, l := range wrapPx(k.info.Desc, ds, float64(row.W-24)) {
						if n < 2 {
							drawText(sub, l, float64(row.X+12), float64(row.Y)+6+lineH(tsLabel)+float64(n)*lineH(ds), ds)
						}
					}
				}
			})
			for _, k := range kinds {
				k := k
				row := rect{X: area.View.X, Y: area.View.Y + k.y - area.Off, W: area.View.W, H: kindH}
				hit := row.Intersect(area.View)
				if hit.Empty() {
					continue
				}
				add := func() { a.addPart(k.info.Kind) }
				w := uiWidget{id: "sced.addkind." + k.info.Kind.Key(), r: hit, label: k.info.Label, click: add}
				if u.add(w) && a.uiPress(hit) {
					add()
				}
			}
		},
		buttons: []modalButton{{id: "cancel", label: "Cancel", closes: true}},
	})
}

// addPart appends a part of kind k, opened and in view.
func (a *App) addPart(k scenario.PartKind) {
	ng := a.draft()
	if ng.ed == nil {
		return
	}
	ed := ng.ed
	ed.def.Parts = append(ed.def.Parts, scenario.NewPart(k))
	n := len(ed.def.Parts) - 1
	ed.open[n] = true
	ed.reveal = n
	a.uiCloseModal("sced.addpart")
}

// editorSave writes the scenario to the scenarios folder, asking first
// before it replaces a file it did not write itself.
func (a *App) editorSave() {
	ng := a.draft()
	ed := ng.ed
	st := a.cfg.Scenarios
	if ed == nil || st == nil {
		return
	}
	def := cloneDef(ed.def)
	def.Builtin, def.Key = false, ""
	data, err := scenario.Encode(def)
	if err != nil {
		a.hint = "cannot save: " + err.Error()
		return
	}
	name := slugOf(def.Name)
	write := func() {
		if err := st.Save(name, data); err != nil {
			a.hint = "save failed: " + err.Error()
			return
		}
		ed.saved, ed.savedAs = data, name
		a.hint = "saved " + name
		ng.refreshScenarios(a)
	}
	files, _ := st.List()
	exists := false
	for _, f := range files {
		if f.Name == name {
			exists = true
		}
	}
	if exists && ed.savedAs != name {
		a.uiConfirm("sced.overwrite", "Overwrite scenario?", fmt.Sprintf("A scenario file named \"%s\" already exists. Replace it?", name), "Overwrite", true, write)
		return
	}
	write()
}

// editorLoad lists the scenario files; picking one replaces the edit.
func (a *App) editorLoad() {
	st := a.cfg.Scenarios
	if st == nil {
		return
	}
	files, err := st.List()
	ng := a.draft()
	if ng.ed != nil {
		ng.ed.loadErr = ""
		if err != nil {
			ng.ed.loadErr = "Can't read the scenarios folder: " + err.Error()
		}
	}
	cols := []tableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return files[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(files[x].Name) < strings.ToLower(files[y].Name) }},
		{ID: "mod", Title: "Modified", W: 150, Cell: func(i int) string { return files[i].Modified },
			Less: func(x, y int) bool { return files[x].Modified < files[y].Modified }},
	}
	a.uiPushModal(modal{
		id: "sced.loadfile", title: "Load scenario", w: 560, h: 440,
		draw: func(a *App, body rect) {
			ed := a.draft().ed
			if ed == nil {
				return
			}
			tb := body
			if ed.loadErr != "" {
				ls := tsBody
				ls.Color = uiDanger
				h := a.textLines(body.X, body.Y, body.W, ed.loadErr, ls)
				_, tb = body.SplitV(h + 8)
			}
			if len(files) == 0 {
				a.textLines(tb.X, tb.Y, tb.W, "No scenario files yet. Save one first.", dimText())
				return
			}
			row := -1
			if a.uiTable("modal.sced.load.table", tb, cols, len(files), &row, 28) && row >= 0 {
				a.loadScenarioFile(files[row].Name)
			}
		},
		buttons: []modalButton{{id: "cancel", label: "Cancel", closes: true}},
	})
}

// loadScenarioFile replaces the edited scenario with a file's.
func (a *App) loadScenarioFile(name string) {
	ng := a.draft()
	ed := ng.ed
	st := a.cfg.Scenarios
	if ed == nil || st == nil {
		return
	}
	data, err := st.Load(name)
	var d scenario.Def
	if err == nil {
		d, err = scenario.Decode(data)
	}
	if err != nil {
		ed.loadErr = fmt.Sprintf("%s: %v", name, err)
		return
	}
	d.Builtin, d.Key = false, ""
	ed.def = d
	ed.saved, ed.savedAs = encDef(d), name
	ed.open = map[int]bool{}
	ed.reveal = -1
	ed.loadErr = ""
	a.uiCloseModal("sced.loadfile")
}

// editorDone hands the edited scenario back to Choose Scenario: a file
// when it was saved and not changed since, otherwise an unsaved scenario
// that lasts until the game quits.
func (a *App) editorDone() {
	ng := a.draft()
	ed := ng.ed
	if ed == nil || scenario.HasErrors(ed.def.Validate()) {
		return
	}
	def := cloneDef(ed.def)
	def.Builtin, def.Key = false, ""
	key := ""
	if ed.saved != nil && bytes.Equal(encDef(def), ed.saved) {
		key = "file." + ed.savedAs
	} else {
		idx := -1
		if strings.HasPrefix(ed.source, "unsaved.") {
			if _, err := fmt.Sscanf(ed.source, "unsaved.%d", &idx); err != nil || idx < 0 || idx >= len(a.ngUnsaved) {
				idx = -1
			}
		}
		if idx >= 0 {
			a.ngUnsaved[idx] = def
		} else {
			a.ngUnsaved = append(a.ngUnsaved, def)
			idx = len(a.ngUnsaved) - 1
		}
		key = fmt.Sprintf("unsaved.%d", idx)
	}
	a.popScreen()
	ng.search = ""
	ng.refreshScenarios(a)
	ng.selectKey(key)
}
