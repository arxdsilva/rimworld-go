package wizard

import (
	"fmt"
	"strings"
	"unicode"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
func (w pawnsPage) poolKeyFor(ng *Draft) string {
	fx := ng.Scenario.Apply()
	return fmt.Sprintf("%d|%d|%d|%v", SetupSeedOf(ng), fx.People, fx.ChooseFrom, fx.Rules)
}

// ensurePool makes the candidate pool when it is missing or stale.
func (w pawnsPage) ensurePool(ng *Draft) {
	key := w.poolKeyFor(ng)
	if ng.Pool != nil && ng.PoolKey == key {
		return
	}
	ng.Pool, ng.PoolKey = game.NewCandidatePool(SetupSeedOf(ng), ng.Scenario.Apply()), key
	ng.PawnSel, ng.NamesOf, ng.NameErr = 0, -1, ""
	ng.drag = rosterDrag{}
}

// undoCharacters restores the pool as it was on entry (the pool is
// deterministic from its seed).
func (w pawnsPage) undoCharacters(ng *Draft) {
	ng.Pool = game.NewCandidatePool(SetupSeedOf(ng), ng.Scenario.Apply())
	ng.NamesOf, ng.NameErr = -1, ""
}

// StartReason is why Start is disabled ("" it is enabled).
func (ng *Draft) StartReason() string {
	if ng.NameErr != "" {
		return ng.NameErr
	}
	if ng.Pool == nil || len(ng.Pool.Selected()) == 0 {
		return "There is nobody to send. The scenario needs at least one colonist."
	}
	return ""
}

func (w pawnsPage) pawnsUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	if ng.Load != nil {
		w.loadingStep()
		w.kit().Suppress = true
		return
	}
	w.ensurePool(ng)
	views := ng.Pool.Views()
	nSel := 0
	for _, v := range views {
		if v.Selected {
			nSel++
		}
	}
	if ng.PawnSel >= len(views) {
		ng.PawnSel = max(0, len(views)-1)
	}
	_, _, content, footer := uikit.DialogFrame(sw, sh)
	roster, rest := content.SplitH(pawnRosterW)
	rest = gfx.Rect{X: rest.X + uikit.DialogPad, Y: rest.Y, W: rest.W - uikit.DialogPad, H: rest.H}
	detail, team := rest.SplitH(rest.W - pawnTeamW - uikit.DialogPad)
	team.X, team.W = team.X+uikit.DialogPad, pawnTeamW

	w.pawnsRoster(ng, roster, views, nSel)
	w.pawnsDetail(ng, detail, views)
	w.pawnsTeam(ng, team)
	w.pawnsFooter(ng, footer)
}

// ---- the footer ----------------------------------------------------------

func (w pawnsPage) pawnsFooter(ng *Draft, footer gfx.Rect) {
	back := uikit.FooterBack(footer)
	uikit.StyledButton(w.kit(), ids.WizardBack, back, "Back", uikit.BtnOpts{}, w.back)
	x := back.Right() + 12
	uikit.StyledButton(w.kit(), "pawns.reroll", gfx.Rect{X: x, Y: back.Y, W: 120, H: uikit.FooterBtnH}, "Randomize", uikit.BtnOpts{
		Tip: "Randomize\nRoll this colonist again. An edited name is kept; hold Shift to roll a new name too."},
		func() { w.rerollSelected(ng) })
	x += 128
	uikit.StyledButton(w.kit(), "pawns.rerollAll", gfx.Rect{X: x, Y: back.Y, W: 130, H: uikit.FooterBtnH}, "Randomize all", uikit.BtnOpts{Tip: "Randomize all\nRoll every candidate again."},
		func() {
			w.kit().Confirm("pawns.rerollAll.confirm", "Randomize everyone?", "Every candidate is rolled again. Names you edited are kept.", "Randomize", false, func() {
				ng.Pool.RandomizeAll()
				ng.NamesOf = -1
			})
		})
	x += 138
	uikit.StyledButton(w.kit(), "pawns.reset", gfx.Rect{X: x, Y: back.Y, W: 130, H: uikit.FooterBtnH}, "Undo changes", uikit.BtnOpts{Tip: "Undo changes\nPut the candidates back as they were when you came here."},
		func() { w.undoCharacters(ng) })

	reason := ng.StartReason()
	start := func() {
		if ng.StartReason() == "" {
			w.beginLoading()
		}
	}
	uikit.StyledButton(w.kit(), ids.WizardStart, uikit.FooterNext(footer), "Start", uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: reason != "", Reason: reason}, start)
	if w.in().Fired(input.Accept) && reason == "" && !w.kit().ModalOpen() && !w.kit().Editing() {
		start()
	}
	if reason != "" {
		st := gfx.Body
		st.Color = uikit.Warning
		w.kit().Label(uikit.FooterNext(footer).X-gfx.TextW(reason, st)-14, footer.Y+(footer.H-float32(gfx.LineH(st)))/2, reason, st)
	}
	if why := w.sh.Env().SessionErr; why != "" {
		st := gfx.Body
		st.Color = uikit.Danger
		w.kit().Label(footer.X+uikit.DialogPad, footer.Y-float32(gfx.LineH(st))-4, "Start failed: "+why, st)
	}
}

// rerollSelected rolls the highlighted candidate again (Shift: new name too).
func (w pawnsPage) rerollSelected(ng *Draft) {
	ng.Pool.Reroll(ng.PawnSel, !w.in().Shift())
	ng.NamesOf = -1
}

// ---- the roster --------------------------------------------------------------

// RosterY is the content y of roster position pos for nSel selected
// candidates: a header, the Selected rows, a header, the rest.
func RosterY(pos, nSel int) float32 {
	if pos < nSel {
		return pawnHeadH + float32(pos)*pawnRowH
	}
	return pawnHeadH + float32(nSel)*pawnRowH + pawnHeadH + float32(pos-nSel)*pawnRowH
}

// RosterTarget is the roster position a drop at content y lands on.
func RosterTarget(yc float32, n, nSel int) int {
	if n == 0 {
		return 0
	}
	best, bestD := 0, float32(1e9)
	for p := 0; p < n; p++ {
		y := RosterY(p, nSel)
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

func (w pawnsPage) pawnsRoster(ng *Draft, r gfx.Rect, views []game.CandidateView, nSel int) {
	u := w.kit()
	w.kit().Panel(r)
	btns := float32(2*28 + 6 + 8)
	listR := gfx.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8 - btns}
	n := len(views)
	nLeft := n - nSel
	total := RosterY(n, nSel) + 4
	if nLeft == 0 {
		total = RosterY(nSel, nSel) + pawnHeadH
	}
	area := w.kit().ScrollArea(pawnRosterID, listR, total, uikit.ScrollOpts{Step: pawnRowH})
	rowAt := func(pos int) gfx.Rect {
		return gfx.Rect{X: area.View.X, Y: listR.Y + RosterY(pos, nSel) - area.Off, W: area.View.W, H: pawnRowH}
	}

	mx, my := w.kit().Host().Cursor()
	for pos := range views {
		pos := pos
		rr := rowAt(pos)
		hit := rr.Intersect(area.View)
		if hit.Empty() {
			continue
		}
		sel := func() { ng.PawnSel = pos; u.Focus = pawnRosterID }
		wd := uikit.Widget{ID: uikit.OptID("pawns.row", pos), Label: views[pos].Nick, R: hit, Click: func() { w.kit().OK(); sel() },
			Tip: views[pos].Nick + "\n" + candidateLine(views[pos])}
		if u.Add(wd) && w.kit().Press(hit) {
			sel()
			ng.drag = rosterDrag{on: true, from: pos, y0: my}
		}
	}
	// A drag in progress: it ends on release and moves the candidate.
	if d := &ng.drag; d.on {
		if absInt(my-d.y0) > pawnDragSlop {
			d.moved = true
		}
		if !w.in().Down(inputframe.Left) {
			if d.moved {
				to := RosterTarget(float32(my)-listR.Y+area.Off, n, nSel)
				if to != d.from {
					ng.Pool.Move(d.from, to)
					ng.PawnSel = to
					ng.NamesOf = -1
				}
			}
			*d = rosterDrag{}
		}
	}
	drag := ng.drag
	selPos := ng.PawnSel
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		hs := gfx.Label
		hs.Color = uikit.Heading
		gfx.Text(sub, fmt.Sprintf("Selected (%d)", nSel), float64(area.View.X+6), float64(listR.Y-area.Off)+3, hs)
		gfx.Text(sub, fmt.Sprintf("Left behind (%d)", nLeft), float64(area.View.X+6), float64(listR.Y+RosterY(nSel, nSel)-pawnHeadH-area.Off)+3, hs)
		for pos, v := range views {
			rr := rowAt(pos)
			if rr.Bottom() < area.View.Y || rr.Y > area.View.Bottom() {
				continue
			}
			w.paintRosterRow(sub, rr, v, pos == selPos, u.Hot == uikit.OptID("pawns.row", pos), pos >= nSel, drag.on && drag.moved && drag.from == pos)
		}
		if drag.on && drag.moved {
			to := RosterTarget(float32(my)-listR.Y+area.Off, n, nSel)
			ty := listR.Y + RosterY(to, nSel) - area.Off
			if to > drag.from {
				ty += pawnRowH
			}
			uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: ty - 1, W: area.View.W, H: 3}, uikit.Accent)
			ghost := gfx.Rect{X: area.View.X, Y: float32(my) - pawnRowH/2, W: area.View.W, H: pawnRowH}
			w.paintRosterRow(dst, ghost, views[drag.from], true, false, false, false)
		}
	})
	_ = mx

	// Up, down and swap under the list.
	by := r.Bottom() - btns + 4
	half := (r.W - 8 - 6) / 2
	i := ng.PawnSel
	upR, dnR := "", ""
	if i <= 0 {
		upR = "Already first."
	}
	if i >= n-1 {
		dnR = "Already last."
	}
	uikit.StyledButton(w.kit(), "pawns.up", gfx.Rect{X: r.X + 4, Y: by, W: half, H: 28}, "Up", uikit.BtnOpts{Disabled: upR != "", Reason: upR, Tip: "Move up\nMove this candidate up the roster."},
		func() { ng.Pool.Move(i, i-1); ng.PawnSel = i - 1; ng.NamesOf = -1 })
	uikit.StyledButton(w.kit(), "pawns.down", gfx.Rect{X: r.X + 4 + half + 6, Y: by, W: half, H: 28}, "Down", uikit.BtnOpts{Disabled: dnR != "", Reason: dnR, Tip: "Move down\nMove this candidate down the roster."},
		func() { ng.Pool.Move(i, i+1); ng.PawnSel = i + 1; ng.NamesOf = -1 })
	swapLabel, swapReason := "Swap with left-behind...", ""
	if i >= nSel {
		swapLabel = "Swap with selected..."
	}
	if nSel == 0 || nLeft == 0 {
		swapReason = "There is nobody to swap with."
	}
	uikit.StyledButton(w.kit(), "pawns.swap", gfx.Rect{X: r.X + 4, Y: by + 34, W: r.W - 8, H: 28}, swapLabel, uikit.BtnOpts{Disabled: swapReason != "", Reason: swapReason},
		func() { w.pushSwap(ng, views, nSel) })
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

func (w pawnsPage) paintRosterRow(dst *ebiten.Image, rr gfx.Rect, v game.CandidateView, selected, hot, left, ghosted bool) {
	switch {
	case selected:
		uikit.FillRect(dst, rr.Inset(1), uikit.LerpRGBA(uikit.Panel, uikit.BtnSelected, 0.8))
		uikit.StrokeRect(dst, rr.Inset(1), uikit.Accent)
	case hot:
		uikit.FillRect(dst, rr.Inset(1), uikit.BtnHover)
	}
	if ghosted {
		return
	}
	pr := gfx.Rect{X: rr.X + 8, Y: rr.Y + (rr.H-pawnThumbH)/2, W: pawnThumbW, H: pawnThumbH}
	uikit.FillRect(dst, pr, uikit.Field)
	if !w.sh.DrawPortrait(dst, v.Look, float64(pr.X), float64(pr.Y), float64(pr.W), float64(pr.H)) {
		DrawSilhouette(dst, pr, "")
	}
	nm := gfx.Label
	if left {
		nm.Color = uikit.TextDim
	}
	gfx.Text(dst, uikit.FitText(v.Nick, nm, rr.W-pawnThumbW-24), float64(rr.X+pawnThumbW+18), float64(rr.Y)+10, nm)
	ts := gfx.Small
	ts.Color = uikit.TextDim
	gfx.Text(dst, uikit.FitText(candidateLine(v), ts, rr.W-pawnThumbW-24), float64(rr.X+pawnThumbW+18), float64(rr.Y)+10+gfx.LineH(nm)+2, ts)
}

// pushSwap lists the candidates of the other group; picking one swaps.
func (w pawnsPage) pushSwap(ng *Draft, views []game.CandidateView, nSel int) {
	i := ng.PawnSel
	var idx []int
	var labels []string
	for p, v := range views {
		if (i < nSel) == (p >= nSel) {
			idx = append(idx, p)
			labels = append(labels, v.Nick+", "+candidateLine(v))
		}
	}
	uikit.Picker(w.kit(), "pawns.swap", "Swap with", labels, -1, func(k int) {
		ng.Pool.Swap(i, idx[k])
		ng.NamesOf = -1
	})
}

// ---- the details ----------------------------------------------------------------

func (w pawnsPage) pawnsDetail(ng *Draft, r gfx.Rect, views []game.CandidateView) {
	u := w.kit()
	w.kit().Panel(r)
	in := r.Inset(12)
	if len(views) == 0 {
		w.kit().Label(in.X, in.Y, "No candidates.", uikit.DimText())
		return
	}
	v := views[ng.PawnSel]
	w.syncNames(ng, v)

	left := gfx.Rect{X: in.X, Y: in.Y, W: in.W - pawnBigW - 16, H: in.H}
	// The portrait.
	pr := gfx.Rect{X: in.Right() - pawnBigW, Y: in.Y, W: pawnBigW, H: pawnBigH}
	u.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, pr, uikit.Field)
		uikit.StrokeRect(dst, pr, uikit.PanelBorder)
		if !w.sh.DrawPortrait(dst, v.Look, float64(pr.X), float64(pr.Y), float64(pr.W), float64(pr.H)) {
			DrawSilhouette(dst, pr, v.Nick)
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
		w.kit().Label(fx, y, l.text, uikit.DimText())
		if l.tip != "" {
			w.kit().Tip(l.id+".tip", gfx.Rect{X: fx, Y: y, W: fw, H: 16}, l.tip)
		}
		ch, _ := w.kit().TextField(l.id, gfx.Rect{X: fx, Y: y + 18, W: fw, H: 28}, &ng.Names[k], uikit.TextFieldSpec{MaxLen: game.MaxNameRunes, Valid: nameRune})
		changed = changed || ch
	}
	if changed {
		if err := ng.Pool.Rename(ng.PawnSel, ng.Names[0], ng.Names[1], ng.Names[2]); err != nil {
			ng.NameErr = err.Error()
		} else {
			ng.NameErr = ""
		}
	}
	y += 50
	if ng.NameErr != "" {
		st := gfx.Small
		st.Color = uikit.Warning
		w.kit().Label(left.X, y-2, ng.NameErr, st)
	}
	y += 16

	// Sex, age, build, faction.
	info := fmt.Sprintf("%s, age %d (biological)", v.Sex, v.AgeYears)
	w.kit().Label(left.X, y, info, gfx.Body)
	y += 20
	badge := faction.DefOf(ng.Scenario.Faction)
	if ng.Scenario.Faction == faction.Player || badge.Label == "" {
		badge = faction.DefOf(faction.Settlers)
	}
	by := y
	u.Queue(func(dst *ebiten.Image) {
		w.DrawEmblem(dst, "ui_icon/faction_"+badge.Key, globe.Accent(badge.Type), left.X, by-1, 18)
		st := uikit.DimText()
		gfx.Text(dst, fmt.Sprintf("%s. Build: %s.", badge.Label, v.Build), float64(left.X+24), float64(by), st)
	})
	y += 24

	// Backstories.
	w.backstoryLine(left.X, y, left.W, "Childhood", v.Childhood, v.ChildhoodDesc, "pawns.child")
	y += 20
	w.backstoryLine(left.X, y, left.W, "Adulthood", v.Adulthood, v.AdulthoodDesc, "pawns.adult")
	y += 26

	// Traits.
	y = w.traitChips(left.X, y, left.W, v)

	// Incapable of.
	incap, full := "None", ""
	col := uikit.TextDim
	if len(v.Incapable) > 0 {
		names := make([]string, len(v.Incapable))
		for i, s := range v.Incapable {
			names[i] = s
			if k := strings.Index(s, " ("); k > 0 {
				names[i] = s[:k]
			}
		}
		incap, full, col = strings.Join(names, ", "), strings.Join(v.Incapable, "\n"), uikit.Danger
	}
	iy := y
	w.kit().Label(left.X, iy, "Incapable of:", gfx.Label)
	u.Queue(func(dst *ebiten.Image) {
		st := gfx.Body
		st.Color = col
		gfx.Text(dst, uikit.FitText(incap, st, left.W-110), float64(left.X+104), float64(iy), st)
	})
	if full != "" {
		w.kit().Tip("pawns.incap.tip", gfx.Rect{X: left.X, Y: y, W: left.W, H: 18}, "Incapable of\n"+full)
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
	w.skillsTable(gfx.Rect{X: in.X, Y: skillsTop, W: in.W, H: pawnSkillHdrH + rowH*float32(len(v.Skills))}, v, rowH)

	hy := in.Bottom() - healthH + 6
	w.kit().Label(in.X, hy, "Health:", gfx.Label)
	text := "No conditions"
	if len(v.Conditions) > 0 {
		text = strings.Join(v.Conditions, "\n")
	}
	st := uikit.DimText()
	if len(v.Conditions) > 0 {
		st.Color = uikit.Warning
	}
	w.kit().Queue(func(dst *ebiten.Image) { gfx.Text(dst, text, float64(in.X+64), float64(hy), st) })
}

// syncNames loads the name drafts from the candidate when no field is
// being edited and the highlighted candidate or the pool changed.
func (w pawnsPage) syncNames(ng *Draft, v game.CandidateView) {
	if w.kit().Editing() {
		return
	}
	want := [3]string{v.First, v.Nick, v.Last}
	if ng.NamesOf != ng.PawnSel || ng.Names != want {
		ng.Names, ng.NamesOf, ng.NameErr = want, ng.PawnSel, ""
	}
}

// backstoryLine is "Childhood: Title" with the description as a tooltip.
func (w pawnsPage) backstoryLine(x, y, width float32, label, title, desc, id string) {
	if title == "" {
		title = "—"
	}
	w.kit().Label(x, y, label+":", gfx.Label)
	st := gfx.Body
	w.kit().Queue(func(dst *ebiten.Image) {
		gfx.Text(dst, uikit.FitText(title, st, width-100), float64(x+90), float64(y), st)
	})
	tip := ""
	if title != "—" {
		tip = title + "\n" + desc
	}
	if tip != "" {
		w.kit().Tip(id, gfx.Rect{X: x, Y: y, W: width, H: 18}, tip)
	}
}

// traitChips lays the traits out as chips and returns the next y.
func (w pawnsPage) traitChips(x, y, width float32, v game.CandidateView) float32 {
	w.kit().Label(x, y+2, "Traits:", gfx.Label)
	cx, cy := x+64, y
	if len(v.Traits) == 0 {
		w.kit().Label(cx, y+2, "None", uikit.DimText())
		return y + 28
	}
	for i, t := range v.Traits {
		t := t
		cw := gfx.TextW(t.Label, gfx.Body) + 18
		if cx+cw > x+width && cx > x+64 {
			cx, cy = x+64, cy+28
		}
		cr := gfx.Rect{X: cx, Y: cy, W: cw, H: 24}
		tip := t.Label + "\n" + t.Desc
		if t.Effects != "" {
			tip += "\n" + t.Effects
		}
		w.kit().Tip(fmt.Sprintf("pawns.trait.%d", i), cr, tip)
		w.kit().Queue(func(dst *ebiten.Image) {
			uikit.FillRect(dst, cr, uikit.Btn)
			uikit.StrokeRect(dst, cr, uikit.BtnBorder)
			st := gfx.Body
			st.Align = gfx.AlignCenter
			gfx.Text(dst, t.Label, float64(cr.X+cr.W/2), float64(cr.Y)+(float64(cr.H)-gfx.LineH(st))/2, st)
		})
		cx += cw + 6
	}
	return cy + 30
}

// SkillLevelText is the level cell: the number, or a dash when incapable.
func SkillLevelText(s game.SkillView) string {
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

// SkillTip is a skill row's hover card.
func SkillTip(s game.SkillView) string {
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
func (w pawnsPage) skillsTable(r gfx.Rect, v game.CandidateView, rowH float32) {
	u := w.kit()
	for i, s := range v.Skills {
		row := gfx.Rect{X: r.X, Y: r.Y + pawnSkillHdrH + float32(i)*rowH, W: r.W, H: rowH}
		w.kit().Tip(fmt.Sprintf("pawns.skill.%d", i), row, SkillTip(s))
	}
	u.Queue(func(dst *ebiten.Image) {
		hs := gfx.Label
		hs.Color = uikit.Heading
		uikit.FillRect(dst, gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: pawnSkillHdrH}, uikit.Btn)
		uikit.StrokeRect(dst, gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: pawnSkillHdrH}, uikit.BtnBorder)
		gfx.Text(dst, "Skill", float64(r.X+8), float64(r.Y)+(pawnSkillHdrH-gfx.LineH(hs))/2, hs)
		gfx.Text(dst, "Passion", float64(r.X+160), float64(r.Y)+(pawnSkillHdrH-gfx.LineH(hs))/2, hs)
		gfx.Text(dst, "Level", float64(r.X+240), float64(r.Y)+(pawnSkillHdrH-gfx.LineH(hs))/2, hs)
		for i, s := range v.Skills {
			row := gfx.Rect{X: r.X, Y: r.Y + pawnSkillHdrH + float32(i)*rowH, W: r.W, H: rowH}
			if i%2 == 1 {
				uikit.FillRect(dst, row, uikit.Fade(uikit.Field, 0x28)) // premultiplied wash
			}
			st := gfx.Body
			if s.Incapable {
				st.Color = uikit.TextDisabled
			}
			ty := float64(row.Y) + (float64(row.H)-gfx.LineH(st))/2
			gfx.Text(dst, s.Label, float64(row.X+8), ty, st)
			if !s.Incapable && s.Passion > 0 {
				key := "ui_icon/passion_minor"
				if s.Passion == 2 {
					key = "ui_icon/passion_major"
				}
				if img := w.art().Image(key, noTint); img != nil {
					gfx.Icon(dst, img, float64(row.X+166), float64(row.Y)+(float64(row.H)-14)/2, 14)
				} else {
					gfx.Text(dst, strings.Repeat("*", s.Passion), float64(row.X+166), ty, st)
				}
			}
			lt := st
			lt.Align = gfx.AlignCenter
			gfx.Text(dst, SkillLevelText(s), float64(row.X+250), ty, lt)
			if !s.Incapable {
				bar := gfx.Rect{X: row.X + 280, Y: row.Y + (row.H-8)/2, W: row.W - 296, H: 8}
				uikit.FillRect(dst, bar, uikit.SliderTrack)
				uikit.FillRect(dst, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(s.Level) / 20, H: bar.H}, uikit.SliderFill)
				uikit.StrokeRect(dst, bar, uikit.PanelBorder)
			}
		}
	})
}

// ---- the team -------------------------------------------------------------------------

func (w pawnsPage) pawnsTeam(ng *Draft, r gfx.Rect) {
	u := w.kit()
	w.kit().Panel(r)
	in := r.Inset(12)
	hs := gfx.Large
	hs.Color = uikit.Heading
	w.kit().Label(in.X, in.Y, "Team", hs)
	rep := ng.Pool.Team()
	y := in.Y + float32(gfx.LineH(hs)) + 8
	w.kit().Label(in.X, y, "Best skill among the selected", uikit.DimText())
	y += 22
	const rowH = 20
	for i, b := range rep.Best {
		row := gfx.Rect{X: in.X, Y: y + float32(i)*rowH, W: in.W, H: rowH}
		who := b.Who
		tip := b.Label + "\nBest level on the team"
		if who == "" {
			tip += ": nobody can do this."
		} else {
			tip += ": " + who + "."
		}
		w.kit().Tip(fmt.Sprintf("pawns.team.%d", i), row, tip)
		b := b
		u.Queue(func(dst *ebiten.Image) {
			st := gfx.Body
			ty := float64(row.Y) + (rowH-gfx.LineH(st))/2
			gfx.Text(dst, b.Label, float64(row.X+2), ty, st)
			lv := "—"
			if b.Who != "" {
				lv = fmt.Sprint(b.Level)
			}
			ls := st
			ls.Align = gfx.AlignRight
			gfx.Text(dst, lv, float64(row.X+118), ty, ls)
			ds := uikit.DimText()
			gfx.Text(dst, uikit.FitText(b.Who, ds, row.W-130), float64(row.X+128), ty, ds)
		})
	}
	y += float32(len(rep.Best))*rowH + 12
	if len(rep.Warnings) == 0 {
		g := gfx.Body
		g.Color = uikit.Good
		w.kit().Label(in.X, y, "The team has what it needs.", g)
		return
	}
	for _, wn := range rep.Warnings {
		st := gfx.Label
		st.Color = uikit.Warning
		for _, l := range gfx.WrapPx("! "+wn, st, float64(in.W)) {
			w.kit().Label(in.X, y, l, st)
			y += float32(gfx.LineH(st))
		}
	}
}
