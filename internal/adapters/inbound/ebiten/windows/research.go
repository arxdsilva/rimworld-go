package windows

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// Research tab: the project list. Click an available project to research
// it; colonists with Research work then study at a research bench.
//
// The list scrolls (PLAY-021: sixteen projects need about 980 px, more than
// the window has at 1280x720). It shows whole rows only: the scroll view is
// researchScrollID over the rows below the header, the wheel, the scroll bar
// and a click on its track move it a row at a time, and the offset is kept
// on whole rows so a project's button is never cut by the window's edge. The
// project being researched is brought into view when the window opens and
// when the project changes, never while the player scrolls.

const (
	researchRowH      = 58
	researchTop       = 44 // the header above the rows
	researchBottomPad = 8
	researchScrollID  = "research.scroll"
	// researchCurKey is the scroll map's note of the project last revealed
	// (its index + 1; absent while the window is closed).
	researchCurKey = "research.scroll.cur"
)

// LockedBy is the research project still gating an unlock key ("" if none).
func (w Research) LockedBy(key string) string {
	if !w.live() {
		return ""
	}
	return w.sh.World().Locked[key]
}

// want is the Research window's wanted size: a row per project.
func (w Research) want() (float32, float32) {
	n := 0
	if w.live() {
		n = len(w.sh.World().Research)
	}
	return 760, float32(max(160, researchTop+n*researchRowH+researchBottomPad))
}

// researchShown is how many whole rows of n projects the window r shows
// (at least one).
func researchShown(r uikit.Button, n int) int {
	rows := max(1, int((r.H-researchTop-researchBottomPad)/researchRowH))
	return min(rows, n)
}

// first is the index of the first project shown: the scroll
// offset in whole rows, kept inside the list.
func (w Research) first(n, shown int) int {
	off := w.kit().Scroll[researchScrollID]
	return max(0, min(n-shown, int(math.Round(float64(off)/researchRowH))))
}

// researchList is the rows' rectangle in the window r.
func researchList(r uikit.Button, shown int) gfx.Rect {
	return gfx.Rect{X: r.X, Y: r.Y + researchTop, W: r.W, H: float32(shown) * researchRowH}
}

// Declare declares the project list's scroll view (the wheel, the bar,
// track clicks) and snaps its offset to whole rows. It also reveals the
// project being researched when the window opens or that project changes.
func (w Research) Declare(sw, sh int) {
	u := w.kit()
	if !w.live() || !w.Open || w.Mode != "research" {
		delete(u.Scroll, researchCurKey) // the next opening starts afresh
		delete(u.Scroll, researchScrollID)
		return
	}
	views := w.sh.World().Research
	r := w.Rect()
	if len(views) == 0 || r.W <= 0 {
		return
	}
	shown := researchShown(r, len(views))
	list := researchList(r, shown)
	cur := -1
	for i, p := range views {
		if p.Current {
			cur = i
			break
		}
	}
	if note := float32(cur + 1); u.Scroll[researchCurKey] != note {
		if u.Scroll == nil {
			u.Scroll = map[string]float32{}
		}
		u.Scroll[researchCurKey] = note
		if cur >= 0 {
			w.kit().ScrollReveal(researchScrollID, list.H, float32(cur)*researchRowH, float32(cur+1)*researchRowH)
		}
	}
	area := w.kit().ScrollArea(researchScrollID, list, float32(len(views))*researchRowH, uikit.ScrollOpts{Step: researchRowH})
	u.Scroll[researchScrollID] = float32(math.Round(float64(area.Off)/researchRowH)) * researchRowH
}

func (w Research) Buttons() []uikit.Button {
	r := w.Rect()
	views := w.sh.World().Research
	shown := researchShown(r, len(views))
	first := w.first(len(views), shown)
	var out []uikit.Button
	for i := first; i < first+shown; i++ {
		p := views[i]
		y := r.Y + researchTop + float32((i-first)*researchRowH)
		b := uikit.Button{ID: ids.PrefixResearchPick + strconv.Itoa(int(p.ID)), X: r.X + 12, Y: y, W: 220, H: researchRowH - 8,
			Label: p.Label, Selected: p.Current, Quiet: true}
		switch {
		case p.Done:
			b.Disabled, b.Reason = true, p.Label+" is already researched"
		case !p.Available:
			b.Disabled, b.Reason = true, "Needs "+strings.Join(p.Requires, ", ")+" first"
		}
		out = append(out, b)
	}
	return out
}

func (w Research) Pick(id string) {
	n, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixResearchPick))
	if err != nil || !w.live() {
		return
	}
	w.sh.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(n)})
}

func (w Research) draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	gfx.Text(screen, "Research", float64(r.X)+12, float64(r.Y)+6, gfx.WithColor(gfx.Label, uikit.Heading))
	hintX := float64(r.X) + 12 + float64(gfx.TextW("Research", gfx.Label)) + 16
	gfx.Text(screen, gfx.Ellipsize("pick a project; colonists with Research work study it at a research bench", gfx.Small, float64(r.X+r.W)-12-hintX),
		hintX, float64(r.Y)+6+(gfx.LineH(gfx.Label)-gfx.LineH(gfx.Small))/2, gfx.WithColor(gfx.Small, uikit.TextDim))
	if !w.HasBench() {
		gfx.Text(screen, "No research bench yet: build one from Architect > Production.", float64(r.X)+12, float64(r.Y)+25, gfx.WithColor(gfx.Small, uikit.Warning))
	}
	mx, my := w.sh.Cursor()
	views := w.sh.World().Research
	shown := researchShown(r, len(views))
	first := w.first(len(views), shown)
	right := float64(r.X+r.W) - 12
	if len(views) > shown {
		right -= uikit.ScrollBarW // the scroll bar's lane
	}
	for k, b := range w.Buttons() {
		p := views[first+k]
		uikit.DrawButton(screen, b, b.Contains(mx, my) && !b.Disabled)
		x, y := int(b.X+b.W)+14, int(b.Y)
		status := "available"
		switch {
		case p.Done:
			status = "done"
		case p.Current:
			status = "researching"
		case !p.Available:
			status = "needs " + strings.Join(p.Requires, ", ")
		}
		maxW := right - float64(x)
		drawResearchBar(screen, float32(x), float32(y+6), 120, 10, p)
		gfx.Text(screen, gfx.Ellipsize(fmt.Sprintf("%d / %d  %s", p.Progress, p.Cost, status), gfx.Body, maxW-130), float64(x+130), float64(y), gfx.Body)
		gfx.Text(screen, gfx.Ellipsize(p.Desc, gfx.Small, maxW), float64(x), float64(y)+20, gfx.Small)
		gfx.Text(screen, gfx.Ellipsize("Unlocks: "+strings.Join(p.Unlocks, ", "), gfx.Small, maxW), float64(x), float64(y)+37, gfx.WithColor(gfx.Small, uikit.TextDim))
	}
}

func drawResearchBar(screen *ebiten.Image, x, y, w, h float32, p game.ResearchView) {
	gfx.FillRect(screen, x, y, w, h, uikit.Field, false)
	c := uikit.SliderFill
	if p.Done {
		c = uikit.Good
	}
	if p.Cost > 0 {
		gfx.FillRect(screen, x, y, w*float32(min(p.Progress, p.Cost))/float32(p.Cost), h, c, false)
	}
	gfx.StrokeRect(screen, x, y, w, h, 1, uikit.PanelBorder, false)
}

// benchName is the bench a building kind works recipes at
// ("research bench"; an unknown kind is its own name).
func benchName(kind string) string {
	if k, ok := construction.Parse(kind); ok {
		return construction.RecipeBench(k).String()
	}
	return kind
}

// HasBench reports a research bench stands: the plain one or the
// electric one (which works the same research, powered).
func (w Research) HasBench() bool {
	for _, b := range w.sh.World().Buildings {
		if benchName(b.Kind) == "research bench" {
			return true
		}
	}
	return false
}
