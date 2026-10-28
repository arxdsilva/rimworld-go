package wizard

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	Def, orig scenario.Def
	source    string // key of the list entry the copy came from ("" for a new scenario)
	Saved     []byte // the file content of the last Save
	SavedAs   string // its file name
	Open      map[int]bool
	reveal    int             // a part to scroll to and show next frame (-1 none)
	cardY     map[int]float32 // content y of each part card, measured last layout
	cardEnd   map[int]float32 // and where each card ends
	LoadErr   string
}

func newEditor(def scenario.Def, source string) *editorState {
	return &editorState{Def: def, orig: CloneDef(def), source: source, Open: map[int]bool{}, reveal: -1, cardY: map[int]float32{}, cardEnd: map[int]float32{}}
}

func encDef(d scenario.Def) []byte {
	b, err := scenario.Encode(d)
	if err != nil {
		return []byte(fmt.Sprintf("%#v", d))
	}
	return b
}

// Dirty reports changes that are neither the state the editor opened with
// nor the last saved file.
func (ed *editorState) Dirty() bool {
	cur := encDef(ed.Def)
	if bytes.Equal(cur, encDef(ed.orig)) {
		return false
	}
	return ed.Saved == nil || !bytes.Equal(cur, ed.Saved)
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

// CopyForEditing is the copy the editor works on for a list entry.
func CopyForEditing(d scenario.Def) scenario.Def {
	d = CloneDef(d)
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
func (w editorPage) openEditor(def scenario.Def, source string) {
	ng := w.draft()
	ng.Ed = newEditor(def, source)
	w.sh.Do(ports.Push{Screen: ports.ScreenScenarioEditor})
}

// SlugOf is a scenario's file name: lower case letters and digits joined
// by dashes.
func SlugOf(name string) string {
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
	EdItems      = scenario.ItemChoices()
	edAnimals    = scenario.AnimalChoices()
	edBuildings  = construction.All()
	edIncidents  = scenario.Incidents()
	edConditions = scenario.Conditions()
	edPawnConds  = scenario.PawnConditions()
	edTraitNames = mapNames(len(edTraits), func(i int) string { return edTraits[i].Label })
	edItemNames  = mapNames(len(EdItems), func(i int) string { return itemLabel(EdItems[i]) })
	edAnimalName = mapNames(len(edAnimals), func(i int) string { return PrettyKey(edAnimals[i].Label()) })
	edBuildNames = mapNames(len(edBuildings), func(i int) string { return PrettyKey(edBuildings[i].Name) })
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

func IndexWhere(n int, is func(int) bool) int {
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
	header, form, preview, problems gfx.Rect
}

func editorRects(content gfx.Rect) editorLayout {
	head, rest := content.SplitV(edHeaderH + 10)
	leftW := float32(int(rest.W * 0.42))
	L := editorLayout{header: gfx.Rect{X: head.X, Y: head.Y, W: head.W, H: edHeaderH}}
	L.form = gfx.Rect{X: rest.X, Y: rest.Y, W: leftW - 8, H: rest.H}
	right := gfx.Rect{X: rest.X + leftW + 8, Y: rest.Y, W: rest.W - leftW - 8, H: rest.H}
	ph := max(120, float32(int(right.H*0.36)))
	L.preview = gfx.Rect{X: right.X, Y: right.Y, W: right.W, H: right.H - ph - 8}
	L.problems = gfx.Rect{X: right.X, Y: right.Bottom() - ph, W: right.W, H: ph}
	return L
}

func (w editorPage) drawScenarioEditor(screen *ebiten.Image, sw, sh int) {
	w.drawNGFrame(screen, sw, sh, "Scenario Editor")
}

func (w editorPage) editorUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	if ng.Ed == nil { // reached without Choose Scenario: edit a fresh scenario
		ng.Ed = newEditor(newScenarioDef(), "")
	}
	ed := ng.Ed
	_, _, content, footer := uikit.DialogFrame(sw, sh)
	L := editorRects(content)
	probs := ed.Def.Validate()
	errs := 0
	for _, p := range probs {
		if p.Severity == scenario.Error {
			errs++
		}
	}

	// Header: name and the file buttons.
	btnW := []float32{84, 84, 84, 150}
	x := L.header.Right()
	var rects [4]gfx.Rect
	for i := 3; i >= 0; i-- {
		x -= btnW[i]
		rects[i] = gfx.Rect{X: x, Y: L.header.Y, W: btnW[i], H: L.header.H}
		x -= 8
	}
	nameR := gfx.Rect{X: L.header.X, Y: L.header.Y, W: max(160, min(360, x-L.header.X)), H: L.header.H}
	w.kit().TextField("sced.name", nameR, &ed.Def.Name, uikit.TextFieldSpec{MaxLen: scenario.MaxName, Placeholder: "Scenario name",
		Normalize: strings.TrimSpace, Tip: "Name\nUp to 40 characters."})
	saveOpts := uikit.BtnOpts{Tip: "Save\nWrite the scenario to the scenarios folder."}
	switch {
	case w.store() == nil:
		saveOpts.Disabled, saveOpts.Reason = true, "Scenario files are unavailable in this build."
	case errs > 0:
		saveOpts.Disabled, saveOpts.Reason = true, fmt.Sprintf("Fix %d problem%s before saving.", errs, plural(errs))
	}
	uikit.StyledButton(w.kit(), "sced.save", rects[0], "Save", saveOpts, w.editorSave)
	uikit.StyledButton(w.kit(), "sced.load", rects[1], "Load", uikit.BtnOpts{Disabled: w.store() == nil, Reason: "Scenario files are unavailable in this build.", Tip: "Load\nReplace this scenario with one from the scenarios folder."}, w.editorLoad)
	uikit.StyledButton(w.kit(), "sced.reset", rects[2], "Reset", uikit.BtnOpts{Tip: "Reset\nBack to how the scenario was when the editor opened."}, func() {
		ed.Def = CloneDef(ed.orig)
		ed.Open = map[int]bool{}
		ed.reveal = -1
	})
	uikit.StyledButton(w.kit(), "sced.seed", rects[3], "Randomize seed", uikit.BtnOpts{Tip: fmt.Sprintf("Randomize seed\nThe scenario's own seed (now %d) only drives its random parts, such as a random pet. It is not the planet seed.", ed.Def.Seed)}, func() {
		ed.Def.Seed = rand.Uint64N(1<<32) + 1
	})

	w.editorForm(ng, ed, L.form, probs)

	// Right column: live preview and validation.
	w.kit().Panel(L.preview)
	w.detailView("sced.preview", L.preview, ed.Def, ScenarioLines(ed.Def))
	w.validationPanel(ed, L.problems, probs)

	done := uikit.BtnOpts{}
	if errs > 0 {
		done.Disabled, done.Reason = true, fmt.Sprintf("Fix %d problem%s", errs, plural(errs))
	}
	w.ngFooter(footer, "Done", "sced.done", done, w.editorDone)
}

// validationPanel lists the problems; clicking one shows its part.
func (w editorPage) validationPanel(ed *editorState, r gfx.Rect, probs []scenario.Problem) {
	u := w.kit()
	w.kit().Panel(r)
	in := r.Inset(10)
	hs := gfx.Label
	hs.Color = uikit.Heading
	w.kit().Label(in.X, in.Y, "Validation", hs)
	list := gfx.Rect{X: in.X, Y: in.Y + float32(gfx.LineH(hs)) + 6, W: in.W, H: in.H - float32(gfx.LineH(hs)) - 6}
	if len(probs) == 0 {
		gs := gfx.Body
		gs.Color = uikit.Good
		w.kit().Label(list.X, list.Y, "No problems found.", gs)
		return
	}
	type prow struct {
		lines []string
		h     float32
		y     float32
	}
	lh := float32(gfx.LineH(gfx.Body))
	var rows []prow
	y := float32(0)
	for _, p := range probs {
		msg := p.Msg
		if p.Part >= 0 {
			msg = fmt.Sprintf("Part %d. %s", p.Part+1, msg)
		}
		lines := gfx.WrapPx(msg, gfx.Body, float64(list.W-uikit.ScrollBarW-30))
		h := float32(len(lines))*lh + 8
		rows = append(rows, prow{lines, h, y})
		y += h
	}
	area := w.kit().ScrollArea("sced.problems", list, y, uikit.ScrollOpts{Step: 40})
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for i, r := range rows {
			ry := area.View.Y + r.y - area.Off
			if ry+r.h < area.View.Y || ry > area.View.Bottom() {
				continue
			}
			c := uikit.Danger
			if probs[i].Severity == scenario.Warning {
				c = uikit.Warning
			}
			if u.Hot == "sced.prob."+uikit.Num(i) {
				uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: ry, W: area.View.W, H: r.h}, uikit.BtnHover)
			}
			uikit.FillRect(sub, gfx.Rect{X: area.View.X + 4, Y: ry + 5, W: 10, H: 10}, c)
			for k, l := range r.lines {
				gfx.Text(sub, l, float64(area.View.X+24), float64(ry)+4+float64(k)*float64(lh), gfx.Body)
			}
		}
	})
	for i, r := range rows {
		i := i
		ry := area.View.Y + r.y - area.Off
		hit := gfx.Rect{X: area.View.X, Y: ry, W: area.View.W, H: r.h}.Intersect(area.View)
		if hit.Empty() {
			continue
		}
		show := func() {
			if p := probs[i].Part; p >= 0 && p < len(ed.Def.Parts) {
				ed.Open[p] = true
				ed.reveal = p
			}
			w.kit().OK()
		}
		wd := uikit.Widget{ID: "sced.prob." + uikit.Num(i), R: hit, Click: show}
		if probs[i].Part >= 0 {
			wd.Tip = "Show the part\nScrolls to the part and opens it."
		}
		if u.Add(wd) && w.kit().Press(hit) {
			show()
		}
	}
}

// ---- the form ----------------------------------------------------------------

// editorForm is the left column: summary, description, the mandatory
// block and the optional parts, in one scroll view.
func (w editorPage) editorForm(ng *Draft, ed *editorState, r gfx.Rect, probs []scenario.Problem) {
	w.kit().Panel(r)
	body, bar := r.SplitV(-(edRowH + 12))
	uikit.StyledButton(w.kit(), "sced.add", gfx.Rect{X: bar.X + 8, Y: bar.Y + 4, W: 150, H: edRowH}, "Add part...", uikit.BtnOpts{Style: uikit.BtnPrimary, Tip: "Add part\nAdd another rule to the scenario."}, w.openAddPart)
	view := body.Inset(6)
	area := w.kit().ScrollArea("sced.form", view, ng.formHeight("sced.form"), uikit.ScrollOpts{Step: 48})
	f := uikit.NewForm(area)
	d := &ed.Def

	// Summary and description.
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Summary", 92)
		w.kit().TextField("sced.summary", c.Inset(2), &d.Summary, uikit.TextFieldSpec{MaxLen: scenario.MaxSummary, Placeholder: "One line shown in the list"})
	}
	if row, ok := f.Row(22); ok {
		uikit.RowLabel(w.kit(), row, "Description", 120)
	}
	if row, ok := f.Row(68); ok {
		w.kit().TextField("sced.desc", row.Inset(2), &d.Desc, uikit.TextFieldSpec{Multiline: true, MaxLen: scenario.MaxDesc, Placeholder: "The story of the start"})
	}
	f.Gap(6)

	// Mandatory block.
	f.SectionTitle(w.sh, "Starting setup")
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Player faction", 120)
		sel := 0
		if d.Faction == faction.Clans {
			sel = 1
		}
		uikit.Segmented(w.kit(), "sced.faction", gfx.Rect{X: c.X, Y: c.Y + 3, W: min(c.W, 240), H: c.H - 6}, []string{"Settlers", "Clan"}, &sel,
			[]string{"Settlers\nIndustrial colonists who ride down in pods.", "Clan\nA tribe that walks in with what it carries."})
		if sel == 1 {
			d.Faction = faction.Clans
		} else {
			d.Faction = faction.Settlers
		}
	}
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Starting people", 120)
		old := d.People
		uikit.Stepper(w.kit(), "sced.people", gfx.Rect{X: c.X, Y: c.Y + 3, W: 112, H: c.H - 6}, &d.People, scenario.MinPeople, scenario.MaxPeople)
		if d.People != old {
			d.ChooseFrom += d.People - old
		}
		d.ChooseFrom = max(d.People, min(d.People+scenario.MaxExtraPool, d.ChooseFrom))
	}
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Choose from", 120)
		uikit.Stepper(w.kit(), "sced.choose", gfx.Rect{X: c.X, Y: c.Y + 3, W: 112, H: c.H - 6}, &d.ChooseFrom, d.People, d.People+scenario.MaxExtraPool)
	}
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Arrival", 120)
		sel := int(d.Arrival)
		uikit.Segmented(w.kit(), "sced.arrival", gfx.Rect{X: c.X, Y: c.Y + 3, W: min(c.W, 330), H: c.H - 6}, []string{"Drop pods", "Standing", "Walk in"}, &sel, nil)
		d.Arrival = scenario.Arrival(sel)
	}
	if row, ok := f.Row(edRowH); ok {
		c := uikit.RowLabel(w.kit(), row, "Tech level", 120)
		sel := int(d.Tech)
		uikit.Segmented(w.kit(), "sced.tech", gfx.Rect{X: c.X, Y: c.Y + 3, W: min(c.W, 330), H: c.H - 6}, []string{"Neolithic", "Industrial", "Spacer"}, &sel, nil)
		d.Tech = faction.Tech(sel)
	}
	f.Gap(8)

	// Optional parts.
	f.SectionTitle(w.sh, "Optional parts")
	partProbs := map[int]bool{}
	for _, p := range probs {
		if p.Part >= 0 && p.Severity == scenario.Error {
			partProbs[p.Part] = true
		}
	}
	for i := 0; i < len(d.Parts); i++ {
		if w.partCard(f, ng, ed, i, partProbs[i]) {
			break // the list changed: lay out again next frame
		}
	}
	ng.setFormHeight("sced.form", f.Content())

	if ed.reveal >= 0 {
		if y, ok := ed.cardY[ed.reveal]; ok {
			end := ed.cardEnd[ed.reveal]
			w.kit().ScrollReveal("sced.form", view.H, y, min(max(end, y+edCardH), y+view.H))
		}
		ed.reveal = -1
	}
}

// partCard lays out part i. It reports the part list changed (a move or a
// delete), so the caller stops this frame's layout.
func (w editorPage) partCard(f *uikit.Form, ng *Draft, ed *editorState, i int, bad bool) (changed bool) {
	u := w.kit()
	p := &ed.Def.Parts[i]
	top := f.Height()
	ed.cardY[i] = top
	row, ok := f.Row(edCardH)
	expanded := ed.Open[i]
	if ok {
		btn := float32(28)
		hdr := gfx.Rect{X: row.X, Y: row.Y + 2, W: row.W - 3*(btn+4), H: edCardH - 4}
		toggle := func() { ed.Open[i] = !ed.Open[i]; w.kit().OK() }
		wd := uikit.Widget{ID: "sced.card." + uikit.Num(i), R: hdr, Label: p.Kind.Label(), Click: toggle, Focusable: true, Tip: p.Kind.Label() + "\n" + p.Kind.Desc()}
		if u.Add(wd) && (w.kit().Press(hdr) || w.kit().KeyAct(wd.ID)) {
			toggle()
		}
		sum := p.Summary()
		u.Queue(func(dst *ebiten.Image) {
			fill := uikit.Btn
			if u.Hot == wd.ID {
				fill = uikit.BtnHover
			}
			uikit.FillRect(dst, hdr, fill)
			bc := uikit.BtnBorder
			if bad {
				bc = uikit.Danger
			}
			uikit.StrokeRect(dst, hdr, bc)
			ty := float64(hdr.Y) + (float64(hdr.H)-gfx.LineH(gfx.Label))/2
			cx, cy := hdr.X+14, hdr.Y+hdr.H/2
			if expanded {
				uikit.FillTri(dst, cx-4, cy-2, cx+4, cy-2, cx, cy+3)
			} else {
				uikit.FillTri(dst, cx-2, cy-4, cx-2, cy+4, cx+3, cy)
			}
			ts := gfx.Label
			if bad {
				ts.Color = uikit.Danger
			}
			title := fmt.Sprintf("%d. %s", i+1, p.Kind.Label())
			gfx.Text(dst, title, float64(hdr.X+28), ty, ts)
			if !expanded {
				tw, _ := gfx.Measure(title, ts)
				ss := uikit.DimText()
				gfx.Text(dst, uikit.FitText(sum, ss, hdr.W-34-float32(tw)-10), float64(hdr.X+28)+tw+10, ty, ss)
			}
		})
		bx := hdr.Right() + 4
		uikit.StyledButton(w.kit(), "sced.up."+uikit.Num(i), gfx.Rect{X: bx, Y: hdr.Y, W: btn, H: hdr.H}, "^", uikit.BtnOpts{Disabled: i == 0, Reason: "Already first.", Tip: "Move up"}, func() {
			ed.Def.Parts[i], ed.Def.Parts[i-1] = ed.Def.Parts[i-1], ed.Def.Parts[i]
			ed.Open[i], ed.Open[i-1] = ed.Open[i-1], ed.Open[i]
			changed = true
		})
		uikit.StyledButton(w.kit(), "sced.down."+uikit.Num(i), gfx.Rect{X: bx + btn + 4, Y: hdr.Y, W: btn, H: hdr.H}, "v", uikit.BtnOpts{Disabled: i == len(ed.Def.Parts)-1, Reason: "Already last.", Tip: "Move down"}, func() {
			ed.Def.Parts[i], ed.Def.Parts[i+1] = ed.Def.Parts[i+1], ed.Def.Parts[i]
			ed.Open[i], ed.Open[i+1] = ed.Open[i+1], ed.Open[i]
			changed = true
		})
		// The button ids are the spec's: sced.up.<i>, sced.down.<i>, sced.del.<i>.
		uikit.StyledButton(w.kit(), "sced.del."+uikit.Num(i), gfx.Rect{X: bx + 2*(btn+4), Y: hdr.Y, W: btn, H: hdr.H}, "x", uikit.BtnOpts{Style: uikit.BtnDanger, Tip: "Remove this part"}, func() {
			ed.Def.Parts = append(ed.Def.Parts[:i], ed.Def.Parts[i+1:]...)
			for k := i; k < len(ed.Def.Parts)+1; k++ {
				ed.Open[k] = ed.Open[k+1]
			}
			delete(ed.Open, len(ed.Def.Parts))
			changed = true
		})
	}
	if expanded && !changed {
		w.partControls(f, ng, ed, i, p)
	}
	if expanded {
		f.Gap(4)
	}
	f.Gap(4)
	ed.cardEnd[i] = f.Height()
	if ok {
		barY, barH, barX := f.View.Y+top-f.Off, f.Height()-top-4, f.X-3
		u.Queue(func(dst *ebiten.Image) {
			uikit.FillRect(dst, gfx.Rect{X: barX, Y: barY, W: 2, H: barH}, uikit.PanelBorder)
		})
	}
	return changed
}

// ctl is a labelled control row: it returns the control's rectangle.
func (w editorPage) ctl(f *uikit.Form, label string) (gfx.Rect, bool) {
	row, ok := f.Row(edRowH)
	if !ok {
		return gfx.Rect{}, false
	}
	row.X += 14
	row.W -= 14
	c := uikit.RowLabel(w.kit(), row, label, 118)
	return gfx.Rect{X: c.X, Y: c.Y + 3, W: c.W, H: c.H - 6}, true
}

// slider is a percent slider with its value at the right.
func (w editorPage) pctSlider(id string, c gfx.Rect, v *int, lo, hi int, title string) {
	sr := gfx.Rect{X: c.X, Y: c.Y - 1, W: max(60, c.W-54), H: 20}
	w.sliderInt(id, sr, v, uikit.SliderSpec{Min: float64(lo), Max: float64(hi), Step: 5, Title: title,
		Format: func(x float64) string { return fmt.Sprintf("%d%%", int(x)) }})
	val := *v
	w.kit().Label(sr.Right()+10, c.Y+(c.H-float32(gfx.LineH(gfx.Body)))/2, fmt.Sprintf("%d%%", val), gfx.Body)
}

// partControls declares the controls of an expanded part.
func (w editorPage) partControls(f *uikit.Form, ng *Draft, ed *editorState, i int, p *scenario.Part) {
	id := func(s string) string { return "sced.p." + uikit.Num(i) + "." + s }
	d := &ed.Def
	chance := func(label string) {
		if c, ok := w.ctl(f, label); ok {
			v := int(p.Chance)
			w.pctSlider(id("chance"), c, &v, scenario.MinChance, scenario.MaxChance, "Chance")
			p.Chance = uint8(v)
		}
	}
	itemRows := func() {
		if c, ok := w.ctl(f, "Item"); ok {
			uikit.Choice(w.kit(), id("item"), c, "Item", edItemNames, IndexWhere(len(EdItems), func(k int) bool { return EdItems[k] == p.Item }), "",
				func(k int) { p.Item = EdItems[k] })
		}
		if c, ok := w.ctl(f, "Quantity"); ok {
			q := int(p.Qty)
			hi := scenario.MaxQty
			if p.Item.IsWeapon() {
				hi = scenario.MaxWeaponQty
			}
			w.ngNum(id("qty"), gfx.Rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &q, scenario.MinQty, hi)
			p.Qty = int32(min(q, 1<<30))
		}
	}
	incidentRow := func() {
		if c, ok := w.ctl(f, "Incident"); ok {
			uikit.Choice(w.kit(), id("incident"), c, "Incident", edIncNames, IndexWhere(len(edIncidents), func(k int) bool { return edIncidents[k].Key == p.Incident }), "",
				func(k int) { p.Incident = edIncidents[k].Key })
		}
	}
	switch p.Kind {
	case scenario.PartForcedTrait:
		if c, ok := w.ctl(f, "Trait"); ok {
			uikit.Choice(w.kit(), id("trait"), c, "Trait", edTraitNames, IndexWhere(len(edTraits), func(k int) bool { return edTraits[k].ID == p.Trait }), "",
				func(k int) { p.Trait = edTraits[k].ID })
		}
		if c, ok := w.ctl(f, "Who gets it"); ok {
			uikit.Choice(w.kit(), id("who"), c, "Who gets the trait", edWhoNames, int(p.Who), "", func(k int) { p.Who = scenario.Who(k) })
		}
		if p.Who == scenario.WhoNRandom {
			if c, ok := w.ctl(f, "How many"); ok {
				n := int(p.N)
				uikit.Stepper(w.kit(), id("n"), gfx.Rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, max(1, d.People))
				p.N = uint8(n)
			}
		}
		chance("Chance")
	case scenario.PartAgeRange:
		if c, ok := w.ctl(f, "Youngest"); ok {
			v := int(p.MinAge)
			uikit.Stepper(w.kit(), id("min"), gfx.Rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &v, scenario.MinAge, scenario.MaxAge)
			p.MinAge = uint8(v)
		}
		if c, ok := w.ctl(f, "Oldest"); ok {
			v := int(p.MaxAge)
			uikit.Stepper(w.kit(), id("max"), gfx.Rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &v, scenario.MinAge, scenario.MaxAge)
			p.MaxAge = uint8(v)
		}
	case scenario.PartForcedCondition:
		if c, ok := w.ctl(f, "Condition"); ok {
			uikit.Choice(w.kit(), id("cond"), c, "Condition", edPawnNames, IndexWhere(len(edPawnConds), func(k int) bool { return edPawnConds[k] == p.PawnCond }), "",
				func(k int) { p.PawnCond = pawngen.Condition(edPawnConds[k]) })
		}
		chance("Chance")
	case scenario.PartNoClothes:
		if row, ok := f.Row(edRowH); ok {
			w.kit().Label(row.X+14, row.Y+6, "Colonists arrive with nothing to wear. No options.", uikit.DimText())
		}
	case scenario.PartStartItem:
		itemRows()
	case scenario.PartScatterItem:
		itemRows()
		if c, ok := w.ctl(f, "Piles"); ok {
			n := int(p.N)
			uikit.Stepper(w.kit(), id("piles"), gfx.Rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, 20)
			p.N = uint8(n)
		}
		if c, ok := w.ctl(f, "Where"); ok {
			sel := 1
			if p.Near {
				sel = 0
			}
			uikit.Segmented(w.kit(), id("near"), gfx.Rect{X: c.X, Y: c.Y, W: min(c.W, 260), H: c.H}, []string{"Near the start", "Anywhere"}, &sel, nil)
			p.Near = sel == 0
		}
	case scenario.PartStartAnimal:
		if c, ok := w.ctl(f, "Random pet"); ok {
			w.kit().Check(id("random"), gfx.Rect{X: c.X, Y: c.Y, W: 220, H: c.H}, "A cat, dog or guinea pig", &p.Random, uikit.CheckOpts{})
		}
		if c, ok := w.ctl(f, "Species"); ok {
			dis := ""
			if p.Random {
				dis = "A random pet is used."
			}
			uikit.Choice(w.kit(), id("species"), c, "Species", edAnimalName, IndexWhere(len(edAnimals), func(k int) bool { return edAnimals[k] == p.Species }), dis,
				func(k int) { p.Species = animal.Species(edAnimals[k]) })
		}
		if c, ok := w.ctl(f, "Count"); ok {
			n := int(p.N)
			uikit.Stepper(w.kit(), id("count"), gfx.Rect{X: c.X, Y: c.Y, W: 112, H: c.H}, &n, 1, scenario.MaxAnimals)
			p.N = uint8(n)
		}
	case scenario.PartCondition:
		if c, ok := w.ctl(f, "Condition"); ok {
			uikit.Choice(w.kit(), id("cond"), c, "Permanent condition", edCondNames, IndexWhere(len(edConditions), func(k int) bool { return edConditions[k] == p.Cond }), "",
				func(k int) { p.Cond = edConditions[k] })
		}
	case scenario.PartScheduledIncident:
		incidentRow()
		if c, ok := w.ctl(f, "First day"); ok {
			v := int(p.FirstDay)
			w.ngNum(id("first"), gfx.Rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &v, scenario.MinFirstDay, scenario.MaxFirstDay)
			p.FirstDay = uint16(min(v, 65535))
		}
		if c, ok := w.ctl(f, "Repeat every"); ok {
			v := int(p.EveryDays)
			w.ngNum(id("every"), gfx.Rect{X: c.X, Y: c.Y, W: 90, H: c.H}, &v, 0, scenario.MaxEveryDays)
			p.EveryDays = uint16(min(v, 65535))
			w.kit().Label(c.X+100, c.Y+(c.H-float32(gfx.LineH(gfx.Body)))/2, "days (0 = once)", uikit.DimText())
		}
	case scenario.PartDisabledIncident:
		incidentRow()
	case scenario.PartDisallowBuilding:
		if c, ok := w.ctl(f, "Building"); ok {
			uikit.Choice(w.kit(), id("building"), c, "Building", edBuildNames, IndexWhere(len(edBuildings), func(k int) bool { return edBuildings[k].Kind == p.Building }), "",
				func(k int) { p.Building = edBuildings[k].Kind })
		}
	case scenario.PartResearch:
		if c, ok := w.ctl(f, "Project"); ok {
			uikit.Choice(w.kit(), id("project"), c, "Research project", edProjNames, int(p.Project), "", func(k int) { p.Project = research.Project(k) })
		}
	case scenario.PartResearchCost:
		if c, ok := w.ctl(f, "Cost"); ok {
			v := int(p.Percent)
			w.pctSlider(id("percent"), c, &v, scenario.MinCostPct, scenario.MaxCostPct, "Research cost")
			p.Percent = uint16(v)
		}
	case scenario.PartIntro:
		if row, ok := f.Row(120); ok {
			row.X += 14
			row.W -= 14
			w.kit().TextField(id("text"), row.Inset(2), &p.Text, uikit.TextFieldSpec{Multiline: true, MaxLen: scenario.MaxIntro, Placeholder: "Shown once after landing"})
		}
	}
}

// ---- add part, save, load, done ------------------------------------------------

// openAddPart opens the categorised list of part kinds.
func (w editorPage) openAddPart() {
	cats := scenario.Catalog()
	const headH, kindH = 28, 52
	total := float32(0)
	for _, c := range cats {
		total += headH + float32(len(c.Kinds))*kindH
	}
	w.kit().Push(uikit.Modal{
		ID: "sced.addpart", Title: "Add part", W: 600, H: 520,
		Draw: func(body gfx.Rect) {
			u := w.kit()
			area := w.kit().ScrollArea("modal.sced.addpart.scroll", body, total, uikit.ScrollOpts{Step: kindH})
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
			u.Queue(func(dst *ebiten.Image) {
				sub := gfx.ClipTo(dst, area.View)
				for _, h := range heads {
					hs := gfx.Label
					hs.Color = uikit.Heading
					yy := area.View.Y + h.y - area.Off
					gfx.Text(sub, h.name, float64(area.View.X+4), float64(yy)+6, hs)
					uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: yy + headH - 3, W: area.View.W, H: 1}, uikit.PanelBorder)
				}
				for _, k := range kinds {
					row := gfx.Rect{X: area.View.X, Y: area.View.Y + k.y - area.Off, W: area.View.W, H: kindH}
					if u.Hot == "sced.addkind."+k.info.Kind.Key() {
						uikit.FillRect(sub, row.Inset(1), uikit.BtnHover)
					}
					gfx.Text(sub, k.info.Label, float64(row.X+12), float64(row.Y)+6, gfx.Label)
					ds := gfx.Small
					ds.Color = uikit.TextDim
					for n, l := range gfx.WrapPx(k.info.Desc, ds, float64(row.W-24)) {
						if n < 2 {
							gfx.Text(sub, l, float64(row.X+12), float64(row.Y)+6+gfx.LineH(gfx.Label)+float64(n)*gfx.LineH(ds), ds)
						}
					}
				}
			})
			for _, k := range kinds {
				k := k
				row := gfx.Rect{X: area.View.X, Y: area.View.Y + k.y - area.Off, W: area.View.W, H: kindH}
				hit := row.Intersect(area.View)
				if hit.Empty() {
					continue
				}
				add := func() { w.addPart(k.info.Kind) }
				wd := uikit.Widget{ID: "sced.addkind." + k.info.Kind.Key(), R: hit, Label: k.info.Label, Click: add}
				if u.Add(wd) && w.kit().Press(hit) {
					add()
				}
			}
		},
		Buttons: []uikit.ModalButton{{ID: "cancel", Label: "Cancel", Closes: true}},
	})
}

// addPart appends a part of kind k, opened and in view.
func (w editorPage) addPart(k scenario.PartKind) {
	ng := w.draft()
	if ng.Ed == nil {
		return
	}
	ed := ng.Ed
	ed.Def.Parts = append(ed.Def.Parts, scenario.NewPart(k))
	n := len(ed.Def.Parts) - 1
	ed.Open[n] = true
	ed.reveal = n
	w.kit().CloseModal("sced.addpart")
}

// editorSave writes the scenario to the scenarios folder, asking first
// before it replaces a file it did not write itself.
func (w editorPage) editorSave() {
	ng := w.draft()
	ed := ng.Ed
	st := w.store()
	if ed == nil || st == nil {
		return
	}
	def := CloneDef(ed.Def)
	def.Builtin, def.Key = false, ""
	data, err := scenario.Encode(def)
	if err != nil {
		w.sh.Hint("cannot save: " + err.Error())
		return
	}
	name := SlugOf(def.Name)
	write := func() {
		if err := st.Save(name, data); err != nil {
			w.sh.Hint("save failed: " + err.Error())
			return
		}
		ed.Saved, ed.SavedAs = data, name
		w.sh.Hint("saved " + name)
		ng.RefreshScenarios(w.store(), w.Unsaved)
	}
	files, _ := st.List()
	exists := false
	for _, f := range files {
		if f.Name == name {
			exists = true
		}
	}
	if exists && ed.SavedAs != name {
		w.kit().Confirm("sced.overwrite", "Overwrite scenario?", fmt.Sprintf("A scenario file named \"%s\" already exists. Replace it?", name), "Overwrite", true, write)
		return
	}
	write()
}

// editorLoad lists the scenario files; picking one replaces the edit.
func (w editorPage) editorLoad() {
	st := w.store()
	if st == nil {
		return
	}
	files, err := st.List()
	ng := w.draft()
	if ng.Ed != nil {
		ng.Ed.LoadErr = ""
		if err != nil {
			ng.Ed.LoadErr = "Can't read the scenarios folder: " + err.Error()
		}
	}
	cols := []uikit.TableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return files[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(files[x].Name) < strings.ToLower(files[y].Name) }},
		{ID: "mod", Title: "Modified", W: 150, Cell: func(i int) string { return files[i].Modified },
			Less: func(x, y int) bool { return files[x].Modified < files[y].Modified }},
	}
	w.kit().Push(uikit.Modal{
		ID: "sced.loadfile", Title: "Load scenario", W: 560, H: 440,
		Draw: func(body gfx.Rect) {
			ed := w.draft().Ed
			if ed == nil {
				return
			}
			tb := body
			if ed.LoadErr != "" {
				ls := gfx.Body
				ls.Color = uikit.Danger
				h := uikit.TextLines(w.kit(), body.X, body.Y, body.W, ed.LoadErr, ls)
				_, tb = body.SplitV(h + 8)
			}
			if len(files) == 0 {
				uikit.TextLines(w.kit(), tb.X, tb.Y, tb.W, "No scenario files yet. Save one first.", uikit.DimText())
				return
			}
			row := -1
			if w.kit().Table("modal.sced.load.table", tb, cols, len(files), &row, 28) && row >= 0 {
				w.loadScenarioFile(files[row].Name)
			}
		},
		Buttons: []uikit.ModalButton{{ID: "cancel", Label: "Cancel", Closes: true}},
	})
}

// loadScenarioFile replaces the edited scenario with a file's.
func (w editorPage) loadScenarioFile(name string) {
	ng := w.draft()
	ed := ng.Ed
	st := w.store()
	if ed == nil || st == nil {
		return
	}
	data, err := st.Load(name)
	var d scenario.Def
	if err == nil {
		d, err = scenario.Decode(data)
	}
	if err != nil {
		ed.LoadErr = fmt.Sprintf("%s: %v", name, err)
		return
	}
	d.Builtin, d.Key = false, ""
	ed.Def = d
	ed.Saved, ed.SavedAs = encDef(d), name
	ed.Open = map[int]bool{}
	ed.reveal = -1
	ed.LoadErr = ""
	w.kit().CloseModal("sced.loadfile")
}

// editorDone hands the edited scenario back to Choose Scenario: a file
// when it was saved and not changed since, otherwise an unsaved scenario
// that lasts until the game quits.
func (w editorPage) editorDone() {
	ng := w.draft()
	ed := ng.Ed
	if ed == nil || scenario.HasErrors(ed.Def.Validate()) {
		return
	}
	def := CloneDef(ed.Def)
	def.Builtin, def.Key = false, ""
	key := ""
	if ed.Saved != nil && bytes.Equal(encDef(def), ed.Saved) {
		key = "file." + ed.SavedAs
	} else {
		idx := -1
		if strings.HasPrefix(ed.source, "unsaved.") {
			if _, err := fmt.Sscanf(ed.source, "unsaved.%d", &idx); err != nil || idx < 0 || idx >= len(w.Unsaved) {
				idx = -1
			}
		}
		if idx >= 0 {
			w.Unsaved[idx] = def
		} else {
			w.Unsaved = append(w.Unsaved, def)
			idx = len(w.Unsaved) - 1
		}
		key = fmt.Sprintf("unsaved.%d", idx)
	}
	w.pop()
	ng.Search = ""
	ng.RefreshScenarios(w.store(), w.Unsaved)
	ng.selectKey(key)
}
