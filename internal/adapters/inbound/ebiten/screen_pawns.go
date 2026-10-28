package ebiten

import (
	"fmt"
	"strings"
	"unicode"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/platform/input"
)

// Create Characters (NG-1 §2.9): the roster, one candidate's details and
// the team's strengths and gaps. The candidates come from the application's
// game.CandidatePool; this screen keeps only what is being typed.

// candidatePool is the consumer-owned view of game.CandidatePool, so the
// screen's tests can run on a stand-in.
type candidatePool interface {
	Views() []game.CandidateView
	Reroll(i int, keepName bool)
	RandomizeAll()
	Rename(i int, first, nick, last string) error
	Move(i, j int)
	Swap(i, j int)
	Team() game.TeamReport
	Selected() []pawngen.Pawn
}

var _ candidatePool = (*game.CandidatePool)(nil)

// rosterDrag is a roster row being dragged.
type rosterDrag struct {
	on, moved bool
	from      int
	y0        int
}

// Layout metrics.
const (
	pawnRosterW   = 240
	pawnTeamW     = 280
	pawnRowH      = 56
	pawnHeadH     = 24
	pawnThumbW    = 40
	pawnThumbH    = 48
	pawnBigW      = 160
	pawnBigH      = 190
	pawnSkillHdrH = 22
	pawnRosterID  = "pawns.roster"
	pawnDragSlop  = 6
)

// nameRune is a character a name may hold.
func nameRune(r rune) bool { return unicode.IsLetter(r) || r == ' ' || r == '\'' || r == '-' }

// poolKeyFor is what the candidate pool is made from: the setup seed and
// the scenario's people rules. A new key makes a new pool.
func (a *App) poolKeyFor(ng *newGameState) string {
	fx := ng.scenario.Apply()
	return fmt.Sprintf("%d|%d|%d|%v", setupSeedOf(ng), fx.People, fx.ChooseFrom, fx.Rules)
}

// ensurePool makes the candidate pool when it is missing or stale.
func (a *App) ensurePool(ng *newGameState) {
	key := a.poolKeyFor(ng)
	if ng.pool != nil && ng.poolKey == key {
		return
	}
	ng.pool, ng.poolKey = game.NewCandidatePool(setupSeedOf(ng), ng.scenario.Apply()), key
	ng.pawnSel, ng.namesOf, ng.nameErr = 0, -1, ""
	ng.drag = rosterDrag{}
}

// undoCharacters restores the pool as it was on entry (the pool is
// deterministic from its seed).
func (a *App) undoCharacters(ng *newGameState) {
	ng.pool = game.NewCandidatePool(setupSeedOf(ng), ng.scenario.Apply())
	ng.namesOf, ng.nameErr = -1, ""
}

// startReason is why Start is disabled ("" it is enabled).
func (ng *newGameState) startReason() string {
	if ng.nameErr != "" {
		return ng.nameErr
	}
	if ng.pool == nil || len(ng.pool.Selected()) == 0 {
		return "There is nobody to send. The scenario needs at least one colonist."
	}
	return ""
}

func (a *App) pawnsUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	if ng.load != nil {
		a.loadingStep()
		a.uiSt().Suppress = true
		return
	}
	a.ensurePool(ng)
	views := ng.pool.Views()
	nSel := 0
	for _, v := range views {
		if v.Selected {
			nSel++
		}
	}
	if ng.pawnSel >= len(views) {
		ng.pawnSel = max(0, len(views)-1)
	}
	_, _, content, footer := dialogFrame(sw, sh)
	roster, rest := content.SplitH(pawnRosterW)
	rest = rect{X: rest.X + dialogPad, Y: rest.Y, W: rest.W - dialogPad, H: rest.H}
	detail, team := rest.SplitH(rest.W - pawnTeamW - dialogPad)
	team.X, team.W = team.X+dialogPad, pawnTeamW

	a.pawnsRoster(ng, roster, views, nSel)
	a.pawnsDetail(ng, detail, views)
	a.pawnsTeam(ng, team)
	a.pawnsFooter(ng, footer)
}

// ---- the footer ----------------------------------------------------------

func (a *App) pawnsFooter(ng *newGameState, footer rect) {
	back := footerBack(footer)
	a.ngButton(ids.WizardBack, back, "Back", btnOpts{}, a.goBack)
	x := back.Right() + 12
	a.ngButton("pawns.reroll", rect{X: x, Y: back.Y, W: 120, H: footerBtnH}, "Randomize", btnOpts{
		Tip: "Randomize\nRoll this colonist again. An edited name is kept; hold Shift to roll a new name too."},
		func() { a.rerollSelected(ng) })
	x += 128
	a.ngButton("pawns.rerollAll", rect{X: x, Y: back.Y, W: 130, H: footerBtnH}, "Randomize all", btnOpts{Tip: "Randomize all\nRoll every candidate again."},
		func() {
			a.uiConfirm("pawns.rerollAll.confirm", "Randomize everyone?", "Every candidate is rolled again. Names you edited are kept.", "Randomize", false, func() {
				ng.pool.RandomizeAll()
				ng.namesOf = -1
			})
		})
	x += 138
	a.ngButton("pawns.reset", rect{X: x, Y: back.Y, W: 130, H: footerBtnH}, "Undo changes", btnOpts{Tip: "Undo changes\nPut the candidates back as they were when you came here."},
		func() { a.undoCharacters(ng) })

	reason := ng.startReason()
	start := func() {
		if ng.startReason() == "" {
			a.beginLoading()
		}
	}
	a.ngButton(ids.WizardStart, footerNext(footer), "Start", btnOpts{Style: btnPrimary, Disabled: reason != "", Reason: reason}, start)
	if a.in.Fired(input.Accept) && reason == "" && !a.uiModalOpen() && !a.uiTextEditing() {
		start()
	}
	if reason != "" {
		st := tsBody
		st.Color = uiWarning
		a.uiLabel(footerNext(footer).X-textW(reason, st)-14, footer.Y+(footer.H-float32(lineH(st)))/2, reason, st)
	}
	if a.sessionErr != "" {
		st := tsBody
		st.Color = uiDanger
		a.uiLabel(footer.X+dialogPad, footer.Y-float32(lineH(st))-4, "Start failed: "+a.sessionErr, st)
	}
}

// rerollSelected rolls the highlighted candidate again (Shift: new name too).
func (a *App) rerollSelected(ng *newGameState) {
	ng.pool.Reroll(ng.pawnSel, !a.in.Shift())
	ng.namesOf = -1
}

// ---- the roster --------------------------------------------------------------

// rosterY is the content y of roster position pos for nSel selected
// candidates: a header, the Selected rows, a header, the rest.
func rosterY(pos, nSel int) float32 {
	if pos < nSel {
		return pawnHeadH + float32(pos)*pawnRowH
	}
	return pawnHeadH + float32(nSel)*pawnRowH + pawnHeadH + float32(pos-nSel)*pawnRowH
}

// rosterTarget is the roster position a drop at content y lands on.
func rosterTarget(yc float32, n, nSel int) int {
	if n == 0 {
		return 0
	}
	best, bestD := 0, float32(1e9)
	for p := 0; p < n; p++ {
		y := rosterY(p, nSel)
		if yc >= y && yc < y+pawnRowH {
			return p
		}
		if d := absf(yc - (y + pawnRowH/2)); d < bestD {
			best, bestD = p, d
		}
	}
	return best
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) pawnsRoster(ng *newGameState, r rect, views []game.CandidateView, nSel int) {
	u := a.uiSt()
	a.uiPanel(r)
	btns := float32(2*28 + 6 + 8)
	listR := rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8 - btns}
	n := len(views)
	nLeft := n - nSel
	total := rosterY(n, nSel) + 4
	if nLeft == 0 {
		total = rosterY(nSel, nSel) + pawnHeadH
	}
	area := a.uiScrollArea(pawnRosterID, listR, total, scrollOpts{Step: pawnRowH})
	rowAt := func(pos int) rect {
		return rect{X: area.View.X, Y: listR.Y + rosterY(pos, nSel) - area.Off, W: area.View.W, H: pawnRowH}
	}

	mx, my := a.currentCursor()
	for pos := range views {
		pos := pos
		rr := rowAt(pos)
		hit := rr.Intersect(area.View)
		if hit.Empty() {
			continue
		}
		sel := func() { ng.pawnSel = pos; u.Focus = pawnRosterID }
		w := uiWidget{ID: optID("pawns.row", pos), Label: views[pos].Nick, R: hit, Click: func() { a.uiOK(); sel() },
			Tip: views[pos].Nick + "\n" + candidateLine(views[pos])}
		if u.Add(w) && a.uiPress(hit) {
			sel()
			ng.drag = rosterDrag{on: true, from: pos, y0: my}
		}
	}
	// A drag in progress: it ends on release and moves the candidate.
	if d := &ng.drag; d.on {
		if absInt(my-d.y0) > pawnDragSlop {
			d.moved = true
		}
		if !a.in.Down(mbLeft) {
			if d.moved {
				to := rosterTarget(float32(my)-listR.Y+area.Off, n, nSel)
				if to != d.from {
					ng.pool.Move(d.from, to)
					ng.pawnSel = to
					ng.namesOf = -1
				}
			}
			*d = rosterDrag{}
		}
	}
	drag := ng.drag
	selPos := ng.pawnSel
	u.Queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		hs := tsLabel
		hs.Color = uiHeading
		drawText(sub, fmt.Sprintf("Selected (%d)", nSel), float64(area.View.X+6), float64(listR.Y-area.Off)+3, hs)
		drawText(sub, fmt.Sprintf("Left behind (%d)", nLeft), float64(area.View.X+6), float64(listR.Y+rosterY(nSel, nSel)-pawnHeadH-area.Off)+3, hs)
		for pos, v := range views {
			rr := rowAt(pos)
			if rr.Bottom() < area.View.Y || rr.Y > area.View.Bottom() {
				continue
			}
			a.paintRosterRow(sub, rr, v, pos == selPos, u.Hot == optID("pawns.row", pos), pos >= nSel, drag.on && drag.moved && drag.from == pos)
		}
		if drag.on && drag.moved {
			to := rosterTarget(float32(my)-listR.Y+area.Off, n, nSel)
			ty := listR.Y + rosterY(to, nSel) - area.Off
			if to > drag.from {
				ty += pawnRowH
			}
			fillRect(sub, rect{X: area.View.X, Y: ty - 1, W: area.View.W, H: 3}, uiAccent)
			ghost := rect{X: area.View.X, Y: float32(my) - pawnRowH/2, W: area.View.W, H: pawnRowH}
			a.paintRosterRow(dst, ghost, views[drag.from], true, false, false, false)
		}
	})
	_ = mx

	// Up, down and swap under the list.
	by := r.Bottom() - btns + 4
	half := (r.W - 8 - 6) / 2
	i := ng.pawnSel
	upR, dnR := "", ""
	if i <= 0 {
		upR = "Already first."
	}
	if i >= n-1 {
		dnR = "Already last."
	}
	a.ngButton("pawns.up", rect{X: r.X + 4, Y: by, W: half, H: 28}, "Up", btnOpts{Disabled: upR != "", Reason: upR, Tip: "Move up\nMove this candidate up the roster."},
		func() { ng.pool.Move(i, i-1); ng.pawnSel = i - 1; ng.namesOf = -1 })
	a.ngButton("pawns.down", rect{X: r.X + 4 + half + 6, Y: by, W: half, H: 28}, "Down", btnOpts{Disabled: dnR != "", Reason: dnR, Tip: "Move down\nMove this candidate down the roster."},
		func() { ng.pool.Move(i, i+1); ng.pawnSel = i + 1; ng.namesOf = -1 })
	swapLabel, swapReason := "Swap with left-behind...", ""
	if i >= nSel {
		swapLabel = "Swap with selected..."
	}
	if nSel == 0 || nLeft == 0 {
		swapReason = "There is nobody to swap with."
	}
	a.ngButton("pawns.swap", rect{X: r.X + 4, Y: by + 34, W: r.W - 8, H: 28}, swapLabel, btnOpts{Disabled: swapReason != "", Reason: swapReason},
		func() { a.pushSwap(ng, views, nSel) })
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// candidateLine is the roster row's second line.
func candidateLine(v game.CandidateView) string {
	t := v.Adulthood
	if t == "" || t == "—" {
		t = v.Childhood
	}
	return t
}

func (a *App) paintRosterRow(dst *ebiten.Image, rr rect, v game.CandidateView, selected, hot, left, ghosted bool) {
	switch {
	case selected:
		fillRect(dst, rr.Inset(1), lerpRGBA(uiPanel, uiBtnSelected, 0.8))
		strokeRect(dst, rr.Inset(1), uiAccent)
	case hot:
		fillRect(dst, rr.Inset(1), uiBtnHover)
	}
	if ghosted {
		return
	}
	pr := rect{X: rr.X + 8, Y: rr.Y + (rr.H-pawnThumbH)/2, W: pawnThumbW, H: pawnThumbH}
	fillRect(dst, pr, uiField)
	if !drawLookPortrait(dst, a.art, v.Look, float64(pr.X), float64(pr.Y), float64(pr.W), float64(pr.H)) {
		drawSilhouette(dst, pr, "")
	}
	nm := tsLabel
	if left {
		nm.Color = uiTextDim
	}
	drawText(dst, fitText(v.Nick, nm, rr.W-pawnThumbW-24), float64(rr.X+pawnThumbW+18), float64(rr.Y)+10, nm)
	ts := tsSmall
	ts.Color = uiTextDim
	drawText(dst, fitText(candidateLine(v), ts, rr.W-pawnThumbW-24), float64(rr.X+pawnThumbW+18), float64(rr.Y)+10+lineH(nm)+2, ts)
}

// pushSwap lists the candidates of the other group; picking one swaps.
func (a *App) pushSwap(ng *newGameState, views []game.CandidateView, nSel int) {
	i := ng.pawnSel
	var idx []int
	var labels []string
	for p, v := range views {
		if (i < nSel) == (p >= nSel) {
			idx = append(idx, p)
			labels = append(labels, v.Nick+", "+candidateLine(v))
		}
	}
	a.pushPicker("pawns.swap", "Swap with", labels, -1, func(k int) {
		ng.pool.Swap(i, idx[k])
		ng.namesOf = -1
	})
}

// ---- the details ----------------------------------------------------------------

func (a *App) pawnsDetail(ng *newGameState, r rect, views []game.CandidateView) {
	u := a.uiSt()
	a.uiPanel(r)
	in := r.Inset(12)
	if len(views) == 0 {
		a.uiLabel(in.X, in.Y, "No candidates.", dimText())
		return
	}
	v := views[ng.pawnSel]
	a.syncNames(ng, v)

	left := rect{X: in.X, Y: in.Y, W: in.W - pawnBigW - 16, H: in.H}
	// The portrait.
	pr := rect{X: in.Right() - pawnBigW, Y: in.Y, W: pawnBigW, H: pawnBigH}
	u.Queue(func(dst *ebiten.Image) {
		fillRect(dst, pr, uiField)
		strokeRect(dst, pr, uiPanelBorder)
		if !drawLookPortrait(dst, a.art, v.Look, float64(pr.X), float64(pr.Y), float64(pr.W), float64(pr.H)) {
			drawSilhouette(dst, pr, v.Nick)
		}
	})

	// Names.
	y := left.Y
	fw := (left.W - 2*10) / 3
	labels := []struct {
		id, text, tip string
	}{
		{"pawns.first", "First", ""},
		{"pawns.nick", "Nickname (i)", "Nickname\nNickname is what everyone calls them."},
		{"pawns.last", "Last", ""},
	}
	changed := false
	for k, l := range labels {
		fx := left.X + float32(k)*(fw+10)
		a.uiLabel(fx, y, l.text, dimText())
		if l.tip != "" {
			a.uiTip(l.id+".tip", rect{X: fx, Y: y, W: fw, H: 16}, l.tip)
		}
		ch, _ := a.uiTextField(l.id, rect{X: fx, Y: y + 18, W: fw, H: 28}, &ng.names[k], textFieldSpec{MaxLen: game.MaxNameRunes, Valid: nameRune})
		changed = changed || ch
	}
	if changed {
		if err := ng.pool.Rename(ng.pawnSel, ng.names[0], ng.names[1], ng.names[2]); err != nil {
			ng.nameErr = err.Error()
		} else {
			ng.nameErr = ""
		}
	}
	y += 50
	if ng.nameErr != "" {
		st := tsSmall
		st.Color = uiWarning
		a.uiLabel(left.X, y-2, ng.nameErr, st)
	}
	y += 16

	// Sex, age, build, faction.
	info := fmt.Sprintf("%s, age %d (biological)", v.Sex, v.AgeYears)
	a.uiLabel(left.X, y, info, tsBody)
	y += 20
	badge := faction.DefOf(ng.scenario.Faction)
	if ng.scenario.Faction == faction.Player || badge.Label == "" {
		badge = faction.DefOf(faction.Settlers)
	}
	by := y
	u.Queue(func(dst *ebiten.Image) {
		a.drawEmblem(dst, "ui_icon/faction_"+badge.Key, globeAccent(badge.Type), left.X, by-1, 18)
		st := dimText()
		drawText(dst, fmt.Sprintf("%s. Build: %s.", badge.Label, v.Build), float64(left.X+24), float64(by), st)
	})
	y += 24

	// Backstories.
	a.backstoryLine(left.X, y, left.W, "Childhood", v.Childhood, v.ChildhoodDesc, "pawns.child")
	y += 20
	a.backstoryLine(left.X, y, left.W, "Adulthood", v.Adulthood, v.AdulthoodDesc, "pawns.adult")
	y += 26

	// Traits.
	y = a.traitChips(left.X, y, left.W, v)

	// Incapable of.
	incap, full := "None", ""
	col := uiTextDim
	if len(v.Incapable) > 0 {
		names := make([]string, len(v.Incapable))
		for i, s := range v.Incapable {
			names[i] = s
			if k := strings.Index(s, " ("); k > 0 {
				names[i] = s[:k]
			}
		}
		incap, full, col = strings.Join(names, ", "), strings.Join(v.Incapable, "\n"), uiDanger
	}
	iy := y
	a.uiLabel(left.X, iy, "Incapable of:", tsLabel)
	u.Queue(func(dst *ebiten.Image) {
		st := tsBody
		st.Color = col
		drawText(dst, fitText(incap, st, left.W-110), float64(left.X+104), float64(iy), st)
	})
	if full != "" {
		a.uiTip("pawns.incap.tip", rect{X: left.X, Y: y, W: left.W, H: 18}, "Incapable of\n"+full)
	}
	y += 26

	// Skills, then health at the bottom.
	healthH := float32(34)
	if len(v.Conditions) > 1 {
		healthH += float32(len(v.Conditions)-1) * 16
	}
	skillsTop := max(y, pr.Bottom()+10)
	avail := in.Bottom() - healthH - skillsTop - pawnSkillHdrH
	rowH := min(float32(20), max(float32(15), avail/float32(max(len(v.Skills), 1))))
	a.skillsTable(rect{X: in.X, Y: skillsTop, W: in.W, H: pawnSkillHdrH + rowH*float32(len(v.Skills))}, v, rowH)

	hy := in.Bottom() - healthH + 6
	a.uiLabel(in.X, hy, "Health:", tsLabel)
	text := "No conditions"
	if len(v.Conditions) > 0 {
		text = strings.Join(v.Conditions, "\n")
	}
	st := dimText()
	if len(v.Conditions) > 0 {
		st.Color = uiWarning
	}
	a.uiSt().Queue(func(dst *ebiten.Image) { drawText(dst, text, float64(in.X+64), float64(hy), st) })
}

// syncNames loads the name drafts from the candidate when no field is
// being edited and the highlighted candidate or the pool changed.
func (a *App) syncNames(ng *newGameState, v game.CandidateView) {
	if a.uiTextEditing() {
		return
	}
	want := [3]string{v.First, v.Nick, v.Last}
	if ng.namesOf != ng.pawnSel || ng.names != want {
		ng.names, ng.namesOf, ng.nameErr = want, ng.pawnSel, ""
	}
}

// backstoryLine is "Childhood: Title" with the description as a tooltip.
func (a *App) backstoryLine(x, y, w float32, label, title, desc, id string) {
	if title == "" {
		title = "—"
	}
	a.uiLabel(x, y, label+":", tsLabel)
	st := tsBody
	a.uiSt().Queue(func(dst *ebiten.Image) {
		drawText(dst, fitText(title, st, w-100), float64(x+90), float64(y), st)
	})
	tip := ""
	if title != "—" {
		tip = title + "\n" + desc
	}
	if tip != "" {
		a.uiTip(id, rect{X: x, Y: y, W: w, H: 18}, tip)
	}
}

// traitChips lays the traits out as chips and returns the next y.
func (a *App) traitChips(x, y, w float32, v game.CandidateView) float32 {
	a.uiLabel(x, y+2, "Traits:", tsLabel)
	cx, cy := x+64, y
	if len(v.Traits) == 0 {
		a.uiLabel(cx, y+2, "None", dimText())
		return y + 28
	}
	for i, t := range v.Traits {
		t := t
		cw := textW(t.Label, tsBody) + 18
		if cx+cw > x+w && cx > x+64 {
			cx, cy = x+64, cy+28
		}
		cr := rect{X: cx, Y: cy, W: cw, H: 24}
		tip := t.Label + "\n" + t.Desc
		if t.Effects != "" {
			tip += "\n" + t.Effects
		}
		a.uiTip(fmt.Sprintf("pawns.trait.%d", i), cr, tip)
		a.uiSt().Queue(func(dst *ebiten.Image) {
			fillRect(dst, cr, uiBtn)
			strokeRect(dst, cr, uiBtnBorder)
			st := tsBody
			st.Align = alignCenter
			drawText(dst, t.Label, float64(cr.X+cr.W/2), float64(cr.Y)+(float64(cr.H)-lineH(st))/2, st)
		})
		cx += cw + 6
	}
	return cy + 30
}

// skillLevelText is the level cell: the number, or a dash when incapable.
func skillLevelText(s game.SkillView) string {
	if s.Incapable {
		return "—"
	}
	return fmt.Sprint(s.Level)
}

func passionText(p int) string {
	switch p {
	case 1:
		return "minor passion: learns faster"
	case 2:
		return "major passion: learns much faster"
	}
	return "no passion"
}

// skillTip is a skill row's hover card.
func skillTip(s game.SkillView) string {
	if s.Incapable {
		return s.Label + "\nIncapable. This colonist can't do this work."
	}
	t := fmt.Sprintf("%s: %s (%d)\n", s.Label, s.LevelLabel, s.Level)
	if s.Affects != "" {
		t += "Affects: " + s.Affects + ". "
	}
	if s.SpeedPct > 0 {
		t += fmt.Sprintf("Work speed %d%%. ", s.SpeedPct)
	}
	return t + "Passion: " + passionText(s.Passion) + "."
}

// skillsTable draws the skills with passion glyphs, level numbers and bars.
func (a *App) skillsTable(r rect, v game.CandidateView, rowH float32) {
	u := a.uiSt()
	for i, s := range v.Skills {
		row := rect{X: r.X, Y: r.Y + pawnSkillHdrH + float32(i)*rowH, W: r.W, H: rowH}
		a.uiTip(fmt.Sprintf("pawns.skill.%d", i), row, skillTip(s))
	}
	u.Queue(func(dst *ebiten.Image) {
		hs := tsLabel
		hs.Color = uiHeading
		fillRect(dst, rect{X: r.X, Y: r.Y, W: r.W, H: pawnSkillHdrH}, uiBtn)
		strokeRect(dst, rect{X: r.X, Y: r.Y, W: r.W, H: pawnSkillHdrH}, uiBtnBorder)
		drawText(dst, "Skill", float64(r.X+8), float64(r.Y)+(pawnSkillHdrH-lineH(hs))/2, hs)
		drawText(dst, "Passion", float64(r.X+160), float64(r.Y)+(pawnSkillHdrH-lineH(hs))/2, hs)
		drawText(dst, "Level", float64(r.X+240), float64(r.Y)+(pawnSkillHdrH-lineH(hs))/2, hs)
		for i, s := range v.Skills {
			row := rect{X: r.X, Y: r.Y + pawnSkillHdrH + float32(i)*rowH, W: r.W, H: rowH}
			if i%2 == 1 {
				fillRect(dst, row, uiFade(uiField, 0x28)) // premultiplied wash
			}
			st := tsBody
			if s.Incapable {
				st.Color = uiTextDisabled
			}
			ty := float64(row.Y) + (float64(row.H)-lineH(st))/2
			drawText(dst, s.Label, float64(row.X+8), ty, st)
			if !s.Incapable && s.Passion > 0 {
				key := "ui_icon/passion_minor"
				if s.Passion == 2 {
					key = "ui_icon/passion_major"
				}
				if img := a.art.Image(key, noTint); img != nil {
					drawIcon(dst, img, float64(row.X+166), float64(row.Y)+(float64(row.H)-14)/2, 14)
				} else {
					drawText(dst, strings.Repeat("*", s.Passion), float64(row.X+166), ty, st)
				}
			}
			lt := st
			lt.Align = alignCenter
			drawText(dst, skillLevelText(s), float64(row.X+250), ty, lt)
			if !s.Incapable {
				bar := rect{X: row.X + 280, Y: row.Y + (row.H-8)/2, W: row.W - 296, H: 8}
				fillRect(dst, bar, uiSliderTrack)
				fillRect(dst, rect{X: bar.X, Y: bar.Y, W: bar.W * float32(s.Level) / 20, H: bar.H}, uiSliderFill)
				strokeRect(dst, bar, uiPanelBorder)
			}
		}
	})
}

// ---- the team -------------------------------------------------------------------------

func (a *App) pawnsTeam(ng *newGameState, r rect) {
	u := a.uiSt()
	a.uiPanel(r)
	in := r.Inset(12)
	hs := tsLarge
	hs.Color = uiHeading
	a.uiLabel(in.X, in.Y, "Team", hs)
	rep := ng.pool.Team()
	y := in.Y + float32(lineH(hs)) + 8
	a.uiLabel(in.X, y, "Best skill among the selected", dimText())
	y += 22
	const rowH = 20
	for i, b := range rep.Best {
		row := rect{X: in.X, Y: y + float32(i)*rowH, W: in.W, H: rowH}
		who := b.Who
		tip := b.Label + "\nBest level on the team"
		if who == "" {
			tip += ": nobody can do this."
		} else {
			tip += ": " + who + "."
		}
		a.uiTip(fmt.Sprintf("pawns.team.%d", i), row, tip)
		b := b
		u.Queue(func(dst *ebiten.Image) {
			st := tsBody
			ty := float64(row.Y) + (rowH-lineH(st))/2
			drawText(dst, b.Label, float64(row.X+2), ty, st)
			lv := "—"
			if b.Who != "" {
				lv = fmt.Sprint(b.Level)
			}
			ls := st
			ls.Align = alignRight
			drawText(dst, lv, float64(row.X+118), ty, ls)
			ds := dimText()
			drawText(dst, fitText(b.Who, ds, row.W-130), float64(row.X+128), ty, ds)
		})
	}
	y += float32(len(rep.Best))*rowH + 12
	if len(rep.Warnings) == 0 {
		g := tsBody
		g.Color = uiGood
		a.uiLabel(in.X, y, "The team has what it needs.", g)
		return
	}
	for _, wn := range rep.Warnings {
		st := tsLabel
		st.Color = uiWarning
		for _, l := range wrapPx("! "+wn, st, float64(in.W)) {
			a.uiLabel(in.X, y, l, st)
			y += float32(lineH(st))
		}
	}
}
