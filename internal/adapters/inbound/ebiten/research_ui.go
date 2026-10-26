package ebiten

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
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

// lockedBy is the research project still gating an unlock key ("" if none).
func (a *App) lockedBy(key string) string {
	if a.renderer == nil {
		return ""
	}
	return a.renderer.world().Locked[key]
}

// researchWant is the Research window's wanted size: a row per project.
func (a *App) researchWant() (float32, float32) {
	n := 0
	if a.renderer != nil {
		n = len(a.renderer.world().Research)
	}
	return 760, float32(max(160, researchTop+n*researchRowH+researchBottomPad))
}

// researchShown is how many whole rows of n projects the window r shows
// (at least one).
func researchShown(r button, n int) int {
	rows := max(1, int((r.h-researchTop-researchBottomPad)/researchRowH))
	return min(rows, n)
}

// researchFirst is the index of the first project shown: the scroll
// offset in whole rows, kept inside the list.
func (a *App) researchFirst(n, shown int) int {
	off := a.uiSt().scroll[researchScrollID]
	return max(0, min(n-shown, int(math.Round(float64(off)/researchRowH))))
}

// researchList is the rows' rectangle in the window r.
func researchList(r button, shown int) rect {
	return rect{r.x, r.y + researchTop, r.w, float32(shown) * researchRowH}
}

// researchUI declares the project list's scroll view (the wheel, the bar,
// track clicks) and snaps its offset to whole rows. It also reveals the
// project being researched when the window opens or that project changes.
func (a *App) researchUI(sw, sh int) {
	u := a.uiSt()
	if a.renderer == nil || !a.workOpen || a.panelMode != "research" {
		delete(u.scroll, researchCurKey) // the next opening starts afresh
		delete(u.scroll, researchScrollID)
		return
	}
	views := a.renderer.world().Research
	r := a.windowRect()
	if len(views) == 0 || r.w <= 0 {
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
	if note := float32(cur + 1); u.scroll[researchCurKey] != note {
		if u.scroll == nil {
			u.scroll = map[string]float32{}
		}
		u.scroll[researchCurKey] = note
		if cur >= 0 {
			a.uiScrollReveal(researchScrollID, list.H, float32(cur)*researchRowH, float32(cur+1)*researchRowH)
		}
	}
	area := a.uiScrollArea(researchScrollID, list, float32(len(views))*researchRowH, scrollOpts{Step: researchRowH})
	u.scroll[researchScrollID] = float32(math.Round(float64(area.Off)/researchRowH)) * researchRowH
}

func (a *App) researchButtons() []button {
	r := a.windowRect()
	views := a.renderer.world().Research
	shown := researchShown(r, len(views))
	first := a.researchFirst(len(views), shown)
	var out []button
	for i := first; i < first+shown; i++ {
		p := views[i]
		y := r.y + researchTop + float32((i-first)*researchRowH)
		b := button{id: "research.pick." + strconv.Itoa(int(p.ID)), x: r.x + 12, y: y, w: 220, h: researchRowH - 8,
			label: p.Label, selected: p.Current, quiet: true}
		switch {
		case p.Done:
			b.disabled, b.reason = true, p.Label+" is already researched"
		case !p.Available:
			b.disabled, b.reason = true, "Needs "+strings.Join(p.Requires, ", ")+" first"
		}
		out = append(out, b)
	}
	return out
}

func (a *App) pickResearch(id string) {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "research.pick."))
	if err != nil || a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.SetResearch, Build: uint8(n)})
}

func (a *App) drawResearch(screen *ebiten.Image, sw, sh int) {
	r := a.windowRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawText(screen, "Research", float64(r.x)+12, float64(r.y)+6, withColor(tsLabel, uiHeading))
	hintX := float64(r.x) + 12 + float64(textW("Research", tsLabel)) + 16
	drawText(screen, ellipsize("pick a project; colonists with Research work study it at a research bench", tsSmall, float64(r.x+r.w)-12-hintX),
		hintX, float64(r.y)+6+(lineH(tsLabel)-lineH(tsSmall))/2, withColor(tsSmall, uiTextDim))
	if !a.hasResearchBench() {
		drawText(screen, "No research bench yet: build one from Architect > Production.", float64(r.x)+12, float64(r.y)+25, withColor(tsSmall, uiWarning))
	}
	mx, my := a.currentCursor()
	views := a.renderer.world().Research
	shown := researchShown(r, len(views))
	first := a.researchFirst(len(views), shown)
	right := float64(r.x+r.w) - 12
	if len(views) > shown {
		right -= scrollBarW // the scroll bar's lane
	}
	for k, b := range a.researchButtons() {
		p := views[first+k]
		drawButton(screen, b, b.contains(mx, my) && !b.disabled)
		x, y := int(b.x+b.w)+14, int(b.y)
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
		drawText(screen, ellipsize(fmt.Sprintf("%d / %d  %s", p.Progress, p.Cost, status), tsBody, maxW-130), float64(x+130), float64(y), tsBody)
		drawText(screen, ellipsize(p.Desc, tsSmall, maxW), float64(x), float64(y)+20, tsSmall)
		drawText(screen, ellipsize("Unlocks: "+strings.Join(p.Unlocks, ", "), tsSmall, maxW), float64(x), float64(y)+37, withColor(tsSmall, uiTextDim))
	}
}

func drawResearchBar(screen *ebiten.Image, x, y, w, h float32, p game.ResearchView) {
	uiFillRect(screen, x, y, w, h, uiField, false)
	c := uiSliderFill
	if p.Done {
		c = uiGood
	}
	if p.Cost > 0 {
		uiFillRect(screen, x, y, w*float32(min(p.Progress, p.Cost))/float32(p.Cost), h, c, false)
	}
	uiStrokeRect(screen, x, y, w, h, 1, uiPanelBorder, false)
}

// hasResearchBench reports a research bench stands: the plain one or the
// electric one (which works the same research, powered).
func (a *App) hasResearchBench() bool {
	for _, b := range a.renderer.world().Buildings {
		if recipeBenchName(b.Kind) == "research bench" {
			return true
		}
	}
	return false
}
